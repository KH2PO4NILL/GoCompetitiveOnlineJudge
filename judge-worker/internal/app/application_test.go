package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeLifecycle struct {
	mu             sync.Mutex
	started        bool
	shutdownCalled bool
	startErr       error
	shutdown       func(context.Context) error
}

func (f *fakeLifecycle) Start() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = true
	return f.startErr
}

func (f *fakeLifecycle) Shutdown(ctx context.Context) error {
	f.mu.Lock()
	f.shutdownCalled = true
	f.mu.Unlock()
	if f.shutdown != nil {
		return f.shutdown(ctx)
	}
	return nil
}

func TestApplicationStartsWaitsAndShutsDown(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	lifecycle := &fakeLifecycle{}
	application, err := New("judge-1", 4, 128, time.Second, lifecycle, logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- application.Run(ctx) }()

	deadline := time.Now().Add(time.Second)
	for {
		lifecycle.mu.Lock()
		started := lifecycle.started
		lifecycle.mu.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("application did not start")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-result; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	lifecycle.mu.Lock()
	shutdownCalled := lifecycle.shutdownCalled
	lifecycle.mu.Unlock()
	if !shutdownCalled {
		t.Fatal("Shutdown() was not called")
	}
	for _, message := range []string{"judge worker started", "judge worker stopping", "judge worker stopped"} {
		if !strings.Contains(output.String(), message) {
			t.Errorf("log output does not contain %q: %s", message, output.String())
		}
	}
}

func TestApplicationReportsStartFailureWithoutShutdown(t *testing.T) {
	startErr := errors.New("start failed")
	lifecycle := &fakeLifecycle{startErr: startErr}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	application, err := New("judge-1", 1, 1, time.Second, lifecycle, logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := application.Run(context.Background()); !errors.Is(err, startErr) {
		t.Fatalf("Run() error = %v, want start error", err)
	}
	lifecycle.mu.Lock()
	shutdownCalled := lifecycle.shutdownCalled
	lifecycle.mu.Unlock()
	if shutdownCalled {
		t.Fatal("Shutdown() called after failed start")
	}
}

func TestApplicationEnforcesShutdownTimeout(t *testing.T) {
	lifecycle := &fakeLifecycle{shutdown: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	application, err := New("judge-1", 1, 1, 20*time.Millisecond, lifecycle, logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	err = application.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("Run() took %s, shutdown timeout was not enforced", elapsed)
	}
}
