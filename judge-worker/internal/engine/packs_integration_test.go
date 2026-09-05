package engine

import (
	"context"
	"crypto/sha256"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/language"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/sandbox"
	"github.com/google/uuid"
)

func TestOfficialLanguagePacks(t *testing.T) {
	if os.Getenv("JUDGE_LANGUAGE_INTEGRATION") != "1" {
		t.Skip("set JUDGE_LANGUAGE_INTEGRATION=1 and preload compiler/runtime images")
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
	executor, err := sandbox.NewDocker(sandbox.DockerOptions{Endpoint: endpoint, WorkerID: "packs-" + uuid.NewString(), User: "65532:65532", AllowedRoots: []sandbox.AllowedRoot{{Path: root, Writable: true}}, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = executor.Close() })
	registry, err := language.NewRegistry(profiles, executor)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(registry, executor, root, logger)
	if err != nil {
		t.Fatal(err)
	}
	for _, pack := range []struct{ id, directory, entry, fixture string }{{"cpp20-v3", "cpp20", "main.cpp", "cases.cpp"}, {"go-v3", "go", "main.go", "cases.go"}} {
		t.Run(pack.id, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("../../languages", pack.directory, "testdata", pack.fixture))
			if err != nil {
				t.Fatal(err)
			}
			var previous [32]byte
			for repetition := range 2 {
				ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
				defer cancel()
				attempt, err := engine.Prepare(ctx, Request{AttemptID: int64(repetition + 1), SubmissionID: 1, ProfileID: pack.id, Sources: map[string][]byte{pack.entry: source}, Limits: ProblemLimits{CPUTime: time.Second, WallTime: 3 * time.Second, MemoryBytes: 64 << 20}})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := attempt.Close(); err != nil {
						t.Error(err)
					}
				})
				built, err := attempt.Build(ctx)
				if err != nil || built.Execution.ExitCode == nil || *built.Execution.ExitCode != 0 {
					t.Fatalf("build: %+v %v", built.Execution, err)
				}
				binary, err := os.ReadFile(filepath.Join(attempt.directory, "artifact", "main"))
				if err != nil {
					t.Fatal(err)
				}
				digest := sha256.Sum256(binary)
				if repetition == 1 {
					if digest != previous {
						t.Fatal("identical source/profile produced different artifacts")
					}
					continue
				}
				previous = digest
				for _, scenario := range []struct {
					mode   string
					reason sandbox.ExitReason
					code   int
				}{{"ac", sandbox.ExitNormal, 0}, {"wa", sandbox.ExitNormal, 0}, {"re", sandbox.ExitNormal, 7}, {"tle", sandbox.ExitTLE, -1}, {"mle", sandbox.ExitMLE, -1}, {"ole", sandbox.ExitOLE, -1}} {
					t.Run(scenario.mode, func(t *testing.T) {
						result, err := attempt.Run(ctx, strings.NewReader(scenario.mode))
						if err != nil || result.Execution.Reason != scenario.reason {
							t.Fatalf("result: %+v %v", result.Execution, err)
						}
						if scenario.code >= 0 && (result.Execution.ExitCode == nil || *result.Execution.ExitCode != scenario.code) {
							t.Fatalf("exit: %+v", result.Execution)
						}
					})
				}
			}
		})
	}
	t.Run("go rejects external modules offline", func(t *testing.T) {
		source, err := os.ReadFile("../../languages/go/testdata/external.go")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		attempt, err := engine.Prepare(ctx, Request{AttemptID: 10, SubmissionID: 1, ProfileID: "go-v3", Sources: map[string][]byte{"main.go": source}, Limits: ProblemLimits{CPUTime: time.Second, WallTime: 3 * time.Second, MemoryBytes: 64 << 20}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := attempt.Close(); err != nil {
				t.Error(err)
			}
		})
		result, err := attempt.Build(ctx)
		if err != nil || result.Execution.Reason != sandbox.ExitNormal || result.Execution.ExitCode == nil || *result.Execution.ExitCode == 0 || len(result.Execution.Stderr) == 0 || len(result.Execution.Stderr) > 1<<20 {
			t.Fatalf("offline module build: %+v, %v", result.Execution, err)
		}
	})
}
