package logic

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	infraauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgepolicy"
	authpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	contestpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const judgeWorkerService = "judge.worker"

type problemCaseSpec struct {
	TestCaseID int64
	Ordinal    int32
	GroupName  string
	MaxScore   int64
}

type normalizedProblemSpec struct {
	ProblemID        int64
	Revision         int64
	AllowedLanguages map[string]struct{}
	Cases            []problemCaseSpec
	CaseByID         map[int64]problemCaseSpec
	MaxScore         int64
}

func requirePrincipal(ctx context.Context) (infraauth.Principal, error) {
	principal, ok := infraauth.PrincipalFromContext(ctx)
	if !ok {
		return infraauth.Principal{}, status.Error(codes.Unauthenticated, "authenticated user required")
	}
	if principal.Status != authpb.UserStatus_USER_STATUS_NORMAL {
		return infraauth.Principal{}, status.Error(codes.PermissionDenied, "user account is unavailable")
	}
	switch principal.Role {
	case authpb.UserRole_USER_ROLE_USER,
		authpb.UserRole_USER_ROLE_ADMIN,
		authpb.UserRole_USER_ROLE_SUPER_ADMIN:
		return principal, nil
	default:
		return infraauth.Principal{}, status.Error(codes.Unauthenticated, "authenticated user is invalid")
	}
}

func requireAdmin(ctx context.Context) (infraauth.Principal, error) {
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return infraauth.Principal{}, err
	}
	if !isAdmin(principal) {
		return infraauth.Principal{}, status.Error(codes.PermissionDenied, "administrator permission required")
	}
	return principal, nil
}

func isAdmin(principal infraauth.Principal) bool {
	return principal.Role == authpb.UserRole_USER_ROLE_ADMIN ||
		principal.Role == authpb.UserRole_USER_ROLE_SUPER_ADMIN
}

func requireJudgeWorker(ctx context.Context) error {
	principal, ok := infraauth.ServicePrincipalFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "trusted judge worker identity required")
	}
	if principal.Name != judgeWorkerService {
		return status.Error(codes.PermissionDenied, "service caller is not allowed")
	}
	return nil
}

func requireSubmitModel(svcCtx *svc.ServiceContext) (model.SubmitModel, error) {
	if svcCtx == nil || svcCtx.SubmitModel == nil {
		return nil, status.Error(codes.Internal, "submit model is unavailable")
	}
	return svcCtx.SubmitModel, nil
}

func runtimeLimits(svcCtx *svc.ServiceContext) config.LimitsConf {
	limits := config.LimitsConf{
		MaxSourceCodeBytes: judgepolicy.MaxSourceCodeBytes, MaxClientRequestBytes: 128,
		MaxPageSize: 100, MaxBatchRejudge: 100, MaxCaseResults: 20000,
		MaxCompileOutputBytes: 4 * 1024 * 1024, MaxJudgeMessageBytes: 1024 * 1024,
	}
	if svcCtx == nil {
		return limits
	}
	configured := svcCtx.Config.Limits
	if configured.MaxSourceCodeBytes > 0 {
		limits.MaxSourceCodeBytes = min(configured.MaxSourceCodeBytes, judgepolicy.MaxSourceCodeBytes)
	}
	if configured.MaxClientRequestBytes > 0 {
		limits.MaxClientRequestBytes = configured.MaxClientRequestBytes
	}
	if configured.MaxPageSize > 0 {
		limits.MaxPageSize = configured.MaxPageSize
	}
	if configured.MaxBatchRejudge > 0 {
		limits.MaxBatchRejudge = configured.MaxBatchRejudge
	}
	if configured.MaxCaseResults > 0 {
		limits.MaxCaseResults = configured.MaxCaseResults
	}
	if configured.MaxCompileOutputBytes > 0 {
		limits.MaxCompileOutputBytes = configured.MaxCompileOutputBytes
	}
	if configured.MaxJudgeMessageBytes > 0 {
		limits.MaxJudgeMessageBytes = configured.MaxJudgeMessageBytes
	}
	return limits
}

func serviceNow(svcCtx *svc.ServiceContext) time.Time {
	if svcCtx != nil && svcCtx.Now != nil {
		return svcCtx.Now().UTC()
	}
	return time.Now().UTC()
}

func leaseDuration(svcCtx *svc.ServiceContext) time.Duration {
	if svcCtx != nil {
		if svcCtx.LeaseDuration > 0 {
			return svcCtx.LeaseDuration
		}
		if svcCtx.Config.Lease.DurationSeconds > 0 {
			return time.Duration(svcCtx.Config.Lease.DurationSeconds) * time.Second
		}
	}
	return 30 * time.Second
}

func mapModelError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, model.ErrInvalidSubmissionInput):
		return status.Error(codes.InvalidArgument, "invalid submission request")
	case errors.Is(err, model.ErrSubmissionNotFound), errors.Is(err, model.ErrAttemptNotFound):
		return status.Error(codes.NotFound, "submission not found")
	case errors.Is(err, model.ErrIdempotencyConflict), errors.Is(err, model.ErrDatabaseConflict):
		return status.Error(codes.Aborted, "submission state changed concurrently")
	case errors.Is(err, model.ErrLeaseMismatch), errors.Is(err, model.ErrLeaseExpired),
		errors.Is(err, model.ErrLeaseHeld), errors.Is(err, model.ErrAttemptAlreadyFinished),
		errors.Is(err, model.ErrStaleAttempt), errors.Is(err, model.ErrReportConflict):
		return status.Error(codes.FailedPrecondition, "submission attempt is no longer valid")
	case errors.Is(err, model.ErrSubmissionStateConflict):
		return status.Error(codes.Aborted, "submission state changed concurrently")
	default:
		return status.Error(codes.Internal, "submit service internal error")
	}
}

func mapDependencyError(err error, dependency string) error {
	if err == nil {
		return nil
	}
	switch status.Code(err) {
	case codes.NotFound:
		return status.Errorf(codes.NotFound, "%s resource not found", dependency)
	case codes.FailedPrecondition:
		return status.Errorf(codes.FailedPrecondition, "%s rejected the request", dependency)
	case codes.ResourceExhausted:
		return status.Errorf(codes.ResourceExhausted, "%s rejected the request", dependency)
	case codes.Canceled:
		return status.FromContextError(context.Canceled).Err()
	case codes.DeadlineExceeded:
		return status.Errorf(codes.Unavailable, "%s service is unavailable", dependency)
	default:
		return status.Errorf(codes.Unavailable, "%s service is unavailable", dependency)
	}
}

func validASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] < 0x20 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func normalizeProblem(
	spec *problempb.JudgeProblemSpec,
	expectedProblemID, requestedRevision int64,
) (*normalizedProblemSpec, error) {
	if spec == nil || spec.ProblemId != expectedProblemID || spec.ProblemId <= 0 || spec.Revision <= 0 {
		return nil, status.Error(codes.Unavailable, "problem service returned an invalid judge spec")
	}
	if requestedRevision > 0 && spec.Revision != requestedRevision {
		return nil, status.Error(codes.Unavailable, "problem service returned the wrong revision")
	}
	if spec.JudgeConfig == nil || spec.JudgeConfig.Settings == nil {
		return nil, status.Error(codes.Unavailable, "problem service returned an incomplete judge spec")
	}
	settings := spec.JudgeConfig.Settings
	if settings.TimeLimitMs <= 0 || settings.MemoryLimitMb <= 0 {
		return nil, status.Error(codes.Unavailable, "problem service returned invalid resource limits")
	}
	if settings.CheckerType < problempb.CheckerType_CHECKER_TYPE_STANDARD ||
		settings.CheckerType > problempb.CheckerType_CHECKER_TYPE_INTERACTIVE {
		return nil, status.Error(codes.Unavailable, "problem service returned an invalid checker type")
	}
	if !validInternalReference(spec.JudgeConfig.TestDataVersion, 255) ||
		!validInternalReference(spec.TestDataPackageKey, 1024) || !validSHA256(spec.TestDataChecksum) {
		return nil, status.Error(codes.Unavailable, "problem service returned invalid test data metadata")
	}
	switch settings.CheckerType {
	case problempb.CheckerType_CHECKER_TYPE_SPECIAL_JUDGE:
		if !validInternalReference(spec.JudgeConfig.CheckerFileKey, 1024) {
			return nil, status.Error(codes.Unavailable, "problem service returned a missing checker artifact")
		}
	case problempb.CheckerType_CHECKER_TYPE_INTERACTIVE:
		if !validInternalReference(spec.JudgeConfig.InteractorFileKey, 1024) {
			return nil, status.Error(codes.Unavailable, "problem service returned a missing interactor artifact")
		}
	}
	result := &normalizedProblemSpec{
		ProblemID: spec.ProblemId, Revision: spec.Revision,
		AllowedLanguages: make(map[string]struct{}, len(spec.JudgeConfig.Settings.AllowedLanguages)),
		CaseByID:         make(map[int64]problemCaseSpec),
	}
	for _, rawLanguage := range spec.JudgeConfig.Settings.AllowedLanguages {
		language := strings.TrimSpace(rawLanguage)
		if language == "" || len(language) > 64 || !validASCII(language) {
			return nil, status.Error(codes.Unavailable, "problem service returned an invalid language policy")
		}
		result.AllowedLanguages[language] = struct{}{}
	}
	if len(result.AllowedLanguages) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "problem has no allowed language")
	}
	ordinals := make(map[int32]struct{})
	caseIDs := make(map[int64]struct{})
	for _, testCase := range spec.TestCases {
		if testCase == nil || testCase.TestCaseId <= 0 || testCase.Ordinal < 0 || testCase.Score < 0 ||
			len(testCase.GroupName) > 128 || !utf8.ValidString(testCase.GroupName) {
			return nil, status.Error(codes.Unavailable, "problem service returned an invalid test case")
		}
		if _, exists := caseIDs[testCase.TestCaseId]; exists {
			return nil, status.Error(codes.Unavailable, "problem service returned duplicate test cases")
		}
		caseIDs[testCase.TestCaseId] = struct{}{}
		if _, exists := ordinals[testCase.Ordinal]; exists {
			return nil, status.Error(codes.Unavailable, "problem service returned duplicate test case ordinals")
		}
		ordinals[testCase.Ordinal] = struct{}{}
		if !testCase.Enabled {
			continue
		}
		item := problemCaseSpec{
			TestCaseID: testCase.TestCaseId, Ordinal: testCase.Ordinal,
			GroupName: testCase.GroupName, MaxScore: int64(testCase.Score),
		}
		if result.MaxScore > math.MaxInt64-item.MaxScore {
			return nil, status.Error(codes.Unavailable, "problem score exceeds supported range")
		}
		result.MaxScore += item.MaxScore
		result.Cases = append(result.Cases, item)
		result.CaseByID[item.TestCaseID] = item
	}
	if len(result.Cases) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "problem has no enabled test cases")
	}
	return result, nil
}

func validInternalReference(value string, maximum int) bool {
	trimmed := strings.TrimSpace(value)
	return value == trimmed && value != "" && len(value) <= maximum && validASCII(value)
}

func validSHA256(value string) bool {
	trimmed := strings.TrimSpace(value)
	if value != trimmed || len(value) != 64 {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') &&
			(character < 'A' || character > 'F') {
			return false
		}
	}
	return true
}

func validateLanguage(spec *normalizedProblemSpec, raw string) (string, error) {
	language := strings.TrimSpace(raw)
	if language == "" || len(language) > 64 || !validASCII(language) {
		return "", status.Error(codes.InvalidArgument, "language is invalid")
	}
	if spec == nil {
		return "", status.Error(codes.Internal, "problem specification is unavailable")
	}
	if _, ok := spec.AllowedLanguages[language]; !ok {
		return "", status.Error(codes.FailedPrecondition, "language is not allowed for this problem")
	}
	return language, nil
}

func validateProblemCaseLimit(spec *normalizedProblemSpec, maximum int) error {
	if spec == nil || maximum <= 0 {
		return status.Error(codes.Internal, "problem test case policy is unavailable")
	}
	if len(spec.Cases) > maximum || len(spec.Cases) > math.MaxInt32 {
		return status.Error(codes.FailedPrecondition, "problem has too many enabled test cases")
	}
	return nil
}

func timestamp(value time.Time, field string) (*timestamppb.Timestamp, error) {
	if value.IsZero() {
		return nil, status.Errorf(codes.Internal, "submission contains an invalid %s", field)
	}
	result := timestamppb.New(value.UTC())
	if err := result.CheckValid(); err != nil {
		return nil, status.Errorf(codes.Internal, "submission contains an invalid %s", field)
	}
	return result, nil
}

func optionalTimestamp(value *time.Time, field string) (*timestamppb.Timestamp, error) {
	if value == nil {
		return nil, nil
	}
	return timestamp(*value, field)
}

func submissionSummaryProto(
	summary model.SubmissionSummary,
	visibility pb.ResultVisibility,
) (*pb.SubmissionSummary, error) {
	if summary.SubmissionID <= 0 || summary.UserID <= 0 || summary.ProblemID <= 0 ||
		summary.ProblemRevision <= 0 || summary.AttemptCount <= 0 || summary.Language == "" ||
		len(summary.Language) > 64 || !validASCII(summary.Language) ||
		(summary.ContestID != nil && *summary.ContestID <= 0) {
		return nil, status.Error(codes.Internal, "submit model returned an invalid summary")
	}
	statusValue := pb.SubmissionStatus(summary.Status)
	if statusValue < pb.SubmissionStatus_SUBMISSION_STATUS_QUEUED ||
		statusValue > pb.SubmissionStatus_SUBMISSION_STATUS_FINISHED {
		return nil, status.Error(codes.Internal, "submit model returned an invalid status")
	}
	if visibility != pb.ResultVisibility_RESULT_VISIBILITY_FULL &&
		visibility != pb.ResultVisibility_RESULT_VISIBILITY_STATUS_ONLY {
		return nil, status.Error(codes.Internal, "result visibility is invalid")
	}
	if summary.Score != nil && *summary.Score < 0 || summary.MaxScore != nil && *summary.MaxScore < 0 ||
		summary.Score != nil && summary.MaxScore != nil && *summary.Score > *summary.MaxScore {
		return nil, status.Error(codes.Internal, "submit model returned an invalid score")
	}
	if summary.Verdict != nil {
		verdict := pb.Verdict(*summary.Verdict)
		if verdict < pb.Verdict_VERDICT_ACCEPTED || verdict > pb.Verdict_VERDICT_SYSTEM_ERROR {
			return nil, status.Error(codes.Internal, "submit model returned an invalid verdict")
		}
	}
	if statusValue == pb.SubmissionStatus_SUBMISSION_STATUS_FINISHED &&
		(summary.Verdict == nil || summary.Score == nil || summary.MaxScore == nil || summary.FinishedAt == nil) {
		return nil, status.Error(codes.Internal, "finished submission is missing its result")
	}
	submittedAt, err := timestamp(summary.SubmittedAt, "submitted_at")
	if err != nil {
		return nil, err
	}
	startedAt, err := optionalTimestamp(summary.JudgingStartedAt, "judging_started_at")
	if err != nil {
		return nil, err
	}
	finishedAt, err := optionalTimestamp(summary.FinishedAt, "finished_at")
	if err != nil {
		return nil, err
	}
	result := &pb.SubmissionSummary{
		SubmissionId: summary.SubmissionID, UserId: summary.UserID, ProblemId: summary.ProblemID,
		ContestId: cloneInt64(summary.ContestID), Language: summary.Language, Status: statusValue,
		ProblemRevision: summary.ProblemRevision, AttemptCount: summary.AttemptCount,
		ResultVisibility: visibility, SubmittedAt: submittedAt,
		JudgingStartedAt: startedAt, FinishedAt: finishedAt,
	}
	if visibility == pb.ResultVisibility_RESULT_VISIBILITY_STATUS_ONLY {
		return result, nil
	}
	if summary.Verdict != nil {
		verdict := pb.Verdict(*summary.Verdict)
		result.Verdict = verdict
	}
	if summary.Score != nil {
		result.Score = *summary.Score
	}
	if summary.MaxScore != nil {
		result.MaxScore = *summary.MaxScore
	}
	return result, nil
}

func submissionDetailProto(
	aggregate *model.SubmissionAggregate,
	visibility pb.ResultVisibility,
	includeSource bool,
) (*pb.SubmissionDetail, error) {
	if aggregate == nil || aggregate.CurrentAttempt == nil {
		return nil, status.Error(codes.Internal, "submit model returned an incomplete submission")
	}
	summary, err := submissionSummaryProto(aggregate.SubmissionSummary, visibility)
	if err != nil {
		return nil, err
	}
	result := &pb.SubmissionDetail{Summary: summary}
	if includeSource {
		result.SourceCode = aggregate.SourceCode
	}
	if visibility != pb.ResultVisibility_RESULT_VISIBILITY_FULL {
		return result, nil
	}
	if !utf8.ValidString(aggregate.CurrentAttempt.CompileOutput) {
		return nil, status.Error(codes.Internal, "submit model returned invalid compiler output")
	}
	result.CompileOutput = aggregate.CurrentAttempt.CompileOutput
	result.CaseResults = make([]*pb.TestCaseResult, len(aggregate.CaseResults))
	for index, item := range aggregate.CaseResults {
		verdict := pb.Verdict(item.Verdict)
		if item.TestCaseID <= 0 || item.Ordinal < 0 || verdict < pb.Verdict_VERDICT_ACCEPTED ||
			verdict > pb.Verdict_VERDICT_SYSTEM_ERROR || item.Score < 0 || item.TimeUsedMS < 0 || item.MemoryUsedKB < 0 ||
			len(item.GroupName) > 128 || !utf8.ValidString(item.GroupName) {
			return nil, status.Error(codes.Internal, "submit model returned an invalid test result")
		}
		result.CaseResults[index] = &pb.TestCaseResult{
			TestCaseId: item.TestCaseID, Ordinal: item.Ordinal, GroupName: item.GroupName,
			Verdict: verdict, Score: item.Score, TimeUsedMs: item.TimeUsedMS,
			MemoryUsedKb: item.MemoryUsedKB,
		}
	}
	return result, nil
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func resolveVisibilities(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	principal infraauth.Principal,
	summaries []model.SubmissionSummary,
) (map[int64]pb.ResultVisibility, error) {
	result := make(map[int64]pb.ResultVisibility, len(summaries))
	if isAdmin(principal) {
		for _, summary := range summaries {
			result[summary.SubmissionID] = pb.ResultVisibility_RESULT_VISIBILITY_FULL
		}
		return result, nil
	}
	references := make([]svc.SubmissionReference, 0)
	for _, summary := range summaries {
		if summary.ContestID == nil {
			result[summary.SubmissionID] = pb.ResultVisibility_RESULT_VISIBILITY_FULL
			continue
		}
		references = append(references, svc.SubmissionReference{
			SubmissionID: summary.SubmissionID, ContestID: *summary.ContestID,
			ProblemID: summary.ProblemID, UserID: summary.UserID,
		})
	}
	if len(references) == 0 {
		return result, nil
	}
	if svcCtx == nil || svcCtx.ContestPolicy == nil {
		return nil, status.Error(codes.Unavailable, "contest policy is unavailable")
	}
	resolved, err := svcCtx.ContestPolicy.ResolveResultVisibility(ctx, references, serviceNow(svcCtx))
	if err != nil {
		return nil, mapDependencyError(err, "contest")
	}
	for _, reference := range references {
		visibility, ok := resolved[reference.SubmissionID]
		if !ok {
			return nil, status.Error(codes.Unavailable, "contest service returned an incomplete visibility policy")
		}
		switch visibility {
		case svc.ResultVisibilityFull:
			result[reference.SubmissionID] = pb.ResultVisibility_RESULT_VISIBILITY_FULL
		case svc.ResultVisibilityStatusOnly:
			result[reference.SubmissionID] = pb.ResultVisibility_RESULT_VISIBILITY_STATUS_ONLY
		default:
			return nil, status.Error(codes.Unavailable, "contest service returned an invalid visibility policy")
		}
	}
	return result, nil
}

func createVisibility(validation *svc.ContestValidation) (pb.ResultVisibility, error) {
	if validation == nil {
		return 0, status.Error(codes.Unavailable, "contest service returned an invalid response")
	}
	switch validation.Mode {
	case contestpb.ContestMode_CONTEST_MODE_ICPC,
		contestpb.ContestMode_CONTEST_MODE_IOI:
		return pb.ResultVisibility_RESULT_VISIBILITY_FULL, nil
	case contestpb.ContestMode_CONTEST_MODE_OI:
		return pb.ResultVisibility_RESULT_VISIBILITY_STATUS_ONLY, nil
	default:
		return 0, status.Error(codes.Unavailable, "contest service returned an invalid mode")
	}
}

func validateUTF8Length(value string, maximum int, field string, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if (required && value == "") || len(value) > maximum || !utf8.ValidString(value) {
		return "", status.Errorf(codes.InvalidArgument, "%s is invalid", field)
	}
	return value, nil
}

func positiveOptional(value *int64, field string) (*int64, error) {
	if value == nil {
		return nil, nil
	}
	if *value <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "%s must be positive", field)
	}
	return cloneInt64(value), nil
}

func int64Pointer(value int64) *int64 { return &value }
