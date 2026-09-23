package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgeevent"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
	"github.com/google/uuid"
)

type fakeRepository struct {
	mu          sync.Mutex
	now         time.Time
	rows        []model.OutboxEvent
	markFailure bool
	claims      int
	lastFailure model.MarkOutboxFailedInput
}

func (repository *fakeRepository) ClaimOutboxEvents(ctx context.Context, input model.ClaimOutboxInput) ([]model.OutboxEvent, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.claims++
	var result []model.OutboxEvent
	for index := range repository.rows {
		row := &repository.rows[index]
		if len(result) == input.Limit {
			break
		}
		if row.EventType != input.EventType || row.Status != model.OutboxStatusPending || row.AvailableAt.After(repository.now) || row.LockedUntil != nil && row.LockedUntil.After(repository.now) {
			continue
		}
		row.PublishAttempts++
		until := repository.now.Add(input.LockDuration)
		row.LockedBy, row.LockedUntil = &input.WorkerID, &until
		result = append(result, *row)
	}
	return result, nil
}
func (repository *fakeRepository) MarkOutboxPublished(ctx context.Context, input model.MarkOutboxPublishedInput) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.markFailure {
		return errors.New("mark unavailable")
	}
	for index := range repository.rows {
		row := &repository.rows[index]
		if row.EventID == input.EventID && row.LockedBy != nil && *row.LockedBy == input.WorkerID {
			row.Status = model.OutboxStatusPublished
			return nil
		}
	}
	return model.ErrOutboxStateConflict
}
func (repository *fakeRepository) MarkOutboxFailed(ctx context.Context, input model.MarkOutboxFailedInput) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.lastFailure = input
	for index := range repository.rows {
		row := &repository.rows[index]
		if row.EventID == input.EventID && row.LockedBy != nil && *row.LockedBy == input.WorkerID {
			row.LockedBy, row.LockedUntil = nil, nil
			row.AvailableAt = input.RetryAt
			if row.PublishAttempts >= input.MaxAttempts {
				row.Status = model.OutboxStatusDead
			}
			return nil
		}
	}
	return model.ErrOutboxStateConflict
}

type fakeSender struct {
	mu      sync.Mutex
	calls   int
	failure bool
	started chan struct{}
	release chan struct{}
}

func (sender *fakeSender) Publish(ctx context.Context, topic, key string, content []byte) error {
	sender.mu.Lock()
	sender.calls++
	sender.mu.Unlock()
	if topic != judgeevent.Topic {
		return errors.New("wrong topic")
	}
	if _, err := judgeevent.Decode(content, key); err != nil {
		return err
	}
	if sender.started != nil {
		sender.started <- struct{}{}
		select {
		case <-sender.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if sender.failure {
		return errors.New("secret must not be logged")
	}
	return nil
}
func fixture(t *testing.T) (*Publisher, *fakeRepository, *fakeSender) {
	t.Helper()
	now := time.Now()
	id := uuid.NewString()
	payload, err := json.Marshal(map[string]any{"event_id": id, "submission_id": 1, "attempt_id": 2, "enqueued_at": now})
	if err != nil {
		t.Fatal(err)
	}
	repository := &fakeRepository{now: now, rows: []model.OutboxEvent{{EventID: id, EventType: model.OutboxEventJudgeTask, AggregateID: 1, AttemptID: 2, EventKey: "1", Payload: payload, Status: model.OutboxStatusPending, CreatedAt: now}}}
	sender := &fakeSender{}
	publisher, err := New(repository, sender, Options{ID: "instance-a", BatchSize: 2, Concurrency: 2, MaxAttempts: 3, Interval: time.Second, ClaimDuration: time.Second, Timeout: 100 * time.Millisecond, Backoff: time.Millisecond, MaxBackoff: time.Second}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return publisher, repository, sender
}

func TestBrokerAckPrecedesMarkAndLostMarkCanRecover(t *testing.T) {
	publisher, repository, sender := fixture(t)
	repository.markFailure = true
	if err := publisher.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repository.rows[0].Status != model.OutboxStatusPending || sender.calls != 1 {
		t.Fatal("marked before durable acknowledgement")
	}
	repository.markFailure = false
	repository.now = repository.now.Add(2 * time.Second)
	publisher.options.ID = "instance-b"
	if err := publisher.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repository.rows[0].Status != model.OutboxStatusPublished || sender.calls != 2 {
		t.Fatal("expired claim did not recover")
	}
}

func TestBackoffAndDeadRetainSafeReason(t *testing.T) {
	publisher, repository, sender := fixture(t)
	sender.failure = true
	for attempt := range 3 {
		if err := publisher.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		if repository.lastFailure.ErrorCode != "broker_unavailable" || repository.lastFailure.MaxAttempts != 3 {
			t.Fatal(repository.lastFailure)
		}
		if attempt < 2 && repository.rows[0].Status != model.OutboxStatusPending {
			t.Fatal("premature dead status")
		}
		repository.now = repository.lastFailure.RetryAt.Add(time.Millisecond)
	}
	if repository.rows[0].Status != model.OutboxStatusDead || publisher.Metrics.Get("judge_outbox_dead_total") != 1 {
		t.Fatal("retry exhaustion not dead")
	}
}

func TestInvalidPayloadNeverPublishes(t *testing.T) {
	publisher, repository, sender := fixture(t)
	repository.rows[0].Payload = []byte(`{"source_code":"secret"}`)
	if err := publisher.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 0 || repository.rows[0].Status != model.OutboxStatusDead {
		t.Fatal("invalid event was published")
	}
}

func TestShutdownDrainsInflightWithoutNewClaims(t *testing.T) {
	publisher, repository, sender := fixture(t)
	sender.started, sender.release = make(chan struct{}, 1), make(chan struct{})
	publisher.Start()
	<-sender.started
	stopped := make(chan error, 1)
	go func() { stopped <- publisher.Stop(context.Background()) }()
	close(sender.release)
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
	if repository.claims != 1 || repository.rows[0].Status != model.OutboxStatusPublished {
		t.Fatal("shutdown did not drain")
	}
}
