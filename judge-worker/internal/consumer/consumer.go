package consumer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/processor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/worker"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgeevent"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgemetrics"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgemq"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
)

type Processor interface {
	Process(context.Context, *pb.JudgeTaskEvent) processor.Result
}
type Options struct {
	Concurrency                              int
	MaxAttempts                              int32
	Visibility, Timeout, Backoff, MaxBackoff time.Duration
	WorkerID                                 string
}

type Driver struct {
	receiver          judgemq.Receiver
	sender            judgemq.Sender
	processor         Processor
	pool              *worker.Pool
	options           Options
	logger            *slog.Logger
	Metrics           *judgemetrics.Metrics
	mu                sync.Mutex
	started, stopping bool
	cancel            context.CancelFunc
	done              chan struct{}
	slots             chan struct{}
}

func New(receiver judgemq.Receiver, sender judgemq.Sender, taskProcessor Processor, pool *worker.Pool, options Options, logger *slog.Logger) (*Driver, error) {
	if receiver == nil || sender == nil || taskProcessor == nil || pool == nil || logger == nil || options.WorkerID == "" || options.Concurrency < 1 || options.Concurrency > worker.MaxConcurrency || options.MaxAttempts < 1 || options.MaxAttempts > 100 || options.Timeout <= 0 || options.Timeout > time.Minute || options.Visibility < 3*options.Timeout || options.Visibility < time.Second || options.Visibility > 12*time.Hour || options.Backoff <= 0 || options.MaxBackoff < options.Backoff || options.MaxBackoff > 12*time.Hour {
		return nil, errors.New("invalid bounded consumer configuration")
	}
	metrics := judgemetrics.New()
	for _, name := range []string{"inflight", "pool_saturation", "ack_total", "ack_failures_total", "retry_total", "dlq_total", "dlq_failures_total", "receive_failures_total", "visibility_failures_total", "delivery_attempts_total", "latency_seconds_sum", "latency_seconds_count"} {
		metrics.Set("judge_consumer_"+name, 0)
	}
	return &Driver{receiver: receiver, sender: sender, processor: taskProcessor, pool: pool, options: options, logger: logger, Metrics: metrics, done: make(chan struct{}), slots: make(chan struct{}, options.Concurrency)}, nil
}

func (driver *Driver) Start() error {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.stopping {
		return errors.New("consumer already stopping")
	}
	if driver.started {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	driver.cancel, driver.started = cancel, true
	go driver.receive(ctx)
	return nil
}

func (driver *Driver) Stop(ctx context.Context) error {
	driver.mu.Lock()
	driver.stopping = true
	started := driver.started
	if driver.cancel != nil {
		driver.cancel()
	}
	driver.mu.Unlock()
	if started {
		<-driver.done
	}
	err := driver.pool.Shutdown(ctx)
	if err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*driver.options.Timeout+time.Minute)
		defer cancel()
		err = errors.Join(err, driver.pool.Wait(cleanupCtx))
	}
	driver.Metrics.Set("judge_consumer_inflight", 0)
	driver.Metrics.Set("judge_consumer_pool_saturation", 0)
	return err
}

func (driver *Driver) receive(ctx context.Context) {
	defer close(driver.done)
	for {
		select {
		case <-ctx.Done():
			return
		case driver.slots <- struct{}{}:
		}
		if ctx.Err() != nil {
			<-driver.slots
			return
		}
		messages, err := driver.receiver.Receive(ctx, 1, driver.options.Visibility)
		if err != nil || len(messages) != 1 || messages[0] == nil {
			<-driver.slots
			if err != nil {
				driver.Metrics.Add("judge_consumer_receive_failures_total", 1)
			}
			if !pause(ctx, driver.options.Backoff) {
				return
			}
			continue
		}
		message := messages[0]
		driver.Metrics.Add("judge_consumer_inflight", 1)
		driver.Metrics.Set("judge_consumer_pool_saturation", float64(len(driver.slots))/float64(driver.options.Concurrency))
		err = driver.pool.Submit(ctx, func(taskCtx context.Context) error {
			defer func() {
				<-driver.slots
				driver.Metrics.Add("judge_consumer_inflight", -1)
				driver.Metrics.Set("judge_consumer_pool_saturation", float64(len(driver.slots))/float64(driver.options.Concurrency))
			}()
			driver.mu.Lock()
			stopping := driver.stopping
			driver.mu.Unlock()
			if !stopping {
				driver.handle(taskCtx, message)
			}
			return nil
		})
		if err != nil {
			<-driver.slots
			driver.Metrics.Add("judge_consumer_inflight", -1)
			return
		}
	}
}

func (driver *Driver) handle(parent context.Context, message *judgemq.Delivery) {
	driver.Metrics.Add("judge_consumer_delivery_attempts_total", float64(max(message.Attempt, 1)))
	key := ""
	if len(message.Keys) == 1 {
		key = message.Keys[0]
	}
	event, err := judgeevent.Decode(message.Body, key)
	result := processor.Result{Action: processor.DeadLetter, Reason: "invalid_event"}
	if err == nil {
		ctx, cancel := context.WithCancel(parent)
		done := make(chan struct{})
		go func() {
			defer close(done)
			ticker := time.NewTicker(driver.options.Visibility / 3)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					renewCtx, stop := context.WithTimeout(ctx, driver.options.Timeout)
					err := driver.receiver.Visibility(renewCtx, message, driver.options.Visibility)
					stop()
					if err != nil {
						driver.Metrics.Add("judge_consumer_visibility_failures_total", 1)
						cancel()
						return
					}
				}
			}
		}()
		result = driver.process(ctx, event)
		lost := ctx.Err() != nil
		cancel()
		<-done
		if lost {
			result = processor.Result{Action: processor.Retry, Reason: "cancelled"}
		}
		driver.Metrics.Observe("judge_consumer_latency_seconds", max(time.Duration(0), time.Since(event.EnqueuedAt.AsTime())))
	}
	if parent.Err() != nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, driver.options.Timeout)
	defer cancel()
	if result.Action == processor.Retry && message.Attempt >= driver.options.MaxAttempts {
		result = processor.Result{Action: processor.DeadLetter, Reason: "retry_exhausted"}
	}
	if result.Action == processor.DeadLetter {
		metadata := map[string]any{"schema_version": 1, "original_message_id": bounded(message.ID), "original_keys": boundedKeys(message.Keys), "reason": safeReason(result.Reason), "delivery_attempt": message.Attempt}
		body, _ := json.Marshal(metadata)
		if err := driver.sender.Publish(ctx, judgeevent.DeadTopic, bounded(key), body); err != nil {
			driver.Metrics.Add("judge_consumer_dlq_failures_total", 1)
			driver.retry(ctx, message)
			return
		}
		driver.Metrics.Add("judge_consumer_dlq_total", 1)
	}
	if result.Action == processor.ACK || result.Action == processor.DeadLetter {
		if err := driver.receiver.Ack(ctx, message); err != nil {
			driver.Metrics.Add("judge_consumer_ack_failures_total", 1)
		} else {
			driver.Metrics.Add("judge_consumer_ack_total", 1)
		}
	} else {
		driver.retry(ctx, message)
	}
	args := []any{"worker_id", driver.options.WorkerID, "stage", "settle", "action", result.Action, "reason", safeReason(result.Reason), "delivery_attempt", message.Attempt}
	if event != nil {
		args = append(args, "submission_id", event.SubmissionId, "attempt_id", event.AttemptId)
	}
	driver.logger.InfoContext(parent, "judge message settled", args...)
}

func (driver *Driver) process(ctx context.Context, event *pb.JudgeTaskEvent) (result processor.Result) {
	defer func() {
		if recover() != nil {
			result = processor.Result{Action: processor.Retry, Reason: "processor_panic"}
		}
	}()
	return driver.processor.Process(ctx, event)
}
func (driver *Driver) retry(ctx context.Context, message *judgemq.Delivery) {
	driver.Metrics.Add("judge_consumer_retry_total", 1)
	delay := min(driver.options.Backoff*time.Duration(1<<min(max(message.Attempt-1, 0), 16)), driver.options.MaxBackoff)
	if err := driver.receiver.Visibility(ctx, message, delay); err != nil {
		driver.Metrics.Add("judge_consumer_visibility_failures_total", 1)
	}
}
func pause(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
func bounded(value string) string {
	if len(value) <= 256 {
		return value
	}
	digest := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(digest[:])
}
func boundedKeys(keys []string) []string {
	result := make([]string, 0, min(len(keys), 16))
	for _, key := range keys[:min(len(keys), 16)] {
		result = append(result, bounded(key))
	}
	return result
}
func safeReason(reason string) string {
	if reason == "" || len(reason) > 64 {
		return "processing_failed"
	}
	for _, character := range reason {
		if character != '_' && (character < 'a' || character > 'z') {
			return "processing_failed"
		}
	}
	return reason
}
