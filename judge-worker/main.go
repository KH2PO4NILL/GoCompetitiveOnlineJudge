package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/app"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/svc"
)

const defaultConfigFile = "judge-worker/etc/judge-worker.yaml"

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Printf("judge worker exited: %v", err)
		os.Exit(1)
	}
}

type serviceRuntime struct {
	lifecycle app.Lifecycle
	logger    *slog.Logger
}

type serviceFactory func(config.Config) (serviceRuntime, error)

func run(parent context.Context, args []string) error {
	return runWithFactory(parent, args, func(c config.Config) (serviceRuntime, error) {
		serviceContext, err := svc.NewServiceContext(c)
		if err != nil {
			return serviceRuntime{}, err
		}
		return serviceRuntime{lifecycle: serviceContext, logger: serviceContext.Logger}, nil
	})
}

func runWithFactory(parent context.Context, args []string, createService serviceFactory) error {
	flags := flag.NewFlagSet("judge-worker", flag.ContinueOnError)
	configFile := flags.String("f", defaultConfigFile, "the config file")
	if err := flags.Parse(args); err != nil {
		return err
	}

	c, err := config.Load(*configFile)
	if err != nil {
		return err
	}
	if createService == nil {
		return errors.New("service factory is required")
	}
	runtime, err := createService(c)
	if err != nil {
		return fmt.Errorf("initialize service context: %w", err)
	}
	application, err := app.New(
		c.Worker.ID,
		c.Worker.Concurrency,
		c.Worker.QueueCapacity,
		c.ShutdownTimeout(),
		runtime.lifecycle,
		runtime.logger,
	)
	if err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}

	signalCtx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return application.Run(signalCtx)
}
