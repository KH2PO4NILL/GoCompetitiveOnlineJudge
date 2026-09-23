package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/checker"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/sandbox"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/testdatacache"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/testdatamanifest"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
)

type judgingSandbox struct {
	*fakeSandbox
	results []sandbox.ExecuteResult
	cancel  context.CancelFunc
}

func (executor *judgingSandbox) Execute(ctx context.Context, request sandbox.ExecuteRequest) (sandbox.ExecuteResult, error) {
	if request.Program == "/opt/judge/build" {
		return executor.fakeSandbox.Execute(ctx, request)
	}
	executor.requests = append(executor.requests, request)
	if executor.cancel != nil {
		executor.cancel()
		return sandbox.ExecuteResult{}, ctx.Err()
	}
	result := executor.results[0]
	executor.results = executor.results[1:]
	return result, nil
}

func judgeFixture(t *testing.T) (*Engine, *judgingSandbox, JudgeInput) {
	t.Helper()
	engine, fake, request := newTestEngine(t)
	zero := 0
	executor := &judgingSandbox{fakeSandbox: fake, results: []sandbox.ExecuteResult{{Reason: sandbox.ExitNormal, ExitCode: &zero, Stdout: []byte("42\n"), CPUTime: 1500 * time.Microsecond, PeakMemory: 2049}, {Reason: sandbox.ExitNormal, ExitCode: &zero, Stdout: []byte("41\n")}}}
	engine.executor = executor
	directory := t.TempDir()
	for _, filename := range []string{"input", "answer"} {
		if err := os.WriteFile(filepath.Join(directory, filename), []byte("42\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	manifest := &testdatamanifest.Manifest{Version: 1, TestCases: []testdatamanifest.TestCase{{Name: "first", Ordinal: 1, Score: 50, InputFile: "input", OutputFile: "answer"}, {Name: "second", Ordinal: 2, Score: 50, InputFile: "input", OutputFile: "answer"}}}
	spec := &problempb.JudgeProblemSpec{ProblemId: 3, Revision: 4, JudgeConfig: &problempb.JudgeConfig{Settings: &problempb.JudgeSettings{CheckerType: problempb.CheckerType_CHECKER_TYPE_STANDARD, TimeLimitMs: 1000, MemoryLimitMb: 64}}, TestCases: []*problempb.TestCaseInfo{{TestCaseId: 20, Name: "second", Ordinal: 2, Score: 50, Enabled: true}, {TestCaseId: 10, Name: "first", Ordinal: 1, Score: 50, Enabled: true}, {TestCaseId: 30, Name: "disabled", Ordinal: 3}}}
	return engine, executor, JudgeInput{Request: request, Spec: spec, Package: &testdatacache.Package{Dir: directory, Manifest: manifest}, Checker: checker.Exact}
}

func TestJudgeCompileOnceAllCasesAndProgress(t *testing.T) {
	engine, executor, input := judgeFixture(t)
	var progress []Progress
	input.Progress = func(value Progress) { progress = append(progress, value) }
	outcome, err := engine.Judge(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Verdict != WA || len(outcome.Cases) != 2 || outcome.Score != 50 || outcome.MaxScore != 100 || outcome.Cases[0].ID != 10 || outcome.Cases[0].TimeMS != 2 || outcome.Cases[0].MemoryKB != 3 {
		t.Fatalf("outcome: %+v", outcome)
	}
	if len(executor.requests) != 3 || progress[0].Stage != "compile" || progress[len(progress)-1] != (Progress{Stage: "done", Completed: 2, Total: 2}) {
		t.Fatalf("progress: %+v", progress)
	}
	entries, err := os.ReadDir(engine.workDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("leaked attempt: %v %v", entries, err)
	}
}

func TestJudgeFailureCleanupAndCompleteResults(t *testing.T) {
	for _, mode := range []string{"compile", "cancel", "sandbox", "missing answer"} {
		t.Run(mode, func(t *testing.T) {
			engine, executor, input := judgeFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "compile":
				executor.buildExit = 1
			case "cancel":
				executor.cancel = cancel
			case "sandbox":
				executor.results[0].Reason = sandbox.ExitSandboxError
			case "missing answer":
				input.Package.Manifest.TestCases[0].OutputFile = "missing"
			}
			outcome, err := engine.Judge(ctx, input)
			if mode == "compile" {
				if err != nil || outcome.Verdict != CE || len(outcome.Cases) != 0 || len(executor.requests) != 1 {
					t.Fatalf("outcome %+v %v", outcome, err)
				}
			} else if err == nil || len(outcome.Cases) != 2 || outcome.Verdict != SE {
				t.Fatalf("outcome %+v %v", outcome, err)
			}
			if mode == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			entries, _ := os.ReadDir(engine.workDir)
			if len(entries) != 0 {
				t.Fatal("leaked work directory")
			}
		})
	}
}

func TestVerdictsAndPrecedence(t *testing.T) {
	for reason, verdict := range map[sandbox.ExitReason]Verdict{sandbox.ExitTLE: TLE, sandbox.ExitMLE: MLE, sandbox.ExitOLE: OLE, sandbox.ExitSignal: RE, sandbox.ExitSandboxError: SE} {
		if got := ExecutionVerdict(sandbox.ExecuteResult{Reason: reason}); got != verdict {
			t.Fatalf("%s: %s", reason, got)
		}
	}
	for _, test := range []struct{ first, second, want Verdict }{{AC, WA, WA}, {TLE, RE, RE}, {MLE, TLE, MLE}, {SE, RE, SE}, {AC, AC, AC}} {
		if got := Aggregate([]CaseResult{{Verdict: test.first}, {Verdict: test.second}}); got != test.want {
			t.Fatal(got)
		}
	}
}
