package engine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/checker"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/language"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/sandbox"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/testdatacache"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/testdatamanifest"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	"github.com/google/uuid"
)

func TestJudgeEngineLanguages(t *testing.T) {
	if os.Getenv("JUDGE_LANGUAGE_INTEGRATION") != "1" {
		t.Skip("requires official language images and Docker")
	}
	root := t.TempDir()
	endpoint := os.Getenv("JUDGE_DOCKER_ENDPOINT")
	if endpoint == "" {
		endpoint = "unix:///var/run/docker.sock"
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	executor, err := sandbox.NewDocker(sandbox.DockerOptions{Endpoint: endpoint, WorkerID: "judge-matrix-" + uuid.NewString(), User: "65532:65532", AllowedRoots: []sandbox.AllowedRoot{{Path: root, Writable: true}}, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	defer executor.Close()
	profiles, err := language.LoadProfiles("")
	if err != nil {
		t.Fatal(err)
	}
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
			data := t.TempDir()
			spec := &problempb.JudgeProblemSpec{ProblemId: 1, Revision: 1, JudgeConfig: &problempb.JudgeConfig{Settings: &problempb.JudgeSettings{CheckerType: problempb.CheckerType_CHECKER_TYPE_STANDARD, TimeLimitMs: 1000, MemoryLimitMb: 64}}}
			manifest := &testdatamanifest.Manifest{Version: 1}
			want := []Verdict{AC, WA, RE, TLE, MLE, OLE}
			for index, mode := range []string{"ac", "wa", "re", "tle", "mle", "ole"} {
				name := fmt.Sprint(index)
				if err := os.WriteFile(filepath.Join(data, name+".in"), []byte(mode), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(data, name+".out"), []byte("42\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				spec.TestCases = append(spec.TestCases, &problempb.TestCaseInfo{TestCaseId: int64(index + 1), Name: name, Ordinal: int32(index), Score: 10, Enabled: true})
				manifest.TestCases = append(manifest.TestCases, testdatamanifest.TestCase{Name: name, Ordinal: int64(index), Score: 10, InputFile: name + ".in", OutputFile: name + ".out"})
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			input := JudgeInput{Request: Request{AttemptID: 1, SubmissionID: 1, ProfileID: pack.id, Sources: map[string][]byte{pack.entry: source}}, Spec: spec, Package: &testdatacache.Package{Dir: data, Manifest: manifest}, Checker: checker.Exact}
			outcome, err := engine.Judge(ctx, input)
			if err != nil || len(outcome.Cases) != len(want) {
				t.Fatalf("judge: %+v %v", outcome, err)
			}
			for index, verdict := range want {
				if outcome.Cases[index].Verdict != verdict {
					t.Fatalf("case %d: %+v, want %s", index, outcome.Cases[index], verdict)
				}
			}
			input.Request.Sources[pack.entry] = []byte("invalid program")
			outcome, err = engine.Judge(ctx, input)
			if err != nil || outcome.Verdict != CE || len(outcome.Cases) != 0 {
				t.Fatalf("CE: %+v %v", outcome, err)
			}
		})
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("remaining attempt directories: %v %v", entries, err)
	}
}
