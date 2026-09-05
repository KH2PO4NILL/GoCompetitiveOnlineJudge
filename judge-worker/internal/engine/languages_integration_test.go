package engine

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/language"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/sandbox"
)

func TestLanguageImagesIntegration(t *testing.T) {
	if os.Getenv("JUDGE_LANGUAGE_INTEGRATION") != "1" {
		t.Skip("set JUDGE_LANGUAGE_INTEGRATION=1 and preload language images")
	}
	profiles, err := language.LoadProfiles(os.Getenv("JUDGE_LANGUAGE_REGISTRY_FILE"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	endpoint := os.Getenv("JUDGE_DOCKER_ENDPOINT")
	if endpoint == "" {
		endpoint = "unix:///var/run/docker.sock"
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	executor, err := sandbox.NewDocker(sandbox.DockerOptions{
		Endpoint: endpoint, WorkerID: "language-integration", User: "65532:65532",
		AllowedRoots: []sandbox.AllowedRoot{{Path: root, Writable: true}}, Logger: logger,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer executor.Close()
	registry, err := language.NewRegistry(profiles, executor)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(registry, executor, root, logger)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ id, directory, entry, fixture string }{
		{"cpp20-v1", "cpp20", "main.cpp", "sum.cpp"},
		{"go-v1", "go", "main.go", "sum.go"},
	} {
		t.Run(test.id, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("../../languages", test.directory, "testdata", test.fixture))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			for _, invalid := range []bool{false, true} {
				if invalid {
					source = []byte("this is not valid source code")
				}
				attempt, err := engine.Prepare(ctx, Request{AttemptID: 1, SubmissionID: 2, ProfileID: test.id,
					Sources: map[string][]byte{test.entry: source}, Limits: ProblemLimits{CPUTime: time.Second, WallTime: 3 * time.Second, MemoryBytes: 128 << 20}})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := attempt.Close(); err != nil {
						t.Errorf("cleanup attempt: %v", err)
					}
				})
				build, err := attempt.Build(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if invalid {
					if build.Execution.ExitCode == nil || *build.Execution.ExitCode == 0 {
						t.Fatalf("invalid source build = %+v", build)
					}
					if _, err := attempt.Run(ctx, nil); err == nil {
						t.Fatal("invalid source ran")
					}
					continue
				}
				if build.Execution.Reason != sandbox.ExitNormal || build.Execution.ExitCode == nil || *build.Execution.ExitCode != 0 {
					t.Fatalf("build = %+v", build)
				}
				for _, input := range []string{"20 22\n", "40 2\n"} {
					result, err := attempt.Run(ctx, strings.NewReader(input))
					if err != nil {
						t.Fatal(err)
					}
					if result.Execution.Reason != sandbox.ExitNormal || result.Execution.ExitCode == nil || *result.Execution.ExitCode != 0 || string(result.Execution.Stdout) != "42\n" || result.Policy != build.Policy {
						t.Fatalf("run = %+v", result)
					}
				}
			}
		})
	}
}
