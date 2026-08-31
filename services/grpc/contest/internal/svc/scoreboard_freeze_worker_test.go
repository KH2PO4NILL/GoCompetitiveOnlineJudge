package svc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/model"
)

type fakeFreezeStore struct {
	mu         sync.Mutex
	candidates []model.ScoreboardFreezeCandidate
	errors     map[int64]error
	calls      []int64
	listedAt   time.Time
	limit      int
}

func (f *fakeFreezeStore) ListDueScoreboardFreezes(_ context.Context, at time.Time, limit int) ([]model.ScoreboardFreezeCandidate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listedAt = at
	f.limit = limit
	return append([]model.ScoreboardFreezeCandidate(nil), f.candidates...), nil
}

func (f *fakeFreezeStore) FreezeScoreboard(_ context.Context, contestID, _ int64) (*model.ContestAggregate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, contestID)
	return &model.ContestAggregate{ContestID: contestID}, f.errors[contestID]
}

func TestScoreboardFreezeWorkerScanContinuesAfterCandidateErrors(t *testing.T) {
	now := time.Date(2026, 8, 30, 4, 0, 0, 0, time.UTC)
	store := &fakeFreezeStore{
		candidates: []model.ScoreboardFreezeCandidate{{ContestID: 1, Revision: 1}, {ContestID: 2, Revision: 2}, {ContestID: 3, Revision: 3}},
		errors:     map[int64]error{1: model.ErrScoreboardNotReady, 2: errors.New("database error")},
	}
	worker, err := NewScoreboardFreezeWorker(store, config.ScoreboardFreezeConf{BatchSize: 7}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewScoreboardFreezeWorker() error = %v", err)
	}
	if err := worker.Scan(context.Background()); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if !store.listedAt.Equal(now) || store.limit != 7 {
		t.Fatalf("list input = (%v, %d)", store.listedAt, store.limit)
	}
	if len(store.calls) != 3 || store.calls[2] != 3 {
		t.Fatalf("freeze calls = %v", store.calls)
	}
}

func TestScoreboardFreezeWorkerLifecycleAndConfig(t *testing.T) {
	store := &fakeFreezeStore{}
	worker, err := NewScoreboardFreezeWorker(store, config.ScoreboardFreezeConf{IntervalSeconds: 3600}, nil)
	if err != nil {
		t.Fatalf("NewScoreboardFreezeWorker() error = %v", err)
	}
	worker.Start()
	worker.Start()
	if err := worker.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := worker.Stop(); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
	if _, err := NewScoreboardFreezeWorker(store, config.ScoreboardFreezeConf{BatchSize: 1001}, nil); err == nil {
		t.Fatal("oversized freeze batch was accepted")
	}
	if _, err := NewScoreboardFreezeWorker(store, config.ScoreboardFreezeConf{IntervalSeconds: 86401}, nil); err == nil {
		t.Fatal("oversized freeze interval was accepted")
	}
}
