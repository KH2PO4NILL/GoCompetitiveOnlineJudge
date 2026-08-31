package svc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/model"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	defaultFreezeInterval = 10 * time.Second
	defaultFreezeBatch    = 100
	defaultFreezeTimeout  = 5 * time.Second
	maxFreezeBatch        = 1000
	maxFreezeInterval     = 24 * time.Hour
	maxFreezeTimeout      = 5 * time.Minute
)

type ScoreboardFreezeStore interface {
	ListDueScoreboardFreezes(context.Context, time.Time, int) ([]model.ScoreboardFreezeCandidate, error)
	FreezeScoreboard(context.Context, int64, int64) (*model.ContestAggregate, error)
}

// ScoreboardFreezeWorker turns an already generated live scoreboard into an
// immutable public snapshot when an ICPC contest reaches its configured freeze
// time. It deliberately does not calculate or mutate live ranking data.
type ScoreboardFreezeWorker struct {
	store            ScoreboardFreezeStore
	interval         time.Duration
	batchSize        int
	operationTimeout time.Duration
	now              func() time.Time

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewScoreboardFreezeWorker(
	store ScoreboardFreezeStore,
	conf config.ScoreboardFreezeConf,
	now func() time.Time,
) (*ScoreboardFreezeWorker, error) {
	if store == nil {
		return nil, errors.New("scoreboard freeze store is required")
	}
	if conf.IntervalSeconds > int64(maxFreezeInterval/time.Second) {
		return nil, fmt.Errorf("scoreboard freeze interval must not exceed %s", maxFreezeInterval)
	}
	interval := time.Duration(conf.IntervalSeconds) * time.Second
	if conf.IntervalSeconds <= 0 {
		interval = defaultFreezeInterval
	}
	if interval <= 0 || interval > maxFreezeInterval {
		return nil, fmt.Errorf("scoreboard freeze interval must not exceed %s", maxFreezeInterval)
	}
	batchSize := conf.BatchSize
	if batchSize <= 0 {
		batchSize = defaultFreezeBatch
	}
	if batchSize > maxFreezeBatch {
		return nil, fmt.Errorf("scoreboard freeze batch size must not exceed %d", maxFreezeBatch)
	}
	if conf.OperationTimeoutSeconds > int64(maxFreezeTimeout/time.Second) {
		return nil, fmt.Errorf("scoreboard freeze operation timeout must not exceed %s", maxFreezeTimeout)
	}
	operationTimeout := time.Duration(conf.OperationTimeoutSeconds) * time.Second
	if conf.OperationTimeoutSeconds <= 0 {
		operationTimeout = defaultFreezeTimeout
	}
	if operationTimeout <= 0 || operationTimeout > maxFreezeTimeout {
		return nil, fmt.Errorf("scoreboard freeze operation timeout must not exceed %s", maxFreezeTimeout)
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &ScoreboardFreezeWorker{
		store: store, interval: interval, batchSize: batchSize,
		operationTimeout: operationTimeout, now: now,
	}, nil
}

func (w *ScoreboardFreezeWorker) Start() {
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
	w.wg.Add(1)
	go w.run(ctx)
}

func (w *ScoreboardFreezeWorker) run(ctx context.Context) {
	defer w.wg.Done()
	w.scanAndLog(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.scanAndLog(ctx)
		}
	}
}

func (w *ScoreboardFreezeWorker) scanAndLog(ctx context.Context) {
	if err := w.Scan(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logx.WithContext(ctx).Errorf("scan due contest scoreboard freezes: %v", err)
	}
}

// Scan performs one deterministic pass and is exported for startup checks and
// unit tests. A failure for one contest is logged and does not stop the batch.
func (w *ScoreboardFreezeWorker) Scan(ctx context.Context) error {
	if w == nil || w.store == nil {
		return errors.New("scoreboard freeze worker is unavailable")
	}
	candidates, err := w.store.ListDueScoreboardFreezes(ctx, w.now().UTC(), w.batchSize)
	if err != nil {
		return err
	}
	logger := logx.WithContext(ctx)
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return err
		}
		opCtx, cancel := context.WithTimeout(ctx, w.operationTimeout)
		_, freezeErr := w.store.FreezeScoreboard(opCtx, candidate.ContestID, candidate.Revision)
		cancel()
		switch {
		case freezeErr == nil,
			errors.Is(freezeErr, model.ErrScoreboardAlreadyFrozen),
			errors.Is(freezeErr, model.ErrContestRevisionConflict):
			continue
		case errors.Is(freezeErr, model.ErrScoreboardNotReady):
			logger.Errorf("contest %d scoreboard is not ready for automatic freeze", candidate.ContestID)
		default:
			logger.Errorf("automatically freeze contest %d scoreboard: %v", candidate.ContestID, freezeErr)
		}
	}
	return nil
}

func (w *ScoreboardFreezeWorker) Stop() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	cancel := w.cancel
	w.cancel = nil
	w.mu.Unlock()
	if cancel != nil {
		cancel()
		w.wg.Wait()
	}
	return nil
}
