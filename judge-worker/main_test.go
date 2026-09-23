package main

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
)

func TestExampleConfigLoads(t *testing.T) {
	setExampleEnvironment(t)
	c, err := config.Load(filepath.Join("etc", "judge-worker.yaml"))
	if err != nil {
		t.Fatalf("load example config: %v", err)
	}
	if c.Worker.ID != "judge-test" {
		t.Fatalf("Worker.ID = %q", c.Worker.ID)
	}
	if c.Worker.Concurrency > config.MaxConcurrency || c.Worker.QueueCapacity > config.MaxQueueCapacity {
		t.Fatalf("example config exceeds worker hard limits: concurrency=%d queue=%d", c.Worker.Concurrency, c.Worker.QueueCapacity)
	}
}

type testLifecycle struct {
	started  bool
	shutdown bool
}

func (l *testLifecycle) Start() error {
	l.started = true
	return nil
}

func (l *testLifecycle) Shutdown(context.Context) error {
	l.shutdown = true
	return nil
}

func TestRunStartsAndStopsWithCancelledParent(t *testing.T) {
	setExampleEnvironment(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	lifecycle := &testLifecycle{}
	started := time.Now()
	err := runWithFactory(ctx, []string{"-f", filepath.Join("etc", "judge-worker.yaml")}, func(config.Config) (serviceRuntime, error) {
		return serviceRuntime{
			lifecycle: lifecycle,
			logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		}, nil
	})
	if err != nil {
		t.Fatalf("runWithFactory() error = %v", err)
	}
	if !lifecycle.started || !lifecycle.shutdown {
		t.Fatalf("lifecycle calls = started %v shutdown %v", lifecycle.started, lifecycle.shutdown)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("runWithFactory() took %s with a cancelled parent", elapsed)
	}
}

func setExampleEnvironment(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	for key, value := range map[string]string{
		"JUDGE_WORKER_ID":                        "judge-test",
		"JUDGE_SUBMIT_RPC_TARGET":                "127.0.0.1:8004",
		"JUDGE_PROBLEM_RPC_TARGET":               "127.0.0.1:8006",
		"SUBMIT_INTERNAL_JUDGE_TOKEN":            "submit-test-token",
		"PROBLEM_INTERNAL_JUDGE_TOKEN":           "problem-test-token",
		"JUDGE_TEST_DATA_CACHE_DIR":              filepath.Join(root, "cache"),
		"JUDGE_OBJECT_STORAGE_ENDPOINT":          "http://127.0.0.1:9000",
		"JUDGE_OBJECT_STORAGE_REGION":            "us-east-1",
		"JUDGE_OBJECT_STORAGE_BUCKET":            "problem-test-data",
		"JUDGE_OBJECT_STORAGE_ACCESS_KEY":        "read-only-access",
		"JUDGE_OBJECT_STORAGE_SECRET_KEY":        "read-only-secret",
		"JUDGE_SANDBOX_WORK_DIR":                 filepath.Join(root, "work"),
		"JUDGE_DOCKER_ENDPOINT":                  config.DefaultDockerEndpoint,
		"JUDGE_DOCKER_ALLOW_UNRESTRICTED_DAEMON": "false",
	} {
		t.Setenv(key, value)
	}
}
