package mapper

import (
	"strings"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/internal/httpbridge"
	authpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	contestpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	submitpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

func InvalidRPCResponse(field string) error {
	return status.Errorf(codes.Unavailable, "upstream RPC response is missing %s", field)
}

func EmailCodePurpose(value string) (authpb.EmailCodePurpose, error) {
	values := map[string]authpb.EmailCodePurpose{
		"login":          authpb.EmailCodePurpose_EMAIL_CODE_PURPOSE_LOGIN,
		"change_email":   authpb.EmailCodePurpose_EMAIL_CODE_PURPOSE_CHANGE_EMAIL,
		"reset_password": authpb.EmailCodePurpose_EMAIL_CODE_PURPOSE_RESET_PASSWORD,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return authpb.EmailCodePurpose_EMAIL_CODE_PURPOSE_UNSPECIFIED,
		status.Error(codes.InvalidArgument, "invalid email code purpose")
}

func UserRole(value string) (authpb.UserRole, error) {
	values := map[string]authpb.UserRole{
		"":            authpb.UserRole_USER_ROLE_UNSPECIFIED,
		"user":        authpb.UserRole_USER_ROLE_USER,
		"admin":       authpb.UserRole_USER_ROLE_ADMIN,
		"super_admin": authpb.UserRole_USER_ROLE_SUPER_ADMIN,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return authpb.UserRole_USER_ROLE_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid user role")
}

func OptionalUserRole(value *string) (*authpb.UserRole, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := UserRole(*value)
	if err != nil {
		return nil, err
	}
	if parsed == authpb.UserRole_USER_ROLE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "role must not be empty")
	}
	return &parsed, nil
}

func UserStatus(value string) (authpb.UserStatus, error) {
	values := map[string]authpb.UserStatus{
		"":         authpb.UserStatus_USER_STATUS_UNSPECIFIED,
		"normal":   authpb.UserStatus_USER_STATUS_NORMAL,
		"disabled": authpb.UserStatus_USER_STATUS_DISABLED,
		"banned":   authpb.UserStatus_USER_STATUS_BANNED,
		"deleted":  authpb.UserStatus_USER_STATUS_DELETED,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return authpb.UserStatus_USER_STATUS_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid user status")
}

func OptionalUserStatus(value *string) (*authpb.UserStatus, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := UserStatus(*value)
	if err != nil {
		return nil, err
	}
	if parsed == authpb.UserStatus_USER_STATUS_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "status must not be empty")
	}
	return &parsed, nil
}

func User(value *authpb.UserInfo) (types.UserInfo, error) {
	if value == nil {
		return types.UserInfo{}, InvalidRPCResponse("user")
	}
	return types.UserInfo{
		UserID: value.UserId, Username: value.Username, Email: value.Email,
		Role:   httpbridge.EnumName(value.Role.String(), "USER_ROLE_"),
		Status: httpbridge.EnumName(value.Status.String(), "USER_STATUS_"), SchoolID: value.SchoolId,
	}, nil
}

func TokenPair(value *authpb.TokenPair) (types.TokenPair, error) {
	if value == nil {
		return types.TokenPair{}, InvalidRPCResponse("tokens")
	}
	return types.TokenPair{AccessToken: value.AccessToken, RefreshToken: value.RefreshToken, ExpiresIn: value.ExpiresIn}, nil
}

func Login(value *authpb.LoginResp) (*types.LoginResp, error) {
	if value == nil {
		return nil, InvalidRPCResponse("login response")
	}
	tokens, err := TokenPair(value.Tokens)
	if err != nil {
		return nil, err
	}
	user, err := User(value.User)
	if err != nil {
		return nil, err
	}
	return &types.LoginResp{Tokens: tokens, User: user}, nil
}

func ProblemDifficulty(value string) (problempb.ProblemDifficulty, error) {
	values := map[string]problempb.ProblemDifficulty{
		"":            problempb.ProblemDifficulty_PROBLEM_DIFFICULTY_UNSPECIFIED,
		"easy":        problempb.ProblemDifficulty_PROBLEM_DIFFICULTY_EASY,
		"easy_plus":   problempb.ProblemDifficulty_PROBLEM_DIFFICULTY_EASY_PLUS,
		"medium":      problempb.ProblemDifficulty_PROBLEM_DIFFICULTY_MEDIUM,
		"medium_plus": problempb.ProblemDifficulty_PROBLEM_DIFFICULTY_MEDIUM_PLUS,
		"hard":        problempb.ProblemDifficulty_PROBLEM_DIFFICULTY_HARD,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return problempb.ProblemDifficulty_PROBLEM_DIFFICULTY_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid problem difficulty")
}

func ProblemStatus(value string) (problempb.ProblemStatus, error) {
	values := map[string]problempb.ProblemStatus{
		"":          problempb.ProblemStatus_PROBLEM_STATUS_UNSPECIFIED,
		"draft":     problempb.ProblemStatus_PROBLEM_STATUS_DRAFT,
		"published": problempb.ProblemStatus_PROBLEM_STATUS_PUBLISHED,
		"archived":  problempb.ProblemStatus_PROBLEM_STATUS_ARCHIVED,
		"deleted":   problempb.ProblemStatus_PROBLEM_STATUS_DELETED,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return problempb.ProblemStatus_PROBLEM_STATUS_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid problem status")
}

func ProblemVisibility(value string) (problempb.ProblemVisibility, error) {
	values := map[string]problempb.ProblemVisibility{
		"":             problempb.ProblemVisibility_PROBLEM_VISIBILITY_UNSPECIFIED,
		"private":      problempb.ProblemVisibility_PROBLEM_VISIBILITY_PRIVATE,
		"public":       problempb.ProblemVisibility_PROBLEM_VISIBILITY_PUBLIC,
		"contest_only": problempb.ProblemVisibility_PROBLEM_VISIBILITY_CONTEST_ONLY,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return problempb.ProblemVisibility_PROBLEM_VISIBILITY_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid problem visibility")
}

func CheckerType(value string) (problempb.CheckerType, error) {
	values := map[string]problempb.CheckerType{
		"":              problempb.CheckerType_CHECKER_TYPE_UNSPECIFIED,
		"standard":      problempb.CheckerType_CHECKER_TYPE_STANDARD,
		"special_judge": problempb.CheckerType_CHECKER_TYPE_SPECIAL_JUDGE,
		"interactive":   problempb.CheckerType_CHECKER_TYPE_INTERACTIVE,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return problempb.CheckerType_CHECKER_TYPE_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid checker type")
}

func ProblemEditable(value types.ProblemEditableFields) (*problempb.ProblemEditableFields, error) {
	difficulty, err := ProblemDifficulty(value.Difficulty)
	if err != nil {
		return nil, err
	}
	visibility, err := ProblemVisibility(value.Visibility)
	if err != nil {
		return nil, err
	}
	checkerType, err := CheckerType(value.JudgeSettings.CheckerType)
	if err != nil {
		return nil, err
	}
	return &problempb.ProblemEditableFields{
		Statement: problemStatementRequest(value.Statement), Difficulty: difficulty,
		Tags: append([]string(nil), value.Tags...), Source: value.Source,
		JudgeSettings: judgeSettingsRequest(value.JudgeSettings, checkerType), Visibility: visibility,
	}, nil
}

func ProblemPatch(value types.ProblemPatch, updateMask []string) (*problempb.ProblemEditableFields, error) {
	selected := maskPaths(updateMask)
	result := new(problempb.ProblemEditableFields)
	if value.Statement != nil && maskContainsPrefix(selected, "statement") {
		result.Statement = new(problempb.ProblemStatement)
		if maskSelected(selected, "statement", "statement.title") {
			result.Statement.Title = dereference(value.Statement.Title)
		}
		if maskSelected(selected, "statement", "statement.description") {
			result.Statement.Description = dereference(value.Statement.Description)
		}
		if maskSelected(selected, "statement", "statement.input_description") {
			result.Statement.InputDescription = dereference(value.Statement.InputDescription)
		}
		if maskSelected(selected, "statement", "statement.output_description") {
			result.Statement.OutputDescription = dereference(value.Statement.OutputDescription)
		}
		if maskSelected(selected, "statement", "statement.samples") {
			result.Statement.Samples = problemSampleRequests(value.Statement.Samples)
		}
		if maskSelected(selected, "statement", "statement.hint") {
			result.Statement.Hint = dereference(value.Statement.Hint)
		}
	}
	if _, ok := selected["difficulty"]; ok && value.Difficulty != nil {
		parsed, err := ProblemDifficulty(*value.Difficulty)
		if err != nil {
			return nil, err
		}
		result.Difficulty = parsed
	}
	if _, ok := selected["tags"]; ok {
		result.Tags = append([]string(nil), value.Tags...)
	}
	if _, ok := selected["source"]; ok {
		result.Source = dereference(value.Source)
	}
	if value.JudgeSettings != nil && maskContainsPrefix(selected, "judge_settings") {
		checkerType := problempb.CheckerType_CHECKER_TYPE_UNSPECIFIED
		if maskSelected(selected, "judge_settings", "judge_settings.checker_type") && value.JudgeSettings.CheckerType != nil {
			parsed, err := CheckerType(*value.JudgeSettings.CheckerType)
			if err != nil {
				return nil, err
			}
			checkerType = parsed
		}
		result.JudgeSettings = new(problempb.JudgeSettings)
		if maskSelected(selected, "judge_settings", "judge_settings.time_limit_ms") {
			result.JudgeSettings.TimeLimitMs = dereferenceInt32(value.JudgeSettings.TimeLimitMs)
		}
		if maskSelected(selected, "judge_settings", "judge_settings.memory_limit_mb") {
			result.JudgeSettings.MemoryLimitMb = dereferenceInt32(value.JudgeSettings.MemoryLimitMB)
		}
		if maskSelected(selected, "judge_settings", "judge_settings.allowed_languages") {
			result.JudgeSettings.AllowedLanguages = append([]string(nil), value.JudgeSettings.AllowedLanguages...)
		}
		if maskSelected(selected, "judge_settings", "judge_settings.checker_type") {
			result.JudgeSettings.CheckerType = checkerType
		}
	}
	if _, ok := selected["visibility"]; ok && value.Visibility != nil {
		parsed, err := ProblemVisibility(*value.Visibility)
		if err != nil {
			return nil, err
		}
		result.Visibility = parsed
	}
	return result, nil
}

func Problem(value *problempb.Problem) (types.Problem, error) {
	if value == nil {
		return types.Problem{}, InvalidRPCResponse("problem")
	}
	return types.Problem{
		ProblemID: value.ProblemId, Statement: ProblemStatement(value.Statement), Metadata: ProblemMetadata(value.Metadata),
		JudgeConfig: JudgeConfig(value.JudgeConfig),
		Status:      httpbridge.EnumName(value.Status.String(), "PROBLEM_STATUS_"),
		Visibility:  httpbridge.EnumName(value.Visibility.String(), "PROBLEM_VISIBILITY_"), Revision: value.Revision,
		CreatedAt: httpbridge.Timestamp(value.CreatedAt), UpdatedAt: httpbridge.Timestamp(value.UpdatedAt),
		PublishedAt: httpbridge.Timestamp(value.PublishedAt),
	}, nil
}

func ProblemStatement(value *problempb.ProblemStatement) types.ProblemStatement {
	if value == nil {
		return types.ProblemStatement{Samples: []types.ProblemSample{}}
	}
	samples := make([]types.ProblemSample, 0, len(value.Samples))
	for _, sample := range value.Samples {
		if sample != nil {
			samples = append(samples, types.ProblemSample{Input: sample.Input, Output: sample.Output, Explanation: sample.Explanation})
		}
	}
	return types.ProblemStatement{
		Title: value.Title, Description: value.Description, InputDescription: value.InputDescription,
		OutputDescription: value.OutputDescription, Samples: samples, Hint: value.Hint,
	}
}

func ProblemMetadata(value *problempb.ProblemMetadata) types.ProblemMetadata {
	if value == nil {
		return types.ProblemMetadata{Tags: []string{}}
	}
	return types.ProblemMetadata{
		Difficulty: httpbridge.EnumName(value.Difficulty.String(), "PROBLEM_DIFFICULTY_"),
		Tags:       cloneStrings(value.Tags), Source: value.Source, AuthorID: value.AuthorId,
	}
}

func JudgeSettings(value *problempb.JudgeSettings) types.JudgeSettings {
	if value == nil {
		return types.JudgeSettings{AllowedLanguages: []string{}}
	}
	return types.JudgeSettings{
		TimeLimitMs: value.TimeLimitMs, MemoryLimitMB: value.MemoryLimitMb,
		AllowedLanguages: cloneStrings(value.AllowedLanguages),
		CheckerType:      httpbridge.EnumName(value.CheckerType.String(), "CHECKER_TYPE_"),
	}
}

func JudgeConfig(value *problempb.JudgeConfig) types.JudgeConfig {
	if value == nil {
		return types.JudgeConfig{Settings: JudgeSettings(nil)}
	}
	return types.JudgeConfig{
		Settings: JudgeSettings(value.Settings), TestDataVersion: value.TestDataVersion,
		CheckerFileKey: value.CheckerFileKey, InteractorFileKey: value.InteractorFileKey,
	}
}

func ProblemSummary(value *problempb.ProblemSummary) types.ProblemSummary {
	if value == nil {
		return types.ProblemSummary{Tags: []string{}}
	}
	return types.ProblemSummary{
		ProblemID: value.ProblemId, Title: value.Title,
		Difficulty: httpbridge.EnumName(value.Difficulty.String(), "PROBLEM_DIFFICULTY_"),
		Tags:       cloneStrings(value.Tags), Status: httpbridge.EnumName(value.Status.String(), "PROBLEM_STATUS_"),
		Visibility: httpbridge.EnumName(value.Visibility.String(), "PROBLEM_VISIBILITY_"),
		AuthorID:   value.AuthorId, UpdatedAt: httpbridge.Timestamp(value.UpdatedAt), Revision: value.Revision,
	}
}

func TestCase(value *problempb.TestCaseInfo) types.TestCaseInfo {
	if value == nil {
		return types.TestCaseInfo{}
	}
	return types.TestCaseInfo{
		TestCaseID: value.TestCaseId, Name: value.Name, Score: value.Score,
		Ordinal: value.Ordinal, Enabled: value.Enabled, GroupName: value.GroupName,
	}
}

func ContestMode(value string) (contestpb.ContestMode, error) {
	values := map[string]contestpb.ContestMode{
		"":     contestpb.ContestMode_CONTEST_MODE_UNSPECIFIED,
		"icpc": contestpb.ContestMode_CONTEST_MODE_ICPC,
		"ioi":  contestpb.ContestMode_CONTEST_MODE_IOI,
		"oi":   contestpb.ContestMode_CONTEST_MODE_OI,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return contestpb.ContestMode_CONTEST_MODE_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid contest mode")
}

func ContestStatus(value string) (contestpb.ContestStatus, error) {
	values := map[string]contestpb.ContestStatus{
		"":      contestpb.ContestStatus_CONTEST_STATUS_UNSPECIFIED,
		"draft": contestpb.ContestStatus_CONTEST_STATUS_DRAFT, "published": contestpb.ContestStatus_CONTEST_STATUS_PUBLISHED,
		"running": contestpb.ContestStatus_CONTEST_STATUS_RUNNING, "finished": contestpb.ContestStatus_CONTEST_STATUS_FINISHED,
		"cancelled": contestpb.ContestStatus_CONTEST_STATUS_CANCELLED, "archived": contestpb.ContestStatus_CONTEST_STATUS_ARCHIVED,
		"deleted": contestpb.ContestStatus_CONTEST_STATUS_DELETED,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return contestpb.ContestStatus_CONTEST_STATUS_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid contest status")
}

func RegistrationPolicy(value string) (contestpb.RegistrationPolicy, error) {
	values := map[string]contestpb.RegistrationPolicy{
		"":                  contestpb.RegistrationPolicy_REGISTRATION_POLICY_UNSPECIFIED,
		"open":              contestpb.RegistrationPolicy_REGISTRATION_POLICY_OPEN,
		"approval_required": contestpb.RegistrationPolicy_REGISTRATION_POLICY_APPROVAL_REQUIRED,
		"invite_only":       contestpb.RegistrationPolicy_REGISTRATION_POLICY_INVITE_ONLY,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return contestpb.RegistrationPolicy_REGISTRATION_POLICY_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid registration policy")
}

func ContestVisibility(value string) (contestpb.ContestVisibility, error) {
	values := map[string]contestpb.ContestVisibility{
		"":         contestpb.ContestVisibility_CONTEST_VISIBILITY_UNSPECIFIED,
		"public":   contestpb.ContestVisibility_CONTEST_VISIBILITY_PUBLIC,
		"unlisted": contestpb.ContestVisibility_CONTEST_VISIBILITY_UNLISTED,
		"private":  contestpb.ContestVisibility_CONTEST_VISIBILITY_PRIVATE,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return contestpb.ContestVisibility_CONTEST_VISIBILITY_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid contest visibility")
}

func ScoreboardVisibility(value string) (contestpb.ScoreboardVisibility, error) {
	values := map[string]contestpb.ScoreboardVisibility{
		"":                  contestpb.ScoreboardVisibility_SCOREBOARD_VISIBILITY_UNSPECIFIED,
		"public":            contestpb.ScoreboardVisibility_SCOREBOARD_VISIBILITY_PUBLIC,
		"participants_only": contestpb.ScoreboardVisibility_SCOREBOARD_VISIBILITY_PARTICIPANTS_ONLY,
		"hidden":            contestpb.ScoreboardVisibility_SCOREBOARD_VISIBILITY_HIDDEN,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return contestpb.ScoreboardVisibility_SCOREBOARD_VISIBILITY_UNSPECIFIED,
		status.Error(codes.InvalidArgument, "invalid scoreboard visibility")
}

func RegistrationStatus(value string) (contestpb.RegistrationStatus, error) {
	values := map[string]contestpb.RegistrationStatus{
		"":          contestpb.RegistrationStatus_REGISTRATION_STATUS_UNSPECIFIED,
		"pending":   contestpb.RegistrationStatus_REGISTRATION_STATUS_PENDING,
		"approved":  contestpb.RegistrationStatus_REGISTRATION_STATUS_APPROVED,
		"rejected":  contestpb.RegistrationStatus_REGISTRATION_STATUS_REJECTED,
		"cancelled": contestpb.RegistrationStatus_REGISTRATION_STATUS_CANCELLED,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return contestpb.RegistrationStatus_REGISTRATION_STATUS_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid registration status")
}

func ParticipantStatus(value string) (contestpb.ParticipantStatus, error) {
	values := map[string]contestpb.ParticipantStatus{
		"":             contestpb.ParticipantStatus_PARTICIPANT_STATUS_UNSPECIFIED,
		"normal":       contestpb.ParticipantStatus_PARTICIPANT_STATUS_NORMAL,
		"disqualified": contestpb.ParticipantStatus_PARTICIPANT_STATUS_DISQUALIFIED,
		"withdrawn":    contestpb.ParticipantStatus_PARTICIPANT_STATUS_WITHDRAWN,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return contestpb.ParticipantStatus_PARTICIPANT_STATUS_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid participant status")
}

func ReviewDecision(value string) (contestpb.RegistrationReviewDecision, error) {
	values := map[string]contestpb.RegistrationReviewDecision{
		"approve": contestpb.RegistrationReviewDecision_REGISTRATION_REVIEW_DECISION_APPROVE,
		"reject":  contestpb.RegistrationReviewDecision_REGISTRATION_REVIEW_DECISION_REJECT,
	}
	if parsed, ok := values[normalize(value)]; ok {
		return parsed, nil
	}
	return contestpb.RegistrationReviewDecision_REGISTRATION_REVIEW_DECISION_UNSPECIFIED,
		status.Error(codes.InvalidArgument, "invalid registration review decision")
}

func ContestEditable(value types.ContestEditableFields) (*contestpb.ContestEditableFields, error) {
	mode, err := ContestMode(value.Mode)
	if err != nil {
		return nil, err
	}
	startAt, err := httpbridge.ParseTimestamp(value.StartAt, "start_at")
	if err != nil {
		return nil, err
	}
	endAt, err := httpbridge.ParseTimestamp(value.EndAt, "end_at")
	if err != nil {
		return nil, err
	}
	registrationStartAt, err := httpbridge.ParseOptionalTimestamp(value.RegistrationStartAt, "registration_start_at")
	if err != nil {
		return nil, err
	}
	registrationEndAt, err := httpbridge.ParseOptionalTimestamp(value.RegistrationEndAt, "registration_end_at")
	if err != nil {
		return nil, err
	}
	policy, err := RegistrationPolicy(value.RegistrationPolicy)
	if err != nil {
		return nil, err
	}
	visibility, err := ContestVisibility(value.Visibility)
	if err != nil {
		return nil, err
	}
	scoreboardVisibility, err := ScoreboardVisibility(value.ScoreboardVisibility)
	if err != nil {
		return nil, err
	}
	result := &contestpb.ContestEditableFields{
		Title: value.Title, Description: value.Description, Mode: mode, StartAt: startAt, EndAt: endAt,
		RegistrationStartAt: registrationStartAt, RegistrationEndAt: registrationEndAt,
		RegistrationPolicy: policy, Visibility: visibility, ScoreboardVisibility: scoreboardVisibility,
		InviteCode: cloneString(value.InviteCode),
	}
	if err := setFreezePolicy(result, value.ScoreboardFreezeAt, value.ScoreboardFreezeBeforeEndSec); err != nil {
		return nil, err
	}
	return result, nil
}

func ContestPatch(value types.ContestPatch, updateMask []string) (*contestpb.ContestEditableFields, error) {
	selected := maskPaths(updateMask)
	result := new(contestpb.ContestEditableFields)
	var err error
	if _, ok := selected["title"]; ok {
		result.Title = dereference(value.Title)
	}
	if _, ok := selected["description"]; ok {
		result.Description = dereference(value.Description)
	}
	if _, ok := selected["mode"]; ok && value.Mode != nil {
		result.Mode, err = ContestMode(*value.Mode)
		if err != nil {
			return nil, err
		}
	}
	if _, ok := selected["start_at"]; ok && value.StartAt != nil {
		result.StartAt, err = httpbridge.ParseTimestamp(*value.StartAt, "start_at")
		if err != nil {
			return nil, err
		}
	}
	if _, ok := selected["end_at"]; ok && value.EndAt != nil {
		result.EndAt, err = httpbridge.ParseTimestamp(*value.EndAt, "end_at")
		if err != nil {
			return nil, err
		}
	}
	if _, ok := selected["registration_start_at"]; ok {
		result.RegistrationStartAt, err = httpbridge.ParseOptionalTimestamp(value.RegistrationStartAt, "registration_start_at")
		if err != nil {
			return nil, err
		}
	}
	if _, ok := selected["registration_end_at"]; ok {
		result.RegistrationEndAt, err = httpbridge.ParseOptionalTimestamp(value.RegistrationEndAt, "registration_end_at")
		if err != nil {
			return nil, err
		}
	}
	if _, ok := selected["registration_policy"]; ok && value.RegistrationPolicy != nil {
		result.RegistrationPolicy, err = RegistrationPolicy(*value.RegistrationPolicy)
		if err != nil {
			return nil, err
		}
	}
	if _, ok := selected["visibility"]; ok && value.Visibility != nil {
		result.Visibility, err = ContestVisibility(*value.Visibility)
		if err != nil {
			return nil, err
		}
	}
	if _, ok := selected["scoreboard_visibility"]; ok && value.ScoreboardVisibility != nil {
		result.ScoreboardVisibility, err = ScoreboardVisibility(*value.ScoreboardVisibility)
		if err != nil {
			return nil, err
		}
	}
	var freezeAt *string
	if _, ok := selected["scoreboard_freeze_at"]; ok {
		freezeAt = value.ScoreboardFreezeAt
	}
	var freezeBeforeEnd *int64
	if _, ok := selected["scoreboard_freeze_before_end"]; ok {
		freezeBeforeEnd = value.ScoreboardFreezeBeforeEndSec
	}
	if freezeAt != nil || freezeBeforeEnd != nil {
		if err := setFreezePolicy(result, freezeAt, freezeBeforeEnd); err != nil {
			return nil, err
		}
	}
	if _, ok := selected["invite_code"]; ok {
		result.InviteCode = cloneString(value.InviteCode)
	}
	return result, nil
}

// ContestUpdateMask 把 HTTP DTO 字段名转换为 Contest Proto 使用的规范路径。
func ContestUpdateMask(paths []string) []string {
	result := append([]string(nil), paths...)
	for index, path := range result {
		if path == "scoreboard_freeze_before_end_seconds" {
			result[index] = "scoreboard_freeze_before_end"
		}
	}
	return result
}

func Contest(value *contestpb.Contest) (types.Contest, error) {
	if value == nil {
		return types.Contest{}, InvalidRPCResponse("contest")
	}
	return types.Contest{
		ContestID: value.ContestId, CreatorID: value.CreatorId, Editable: ContestEditableView(value.Editable),
		Status: httpbridge.EnumName(value.Status.String(), "CONTEST_STATUS_"), Revision: value.Revision,
		CreatedAt: httpbridge.Timestamp(value.CreatedAt), UpdatedAt: httpbridge.Timestamp(value.UpdatedAt),
		PublishedAt: httpbridge.Timestamp(value.PublishedAt), CancelledAt: httpbridge.Timestamp(value.CancelledAt),
		ArchivedAt: httpbridge.Timestamp(value.ArchivedAt), DeletedAt: httpbridge.Timestamp(value.DeletedAt),
		ScoreboardFrozen: value.ScoreboardFrozen, ScoreboardFrozenAt: httpbridge.Timestamp(value.ScoreboardFrozenAt),
	}, nil
}

func ContestEditableView(value *contestpb.ContestEditableFields) types.ContestEditableFields {
	if value == nil {
		return types.ContestEditableFields{}
	}
	result := types.ContestEditableFields{
		Title: value.Title, Description: value.Description, Mode: httpbridge.EnumName(value.Mode.String(), "CONTEST_MODE_"),
		StartAt: valueOrEmpty(httpbridge.Timestamp(value.StartAt)), EndAt: valueOrEmpty(httpbridge.Timestamp(value.EndAt)),
		RegistrationStartAt:  httpbridge.Timestamp(value.RegistrationStartAt),
		RegistrationEndAt:    httpbridge.Timestamp(value.RegistrationEndAt),
		RegistrationPolicy:   httpbridge.EnumName(value.RegistrationPolicy.String(), "REGISTRATION_POLICY_"),
		Visibility:           httpbridge.EnumName(value.Visibility.String(), "CONTEST_VISIBILITY_"),
		ScoreboardVisibility: httpbridge.EnumName(value.ScoreboardVisibility.String(), "SCOREBOARD_VISIBILITY_"),
		InviteCode:           cloneString(value.InviteCode),
	}
	if freezeAt := value.GetScoreboardFreezeAt(); freezeAt != nil {
		result.ScoreboardFreezeAt = httpbridge.Timestamp(freezeAt)
	}
	if beforeEnd := value.GetScoreboardFreezeBeforeEnd(); beforeEnd != nil {
		seconds := httpbridge.DurationSeconds(beforeEnd)
		result.ScoreboardFreezeBeforeEndSec = &seconds
	}
	return result
}

func ContestSummary(value *contestpb.ContestSummary) types.ContestSummary {
	if value == nil {
		return types.ContestSummary{}
	}
	return types.ContestSummary{
		ContestID: value.ContestId, Title: value.Title, Mode: httpbridge.EnumName(value.Mode.String(), "CONTEST_MODE_"),
		Status: httpbridge.EnumName(value.Status.String(), "CONTEST_STATUS_"), StartAt: httpbridge.Timestamp(value.StartAt),
		EndAt:              httpbridge.Timestamp(value.EndAt),
		RegistrationPolicy: httpbridge.EnumName(value.RegistrationPolicy.String(), "REGISTRATION_POLICY_"),
		Visibility:         httpbridge.EnumName(value.Visibility.String(), "CONTEST_VISIBILITY_"),
		RegistrationOpen:   value.RegistrationOpen, ParticipantCount: value.ParticipantCount,
	}
}

func ContestProblem(value *contestpb.ContestProblem) types.ContestProblem {
	if value == nil {
		return types.ContestProblem{}
	}
	return types.ContestProblem{
		ProblemID: value.ProblemId, Label: value.Label, DisplayOrder: value.DisplayOrder, MaxScore: value.MaxScore,
	}
}

func ContestProblemRequest(value types.ContestProblem) *contestpb.ContestProblem {
	return &contestpb.ContestProblem{
		ProblemId: value.ProblemID, Label: value.Label, DisplayOrder: value.DisplayOrder, MaxScore: value.MaxScore,
	}
}

func ContestRegistration(value *contestpb.ContestRegistration) (types.ContestRegistration, error) {
	if value == nil {
		return types.ContestRegistration{}, InvalidRPCResponse("registration")
	}
	return types.ContestRegistration{
		RegistrationID: value.RegistrationId, ContestID: value.ContestId, UserID: value.UserId,
		Status:          httpbridge.EnumName(value.Status.String(), "REGISTRATION_STATUS_"),
		ApplicationNote: value.ApplicationNote, ReviewerID: value.ReviewerId, ReviewNote: value.ReviewNote,
		Revision: value.Revision, CreatedAt: httpbridge.Timestamp(value.CreatedAt),
		ReviewedAt: httpbridge.Timestamp(value.ReviewedAt), CancelledAt: httpbridge.Timestamp(value.CancelledAt),
		UpdatedAt: httpbridge.Timestamp(value.UpdatedAt),
	}, nil
}

func ContestParticipant(value *contestpb.ContestParticipant) *types.ContestParticipant {
	if value == nil {
		return nil
	}
	return &types.ContestParticipant{
		ContestID: value.ContestId, UserID: value.UserId,
		Status:      httpbridge.EnumName(value.Status.String(), "PARTICIPANT_STATUS_"),
		QualifiedAt: httpbridge.Timestamp(value.QualifiedAt), UpdatedAt: httpbridge.Timestamp(value.UpdatedAt),
	}
}

func SubmissionStatus(value string) (*submitpb.SubmissionStatus, error) {
	if normalize(value) == "" {
		return nil, nil
	}
	values := map[string]submitpb.SubmissionStatus{
		"queued":   submitpb.SubmissionStatus_SUBMISSION_STATUS_QUEUED,
		"judging":  submitpb.SubmissionStatus_SUBMISSION_STATUS_JUDGING,
		"finished": submitpb.SubmissionStatus_SUBMISSION_STATUS_FINISHED,
	}
	parsed, ok := values[normalize(value)]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid submission status")
	}
	return &parsed, nil
}

func Verdict(value string) (*submitpb.Verdict, error) {
	if normalize(value) == "" {
		return nil, nil
	}
	values := map[string]submitpb.Verdict{
		"accepted":              submitpb.Verdict_VERDICT_ACCEPTED,
		"partial_accepted":      submitpb.Verdict_VERDICT_PARTIAL_ACCEPTED,
		"wrong_answer":          submitpb.Verdict_VERDICT_WRONG_ANSWER,
		"time_limit_exceeded":   submitpb.Verdict_VERDICT_TIME_LIMIT_EXCEEDED,
		"memory_limit_exceeded": submitpb.Verdict_VERDICT_MEMORY_LIMIT_EXCEEDED,
		"runtime_error":         submitpb.Verdict_VERDICT_RUNTIME_ERROR,
		"compile_error":         submitpb.Verdict_VERDICT_COMPILE_ERROR,
		"output_limit_exceeded": submitpb.Verdict_VERDICT_OUTPUT_LIMIT_EXCEEDED,
		"system_error":          submitpb.Verdict_VERDICT_SYSTEM_ERROR,
	}
	parsed, ok := values[normalize(value)]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid verdict")
	}
	return &parsed, nil
}

func SubmissionSummary(value *submitpb.SubmissionSummary) types.SubmissionSummary {
	if value == nil {
		return types.SubmissionSummary{}
	}
	return types.SubmissionSummary{
		SubmissionID: value.SubmissionId, UserID: value.UserId, ProblemID: value.ProblemId,
		ContestID: cloneInt64(value.ContestId), Language: value.Language,
		Status:  httpbridge.EnumName(value.Status.String(), "SUBMISSION_STATUS_"),
		Verdict: httpbridge.EnumName(value.Verdict.String(), "VERDICT_"),
		Score:   value.Score, MaxScore: value.MaxScore, ProblemRevision: value.ProblemRevision,
		AttemptCount:     value.AttemptCount,
		ResultVisibility: httpbridge.EnumName(value.ResultVisibility.String(), "RESULT_VISIBILITY_"),
		SubmittedAt:      httpbridge.Timestamp(value.SubmittedAt), JudgingStartedAt: httpbridge.Timestamp(value.JudgingStartedAt),
		FinishedAt: httpbridge.Timestamp(value.FinishedAt),
	}
}

func SubmissionDetail(value *submitpb.SubmissionDetail) (types.SubmissionDetail, error) {
	if value == nil || value.Summary == nil {
		return types.SubmissionDetail{}, InvalidRPCResponse("submission")
	}
	caseResults := make([]types.TestCaseResult, 0, len(value.CaseResults))
	for _, result := range value.CaseResults {
		if result != nil {
			caseResults = append(caseResults, types.TestCaseResult{
				TestCaseID: result.TestCaseId, Ordinal: result.Ordinal, GroupName: result.GroupName,
				Verdict: httpbridge.EnumName(result.Verdict.String(), "VERDICT_"), Score: result.Score,
				TimeUsedMs: result.TimeUsedMs, MemoryUsedKB: result.MemoryUsedKb,
			})
		}
	}
	return types.SubmissionDetail{
		Summary: SubmissionSummary(value.Summary), SourceCode: value.SourceCode,
		CompileOutput: value.CompileOutput, CaseResults: caseResults,
	}, nil
}

func problemStatementRequest(value types.ProblemStatement) *problempb.ProblemStatement {
	return &problempb.ProblemStatement{
		Title: value.Title, Description: value.Description, InputDescription: value.InputDescription,
		OutputDescription: value.OutputDescription, Samples: problemSampleRequests(value.Samples), Hint: value.Hint,
	}
}

func problemSampleRequests(values []types.ProblemSample) []*problempb.ProblemSample {
	result := make([]*problempb.ProblemSample, 0, len(values))
	for _, value := range values {
		result = append(result, &problempb.ProblemSample{Input: value.Input, Output: value.Output, Explanation: value.Explanation})
	}
	return result
}

func judgeSettingsRequest(value types.JudgeSettings, checkerType problempb.CheckerType) *problempb.JudgeSettings {
	return &problempb.JudgeSettings{
		TimeLimitMs: value.TimeLimitMs, MemoryLimitMb: value.MemoryLimitMB,
		AllowedLanguages: append([]string(nil), value.AllowedLanguages...), CheckerType: checkerType,
	}
}

func setFreezePolicy(target *contestpb.ContestEditableFields, freezeAt *string, beforeEndSeconds *int64) error {
	if freezeAt != nil && beforeEndSeconds != nil {
		return status.Error(codes.InvalidArgument, "scoreboard freeze policies are mutually exclusive")
	}
	if freezeAt != nil {
		parsed, err := httpbridge.ParseTimestamp(*freezeAt, "scoreboard_freeze_at")
		if err != nil {
			return err
		}
		target.ScoreboardFreezePolicy = &contestpb.ContestEditableFields_ScoreboardFreezeAt{ScoreboardFreezeAt: parsed}
	}
	if beforeEndSeconds != nil {
		const maxDurationSeconds = int64((time.Duration(1<<63 - 1)) / time.Second)
		if *beforeEndSeconds <= 0 || *beforeEndSeconds > maxDurationSeconds {
			return status.Error(codes.InvalidArgument, "scoreboard_freeze_before_end_seconds is invalid")
		}
		duration := durationpb.New(time.Duration(*beforeEndSeconds) * time.Second)
		if err := duration.CheckValid(); err != nil {
			return status.Error(codes.InvalidArgument, "scoreboard_freeze_before_end_seconds is invalid")
		}
		target.ScoreboardFreezePolicy = &contestpb.ContestEditableFields_ScoreboardFreezeBeforeEnd{
			ScoreboardFreezeBeforeEnd: duration,
		}
	}
	return nil
}

func normalize(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func dereference(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func dereferenceInt32(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneStrings(values []string) []string {
	return append([]string{}, values...)
}

func maskPaths(paths []string) map[string]struct{} {
	result := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		result[path] = struct{}{}
	}
	return result
}

func maskContainsPrefix(paths map[string]struct{}, prefix string) bool {
	if _, ok := paths[prefix]; ok {
		return true
	}
	for path := range paths {
		if strings.HasPrefix(path, prefix+".") {
			return true
		}
	}
	return false
}

func maskSelected(paths map[string]struct{}, parent, child string) bool {
	if _, ok := paths[parent]; ok {
		return true
	}
	_, ok := paths[child]
	return ok
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
