package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/logging"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/testdatacache"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/worker"
)

type ServiceContext struct {
	Config        config.Config
	Logger        *slog.Logger
	Pool          *worker.Pool
	TestDataCache *testdatacache.Cache
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	c.ApplyDefaults()
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate judge worker runtime config: %w", err)
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
	pool, err := worker.NewPool(
		c.Worker.Concurrency,
		c.Worker.QueueCapacity,
		c.TaskTimeout(),
		func(err error) { logger.Error("worker task failed", "error", err) },
	)
	if err != nil {
		return nil, fmt.Errorf("configure worker pool: %w", err)
	}
	return &ServiceContext{Config: c, Logger: logger, Pool: pool, TestDataCache: dataCache}, nil
}

func (s *ServiceContext) Start() error {
	if s == nil || s.Pool == nil || s.TestDataCache == nil {
		return errors.New("judge worker service context is unavailable")
	}
	s.TestDataCache.Start()
	if err := s.Pool.Start(); err != nil {
		s.TestDataCache.Close()
		return err
	}
	return nil
}

func (s *ServiceContext) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if s.TestDataCache != nil {
		defer s.TestDataCache.Close()
	}
	if s.Pool == nil {
		return nil
	}
	return s.Pool.Shutdown(ctx)
}
