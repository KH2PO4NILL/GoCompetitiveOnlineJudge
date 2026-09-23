package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/logging"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/sandbox"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/testdatacache"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/worker"
)

type ServiceContext struct {
	Config        config.Config
	Logger        *slog.Logger
	Pool          *worker.Pool
	TestDataCache *testdatacache.Cache
	Sandbox       sandbox.Sandbox

	lifecycleMu sync.Mutex
	started     bool
	closed      bool
	shutdownErr error
}

type sandboxFactory func(config.Config, *slog.Logger) (sandbox.Sandbox, error)

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	return newServiceContext(c, newDockerSandbox)
}

func newServiceContext(c config.Config, createSandbox sandboxFactory) (*ServiceContext, error) {
	c.ApplyDefaults()
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate judge worker runtime config: %w", err)
	}
	if createSandbox == nil {
		return nil, errors.New("sandbox factory is required")
	}
	logger, err := logging.New(c.Log, os.Stdout)
	if err != nil {
		return nil, fmt.Errorf("configure logger: %w", err)
	}
	for name, dir := range map[string]string{
		"test data cache": c.TestDataCache.Dir,
		"sandbox work":    c.Sandbox.WorkDir,
	} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create %s directory %q: %w", name, dir, err)
		}
	}
	if err := validateDisjointRoots(c.TestDataCache.Dir, c.Sandbox.WorkDir); err != nil {
		return nil, err
	}
	objectStore, err := testdatacache.NewS3Store(c.ObjectStorage)
	if err != nil {
		return nil, fmt.Errorf("configure read-only test data object store: %w", err)
	}
	dataCache, err := testdatacache.New(c.TestDataCache, objectStore)
	if err != nil {
		return nil, fmt.Errorf("configure test data cache: %w", err)
	}
	if err := dataCache.Cleanup(context.Background()); err != nil {
		return nil, fmt.Errorf("clean test data cache: %w", err)
	}
	executor, err := createSandbox(c, logger)
	if err != nil {
		return nil, fmt.Errorf("configure sandbox: %w", err)
	}
	pool, err := worker.NewPool(
		c.Worker.Concurrency,
		c.Worker.QueueCapacity,
		c.TaskTimeout(),
		func(err error) { logger.Error("worker task failed", "error", err) },
	)
	if err != nil {
		_ = executor.Close()
		return nil, fmt.Errorf("configure worker pool: %w", err)
	}
	return &ServiceContext{
		Config: c, Logger: logger, Pool: pool, TestDataCache: dataCache, Sandbox: executor,
	}, nil
}

func newDockerSandbox(c config.Config, logger *slog.Logger) (sandbox.Sandbox, error) {
	return sandbox.NewDocker(sandbox.DockerOptions{
		Endpoint:                c.Sandbox.Docker.Endpoint,
		AllowUnrestrictedDaemon: c.Sandbox.Docker.AllowUnrestrictedDaemon,
		WorkerID:                c.Worker.ID,
		User:                    c.Sandbox.Docker.User,
		AllowedRoots: []sandbox.AllowedRoot{
			{Path: c.TestDataCache.Dir, Writable: false},
			{Path: c.Sandbox.WorkDir, Writable: true},
		},
		APITimeout:  c.DockerAPITimeout(),
		StopTimeout: c.DockerStopTimeout(),
		NanoCPUs:    c.Sandbox.Docker.NanoCPUs,
		Logger:      logger,
	})
}

func validateDisjointRoots(first, second string) error {
	resolved := make([]string, 2)
	for index, value := range []string{first, second} {
		absolute, err := filepath.Abs(value)
		if err != nil {
			return fmt.Errorf("resolve sandbox host directory %q: %w", value, err)
		}
		resolved[index], err = filepath.EvalSymlinks(absolute)
		if err != nil {
			return fmt.Errorf("resolve sandbox host directory %q: %w", value, err)
		}
	}
	for _, pair := range [][2]string{{resolved[0], resolved[1]}, {resolved[1], resolved[0]}} {
		relative, err := filepath.Rel(pair[0], pair[1])
		if err != nil {
			return fmt.Errorf("compare sandbox host directories: %w", err)
		}
		if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
			return errors.New("TestDataCache.Dir and Sandbox.WorkDir must be disjoint directories")
		}
	}
	return nil
}

func (s *ServiceContext) Start() error {
	if s == nil || s.Pool == nil || s.TestDataCache == nil || s.Sandbox == nil {
		return errors.New("judge worker service context is unavailable")
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.closed {
		return errors.New("judge worker service context is already closed")
	}
	if s.started {
		return nil
	}

	cleanupCtx, cancel := context.WithTimeout(context.Background(), s.Config.ShutdownTimeout())
	cleanup, err := s.Sandbox.Cleanup(cleanupCtx)
	cancel()
	if err != nil {
		s.closed = true
		closeErr := s.Sandbox.Close()
		return errors.Join(fmt.Errorf("clean orphaned sandbox containers: %w", err), closeSandboxError(closeErr))
	}
	s.Logger.Info("sandbox startup cleanup completed",
		"found", cleanup.Found, "removed", cleanup.Removed, "failed", cleanup.Failed,
	)
	s.TestDataCache.Start()
	if err := s.Pool.Start(); err != nil {
		s.closed = true
		s.TestDataCache.Close()
		return errors.Join(err, closeSandboxError(s.Sandbox.Close()))
	}
	s.started = true
	return nil
}

func (s *ServiceContext) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.closed {
		return s.shutdownErr
	}
	s.closed = true

	var shutdownErrors []error
	if s.Pool != nil {
		if err := s.Pool.Shutdown(ctx); err != nil {
			shutdownErrors = append(shutdownErrors, err)
		}
	}
	if s.TestDataCache != nil {
		s.TestDataCache.Close()
	}
	if s.Sandbox != nil {
		if err := s.Sandbox.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("close sandbox: %w", err))
		}
	}
	s.shutdownErr = errors.Join(shutdownErrors...)
	return s.shutdownErr
}

func closeSandboxError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("close sandbox: %w", err)
}
