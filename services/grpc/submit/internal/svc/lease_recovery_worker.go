package svc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"

	"github.com/zeromicro/go-zero/core/logx"
)

type LeaseRecoveryWorker struct {
	model    LeaseRecoveryModel
	interval time.Duration
	batch    int
	timeout  time.Duration
	now      func() time.Time

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

type LeaseRecoveryModel interface {
	ListExpiredLeases(ctx context.Context, before time.Time, limit int) ([]model.ExpiredLease, error)
	SupersedeExpiredAttempt(ctx context.Context, input model.SupersedeAttemptInput) (*model.Attempt, error)
}

func NewLeaseRecoveryWorker(
	repository LeaseRecoveryModel,
	interval time.Duration,
	batch int,
	timeout time.Duration,
	now func() time.Time,
) (*LeaseRecoveryWorker, error) {
	if repository == nil {
		return nil, errors.New("lease recovery requires SubmitModel")
	}
	if interval <= 0 || batch <= 0 || batch > 1000 || timeout <= 0 {
		return nil, errors.New("invalid lease recovery configuration")
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &LeaseRecoveryWorker{
		model: repository, interval: interval, batch: batch, timeout: timeout, now: now,
	}, nil
}

func (w *LeaseRecoveryWorker) Start() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.done = make(chan struct{})
	go w.run(ctx)
}

func (w *LeaseRecoveryWorker) Stop() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	cancel, done := w.cancel, w.done
	w.cancel, w.done = nil, nil
	w.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	<-done
	return nil
}

func (w *LeaseRecoveryWorker) run(ctx context.Context) {
	defer close(w.done)
	w.runAndLog(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runAndLog(ctx)
		}
	}
}

func (w *LeaseRecoveryWorker) runAndLog(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, w.timeout)
	defer cancel()
	if err := w.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logx.Errorf("recover expired submit leases: %v", err)
	}
}

func (w *LeaseRecoveryWorker) RunOnce(ctx context.Context) error {
	if w == nil || w.model == nil {
		return errors.New("lease recovery worker is unavailable")
	}
	leases, err := w.model.ListExpiredLeases(ctx, w.now().UTC(), w.batch)
	if err != nil {
		return err
	}
	var failures []error
	for _, lease := range leases {
		_, err := w.model.SupersedeExpiredAttempt(ctx, model.SupersedeAttemptInput{
			SubmissionID: lease.SubmissionID,
			AttemptID:    lease.AttemptID,
			Reason:       "judge lease expired",
		})
		if err == nil || errors.Is(err, model.ErrStaleAttempt) ||
			errors.Is(err, model.ErrAttemptAlreadyFinished) || errors.Is(err, model.ErrLeaseHeld) {
			continue
		}
		failures = append(failures, fmt.Errorf(
			"submission %d attempt %d: %w", lease.SubmissionID, lease.AttemptID, err,
		))
	}
	return errors.Join(failures...)
}
