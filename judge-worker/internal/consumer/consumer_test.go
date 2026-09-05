package consumer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/processor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/worker"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgeevent"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgemq"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type processFunc func(context.Context, *pb.JudgeTaskEvent) processor.Result

func (function processFunc) Process(ctx context.Context, event *pb.JudgeTaskEvent) processor.Result {
	return function(ctx, event)
}

type fakeBroker struct {
	messages                                 chan *judgemq.Delivery
	mu                                       sync.Mutex
	acks, publishes                          int
	visibility                               []time.Duration
	ackFailure, publishFailure, renewFailure bool
	deadBody                                 []byte
}

func (broker *fakeBroker) Receive(ctx context.Context, maximum int32, _ time.Duration) ([]*judgemq.Delivery, error) {
	if maximum != 1 {
		return nil, errors.New("unbounded prefetch")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case message := <-broker.messages:
		return []*judgemq.Delivery{message}, nil
	}
}
func (broker *fakeBroker) Ack(_ context.Context, _ *judgemq.Delivery) error {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if broker.ackFailure {
		return errors.New("ack response lost")
	}
	broker.acks++
	return nil
}
func (broker *fakeBroker) Visibility(_ context.Context, _ *judgemq.Delivery, duration time.Duration) error {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	broker.visibility = append(broker.visibility, duration)
	if broker.renewFailure {
		return errors.New("renew unavailable")
	}
	return nil
}
func (broker *fakeBroker) Publish(_ context.Context, topic, _ string, body []byte) error {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if topic != judgeevent.DeadTopic {
		return errors.New("wrong topic")
	}
	if broker.publishFailure {
		return errors.New("broker unavailable")
	}
	broker.publishes++
	broker.deadBody = append([]byte(nil), body...)
	return nil
}
func delivery(t *testing.T) *judgemq.Delivery {
	t.Helper()
	body, err := judgeevent.Encode(&pb.JudgeTaskEvent{SchemaVersion: 1, EventId: uuid.NewString(), SubmissionId: 1, AttemptId: 2, EnqueuedAt: timestamppb.Now()})
	if err != nil {
		t.Fatal(err)
	}
	return &judgemq.Delivery{ID: "message-1", Keys: []string{"1"}, Body: body, Attempt: 1}
}
func fixture(t *testing.T, handler processFunc) (*Driver, *fakeBroker) {
	t.Helper()
	broker := &fakeBroker{messages: make(chan *judgemq.Delivery, 100)}
	pool, err := worker.NewPool(2, 2, time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Start(); err != nil {
		t.Fatal(err)
	}
	driver, err := New(broker, broker, handler, pool, Options{WorkerID: "test-worker", Concurrency: 2, MaxAttempts: 3, Visibility: time.Second, Timeout: 100 * time.Millisecond, Backoff: time.Millisecond, MaxBackoff: time.Second}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = driver.Stop(ctx)
	})
	return driver, broker
}

func TestResultMappingAndSafeDeadLetter(t *testing.T) {
	for _, action := range []processor.Action{processor.ACK, processor.Retry, processor.DeadLetter} {
		t.Run(string(action), func(t *testing.T) {
			driver, broker := fixture(t, func(context.Context, *pb.JudgeTaskEvent) processor.Result {
				return processor.Result{Action: action, Reason: "known_failure"}
			})
			driver.handle(context.Background(), delivery(t))
			if action == processor.Retry && (broker.acks != 0 || len(broker.visibility) != 1) {
				t.Fatal("retry acknowledged")
			}
			if action != processor.Retry && broker.acks != 1 {
				t.Fatal("terminal result not acknowledged")
			}
			if action == processor.DeadLetter && (broker.publishes != 1 || !bytes.Contains(broker.deadBody, []byte("original_message_id"))) {
				t.Fatal("missing DLQ audit")
			}
		})
	}
	driver, broker := fixture(t, func(context.Context, *pb.JudgeTaskEvent) processor.Result {
		t.Fatal("invalid event processed")
		return processor.Result{}
	})
	message := delivery(t)
	message.Body = []byte("secret-source-token-object-key")
	broker.publishFailure = true
	driver.handle(context.Background(), message)
	if broker.acks != 0 {
		t.Fatal("ack before DLQ broker confirmation")
	}
	broker.publishFailure = false
	driver.handle(context.Background(), message)
	if broker.acks != 1 || bytes.Contains(broker.deadBody, message.Body) {
		t.Fatal("DLQ leaks raw body")
	}
}

func TestBoundedReceiveAndGracefulDrain(t *testing.T) {
	started, release := make(chan struct{}, 10), make(chan struct{})
	var active, maximum atomic.Int32
	driver, broker := fixture(t, func(ctx context.Context, _ *pb.JudgeTaskEvent) processor.Result {
		count := active.Add(1)
		defer active.Add(-1)
		for old := maximum.Load(); count > old && !maximum.CompareAndSwap(old, count); old = maximum.Load() {
		}
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			t.Error("graceful drain cancelled active task")
		}
		return processor.Result{Action: processor.ACK}
	})
	for range 10 {
		broker.messages <- delivery(t)
	}
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("workers did not start")
		}
	}
	if len(broker.messages) != 8 || maximum.Load() != 2 {
		t.Fatal("prefetch/concurrency exceeded hard bound")
	}
	stopped := make(chan error, 1)
	go func() { stopped <- driver.Stop(context.Background()) }()
	deadline := time.After(time.Second)
	for {
		driver.mu.Lock()
		stopping := driver.stopping
		driver.mu.Unlock()
		if stopping {
			break
		}
		select {
		case <-deadline:
			t.Fatal("did not stop receive")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	close(release)
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
	if broker.acks != 2 || len(broker.messages) != 8 {
		t.Fatal("shutdown acquired new work or lost active ACK")
	}
}

func TestForcedShutdownJoinsTasks(t *testing.T) {
	started, exited := make(chan struct{}), make(chan struct{})
	driver, broker := fixture(t, func(ctx context.Context, _ *pb.JudgeTaskEvent) processor.Result {
		close(started)
		<-ctx.Done()
		close(exited)
		return processor.Result{Action: processor.Retry}
	})
	broker.messages <- delivery(t)
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := driver.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	select {
	case <-exited:
	default:
		t.Fatal("task goroutine outlived shutdown")
	}
	if broker.acks != 0 {
		t.Fatal("forced cancellation ACKed unfinished work")
	}
}

func TestVisibilityFailureCancelsProcessor(t *testing.T) {
	driver, broker := fixture(t, func(ctx context.Context, _ *pb.JudgeTaskEvent) processor.Result {
		<-ctx.Done()
		return processor.Result{Action: processor.ACK}
	})
	broker.renewFailure = true
	driver.handle(context.Background(), delivery(t))
	if broker.acks != 0 || driver.Metrics.Get("judge_consumer_retry_total") != 1 {
		t.Fatal("lost visibility accepted result")
	}
}

func TestAckLossAndDuplicateDoNotReapply(t *testing.T) {
	var applied atomic.Int32
	driver, broker := fixture(t, func(context.Context, *pb.JudgeTaskEvent) processor.Result {
		applied.CompareAndSwap(0, 1)
		return processor.Result{Action: processor.ACK, Reason: "duplicate"}
	})
	message := delivery(t)
	broker.ackFailure = true
	driver.handle(context.Background(), message)
	broker.ackFailure = false
	driver.handle(context.Background(), message)
	if applied.Load() != 1 || broker.acks != 1 || driver.Metrics.Get("judge_consumer_ack_failures_total") != 1 {
		t.Fatal("lost ACK handling failed")
	}
}

func TestRetryExhaustionAndLogRedaction(t *testing.T) {
	driver, broker := fixture(t, func(context.Context, *pb.JudgeTaskEvent) processor.Result { panic("secret-source-token-object-key") })
	var logs bytes.Buffer
	driver.logger = slog.New(slog.NewJSONHandler(&logs, nil))
	message := delivery(t)
	message.Attempt = 3
	driver.handle(context.Background(), message)
	if broker.publishes != 1 || broker.acks != 1 {
		t.Fatal("exhausted retries not dead-lettered")
	}
	if strings.Contains(logs.String(), "secret-source") {
		t.Fatal("panic leaked in logs")
	}
	for _, field := range []string{"submission_id", "attempt_id", "worker_id", "stage"} {
		if !strings.Contains(logs.String(), field) {
			t.Fatal("missing log context", field)
		}
	}
}
