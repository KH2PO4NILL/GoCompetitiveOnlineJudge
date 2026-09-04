package svc

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
)

func TestServiceContextCreatesDirectoriesAndRunsPool(t *testing.T) {
	root := t.TempDir()
	c := config.Config{
		Worker:       config.WorkerConf{ID: "judge-test", Concurrency: 1, QueueCapacity: 2},
		SubmitRPC:    config.RPCConf{Target: "127.0.0.1:8004"},
		ProblemRPC:   config.RPCConf{Target: "127.0.0.1:8006"},
		InternalAuth: config.InternalAuthConf{Name: "judge.worker", SubmitToken: "submit-secret", ProblemToken: "problem-secret"},
		TestDataCache: config.TestDataCacheConf{
			Dir: filepath.Join(root, "cache"),
		},
		Sandbox: config.SandboxConf{
			Driver: "docker", WorkDir: filepath.Join(root, "work"),
			Docker: config.DockerConf{Endpoint: config.DefaultDockerEndpoint},
		},
		Log:      config.LogConf{Level: "error", Format: "text"},
		Metrics:  config.MetricsConf{ListenOn: config.DefaultMetricsListenOn, Path: config.DefaultMetricsPath},
		Timeouts: config.TimeoutConf{RPCSeconds: 5, TaskSeconds: 5, ShutdownSeconds: 1},
	}
	serviceContext, err := NewServiceContext(c)
	if err != nil {
		t.Fatalf("NewServiceContext() error = %v", err)
	}
	if err := serviceContext.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
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
}
