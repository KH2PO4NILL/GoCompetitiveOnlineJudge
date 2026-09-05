package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Lifecycle interface {
	Start() error
	Shutdown(context.Context) error
}

type Application struct {
	workerID        string
	concurrency     int
	queueCapacity   int
	shutdownTimeout time.Duration
	lifecycle       Lifecycle
	logger          *slog.Logger
}

func New(
	workerID string,
	concurrency int,
	queueCapacity int,
	shutdownTimeout time.Duration,
	lifecycle Lifecycle,
	logger *slog.Logger,
) (*Application, error) {
	if lifecycle == nil {
		return nil, fmt.Errorf("judge worker lifecycle is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("judge worker logger is required")
	}
	if shutdownTimeout <= 0 {
		return nil, fmt.Errorf("judge worker shutdown timeout must be positive")
	}
	return &Application{
		workerID: workerID, concurrency: concurrency, queueCapacity: queueCapacity,
		shutdownTimeout: shutdownTimeout, lifecycle: lifecycle, logger: logger,
	}, nil
}

func (a *Application) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := a.lifecycle.Start(); err != nil {
		return fmt.Errorf("start judge worker: %w", err)
	}
	a.logger.Info("judge worker started",
		"worker_id", a.workerID,
		"concurrency", a.concurrency,
		"queue_capacity", a.queueCapacity,
	)

	<-ctx.Done()
	a.logger.Info("judge worker stopping", "worker_id", a.workerID)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()
	if err := a.lifecycle.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("stop judge worker: %w", err)
	}
	a.logger.Info("judge worker stopped", "worker_id", a.workerID)
	return nil
}
