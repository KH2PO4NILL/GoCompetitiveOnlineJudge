package logic

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	infraauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	authpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	maxProblemPageSize  = 100
	maxBatchSize        = 100
	maxProblemBodyRunes = 1 << 20
	maxProblemSamples   = 100
	maxSampleIORunes    = 1 << 20
	maxExplanationRunes = 1 << 18
	maxProblemTags      = 50
	maxAllowedLanguages = 64
)

func requirePrincipal(ctx context.Context) (infraauth.Principal, error) {
	principal, ok := infraauth.PrincipalFromContext(ctx)
	if !ok {
		return infraauth.Principal{}, status.Error(codes.Unauthenticated, "authentication required")
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
		return infraauth.Principal{}, status.Error(codes.PermissionDenied, "user role is unavailable")
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

func requireInternalService(ctx context.Context) (infraauth.ServicePrincipal, error) {
	principal, ok := infraauth.ServicePrincipalFromContext(ctx)
	if !ok {
		return infraauth.ServicePrincipal{}, status.Error(codes.Unauthenticated, "trusted service identity required")
	}
	return principal, nil
}

func isAdmin(principal infraauth.Principal) bool {
	return principal.Role == authpb.UserRole_USER_ROLE_ADMIN ||
		principal.Role == authpb.UserRole_USER_ROLE_SUPER_ADMIN
}

func authorizeProblem(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	problemID int64,
) (infraauth.Principal, *model.ProblemAggregate, error) {
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return infraauth.Principal{}, nil, err
	}
	problem, err := svcCtx.ProblemModel.GetForEdit(ctx, problemID)
	if err != nil {
		return infraauth.Principal{}, nil, mapModelError(err)
	}
	if problem.AuthorID != principal.UserID && !isAdmin(principal) {
		return infraauth.Principal{}, nil, status.Error(codes.PermissionDenied, "problem permission denied")
	}
	return principal, problem, nil
}

func mapModelError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, model.ErrInvalidProblemInput):
		return status.Error(codes.InvalidArgument, "invalid problem input")
	case errors.Is(err, model.ErrProblemNotFound),
		errors.Is(err, model.ErrTestDataUploadNotFound):
		return status.Error(codes.NotFound, "problem resource not found")
	case errors.Is(err, model.ErrProblemRevisionConflict):
		return status.Error(codes.Aborted, "problem revision conflict")
	case errors.Is(err, model.ErrProblemAlreadyDeleted):
		return status.Error(codes.FailedPrecondition, "problem is already deleted")
	case errors.Is(err, model.ErrProblemNotDeleted):
		return status.Error(codes.FailedPrecondition, "problem is not deleted")
	case errors.Is(err, model.ErrProblemStateConflict):
		return status.Error(codes.FailedPrecondition, "problem state conflict")
	case errors.Is(err, model.ErrProblemNotPublishable):
		return status.Error(codes.FailedPrecondition, "problem is not publishable")
	case errors.Is(err, model.ErrTestDataUploadExpired):
		return status.Error(codes.FailedPrecondition, "test data upload expired")
	case errors.Is(err, model.ErrTestDataUploadFinalized):
		return status.Error(codes.FailedPrecondition, "test data upload was already finalized with different data")
	case errors.Is(err, model.ErrTestDataUploadConflict):
		return status.Error(codes.FailedPrecondition, "test data upload conflict")
	default:
		return status.Error(codes.Internal, "problem service internal error")
	}
}

func editableFromProto(fields *pb.ProblemEditableFields) (model.EditableProblem, error) {
	if fields == nil || fields.Statement == nil || fields.JudgeSettings == nil {
		return model.EditableProblem{}, status.Error(codes.InvalidArgument, "problem, statement and judge_settings are required")
	}
	if err := validateEditableProto(fields); err != nil {
		return model.EditableProblem{}, err
	}
	return model.EditableProblem{
		Title:             fields.Statement.Title,
		Description:       fields.Statement.Description,
		InputDescription:  fields.Statement.InputDescription,
		OutputDescription: fields.Statement.OutputDescription,
		Samples:           modelSamples(fields.Statement.Samples),
		Hint:              fields.Statement.Hint,
		Difficulty:        int64(fields.Difficulty),
		Tags:              append([]string(nil), fields.Tags...),
		Source:            fields.Source,
		TimeLimitMS:       int64(fields.JudgeSettings.TimeLimitMs),
		MemoryLimitMB:     int64(fields.JudgeSettings.MemoryLimitMb),
		AllowedLanguages:  append([]string(nil), fields.JudgeSettings.AllowedLanguages...),
		CheckerType:       int64(fields.JudgeSettings.CheckerType),
		Visibility:        int64(fields.Visibility),
	}, nil
}

func validateEditableProto(fields *pb.ProblemEditableFields) error {
	if fields == nil {
		return status.Error(codes.InvalidArgument, "problem is required")
	}
	if fields.Statement != nil {
		if err := validateStatementProto(fields.Statement); err != nil {
			return err
		}
	}
	if len(fields.Tags) > maxProblemTags {
		return status.Errorf(codes.InvalidArgument, "tags must not exceed %d entries", maxProblemTags)
	}
	for _, tag := range fields.Tags {
		if !utf8.ValidString(tag) || utf8.RuneCountInString(tag) > 64 {
			return status.Error(codes.InvalidArgument, "tag must be valid UTF-8 and at most 64 characters")
		}
	}
	if !utf8.ValidString(fields.Source) || utf8.RuneCountInString(fields.Source) > 255 {
		return status.Error(codes.InvalidArgument, "source must be valid UTF-8 and at most 255 characters")
	}
	if fields.JudgeSettings != nil {
		if len(fields.JudgeSettings.AllowedLanguages) > maxAllowedLanguages {
			return status.Errorf(codes.InvalidArgument, "allowed_languages must not exceed %d entries", maxAllowedLanguages)
		}
		for _, language := range fields.JudgeSettings.AllowedLanguages {
			if !utf8.ValidString(language) || utf8.RuneCountInString(language) > 64 {
				return status.Error(codes.InvalidArgument, "allowed language must be valid UTF-8 and at most 64 characters")
			}
		}
	}
	return nil
}

func validateStatementProto(statement *pb.ProblemStatement) error {
	values := []struct {
		name  string
		value string
		limit int
	}{
		{name: "title", value: statement.Title, limit: 200},
		{name: "description", value: statement.Description, limit: maxProblemBodyRunes},
		{name: "input_description", value: statement.InputDescription, limit: maxProblemBodyRunes},
		{name: "output_description", value: statement.OutputDescription, limit: maxProblemBodyRunes},
		{name: "hint", value: statement.Hint, limit: maxProblemBodyRunes},
	}
	for _, item := range values {
		if !utf8.ValidString(item.value) || utf8.RuneCountInString(item.value) > item.limit {
			return status.Errorf(codes.InvalidArgument, "%s must be valid UTF-8 and at most %d characters", item.name, item.limit)
		}
	}
	if len(statement.Samples) > maxProblemSamples {
		return status.Errorf(codes.InvalidArgument, "samples must not exceed %d entries", maxProblemSamples)
	}
	for _, sample := range statement.Samples {
		if sample == nil {
			return status.Error(codes.InvalidArgument, "problem sample cannot be nil")
		}
		if !utf8.ValidString(sample.Input) || !utf8.ValidString(sample.Output) || !utf8.ValidString(sample.Explanation) ||
			utf8.RuneCountInString(sample.Input) > maxSampleIORunes ||
			utf8.RuneCountInString(sample.Output) > maxSampleIORunes ||
			utf8.RuneCountInString(sample.Explanation) > maxExplanationRunes {
			return status.Error(codes.InvalidArgument, "sample text is invalid or too large")
		}
	}
	return nil
}

func modelSamples(samples []*pb.ProblemSample) []model.ProblemSample {
	result := make([]model.ProblemSample, 0, len(samples))
	for _, sample := range samples {
		if sample == nil {
			continue
		}
		result = append(result, model.ProblemSample{
			Input: sample.Input, Output: sample.Output, Explanation: sample.Explanation,
		})
	}
	return result
}

func problemPatch(fields *pb.ProblemEditableFields, mask *fieldmaskpb.FieldMask) (model.ProblemPatch, error) {
	if fields == nil || mask == nil || len(mask.Paths) == 0 {
		return model.ProblemPatch{}, status.Error(codes.InvalidArgument, "problem and a non-empty update_mask are required")
	}
	if err := validateFieldMask(mask.Paths); err != nil {
		return model.ProblemPatch{}, err
	}
	if err := validateEditableProto(fields); err != nil {
		return model.ProblemPatch{}, err
	}
	patch := model.ProblemPatch{}
	for _, path := range mask.Paths {
		switch path {
		case "statement":
			if fields.Statement == nil {
				return model.ProblemPatch{}, status.Error(codes.InvalidArgument, "statement is required by update_mask")
			}
			patchStatement(&patch, fields.Statement)
		case "statement.title":
			if fields.Statement == nil {
				return model.ProblemPatch{}, status.Error(codes.InvalidArgument, "statement is required by update_mask")
			}
			patch.Title = stringPointer(fields.Statement.Title)
		case "statement.description":
			if fields.Statement == nil {
				return model.ProblemPatch{}, status.Error(codes.InvalidArgument, "statement is required by update_mask")
			}
			patch.Description = stringPointer(fields.Statement.Description)
		case "statement.input_description":
			if fields.Statement == nil {
				return model.ProblemPatch{}, status.Error(codes.InvalidArgument, "statement is required by update_mask")
			}
			patch.InputDescription = stringPointer(fields.Statement.InputDescription)
		case "statement.output_description":
			if fields.Statement == nil {
				return model.ProblemPatch{}, status.Error(codes.InvalidArgument, "statement is required by update_mask")
			}
			patch.OutputDescription = stringPointer(fields.Statement.OutputDescription)
		case "statement.samples":
			if fields.Statement == nil {
				return model.ProblemPatch{}, status.Error(codes.InvalidArgument, "statement is required by update_mask")
			}
			samples := modelSamples(fields.Statement.Samples)
			patch.Samples = &samples
		case "statement.hint":
			if fields.Statement == nil {
				return model.ProblemPatch{}, status.Error(codes.InvalidArgument, "statement is required by update_mask")
			}
			patch.Hint = stringPointer(fields.Statement.Hint)
		case "difficulty":
			value := int64(fields.Difficulty)
			patch.Difficulty = &value
		case "tags":
			value := append([]string(nil), fields.Tags...)
			patch.Tags = &value
		case "source":
			patch.Source = stringPointer(fields.Source)
		case "judge_settings":
			if fields.JudgeSettings == nil {
				return model.ProblemPatch{}, status.Error(codes.InvalidArgument, "judge_settings is required by update_mask")
			}
			patchJudgeSettings(&patch, fields.JudgeSettings)
		case "judge_settings.time_limit_ms":
			if fields.JudgeSettings == nil {
				return model.ProblemPatch{}, status.Error(codes.InvalidArgument, "judge_settings is required by update_mask")
			}
			value := int64(fields.JudgeSettings.TimeLimitMs)
			patch.TimeLimitMS = &value
		case "judge_settings.memory_limit_mb":
			if fields.JudgeSettings == nil {
				return model.ProblemPatch{}, status.Error(codes.InvalidArgument, "judge_settings is required by update_mask")
			}
			value := int64(fields.JudgeSettings.MemoryLimitMb)
			patch.MemoryLimitMB = &value
		case "judge_settings.allowed_languages":
			if fields.JudgeSettings == nil {
				return model.ProblemPatch{}, status.Error(codes.InvalidArgument, "judge_settings is required by update_mask")
			}
			value := append([]string(nil), fields.JudgeSettings.AllowedLanguages...)
			patch.AllowedLanguages = &value
		case "judge_settings.checker_type":
			if fields.JudgeSettings == nil {
				return model.ProblemPatch{}, status.Error(codes.InvalidArgument, "judge_settings is required by update_mask")
			}
			value := int64(fields.JudgeSettings.CheckerType)
			patch.CheckerType = &value
		case "visibility":
			value := int64(fields.Visibility)
			patch.Visibility = &value
		default:
			return model.ProblemPatch{}, status.Errorf(codes.InvalidArgument, "unsupported update_mask path %q", path)
		}
	}
	return patch, nil
}

func validateFieldMask(paths []string) error {
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if path == "" || strings.TrimSpace(path) != path {
			return status.Error(codes.InvalidArgument, "update_mask contains an invalid path")
		}
		if _, ok := seen[path]; ok {
			return status.Errorf(codes.InvalidArgument, "update_mask contains duplicate path %q", path)
		}
		seen[path] = struct{}{}
	}
	for parent := range seen {
		for child := range seen {
			if parent != child && strings.HasPrefix(child, parent+".") {
				return status.Errorf(codes.InvalidArgument, "update_mask contains overlapping paths %q and %q", parent, child)
			}
		}
	}
	return nil
}

func patchStatement(patch *model.ProblemPatch, statement *pb.ProblemStatement) {
	patch.Title = stringPointer(statement.Title)
	patch.Description = stringPointer(statement.Description)
	patch.InputDescription = stringPointer(statement.InputDescription)
	patch.OutputDescription = stringPointer(statement.OutputDescription)
	samples := modelSamples(statement.Samples)
	patch.Samples = &samples
	patch.Hint = stringPointer(statement.Hint)
}

func patchJudgeSettings(patch *model.ProblemPatch, settings *pb.JudgeSettings) {
	timeLimit := int64(settings.TimeLimitMs)
	memoryLimit := int64(settings.MemoryLimitMb)
	languages := append([]string(nil), settings.AllowedLanguages...)
	checkerType := int64(settings.CheckerType)
	patch.TimeLimitMS = &timeLimit
	patch.MemoryLimitMB = &memoryLimit
	patch.AllowedLanguages = &languages
	patch.CheckerType = &checkerType
}

func stringPointer(value string) *string { return &value }

func managementProblemProto(ctx context.Context, svcCtx *svc.ServiceContext, problem *model.ProblemAggregate) (*pb.Problem, error) {
	version := ""
	if problem != nil && problem.CurrentTestDataVersionID != nil {
		var err error
		version, err = svcCtx.ProblemModel.GetTestDataVersion(ctx, problem.ProblemID, *problem.CurrentTestDataVersionID)
		if err != nil {
			return nil, mapModelError(err)
		}
	}
	return problemProto(problem, version), nil
}

func problemProto(problem *model.ProblemAggregate, testDataVersion string) *pb.Problem {
	if problem == nil {
		return nil
	}
	result := &pb.Problem{
		ProblemId:   problem.ProblemID,
		Statement:   statementProto(problem.Editable),
		Metadata:    metadataProto(problem.Editable, problem.AuthorID),
		JudgeConfig: judgeConfigProto(problem.Editable, testDataVersion),
		Status:      pb.ProblemStatus(problem.Status),
		Visibility:  pb.ProblemVisibility(problem.Editable.Visibility),
		Revision:    problem.Revision,
		CreatedAt:   timestamppb.New(problem.CreatedAt),
		UpdatedAt:   timestamppb.New(problem.UpdatedAt),
	}
	if problem.PublishedAt != nil {
		result.PublishedAt = timestamppb.New(*problem.PublishedAt)
	}
	return result
}

func practiceProblemProto(problem *model.ProblemAggregate) *pb.PracticeProblemInfo {
	if problem == nil {
		return nil
	}
	return &pb.PracticeProblemInfo{
		ProblemId:     problem.ProblemID,
		Statement:     statementProto(problem.Editable),
		Metadata:      metadataProto(problem.Editable, problem.AuthorID),
		JudgeSettings: judgeSettingsProto(problem.Editable),
		Revision:      problem.Revision,
	}
}

func statementProto(editable model.EditableProblem) *pb.ProblemStatement {
	samples := make([]*pb.ProblemSample, 0, len(editable.Samples))
	for _, sample := range editable.Samples {
		samples = append(samples, &pb.ProblemSample{
			Input: sample.Input, Output: sample.Output, Explanation: sample.Explanation,
		})
	}
	return &pb.ProblemStatement{
		Title: editable.Title, Description: editable.Description,
		InputDescription: editable.InputDescription, OutputDescription: editable.OutputDescription,
		Samples: samples, Hint: editable.Hint,
	}
}

func metadataProto(editable model.EditableProblem, authorID int64) *pb.ProblemMetadata {
	return &pb.ProblemMetadata{
		Difficulty: pb.ProblemDifficulty(editable.Difficulty),
		Tags:       append([]string(nil), editable.Tags...),
		Source:     editable.Source,
		AuthorId:   authorID,
	}
}

func judgeSettingsProto(editable model.EditableProblem) *pb.JudgeSettings {
	return &pb.JudgeSettings{
		TimeLimitMs:      int32(editable.TimeLimitMS),
		MemoryLimitMb:    int32(editable.MemoryLimitMB),
		AllowedLanguages: append([]string(nil), editable.AllowedLanguages...),
		CheckerType:      pb.CheckerType(editable.CheckerType),
	}
}

func judgeConfigProto(editable model.EditableProblem, version string) *pb.JudgeConfig {
	return &pb.JudgeConfig{
		Settings:          judgeSettingsProto(editable),
		TestDataVersion:   version,
		CheckerFileKey:    dereferenceString(editable.CheckerFileKey),
		InteractorFileKey: dereferenceString(editable.InteractorFileKey),
	}
}

func dereferenceString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func summaryProto(summary model.ProblemSummary) *pb.ProblemSummary {
	return &pb.ProblemSummary{
		ProblemId: summary.ProblemID, Title: summary.Title,
		Difficulty: pb.ProblemDifficulty(summary.Difficulty),
		Tags:       append([]string(nil), summary.Tags...),
		Status:     pb.ProblemStatus(summary.Status),
		Visibility: pb.ProblemVisibility(summary.Visibility),
		AuthorId:   summary.AuthorID,
		UpdatedAt:  timestamppb.New(summary.UpdatedAt),
		Revision:   summary.Revision,
	}
}

func testCaseProto(testCase model.TestCaseRecord) *pb.TestCaseInfo {
	return &pb.TestCaseInfo{
		TestCaseId: testCase.ID,
		Name:       testCase.Name,
		Score:      int32(testCase.Score),
		Ordinal:    int32(testCase.Ordinal),
		Enabled:    testCase.Enabled,
		GroupName:  testCase.GroupName,
	}
}

func judgeSpecProto(spec *model.JudgeProblemSpec) *pb.JudgeProblemSpec {
	if spec == nil {
		return nil
	}
	testCases := make([]*pb.TestCaseInfo, 0, len(spec.TestCases))
	for _, testCase := range spec.TestCases {
		testCases = append(testCases, testCaseProto(testCase))
	}
	return &pb.JudgeProblemSpec{
		ProblemId: spec.ProblemID,
		Revision:  spec.Revision,
		JudgeConfig: &pb.JudgeConfig{
			Settings: &pb.JudgeSettings{
				TimeLimitMs: int32(spec.TimeLimitMS), MemoryLimitMb: int32(spec.MemoryLimitMB),
				AllowedLanguages: append([]string(nil), spec.AllowedLanguages...),
				CheckerType:      pb.CheckerType(spec.CheckerType),
			},
			TestDataVersion:   spec.TestDataVersion,
			CheckerFileKey:    dereferenceString(spec.CheckerFileKey),
			InteractorFileKey: dereferenceString(spec.InteractorFileKey),
		},
		TestDataPackageKey: spec.TestDataPackageKey,
		TestDataChecksum:   spec.TestDataChecksum,
		TestCases:          testCases,
	}
}

func validateListRequest(in *pb.ListProblemsReq) error {
	if in == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if in.Page < 0 || in.PageSize < 0 {
		return status.Error(codes.InvalidArgument, "page and page_size cannot be negative")
	}
	if in.PageSize > maxProblemPageSize {
		return status.Errorf(codes.InvalidArgument, "page_size must not exceed %d", maxProblemPageSize)
	}
	if !utf8.ValidString(in.Keyword) || utf8.RuneCountInString(in.Keyword) > 200 {
		return status.Error(codes.InvalidArgument, "keyword must be valid UTF-8 and at most 200 characters")
	}
	if in.Difficulty < pb.ProblemDifficulty_PROBLEM_DIFFICULTY_UNSPECIFIED || in.Difficulty > pb.ProblemDifficulty_PROBLEM_DIFFICULTY_HARD {
		return status.Error(codes.InvalidArgument, "invalid difficulty")
	}
	if in.Status < pb.ProblemStatus_PROBLEM_STATUS_UNSPECIFIED || in.Status > pb.ProblemStatus_PROBLEM_STATUS_DELETED {
		return status.Error(codes.InvalidArgument, "invalid status")
	}
	if in.Visibility < pb.ProblemVisibility_PROBLEM_VISIBILITY_UNSPECIFIED || in.Visibility > pb.ProblemVisibility_PROBLEM_VISIBILITY_CONTEST_ONLY {
		return status.Error(codes.InvalidArgument, "invalid visibility")
	}
	if len(in.Tags) > maxProblemTags {
		return status.Errorf(codes.InvalidArgument, "tags must not exceed %d entries", maxProblemTags)
	}
	for _, tag := range in.Tags {
		if !utf8.ValidString(tag) || utf8.RuneCountInString(tag) > 64 {
			return status.Error(codes.InvalidArgument, "tag must be valid UTF-8 and at most 64 characters")
		}
	}
	if in.AuthorId != nil && *in.AuthorId <= 0 {
		return status.Error(codes.InvalidArgument, "author_id must be positive")
	}
	return nil
}

func listFilter(in *pb.ListProblemsReq) model.ProblemListFilter {
	filter := model.ProblemListFilter{
		Page: int(in.Page), PageSize: int(in.PageSize), Keyword: in.Keyword,
		Tags: append([]string(nil), in.Tags...), AuthorID: in.AuthorId,
	}
	if in.Difficulty != pb.ProblemDifficulty_PROBLEM_DIFFICULTY_UNSPECIFIED {
		value := int64(in.Difficulty)
		filter.Difficulty = &value
	}
	if in.Status != pb.ProblemStatus_PROBLEM_STATUS_UNSPECIFIED {
		value := int64(in.Status)
		filter.Status = &value
	}
	if in.Visibility != pb.ProblemVisibility_PROBLEM_VISIBILITY_UNSPECIFIED {
		value := int64(in.Visibility)
		filter.Visibility = &value
	}
	return filter
}

func managementListRequested(in *pb.ListProblemsReq) bool {
	return in.Status != pb.ProblemStatus_PROBLEM_STATUS_UNSPECIFIED ||
		in.Visibility != pb.ProblemVisibility_PROBLEM_VISIBILITY_UNSPECIFIED ||
		in.AuthorId != nil
}

func validatePositiveProblemRevision(problemID, revision int64) error {
	if problemID <= 0 {
		return status.Error(codes.InvalidArgument, "problem_id must be positive")
	}
	if revision <= 0 {
		return status.Error(codes.InvalidArgument, "expected_revision must be positive")
	}
	return nil
}

func invalidDependency(name string) error {
	return status.Error(codes.Unavailable, fmt.Sprintf("%s is unavailable", name))
}

func validSHA256String(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validateTestCaseMetadata(testCases []model.TestCaseInput) error {
	if len(testCases) == 0 {
		return status.Error(codes.FailedPrecondition, "test data package contains no test cases")
	}
	if len(testCases) > 10000 {
		return status.Error(codes.ResourceExhausted, "test data package contains too many test cases")
	}
	for _, testCase := range testCases {
		if testCase.Score < 0 || testCase.Score > math.MaxInt32 ||
			testCase.Ordinal < 0 || testCase.Ordinal > math.MaxInt32 {
			return status.Error(codes.InvalidArgument, "test case score or ordinal is out of range")
		}
	}
	return nil
}
