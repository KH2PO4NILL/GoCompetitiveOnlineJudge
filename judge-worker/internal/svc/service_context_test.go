package svc

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/sandbox"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/workspace"
)

type fakeSandbox struct {
	cleanupResult sandbox.CleanupResult
	cleanupErr    error
	closeErr      error
	cleanupCalled bool
	cleanupCalls  int
	closed        bool
	closeCalls    int
}

func TestWorkspaceOwnershipIsHeldUntilCancelledTasksExit(t *testing.T) {
	settings := testConfig(t.TempDir())
	service, err := newServiceContext(settings, func(config.Config, *slog.Logger) (sandbox.Sandbox, error) { return &fakeSandbox{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Shutdown(context.Background()) })
	if err := service.Start(); err != nil {
		t.Fatal(err)
	}
	if duplicate, err := newServiceContext(settings, func(config.Config, *slog.Logger) (sandbox.Sandbox, error) {
		t.Error("duplicate worker reached the sandbox factory")
		return &fakeSandbox{}, nil
	}); err == nil {
		_ = duplicate.Shutdown(context.Background())
		t.Fatal("duplicate worker took over an active workspace")
	}
	entered, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	if err := service.Pool.Submit(context.Background(), func(ctx context.Context) error {
		close(entered)
		<-ctx.Done()
		close(cancelled)
		<-release
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	finished := make(chan error, 1)
	go func() { finished <- service.Shutdown(ctx) }()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("task was not cancelled")
	}
	if stolen, err := workspace.Open(settings.Sandbox.WorkDir, settings.Worker.ID); err == nil {
		_ = stolen.Close()
		t.Fatal("workspace released while cancelled task was still active")
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not join the task")
	}
	restarted, err := workspace.Open(settings.Sandbox.WorkDir, settings.Worker.ID)
	if err != nil {
		t.Fatal("workspace lock was not released", err)
	}
	_ = restarted.Close()
}

func (f *fakeSandbox) ResolveImage(context.Context, string) (string, error) {
	return "sha256:" + strings.Repeat("a", 64), nil
}

func (f *fakeSandbox) Execute(context.Context, sandbox.ExecuteRequest) (sandbox.ExecuteResult, error) {
	return sandbox.ExecuteResult{}, errors.New("not implemented")
}

func (f *fakeSandbox) Cleanup(context.Context) (sandbox.CleanupResult, error) {
	f.cleanupCalled = true
	f.cleanupCalls++
	return f.cleanupResult, f.cleanupErr
}

func (f *fakeSandbox) Close() error {
	f.closed = true
	f.closeCalls++
	return f.closeErr
}

func TestServiceContextCreatesDirectoriesCleansSandboxAndRunsPool(t *testing.T) {
	root := t.TempDir()
	c := testConfig(root)
	executor := &fakeSandbox{cleanupResult: sandbox.CleanupResult{Found: 2, Removed: 2}}
	serviceContext, err := newServiceContext(c, func(config.Config, *slog.Logger) (sandbox.Sandbox, error) {
		return executor, nil
	})
	if err != nil {
		t.Fatalf("newServiceContext() error = %v", err)
	}
	if serviceContext.Languages == nil || serviceContext.Engine == nil {
		t.Fatal("language registry and engine were not initialized")
	}
	orphan := filepath.Join(serviceContext.workspace.Dir, "attempt-1-123", "artifact")
	if err := os.MkdirAll(orphan, 0700); err != nil {
		t.Fatal(err)
	}
	if err := serviceContext.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !executor.cleanupCalled {
		t.Fatal("Start() did not clean orphaned sandbox containers")
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("startup retained owned orphan: %v", err)
	}
	completed := make(chan struct{})
	if err := serviceContext.Pool.Submit(context.Background(), func(context.Context) error {
		close(completed)
		return nil
	}); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("worker pool did not execute task")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := serviceContext.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if !executor.closed {
		t.Fatal("Shutdown() did not close sandbox")
	}
}

func TestServiceContextRejectsInvalidProfilesBeforeSandboxCreation(t *testing.T) {
	root := t.TempDir()
	c := testConfig(root)
	c.LanguageRegistryFile = filepath.Join(root, "invalid.yaml")
	if err := os.WriteFile(c.LanguageRegistryFile, []byte("profiles: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	_, err := newServiceContext(c, func(config.Config, *slog.Logger) (sandbox.Sandbox, error) {
		called = true
		return &fakeSandbox{}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "language registry") || called {
		t.Fatalf("error = %v, sandbox created = %v", err, called)
	}
}

func TestServiceContextLifecycleIsIdempotent(t *testing.T) {
	executor := &fakeSandbox{}
	serviceContext, err := newServiceContext(testConfig(t.TempDir()), func(config.Config, *slog.Logger) (sandbox.Sandbox, error) {
		return executor, nil
	})
	if err != nil {
		t.Fatalf("newServiceContext() error = %v", err)
	}
	if err := serviceContext.Start(); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	if err := serviceContext.Start(); err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	if executor.cleanupCalls != 1 {
		t.Fatalf("sandbox cleanup calls = %d, want 1", executor.cleanupCalls)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := serviceContext.Shutdown(ctx); err != nil {
		t.Fatalf("first Shutdown() error = %v", err)
	}
	if err := serviceContext.Shutdown(ctx); err != nil {
		t.Fatalf("second Shutdown() error = %v", err)
	}
	if executor.closeCalls != 1 {
		t.Fatalf("sandbox close calls = %d, want 1", executor.closeCalls)
	}
	if err := serviceContext.Start(); err == nil || !strings.Contains(err.Error(), "already closed") {
		t.Fatalf("Start() after shutdown error = %v", err)
	}
}

func TestServiceContextStartStopsWhenSandboxCleanupFails(t *testing.T) {
	executor := &fakeSandbox{cleanupErr: errors.New("daemon unavailable")}
	serviceContext, err := newServiceContext(testConfig(t.TempDir()), func(config.Config, *slog.Logger) (sandbox.Sandbox, error) {
		return executor, nil
	})
	if err != nil {
		t.Fatalf("newServiceContext() error = %v", err)
	}
	if err := serviceContext.Start(); err == nil || !strings.Contains(err.Error(), "clean orphaned sandbox containers") {
		t.Fatalf("Start() error = %v", err)
	}
	if !executor.closed {
		t.Fatal("sandbox was not closed after startup cleanup failure")
	}
}

func TestServiceContextRejectsOverlappingSandboxRoots(t *testing.T) {
	root := t.TempDir()
	c := testConfig(root)
	c.Sandbox.WorkDir = filepath.Join(c.TestDataCache.Dir, "work")
	_, err := newServiceContext(c, func(config.Config, *slog.Logger) (sandbox.Sandbox, error) {
		return &fakeSandbox{}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "must be disjoint") {
		t.Fatalf("newServiceContext() error = %v", err)
	}
}

func testConfig(root string) config.Config {
	return config.Config{
		Worker:       config.WorkerConf{ID: "judge-test", Concurrency: 1, QueueCapacity: 2},
		SubmitRPC:    config.RPCConf{Target: "127.0.0.1:8004"},
		ProblemRPC:   config.RPCConf{Target: "127.0.0.1:8006"},
		InternalAuth: config.InternalAuthConf{Name: "judge.worker", SubmitToken: "submit-secret", ProblemToken: "problem-secret"},
		TestDataCache: config.TestDataCacheConf{
			Dir: filepath.Join(root, "cache"),
		},
		ObjectStorage: config.ObjectStorageConf{
			Endpoint: "http://127.0.0.1:9000", Region: "us-east-1", Bucket: "problem-test-data",
			AccessKey: "read-only-access", SecretKey: "read-only-secret", UsePathStyle: true, RequestTimeoutSeconds: 60,
		},
		Sandbox: config.SandboxConf{
			Driver: "docker", WorkDir: filepath.Join(root, "work"),
			Docker: config.DockerConf{Endpoint: config.DefaultDockerEndpoint},
		},
		Log:      config.LogConf{Level: "error", Format: "text"},
		Metrics:  config.MetricsConf{ListenOn: config.DefaultMetricsListenOn, Path: config.DefaultMetricsPath},
		Timeouts: config.TimeoutConf{RPCSeconds: 5, TaskSeconds: 5, ShutdownSeconds: 1},
	}
}
