package outbox

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgeevent"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgemetrics"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgemq"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
	"google.golang.org/protobuf/encoding/protojson"
)

type Repository interface {
	ClaimOutboxEvents(context.Context, model.ClaimOutboxInput) ([]model.OutboxEvent, error)
	MarkOutboxPublished(context.Context, model.MarkOutboxPublishedInput) error
	MarkOutboxFailed(context.Context, model.MarkOutboxFailedInput) error
}
type Options struct {
	ID                                                    string
	BatchSize, Concurrency                                int
	MaxAttempts                                           int32
	Interval, ClaimDuration, Timeout, Backoff, MaxBackoff time.Duration
}

func (options Options) Validate() error {
	if options.ID == "" || len(options.ID) > 128 || options.BatchSize < 1 || options.BatchSize > 100 || options.Concurrency < 1 || options.Concurrency > 32 || options.MaxAttempts < 1 || options.MaxAttempts > 100 || options.Interval < time.Millisecond || options.Interval > time.Minute || options.Timeout <= 0 || options.Timeout > time.Minute || options.Backoff <= 0 || options.MaxBackoff < options.Backoff || options.MaxBackoff > time.Hour || options.ClaimDuration > time.Hour || options.ClaimDuration <= time.Duration(2*((options.BatchSize+options.Concurrency-1)/options.Concurrency)+1)*options.Timeout {
		return errors.New("invalid bounded outbox configuration or claim shorter than batch budget")
	}
	return nil
}

type Publisher struct {
	repository Repository
	sender     judgemq.Sender
	options    Options
	logger     *slog.Logger
	Metrics    *judgemetrics.Metrics
	once       sync.Once
	stopClaim  context.CancelFunc
	stopWork   context.CancelFunc
	done       chan struct{}
}

func New(repository Repository, sender judgemq.Sender, options Options, logger *slog.Logger) (*Publisher, error) {
	if repository == nil || sender == nil || logger == nil {
		return nil, errors.New("outbox dependencies required")
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	metrics := judgemetrics.New()
	for _, name := range []string{"published_total", "failed_total", "dead_total", "backlog", "claim_failures_total", "mark_failures_total", "inflight", "publish_latency_seconds_sum", "publish_latency_seconds_count"} {
		metrics.Set("judge_outbox_"+name, 0)
	}
	return &Publisher{repository: repository, sender: sender, options: options, logger: logger, Metrics: metrics, done: make(chan struct{})}, nil
}
func (publisher *Publisher) Start() {
	publisher.once.Do(func() {
		claimCtx, stopClaim := context.WithCancel(context.Background())
		workCtx, stopWork := context.WithCancel(context.Background())
		publisher.stopClaim, publisher.stopWork = stopClaim, stopWork
		go func() {
			defer close(publisher.done)
			for {
				if claimCtx.Err() != nil {
					return
				}
				_ = publisher.tick(claimCtx, workCtx)
				timer := time.NewTimer(publisher.options.Interval)
				select {
				case <-claimCtx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
		}()
	})
}
func (publisher *Publisher) Stop(ctx context.Context) error {
	if publisher.stopClaim == nil {
		return nil
	}
	publisher.stopClaim()
	select {
	case <-publisher.done:
		publisher.stopWork()
		return nil
	case <-ctx.Done():
		publisher.stopWork()
		<-publisher.done
		return ctx.Err()
	}
}
func (publisher *Publisher) Tick(ctx context.Context) error { return publisher.tick(ctx, ctx) }
func (publisher *Publisher) tick(claimParent, workCtx context.Context) error {
	claimCtx, cancel := context.WithTimeout(claimParent, publisher.options.Timeout)
	events, err := publisher.repository.ClaimOutboxEvents(claimCtx, model.ClaimOutboxInput{WorkerID: publisher.options.ID, EventType: model.OutboxEventJudgeTask, Limit: publisher.options.BatchSize, LockDuration: publisher.options.ClaimDuration})
	cancel()
	if err != nil {
		publisher.Metrics.Add("judge_outbox_claim_failures_total", 1)
		return err
	}
	if len(events) > publisher.options.BatchSize {
		return errors.New("repository exceeded outbox batch bound")
	}
	if counter, ok := publisher.repository.(interface {
		JudgeOutboxBacklog(context.Context) (int64, error)
	}); ok {
		ctx, cancel := context.WithTimeout(claimParent, publisher.options.Timeout)
		if count, err := counter.JudgeOutboxBacklog(ctx); err == nil {
			publisher.Metrics.Set("judge_outbox_backlog", float64(count))
		}
		cancel()
	}
	jobs := make(chan model.OutboxEvent, len(events))
	for _, event := range events {
		jobs <- event
	}
	close(jobs)
	var workers sync.WaitGroup
	for range min(publisher.options.Concurrency, len(events)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for event := range jobs {
				publisher.publish(workCtx, event)
			}
		}()
	}
	workers.Wait()
	return nil
}
func (publisher *Publisher) publish(parent context.Context, row model.OutboxEvent) {
	ctx, cancel := context.WithTimeout(parent, publisher.options.Timeout)
	defer cancel()
	publisher.Metrics.Add("judge_outbox_inflight", 1)
	defer publisher.Metrics.Add("judge_outbox_inflight", -1)
	event := &pb.JudgeTaskEvent{}
	var err error
	if len(row.Payload) > judgeevent.MaxBytes {
		err = errors.New("outbox payload exceeds byte limit")
	} else {
		err = protojson.Unmarshal(row.Payload, event)
	}
	if err == nil {
		event.SchemaVersion = judgeevent.SchemaVersion
		err = judgeevent.Validate(event)
	}
	if err == nil && (event.EventId != row.EventID || event.SubmissionId != row.AggregateID || event.AttemptId != row.AttemptID || row.EventKey != strconv.FormatInt(row.AggregateID, 10) || row.EventType != model.OutboxEventJudgeTask) {
		err = errors.New("outbox routing mismatch")
	}
	var payload []byte
	if err == nil {
		payload, err = judgeevent.Encode(event)
	}
	maxAttempts, reason := publisher.options.MaxAttempts, "broker_unavailable"
	if err != nil {
		maxAttempts, reason = 1, "invalid_event"
	} else {
		err = publisher.sender.Publish(ctx, judgeevent.Topic, row.EventKey, payload)
	}
	markCtx, markCancel := context.WithTimeout(parent, publisher.options.Timeout)
	defer markCancel()
	if err == nil {
		if err := publisher.repository.MarkOutboxPublished(markCtx, model.MarkOutboxPublishedInput{EventID: row.EventID, WorkerID: publisher.options.ID}); err != nil {
			publisher.Metrics.Add("judge_outbox_mark_failures_total", 1)
			return
		}
		publisher.Metrics.Add("judge_outbox_published_total", 1)
		publisher.Metrics.Observe("judge_outbox_publish_latency_seconds", max(time.Duration(0), time.Since(row.CreatedAt)))
		return
	}
	delay := min(publisher.options.Backoff*time.Duration(1<<min(max(row.PublishAttempts-1, 0), 16)), publisher.options.MaxBackoff)
	if err := publisher.repository.MarkOutboxFailed(markCtx, model.MarkOutboxFailedInput{EventID: row.EventID, WorkerID: publisher.options.ID, ErrorCode: reason, RetryAt: time.Now().Add(delay), MaxAttempts: maxAttempts}); err != nil {
		publisher.Metrics.Add("judge_outbox_mark_failures_total", 1)
		return
	}
	publisher.Metrics.Add("judge_outbox_failed_total", 1)
	if row.PublishAttempts >= maxAttempts {
		publisher.Metrics.Add("judge_outbox_dead_total", 1)
	}
	publisher.logger.WarnContext(parent, "judge outbox publish failed", "event_id", row.EventID, "submission_id", row.AggregateID, "attempt_id", row.AttemptID, "reason", reason)
}
