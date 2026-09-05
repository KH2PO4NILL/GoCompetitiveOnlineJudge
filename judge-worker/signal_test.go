package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
)

type signalTestLifecycle struct {
	shutdown bool
}

func (lifecycle *signalTestLifecycle) Start() error {
	_, err := fmt.Fprintln(os.Stdout, "ready")
	return err
}

func (lifecycle *signalTestLifecycle) Shutdown(ctx context.Context) error {
	if ctx.Err() != nil {
		return fmt.Errorf("shutdown received cancelled context: %w", ctx.Err())
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return fmt.Errorf("shutdown grace period is unbounded")
	}
	lifecycle.shutdown = true
	return nil
}

func TestRunHandlesSIGTERM(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix SIGTERM")
	}
	if os.Getenv("GCOJ_TEST_SIGTERM_HELPER") == "1" {
		setExampleEnvironment(t)
		lifecycle := &signalTestLifecycle{}
		err := runWithFactory(context.Background(), []string{"-f", filepath.Join("etc", "judge-worker.yaml")}, func(config.Config) (serviceRuntime, error) {
			return serviceRuntime{lifecycle: lifecycle, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, nil
		})
		if err != nil || !lifecycle.shutdown {
			t.Fatal("SIGTERM did not drain lifecycle", err)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunHandlesSIGTERM$")
	command.Env = append(os.Environ(), "GCOJ_TEST_SIGTERM_HELPER=1")
	var diagnostics bytes.Buffer
	command.Stderr = &diagnostics
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		_ = command.Wait()
	})
	ready, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || ready != "ready\n" {
		t.Fatal("child worker did not start", err)
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("child worker did not exit gracefully: %v; %s", err, diagnostics.String())
	}
}
