package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/checker"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/consumer"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/engine"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/language"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/logging"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/processor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/sandbox"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/testdatacache"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/worker"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/workspace"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgemetrics"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgemq"
)

type ServiceContext struct {
	Config        config.Config
	Logger        *slog.Logger
	Pool          *worker.Pool
	TestDataCache *testdatacache.Cache
	Sandbox       sandbox.Sandbox
	Languages     language.Registry
	Engine        *engine.Engine
	Processor     *processor.Processor
	closeRPC      func() error
	Consumer      *consumer.Driver
	mqConsumer    *judgemq.Consumer
	mqProducer    *judgemq.Producer
	metrics       *http.Server

	lifecycleMu sync.Mutex
	started     bool
	closed      bool
	shutdownErr error
	workspace   *workspace.Workspace
}

type sandboxFactory func(config.Config, *slog.Logger) (sandbox.Sandbox, error)

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	return newServiceContext(c, newDockerSandbox)
}

func newServiceContext(c config.Config, createSandbox sandboxFactory) (_ *ServiceContext, resultErr error) {
	c.ApplyDefaults()
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate judge worker runtime config: %w", err)
	}
	if createSandbox == nil {
		return nil, errors.New("sandbox factory is required")
	}
	profiles, err := language.LoadProfiles(c.LanguageRegistryFile)
	if err != nil {
		return nil, fmt.Errorf("load language registry: %w", err)
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
	work, err := workspace.Open(c.Sandbox.WorkDir, c.Worker.ID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			_ = work.Close()
		}
	}()
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
	resolver, ok := executor.(language.ImageResolver)
	if !ok {
		_ = executor.Close()
		return nil, errors.New("sandbox must support immutable image resolution")
	}
	registry, err := language.NewRegistry(profiles, resolver)
	if err != nil {
		_ = executor.Close()
		return nil, fmt.Errorf("configure language registry: %w", err)
	}
	judgeEngine, err := engine.New(registry, executor, work.Dir, logger)
	if err != nil {
		_ = executor.Close()
		return nil, fmt.Errorf("configure judge engine: %w", err)
	}
	for _, profile := range profiles {
		logger.Info("language profile loaded", "profile_id", profile.ID, "schema_version", profile.SchemaVersion,
			"enabled", profile.Enabled, "compiler_image", profile.Build.Image, "runtime_image", profile.Run.Image)
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
	submit, problem, closeRPC, err := processor.NewGateways(c.SubmitRPC.Target, c.ProblemRPC.Target, c.InternalAuth.SubmitToken, c.InternalAuth.ProblemToken, c.RPCTimeout())
	if err != nil {
		_ = executor.Close()
		return nil, fmt.Errorf("configure judge RPC gateways: %w", err)
	}
	taskProcessor, err := processor.New(processor.Options{WorkerID: c.Worker.ID, Submit: submit, Problem: problem, Cache: dataCache, Judge: judgeEngine, Logger: logger, Profiles: c.Processor.Profiles, Checker: checker.Mode(c.Processor.Checker), HeartbeatInterval: time.Duration(c.Processor.HeartbeatSeconds) * time.Second, RetryBackoff: time.Duration(c.Processor.RetryBackoffMS) * time.Millisecond, ReportAttempts: c.Processor.ReportAttempts})
	if err != nil {
		_ = closeRPC()
		_ = executor.Close()
		return nil, fmt.Errorf("configure task processor: %w", err)
	}
	result := &ServiceContext{
		Config: c, Logger: logger, Pool: pool, TestDataCache: dataCache, Sandbox: executor,
		Languages: registry, Engine: judgeEngine, Processor: taskProcessor, closeRPC: closeRPC,
		workspace: work,
	}
	if c.Consumer.Enabled {
		settings := judgemq.Config{Endpoint: c.Consumer.Endpoint, TLS: c.Consumer.TLS, AccessKey: c.Consumer.AccessKey, SecretKey: c.Consumer.SecretKey, Timeout: time.Duration(c.Consumer.TimeoutSeconds) * time.Second}
		result.mqProducer, err = judgemq.NewProducer(settings)
		if err == nil {
			result.mqConsumer, err = judgemq.NewConsumer(settings, "")
		}
		if err == nil {
			result.Consumer, err = consumer.New(result.mqConsumer, result.mqProducer, taskProcessor, pool, consumer.Options{WorkerID: c.Worker.ID, Concurrency: c.Worker.Concurrency, MaxAttempts: c.Consumer.MaxAttempts, Visibility: time.Duration(c.Consumer.VisibilitySeconds) * time.Second, Timeout: settings.Timeout, Backoff: time.Duration(c.Consumer.BackoffSeconds) * time.Second, MaxBackoff: time.Duration(c.Consumer.MaxBackoffSeconds) * time.Second}, logger)
		}
		if err != nil {
			_ = result.Shutdown(context.Background())
			return nil, errors.New("configure judge consumer failed")
		}
	}
	return result, nil
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

func (s *ServiceContext) Start() (startErr error) {
	if s == nil || s.Pool == nil || s.TestDataCache == nil || s.Sandbox == nil {
		return errors.New("judge worker service context is unavailable")
	}
	s.lifecycleMu.Lock()
	defer func() {
		s.lifecycleMu.Unlock()
		if startErr != nil {
			ctx, cancel := context.WithTimeout(context.Background(), s.Config.ShutdownTimeout())
			defer cancel()
			startErr = errors.Join(startErr, s.Shutdown(ctx))
		}
	}()
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
		return fmt.Errorf("clean orphaned sandbox containers: %w", err)
	}
	s.Logger.Info("sandbox startup cleanup completed",
		"found", cleanup.Found, "removed", cleanup.Removed, "failed", cleanup.Failed,
	)
	if s.workspace != nil {
		if err := s.workspace.Cleanup(context.Background()); err != nil {
			return fmt.Errorf("clean orphaned attempt directories: %w", err)
		}
	}
	s.TestDataCache.Start()
	if err := s.Pool.Start(); err != nil {
		return err
	}
	if s.Consumer != nil {
		if err := s.mqConsumer.Start(); err != nil {
			return errors.New("start RocketMQ consumer failed")
		}
	}
	if s.Config.Metrics.Enabled {
		metrics := judgemetrics.New()
		if s.Consumer != nil {
			metrics = s.Consumer.Metrics
		}
		var err error
		s.metrics, err = metrics.Listen(s.Config.Metrics.ListenOn, s.Config.Metrics.Path)
		if err != nil {
			return errors.New("start judge metrics listener failed")
		}
	}
	if s.Consumer != nil {
		if err := s.Consumer.Start(); err != nil {
			return err
		}
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
	if s.Consumer != nil {
		shutdownErrors = append(shutdownErrors, s.Consumer.Stop(ctx))
	} else if s.Pool != nil {
		if err := s.Pool.Shutdown(ctx); err != nil {
			shutdownErrors = append(shutdownErrors, err)
		}
		shutdownErrors = append(shutdownErrors, s.Pool.Wait(context.Background()))
	}
	if s.mqConsumer != nil {
		shutdownErrors = append(shutdownErrors, s.mqConsumer.Close())
	}
	if s.mqProducer != nil {
		shutdownErrors = append(shutdownErrors, s.mqProducer.Close())
	}
	shutdownErrors = append(shutdownErrors, judgemetrics.Close(ctx, s.metrics))
	if s.TestDataCache != nil {
		s.TestDataCache.Close()
	}
	if s.Sandbox != nil {
		if err := s.Sandbox.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("close sandbox: %w", err))
		}
	}
	if s.closeRPC != nil {
		shutdownErrors = append(shutdownErrors, s.closeRPC())
	}
	shutdownErrors = append(shutdownErrors, s.workspace.Close())
	s.shutdownErr = errors.Join(shutdownErrors...)
	return s.shutdownErr
}

func closeSandboxError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("close sandbox: %w", err)
}
