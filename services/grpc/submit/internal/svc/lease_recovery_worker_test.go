package svc

import (
	"context"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type lifecycleRecoveryModel struct{ active, max atomic.Int32 }

func (m *lifecycleRecoveryModel) ListExpiredLeases(ctx context.Context, _ time.Time, _ int) ([]model.ExpiredLease, error) {
	n := m.active.Add(1)
	defer m.active.Add(-1)
	for previous := m.max.Load(); n > previous; previous = m.max.Load() {
		if m.max.CompareAndSwap(previous, n) {
			break
		}
	}
	<-ctx.Done()
	return nil, ctx.Err()
}
func (*lifecycleRecoveryModel) SupersedeExpiredAttempt(context.Context, model.SupersedeAttemptInput) (*model.Attempt, error) {
	return nil, nil
}
func TestLeaseRecoveryConcurrentLifecycle(t *testing.T) {
	repo := &lifecycleRecoveryModel{}
	worker, err := NewLeaseRecoveryWorker(repo, time.Hour, 1, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					worker.Start()
					worker.Stop()
					worker.Stop()
				}
			}()
		}
		wg.Wait()
		worker.Stop()
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("concurrent lifecycle deadlocked")
	}
	if repo.active.Load() != 0 || repo.max.Load() > 1 {
		t.Fatalf("overlapping or leaked recovery loops: active=%d maximum=%d", repo.active.Load(), repo.max.Load())
	}
}
