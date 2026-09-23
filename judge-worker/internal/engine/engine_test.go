package engine

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/language"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/sandbox"
)

type fakeSandbox struct {
	requests     []sandbox.ExecuteRequest
	digest       string
	resolveCalls int
	buildExit    int
	artifact     func(string) error
	executeErr   error
}

func (executor *fakeSandbox) ResolveImage(context.Context, string) (string, error) {
	executor.resolveCalls++
	return executor.digest, nil
}

func (executor *fakeSandbox) Execute(_ context.Context, request sandbox.ExecuteRequest) (sandbox.ExecuteResult, error) {
	if err := request.Validate(); err != nil {
		return sandbox.ExecuteResult{}, err
	}
	executor.requests = append(executor.requests, request)
	if executor.executeErr != nil {
		return sandbox.ExecuteResult{}, executor.executeErr
	}
	exitCode := 0
	if request.Program == language.BuildProgram {
		exitCode = executor.buildExit
		if exitCode == 0 {
			for _, mount := range request.Mounts {
				if mount.Target == language.ArtifactDirectory {
					if executor.artifact != nil {
						if err := executor.artifact(mount.Source); err != nil {
							return sandbox.ExecuteResult{}, err
						}
					} else if err := os.WriteFile(filepath.Join(mount.Source, "main"), []byte("fake executable"), 0o755); err != nil {
						return sandbox.ExecuteResult{}, err
					}
				}
			}
		}
	}
	return sandbox.ExecuteResult{Reason: sandbox.ExitNormal, ExitCode: &exitCode, Stdout: []byte("42\n")}, nil
}

func (*fakeSandbox) Cleanup(context.Context) (sandbox.CleanupResult, error) {
	return sandbox.CleanupResult{}, nil
}
func (*fakeSandbox) Close() error { return nil }

func newTestEngine(t *testing.T) (*Engine, *fakeSandbox, Request) {
	t.Helper()
	profiles, err := language.LoadProfiles("")
	if err != nil {
		t.Fatal(err)
	}
	profile := profiles[0]
	profile.ID = "fake-v1"
	profile.Source.EntryFile, profile.Source.Extensions = "main.fake", []string{".fake"}
	profile.Build.Image, profile.Run.Image = "fake-compiler:1.0.0", "fake-runtime:1.0.0"
	profile.Build.Args = []string{"literal; argument", "$(not-a-command)"}
	profile.TimeMultiplier, profile.MemoryMultiplier = 2, 1.5
	executor := &fakeSandbox{digest: "sha256:" + strings.Repeat("a", 64)}
	registry, err := language.NewRegistry([]language.Profile{profile}, executor)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(registry, executor, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	request := Request{AttemptID: 1, SubmissionID: 2, ProfileID: profile.ID,
		Sources: map[string][]byte{"main.fake": []byte("fake source")},
		Limits:  ProblemLimits{CPUTime: time.Second, WallTime: 2 * time.Second, MemoryBytes: 64 << 20}}
	return engine, executor, request
}

func TestFakeProfileBuildRunWithoutLanguageBranches(t *testing.T) {
	engine, executor, request := newTestEngine(t)
	t.Setenv("HOST_SECRET", "must-not-leak")
	attempt, err := engine.Prepare(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer attempt.Close()
	audit := attempt.Audit()
	executor.digest = "sha256:" + strings.Repeat("b", 64)
	request.Sources["main.fake"][0] = 'X'
	if _, err := attempt.Run(context.Background(), nil); !errors.Is(err, ErrAttemptState) {
		t.Fatal(err)
	}
	build, err := attempt.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if build.Policy != audit || len(executor.requests) != 1 {
		t.Fatal("build lost fixed policy")
	}
	buildRequest := executor.requests[0]
	if buildRequest.Image != audit.CompilerDigest || buildRequest.Args[0] != "literal; argument" || buildRequest.Args[1] != "$(not-a-command)" {
		t.Fatalf("build request = %+v", buildRequest)
	}
	if len(buildRequest.Mounts) != 2 || !buildRequest.Mounts[0].ReadOnly || buildRequest.Mounts[1].ReadOnly {
		t.Fatalf("build mounts = %+v", buildRequest.Mounts)
	}
	content, err := os.ReadFile(filepath.Join(buildRequest.Mounts[0].Source, "main.fake"))
	if err != nil || string(content) != "fake source" {
		t.Fatalf("source = %q, %v", content, err)
	}
	for _, environment := range buildRequest.Env {
		if strings.HasPrefix(environment, "HOST_SECRET=") {
			t.Fatal("inherited host environment")
		}
	}
	for range 2 {
		input := strings.NewReader("20 22\n")
		result, err := attempt.Run(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		if result.Policy != audit || string(result.Execution.Stdout) != "42\n" {
			t.Fatalf("run = %+v", result)
		}
		runRequest := executor.requests[len(executor.requests)-1]
		if runRequest.Image != audit.RuntimeDigest || len(runRequest.Mounts) != 1 || !runRequest.Mounts[0].ReadOnly || runRequest.Stdin != input {
			t.Fatalf("run request = %+v", runRequest)
		}
		if runRequest.Limits.CPUTime != 2*time.Second || runRequest.Limits.WallTime != 4*time.Second || runRequest.Limits.MemoryBytes != 96<<20 {
			t.Fatalf("limits = %+v", runRequest.Limits)
		}
	}
	if executor.resolveCalls != 2 {
		t.Fatalf("images resolved %d times; expected only during Prepare", executor.resolveCalls)
	}
	if _, err := attempt.Build(context.Background()); !errors.Is(err, ErrAttemptState) {
		t.Fatal(err)
	}
	if err := attempt.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(attempt.directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if _, err := attempt.Run(context.Background(), nil); !errors.Is(err, ErrAttemptState) {
		t.Fatal(err)
	}
}

func TestPrepareRejectsInvalidInput(t *testing.T) {
	for name, mutate := range map[string]func(*Request){
		"unknown profile":   func(request *Request) { request.ProfileID = "unknown-v1" },
		"attempt id":        func(request *Request) { request.AttemptID = 0 },
		"missing entry":     func(request *Request) { request.Sources = map[string][]byte{"other.fake": []byte("source")} },
		"source traversal":  func(request *Request) { request.Sources["../main.fake"] = []byte("bad") },
		"too much source":   func(request *Request) { request.Sources["main.fake"] = make([]byte, (1<<20)+1) },
		"zero cpu":          func(request *Request) { request.Limits.CPUTime = 0 },
		"wall below cpu":    func(request *Request) { request.Limits.WallTime = time.Nanosecond },
		"scaled time cap":   func(request *Request) { request.Limits.CPUTime = time.Minute; request.Limits.WallTime = time.Minute },
		"scaled memory cap": func(request *Request) { request.Limits.MemoryBytes = 2 << 30 },
		"overflow":          func(request *Request) { request.Limits.MemoryBytes = 1 << 62 },
	} {
		t.Run(name, func(t *testing.T) {
			engine, executor, request := newTestEngine(t)
			mutate(&request)
			if _, err := engine.Prepare(context.Background(), request); err == nil {
				t.Fatal("invalid request accepted")
			}
			if len(executor.requests) != 0 {
				t.Fatal("invalid request executed")
			}
			entries, err := os.ReadDir(engine.workDir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("leaked attempt directories: %v, %v", entries, err)
			}
		})
	}
}

func TestBuildFailurePreventsRun(t *testing.T) {
	engine, executor, request := newTestEngine(t)
	executor.buildExit = 1
	attempt, err := engine.Prepare(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer attempt.Close()
	result, err := attempt.Build(context.Background())
	if err != nil || result.Execution.ExitCode == nil || *result.Execution.ExitCode != 1 {
		t.Fatalf("build = %+v, %v", result, err)
	}
	if _, err := attempt.Run(context.Background(), nil); !errors.Is(err, ErrAttemptState) {
		t.Fatal(err)
	}
	if len(executor.requests) != 1 {
		t.Fatal("ran after failed build")
	}
}

func TestArtifactValidation(t *testing.T) {
	for name, artifact := range map[string]func(string) error{
		"missing":   func(string) error { return nil },
		"symlink":   func(directory string) error { return os.Symlink("/etc/passwd", filepath.Join(directory, "main")) },
		"directory": func(directory string) error { return os.Mkdir(filepath.Join(directory, "main"), 0o755) },
		"extra": func(directory string) error {
			return os.WriteFile(filepath.Join(directory, "unexpected"), []byte("bad"), 0o644)
		},
		"empty": func(directory string) error { return os.WriteFile(filepath.Join(directory, "main"), nil, 0o755) },
		"unreadable": func(directory string) error {
			return os.WriteFile(filepath.Join(directory, "main"), []byte("bad"), 0o700)
		},
		"oversized": func(directory string) error {
			file, err := os.OpenFile(filepath.Join(directory, "main"), os.O_CREATE|os.O_WRONLY, 0o755)
			if err != nil {
				return err
			}
			defer file.Close()
			return file.Truncate((64 << 20) + 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			engine, executor, request := newTestEngine(t)
			executor.artifact = artifact
			attempt, err := engine.Prepare(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			defer attempt.Close()
			if _, err := attempt.Build(context.Background()); err == nil {
				t.Fatal("invalid artifact accepted")
			}
			if _, err := attempt.Run(context.Background(), nil); !errors.Is(err, ErrAttemptState) {
				t.Fatal(err)
			}
		})
	}
}

func TestEffectiveLimitsAffectAttemptDigest(t *testing.T) {
	engine, _, request := newTestEngine(t)
	first, err := engine.Prepare(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	request.Limits.MemoryBytes++
	second, err := engine.Prepare(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if first.Audit().PolicyDigest == second.Audit().PolicyDigest {
		t.Fatal("effective limits missing from policy digest")
	}
}

func TestSandboxErrorPreservesAudit(t *testing.T) {
	engine, executor, request := newTestEngine(t)
	attempt, err := engine.Prepare(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer attempt.Close()
	executor.executeErr = context.DeadlineExceeded
	result, err := attempt.Build(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) || result.Policy != attempt.Audit() {
		t.Fatalf("result = %+v, %v", result, err)
	}
	if _, err := attempt.Run(context.Background(), nil); !errors.Is(err, ErrAttemptState) {
		t.Fatal(err)
	}
}

func TestSourcePermissionsIgnoreRestrictiveHostUmask(t *testing.T) {
	engine, _, request := newTestEngine(t)
	previous := unix.Umask(0o077)
	defer unix.Umask(previous)
	attempt, err := engine.Prepare(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer attempt.Close()
	for name, want := range map[string]os.FileMode{".": 0o700, "source": 0o755, "source/main.fake": 0o444, "artifact": 0o777} {
		info, err := os.Stat(filepath.Join(attempt.directory, name))
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("%s: info = %v, error = %v, want mode %o", name, info, err, want)
		}
		if name == "source/main.fake" && info.ModTime().Unix() != 0 {
			t.Fatal("source timestamp is not reproducible")
		}
	}
}
