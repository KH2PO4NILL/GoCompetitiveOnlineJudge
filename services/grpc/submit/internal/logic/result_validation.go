package logic

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type normalizedJudgeResult struct {
	Verdict       int64
	Score         int64
	MaxScore      int64
	CompileOutput string
	CaseResults   []model.CaseResult
}

func normalizeJudgeResult(
	in *pb.ReportJudgeResultReq,
	spec *normalizedProblemSpec,
	limits config.LimitsConf,
) (*normalizedJudgeResult, error) {
	if in == nil || spec == nil {
		return nil, status.Error(codes.InvalidArgument, "judge report is invalid")
	}
	if len(in.CompileOutput) > limits.MaxCompileOutputBytes {
		return nil, status.Error(codes.ResourceExhausted, "compile_output exceeds the limit")
	}
	if !utf8.ValidString(in.CompileOutput) {
		return nil, status.Error(codes.InvalidArgument, "compile_output is invalid")
	}
	if len(in.CaseResults) > limits.MaxCaseResults {
		return nil, status.Error(codes.ResourceExhausted, "too many test case results")
	}
	if !validReportVerdict(in.Verdict) || in.Score < 0 || in.MaxScore < 0 {
		return nil, status.Error(codes.InvalidArgument, "worker aggregate result is invalid")
	}
	if in.Verdict == pb.Verdict_VERDICT_COMPILE_ERROR {
		if len(in.CaseResults) != 0 {
			return nil, status.Error(codes.InvalidArgument, "compile error report cannot contain test case results")
		}
		return &normalizedJudgeResult{
			Verdict: int64(pb.Verdict_VERDICT_COMPILE_ERROR), MaxScore: spec.MaxScore,
			CompileOutput: in.CompileOutput, CaseResults: []model.CaseResult{},
		}, nil
	}
	if len(in.CaseResults) != len(spec.Cases) {
		return nil, status.Error(codes.InvalidArgument, "judge report does not contain every enabled test case")
	}

	received := make(map[int64]*pb.JudgeTestCaseResult, len(in.CaseResults))
	for _, item := range in.CaseResults {
		if item != nil && len(item.JudgeMessage) > limits.MaxJudgeMessageBytes {
			return nil, status.Error(codes.ResourceExhausted, "judge_message exceeds the limit")
		}
		if item == nil || item.TestCaseId <= 0 || item.Score < 0 || item.TimeUsedMs < 0 || item.MemoryUsedKb < 0 ||
			item.Ordinal < 0 || len(item.GroupName) > 128 || !utf8.ValidString(item.GroupName) ||
			!utf8.ValidString(item.JudgeMessage) ||
			!validCaseVerdict(item.Verdict) {
			return nil, status.Error(codes.InvalidArgument, "test case result is invalid")
		}
		if _, exists := received[item.TestCaseId]; exists {
			return nil, status.Error(codes.InvalidArgument, "test case result is duplicated")
		}
		if _, exists := spec.CaseByID[item.TestCaseId]; !exists {
			return nil, status.Error(codes.InvalidArgument, "test case result is unknown")
		}
		received[item.TestCaseId] = item
	}

	result := &normalizedJudgeResult{
		MaxScore: spec.MaxScore, CompileOutput: in.CompileOutput,
		CaseResults: make([]model.CaseResult, 0, len(spec.Cases)),
	}
	caseVerdicts := make([]pb.Verdict, 0, len(spec.Cases))
	for _, expected := range spec.Cases {
		item, ok := received[expected.TestCaseID]
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "judge report is missing a test case")
		}
		if item.Score > expected.MaxScore {
			return nil, status.Error(codes.InvalidArgument, "test case score exceeds its maximum")
		}
		if item.Verdict == pb.Verdict_VERDICT_ACCEPTED && item.Score != expected.MaxScore {
			return nil, status.Error(codes.InvalidArgument, "accepted test case must receive full score")
		}
		if result.Score > math.MaxInt64-item.Score {
			return nil, status.Error(codes.InvalidArgument, "judge report score exceeds supported range")
		}
		result.Score += item.Score
		caseVerdicts = append(caseVerdicts, item.Verdict)
		result.CaseResults = append(result.CaseResults, model.CaseResult{
			TestCaseID: expected.TestCaseID, Ordinal: expected.Ordinal, GroupName: expected.GroupName,
			Verdict: int64(item.Verdict), Score: item.Score, TimeUsedMS: item.TimeUsedMs,
			MemoryUsedKB: item.MemoryUsedKb, JudgeMessage: item.JudgeMessage,
		})
	}
	result.Verdict = int64(aggregateVerdict(caseVerdicts, result.Score, result.MaxScore))
	return result, nil
}

func validReportVerdict(value pb.Verdict) bool {
	return value >= pb.Verdict_VERDICT_ACCEPTED && value <= pb.Verdict_VERDICT_SYSTEM_ERROR
}

func validCaseVerdict(value pb.Verdict) bool {
	switch value {
	case pb.Verdict_VERDICT_ACCEPTED,
		pb.Verdict_VERDICT_PARTIAL_ACCEPTED,
		pb.Verdict_VERDICT_WRONG_ANSWER,
		pb.Verdict_VERDICT_TIME_LIMIT_EXCEEDED,
		pb.Verdict_VERDICT_MEMORY_LIMIT_EXCEEDED,
		pb.Verdict_VERDICT_RUNTIME_ERROR,
		pb.Verdict_VERDICT_OUTPUT_LIMIT_EXCEEDED,
		pb.Verdict_VERDICT_SYSTEM_ERROR:
		return true
	default:
		return false
	}
}

// aggregateVerdict fixes the service-side precedence used for persisted
// results. Infrastructure failures take precedence, then execution/resource
// failures, wrong answers, partial results, and finally accepted.
func aggregateVerdict(verdicts []pb.Verdict, score, maxScore int64) pb.Verdict {
	priority := []pb.Verdict{
		pb.Verdict_VERDICT_SYSTEM_ERROR,
		pb.Verdict_VERDICT_RUNTIME_ERROR,
		pb.Verdict_VERDICT_MEMORY_LIMIT_EXCEEDED,
		pb.Verdict_VERDICT_TIME_LIMIT_EXCEEDED,
		pb.Verdict_VERDICT_OUTPUT_LIMIT_EXCEEDED,
		pb.Verdict_VERDICT_WRONG_ANSWER,
		pb.Verdict_VERDICT_PARTIAL_ACCEPTED,
	}
	for _, candidate := range priority {
		for _, actual := range verdicts {
			if actual == candidate {
				return candidate
			}
		}
	}
	if len(verdicts) > 0 && score == maxScore {
		return pb.Verdict_VERDICT_ACCEPTED
	}
	return pb.Verdict_VERDICT_PARTIAL_ACCEPTED
}

func normalizeWorkerIdentity(value string, maximum int, field string, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if (required && value == "") || len(value) > maximum || (value != "" && !validASCII(value)) {
		return "", status.Errorf(codes.InvalidArgument, "%s is invalid", field)
	}
	return value, nil
}
