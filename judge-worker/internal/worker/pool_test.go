package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestNewPoolEnforcesHardLimits(t *testing.T) {
	for _, test := range []struct {
		concurrency int
		queue       int
	}{
		{concurrency: 0, queue: 1},
		{concurrency: MaxConcurrency + 1, queue: 1},
		{concurrency: 1, queue: 0},
		{concurrency: 1, queue: MaxQueueCapacity + 1},
	} {
		if _, err := NewPool(test.concurrency, test.queue, time.Second, nil); err == nil {
			t.Fatalf("NewPool(%d, %d) error = nil", test.concurrency, test.queue)
		}
	}
}

func TestSubmitRejectsAlreadyCanceledContext(t *testing.T) {
	pool := mustPool(t, 1, 1, time.Second, nil)
	if err := pool.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer shutdown(t, pool)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for range 64 {
		err := pool.Submit(ctx, func(context.Context) error { return nil })
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Submit() error = %v, want context.Canceled", err)
		}
	}
}

func TestSubmitRejectsContextCanceledWhileWaitingForStateLock(t *testing.T) {
	pool := mustPool(t, 1, 1, time.Second, nil)
	if err := pool.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer shutdown(t, pool)

	pool.mu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- pool.Submit(ctx, func(context.Context) error { return nil })
	}()

	deadline := time.Now().Add(time.Second)
	for len(pool.slots) != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(pool.slots) != 0 {
		pool.mu.Unlock()
		cancel()
		t.Fatal("Submit() did not reserve a queue slot")
	}
	cancel()
	pool.mu.Unlock()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Submit() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Submit() did not return after cancellation")
	}
}

func TestPoolLimitsConcurrency(t *testing.T) {
	pool := mustPool(t, 3, 12, time.Second, nil)
	if err := pool.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	var active atomic.Int32
	var maximum atomic.Int32
	var completed atomic.Int32
	started := make(chan struct{}, 12)
	release := make(chan struct{})
	task := func(context.Context) error {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			previous := maximum.Load()
			if current <= previous || maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		started <- struct{}{}
		<-release
		completed.Add(1)
		return nil
	}
	for range 12 {
		if err := pool.Submit(context.Background(), task); err != nil {
			t.Fatalf("Submit() error = %v", err)
		}
	}
	for range 3 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("workers did not start")
		}
	}
	if got := maximum.Load(); got != 3 {
		t.Fatalf("maximum concurrency = %d, want 3", got)
	}
	close(release)
	shutdown(t, pool)
	if got := completed.Load(); got != 12 {
		t.Fatalf("completed tasks = %d, want 12", got)
	}
}

func TestPoolQueueIsBounded(t *testing.T) {
	pool := mustPool(t, 1, 1, time.Second, nil)
	if err := pool.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	blocking := func(context.Context) error {
		close(started)
		<-release
		return nil
	}
	if err := pool.TrySubmit(blocking); err != nil {
		t.Fatalf("TrySubmit(first) error = %v", err)
	}
	<-started
	if err := pool.TrySubmit(func(context.Context) error { return nil }); err != nil {
		t.Fatalf("TrySubmit(queued) error = %v", err)
	}
	if err := pool.TrySubmit(func(context.Context) error { return nil }); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("TrySubmit(full) error = %v, want ErrQueueFull", err)
	}
	close(release)
	shutdown(t, pool)
}

func TestShutdownRejectsNewTasksAndDrainsAcceptedTasks(t *testing.T) {
	pool := mustPool(t, 1, 2, time.Second, nil)
	if err := pool.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	var completed atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	if err := pool.Submit(context.Background(), func(context.Context) error {
		close(started)
		<-release
		completed.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("Submit(first) error = %v", err)
	}
	<-started
	if err := pool.Submit(context.Background(), func(context.Context) error {
		completed.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("Submit(second) error = %v", err)
	}

	shutdownResult := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		shutdownResult <- pool.Shutdown(ctx)
	}()
	deadline := time.Now().Add(time.Second)
	for {
		err := pool.TrySubmit(func(context.Context) error { return nil })
		if errors.Is(err, ErrStopping) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pool did not stop accepting tasks, last error = %v", err)
		}
		time.Sleep(time.Millisecond)
	}
	close(release)
	if err := <-shutdownResult; err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if got := completed.Load(); got != 2 {
		t.Fatalf("completed tasks = %d, want 2", got)
	}
}

func TestShutdownDeadlineCancelsInflightTask(t *testing.T) {
	pool := mustPool(t, 1, 1, time.Minute, nil)
	if err := pool.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	started := make(chan struct{})
	cancelled := make(chan struct{})
	if err := pool.Submit(context.Background(), func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	}); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := pool.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown() error = %v, want deadline exceeded", err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("in-flight task did not receive cancellation")
	}
	shutdown(t, pool)
}

func TestShutdownDeadlineIsNotBlockedByWaitingSubmit(t *testing.T) {
	pool := mustPool(t, 1, 1, time.Minute, nil)
	if err := pool.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	started := make(chan struct{})
	if err := pool.Submit(context.Background(), func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}); err != nil {
		t.Fatalf("Submit(in-flight) error = %v", err)
	}
	<-started
	if err := pool.Submit(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatalf("Submit(queued) error = %v", err)
	}

	submitResult := make(chan error, 1)
	go func() {
		submitResult <- pool.Submit(context.Background(), func(context.Context) error { return nil })
	}()
	// Give the submitter an opportunity to block waiting for queue capacity.
	time.Sleep(10 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	startedShutdown := time.Now()
	if err := pool.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown() error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(startedShutdown); elapsed > 200*time.Millisecond {
		t.Fatalf("Shutdown() exceeded its deadline by too much: %s", elapsed)
	}
	select {
	case err := <-submitResult:
		if !errors.Is(err, ErrStopping) {
			t.Fatalf("blocked Submit() error = %v, want ErrStopping", err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked Submit() did not wake during shutdown")
	}
	shutdown(t, pool)
}

func TestPoolRecoversTaskPanic(t *testing.T) {
	errorsSeen := make(chan error, 1)
	pool := mustPool(t, 1, 1, time.Second, func(err error) { errorsSeen <- err })
	if err := pool.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := pool.Submit(context.Background(), func(context.Context) error { panic("boom") }); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	select {
	case err := <-errorsSeen:
		if err == nil || err.Error() != "worker task panicked: boom" {
			t.Fatalf("panic error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("panic was not reported")
	}
	shutdown(t, pool)
}

func mustPool(t *testing.T, concurrency, queue int, timeout time.Duration, onError ErrorHandler) *Pool {
	t.Helper()
	pool, err := NewPool(concurrency, queue, timeout, onError)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	return pool
}

func shutdown(t *testing.T, pool *Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := pool.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}
