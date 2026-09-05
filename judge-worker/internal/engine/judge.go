package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/checker"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/language"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/sandbox"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/testdatacache"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/testdatamanifest"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	"google.golang.org/protobuf/proto"
)

type Verdict string

const (
	AC  Verdict = "AC"
	WA  Verdict = "WA"
	CE  Verdict = "CE"
	RE  Verdict = "RE"
	TLE Verdict = "TLE"
	MLE Verdict = "MLE"
	OLE Verdict = "OLE"
	SE  Verdict = "SE"
)

type Progress struct {
	Stage            string
	Completed, Total int32
}
type JudgeInput struct {
	Request  Request
	Spec     *problempb.JudgeProblemSpec
	Package  *testdatacache.Package
	Checker  checker.Mode
	Progress func(Progress)
}
type CaseResult struct {
	ID         int64
	Ordinal    int32
	Group      string
	Verdict    Verdict
	Score      int64
	TimeMS     int64
	MemoryKB   int64
	WallTimeMS int64
	Message    string
}
type Outcome struct {
	Policy                     language.Audit
	ProblemID, ProblemRevision int64
	Verdict                    Verdict
	Score, MaxScore            int64
	CompileOutput              string
	Cases                      []CaseResult
}

func (engine *Engine) Judge(ctx context.Context, input JudgeInput) (outcome Outcome, resultErr error) {
	outcome.Verdict = SE
	if input.Spec == nil || input.Spec.ProblemId <= 0 || input.Spec.Revision <= 0 || input.Spec.JudgeConfig.GetSettings() == nil || input.Package == nil || input.Package.Manifest == nil {
		return outcome, errors.New("fixed problem spec and verified test data package are required")
	}
	input.Spec = proto.Clone(input.Spec).(*problempb.JudgeProblemSpec)
	outcome.ProblemID, outcome.ProblemRevision = input.Spec.ProblemId, input.Spec.Revision
	settings := input.Spec.JudgeConfig.Settings
	if settings.CheckerType != problempb.CheckerType_CHECKER_TYPE_STANDARD || settings.TimeLimitMs <= 0 || settings.MemoryLimitMb <= 0 {
		return outcome, errors.New("only Standard Checker with positive limits is supported")
	}
	if input.Checker == "" {
		input.Checker = checker.Tokens
	}
	if input.Checker != checker.Exact && input.Checker != checker.Tokens && input.Checker != checker.TrailingWhitespace {
		return outcome, errors.New("invalid checker mode")
	}
	cases := make([]*problempb.TestCaseInfo, 0, len(input.Spec.TestCases))
	seen := map[int64]bool{}
	manifestCases := map[string]testdatamanifest.TestCase{}
	for _, item := range input.Package.Manifest.TestCases {
		manifestCases[item.Name] = item
	}
	for _, item := range input.Spec.TestCases {
		if item == nil || item.TestCaseId <= 0 || seen[item.TestCaseId] || item.Score < 0 {
			return outcome, errors.New("invalid or duplicate test case")
		}
		seen[item.TestCaseId] = true
		if !item.Enabled {
			continue
		}
		manifestCase, exists := manifestCases[item.Name]
		if !exists || !manifestCase.IsEnabled() || manifestCase.Score != int64(item.Score) || manifestCase.Ordinal != int64(item.Ordinal) || manifestCase.GroupName != item.GroupName {
			return outcome, errors.New("manifest and fixed test case metadata differ")
		}
		cases = append(cases, item)
	}
	if len(cases) == 0 || len(cases) > testdatamanifest.MaxTestCases {
		return outcome, errors.New("problem requires a bounded nonempty enabled case list")
	}
	sort.Slice(cases, func(first, second int) bool {
		if cases[first].Ordinal != cases[second].Ordinal {
			return cases[first].Ordinal < cases[second].Ordinal
		}
		return cases[first].TestCaseId < cases[second].TestCaseId
	})
	for _, item := range cases {
		outcome.MaxScore += int64(item.Score)
		outcome.Cases = append(outcome.Cases, CaseResult{ID: item.TestCaseId, Ordinal: item.Ordinal, Group: item.GroupName, Verdict: SE, Message: "not executed"})
	}
	progress := func(stage string, completed int) {
		if input.Progress != nil {
			input.Progress(Progress{Stage: stage, Completed: int32(completed), Total: int32(len(cases))})
		}
	}
	progress("compile", 0)
	input.Request.Limits = ProblemLimits{CPUTime: time.Duration(settings.TimeLimitMs) * time.Millisecond, WallTime: time.Duration(settings.TimeLimitMs) * 3 * time.Millisecond, MemoryBytes: int64(settings.MemoryLimitMb) << 20}
	input.Request.Binding = PolicyBinding{ProblemID: input.Spec.ProblemId, Revision: input.Spec.Revision, Checker: string(input.Checker)}
	attempt, err := engine.Prepare(ctx, input.Request)
	if err != nil {
		return outcome, err
	}
	outcome.Policy = attempt.Audit()
	defer func() {
		if err := attempt.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("clean attempt: %w", err))
		}
	}()
	build, err := attempt.Build(ctx)
	outcome.CompileOutput = boundedDiagnostic(build.Execution.Stderr, build.Execution.Stdout)
	if err != nil {
		return outcome, err
	}
	if build.Execution.Reason == sandbox.ExitSandboxError {
		return outcome, errors.New("compiler sandbox infrastructure failure")
	}
	if build.Execution.Reason != sandbox.ExitNormal || build.Execution.ExitCode == nil || *build.Execution.ExitCode != 0 {
		outcome.Verdict, outcome.Cases = CE, nil
		progress("done", 0)
		return outcome, nil
	}
	root, err := os.OpenRoot(input.Package.Dir)
	if err != nil {
		return outcome, err
	}
	defer root.Close()
	for index, item := range cases {
		if err := ctx.Err(); err != nil {
			return outcome, err
		}
		progress("run", index)
		manifestCase := manifestCases[item.Name]
		inputFile, err := openCaseFile(root, manifestCase.InputFile)
		if err != nil {
			return outcome, err
		}
		actual, runErr := attempt.Run(ctx, inputFile)
		closeErr := inputFile.Close()
		result := &outcome.Cases[index]
		result.Verdict = ExecutionVerdict(actual.Execution)
		result.TimeMS = (actual.Execution.CPUTime + time.Millisecond - 1).Milliseconds()
		result.WallTimeMS = (actual.Execution.WallTime + time.Millisecond - 1).Milliseconds()
		result.MemoryKB = (actual.Execution.PeakMemory + 1023) / 1024
		result.Message = ""
		if runErr != nil || closeErr != nil {
			result.Verdict = SE
			return outcome, errors.Join(runErr, closeErr)
		}
		if result.Verdict == SE {
			return outcome, errors.New("runtime sandbox infrastructure failure")
		}
		if result.Verdict == AC {
			expected, err := openCaseFile(root, manifestCase.OutputFile)
			if err != nil {
				result.Verdict = SE
				return outcome, err
			}
			equal, checkErr := checker.Compare(ctx, input.Checker, expected, bytes.NewReader(actual.Execution.Stdout))
			closeErr := expected.Close()
			if checkErr != nil || closeErr != nil {
				result.Verdict = SE
				return outcome, errors.Join(checkErr, closeErr)
			}
			if !equal {
				result.Verdict = WA
			}
		}
		if result.Verdict == AC {
			result.Score = int64(item.Score)
			outcome.Score += result.Score
		}
		progress("run", index+1)
	}
	outcome.Verdict = Aggregate(outcome.Cases)
	progress("done", len(cases))
	return outcome, nil
}

func ExecutionVerdict(result sandbox.ExecuteResult) Verdict {
	switch result.Reason {
	case sandbox.ExitTLE:
		return TLE
	case sandbox.ExitMLE:
		return MLE
	case sandbox.ExitOLE:
		return OLE
	case sandbox.ExitSignal:
		return RE
	case sandbox.ExitNormal:
		if result.ExitCode == nil {
			return SE
		}
		if *result.ExitCode != 0 {
			return RE
		}
		return AC
	default:
		return SE
	}
}

func Aggregate(cases []CaseResult) Verdict {
	if len(cases) == 0 {
		return SE
	}
	for _, verdict := range []Verdict{SE, RE, MLE, TLE, OLE, WA} {
		for _, result := range cases {
			if result.Verdict == verdict {
				return verdict
			}
		}
	}
	for _, result := range cases {
		if result.Verdict != AC {
			return SE
		}
	}
	return AC
}

func openCaseFile(root *os.Root, name string) (*os.File, error) {
	if _, err := testdatamanifest.SafePath(name); err != nil {
		return nil, err
	}
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > checker.MaxBytes {
		return nil, errors.New("test data must be a bounded regular file")
	}
	return root.Open(name)
}

func boundedDiagnostic(parts ...[]byte) string {
	var output bytes.Buffer
	for _, part := range parts {
		_, _ = io.CopyN(&output, bytes.NewReader(part), int64((1<<20)-output.Len()))
	}
	return strings.ToValidUTF8(output.String(), "?")
}
