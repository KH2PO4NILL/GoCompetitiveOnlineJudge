package mapper

import (
	"strings"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/internal/httpbridge"
	authpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	contestpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	submitpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func InvalidRPCResponse(field string) error {
	return status.Errorf(codes.Unavailable, "upstream RPC response is missing %s", field)
}

func EmailCodePurpose(value string) (authpb.EmailCodePurpose, error) {
	values := map[string]authpb.EmailCodePurpose{
		"register":       authpb.EmailCodePurpose_EMAIL_CODE_PURPOSE_REGISTER,
		"login":          authpb.EmailCodePurpose_EMAIL_CODE_PURPOSE_LOGIN,
		"change_email":   authpb.EmailCodePurpose_EMAIL_CODE_PURPOSE_CHANGE_EMAIL,
		"reset_password": authpb.EmailCodePurpose_EMAIL_CODE_PURPOSE_RESET_PASSWORD,
	}
	if parsed, ok := values[strings.ToLower(strings.TrimSpace(value))]; ok {
		return parsed, nil
	}
	return authpb.EmailCodePurpose_EMAIL_CODE_PURPOSE_UNSPECIFIED,
		status.Error(codes.InvalidArgument, "invalid email code purpose")
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
	return types.TokenPair{
		AccessToken: value.AccessToken, RefreshToken: value.RefreshToken, ExpiresIn: value.ExpiresIn,
	}, nil
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
	if parsed, ok := values[strings.ToLower(strings.TrimSpace(value))]; ok {
		return parsed, nil
	}
	return problempb.ProblemDifficulty_PROBLEM_DIFFICULTY_UNSPECIFIED,
		status.Error(codes.InvalidArgument, "invalid problem difficulty")
}

func ProblemStatement(value *problempb.ProblemStatement) types.ProblemStatement {
	if value == nil {
		return types.ProblemStatement{Samples: []types.ProblemSample{}}
	}
	samples := make([]types.ProblemSample, 0, len(value.Samples))
	for _, sample := range value.Samples {
		if sample != nil {
			samples = append(samples, types.ProblemSample{
				Input: sample.Input, Output: sample.Output, Explanation: sample.Explanation,
			})
		}
	}
	return types.ProblemStatement{
		Title: value.Title, Description: value.Description,
		InputDescription: value.InputDescription, OutputDescription: value.OutputDescription,
		Samples: samples, Hint: value.Hint,
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

func PracticeProblem(value *problempb.PracticeProblemInfo) (types.PracticeProblemInfo, error) {
	if value == nil {
		return types.PracticeProblemInfo{}, InvalidRPCResponse("problem")
	}
	return types.PracticeProblemInfo{
		ProblemID: value.ProblemId, Statement: ProblemStatement(value.Statement),
		Metadata: ProblemMetadata(value.Metadata), JudgeSettings: JudgeSettings(value.JudgeSettings),
		Revision: value.Revision,
	}, nil
}

func ProblemSummary(value *problempb.ProblemSummary) types.ProblemSummary {
	if value == nil {
		return types.ProblemSummary{Tags: []string{}}
	}
	return types.ProblemSummary{
		ProblemID: value.ProblemId, Title: value.Title,
		Difficulty: httpbridge.EnumName(value.Difficulty.String(), "PROBLEM_DIFFICULTY_"),
		Tags:       cloneStrings(value.Tags),
		Status:     httpbridge.EnumName(value.Status.String(), "PROBLEM_STATUS_"),
		Visibility: httpbridge.EnumName(value.Visibility.String(), "PROBLEM_VISIBILITY_"),
		AuthorID:   value.AuthorId, UpdatedAt: httpbridge.Timestamp(value.UpdatedAt), Revision: value.Revision,
	}
}

func ContestStatus(value string) (contestpb.ContestStatus, error) {
	values := map[string]contestpb.ContestStatus{
		"":      contestpb.ContestStatus_CONTEST_STATUS_UNSPECIFIED,
		"draft": contestpb.ContestStatus_CONTEST_STATUS_DRAFT, "published": contestpb.ContestStatus_CONTEST_STATUS_PUBLISHED,
		"running": contestpb.ContestStatus_CONTEST_STATUS_RUNNING, "finished": contestpb.ContestStatus_CONTEST_STATUS_FINISHED,
		"cancelled": contestpb.ContestStatus_CONTEST_STATUS_CANCELLED, "archived": contestpb.ContestStatus_CONTEST_STATUS_ARCHIVED,
		"deleted": contestpb.ContestStatus_CONTEST_STATUS_DELETED,
	}
	if parsed, ok := values[strings.ToLower(strings.TrimSpace(value))]; ok {
		return parsed, nil
	}
	return contestpb.ContestStatus_CONTEST_STATUS_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid contest status")
}

func ContestMode(value string) (contestpb.ContestMode, error) {
	values := map[string]contestpb.ContestMode{
		"":     contestpb.ContestMode_CONTEST_MODE_UNSPECIFIED,
		"icpc": contestpb.ContestMode_CONTEST_MODE_ICPC,
		"ioi":  contestpb.ContestMode_CONTEST_MODE_IOI,
		"oi":   contestpb.ContestMode_CONTEST_MODE_OI,
	}
	if parsed, ok := values[strings.ToLower(strings.TrimSpace(value))]; ok {
		return parsed, nil
	}
	return contestpb.ContestMode_CONTEST_MODE_UNSPECIFIED, status.Error(codes.InvalidArgument, "invalid contest mode")
}

func ContestSummary(value *contestpb.ContestSummary) types.ContestSummary {
	if value == nil {
		return types.ContestSummary{}
	}
	return types.ContestSummary{
		ContestID: value.ContestId, Title: value.Title,
		Mode:    httpbridge.EnumName(value.Mode.String(), "CONTEST_MODE_"),
		Status:  httpbridge.EnumName(value.Status.String(), "CONTEST_STATUS_"),
		StartAt: httpbridge.Timestamp(value.StartAt), EndAt: httpbridge.Timestamp(value.EndAt),
		RegistrationPolicy: httpbridge.EnumName(value.RegistrationPolicy.String(), "REGISTRATION_POLICY_"),
		Visibility:         httpbridge.EnumName(value.Visibility.String(), "CONTEST_VISIBILITY_"),
		RegistrationOpen:   value.RegistrationOpen, ParticipantCount: value.ParticipantCount,
	}
}

func ContestDetail(value *contestpb.ContestDetail) (types.ContestDetail, error) {
	if value == nil {
		return types.ContestDetail{}, InvalidRPCResponse("contest")
	}
	return types.ContestDetail{
		ContestID: value.ContestId, Title: value.Title, Description: value.Description,
		Mode:    httpbridge.EnumName(value.Mode.String(), "CONTEST_MODE_"),
		Status:  httpbridge.EnumName(value.Status.String(), "CONTEST_STATUS_"),
		StartAt: httpbridge.Timestamp(value.StartAt), EndAt: httpbridge.Timestamp(value.EndAt),
		RegistrationStartAt:  httpbridge.Timestamp(value.RegistrationStartAt),
		RegistrationEndAt:    httpbridge.Timestamp(value.RegistrationEndAt),
		RegistrationPolicy:   httpbridge.EnumName(value.RegistrationPolicy.String(), "REGISTRATION_POLICY_"),
		Visibility:           httpbridge.EnumName(value.Visibility.String(), "CONTEST_VISIBILITY_"),
		ScoreboardVisibility: httpbridge.EnumName(value.ScoreboardVisibility.String(), "SCOREBOARD_VISIBILITY_"),
		RegistrationOpen:     value.RegistrationOpen, ScoreboardFrozen: value.ScoreboardFrozen,
		ParticipantCount: value.ParticipantCount,
	}, nil
}

func ContestProblemSummary(value *contestpb.ContestProblemSummary) types.ContestProblemSummary {
	if value == nil {
		return types.ContestProblemSummary{}
	}
	return types.ContestProblemSummary{
		ProblemID: value.ProblemId, Label: value.Label, DisplayOrder: value.DisplayOrder, MaxScore: value.MaxScore,
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

func MyContestStatus(value *contestpb.MyContestStatus) (types.MyContestStatus, error) {
	if value == nil {
		return types.MyContestStatus{}, InvalidRPCResponse("contest status")
	}
	result := types.MyContestStatus{
		ContestID: value.ContestId, Participant: ContestParticipant(value.Participant),
		CanViewProblems: value.CanViewProblems, CanSubmit: value.CanSubmit,
	}
	if value.Registration != nil {
		registration, err := ContestRegistration(value.Registration)
		if err != nil {
			return types.MyContestStatus{}, err
		}
		result.Registration = &registration
	}
	return result, nil
}

func Scoreboard(value *contestpb.Scoreboard) (types.Scoreboard, error) {
	if value == nil {
		return types.Scoreboard{}, InvalidRPCResponse("scoreboard")
	}
	rows := make([]types.ScoreboardRow, 0, len(value.Rows))
	for _, row := range value.Rows {
		if row != nil {
			rows = append(rows, scoreboardRow(row))
		}
	}
	return types.Scoreboard{
		ContestID: value.ContestId, Mode: httpbridge.EnumName(value.Mode.String(), "CONTEST_MODE_"),
		GeneratedAt: httpbridge.Timestamp(value.GeneratedAt), Frozen: value.Frozen,
		View: httpbridge.EnumName(value.View.String(), "SCOREBOARD_VIEW_"), Total: value.Total, Rows: rows,
	}, nil
}

func scoreboardRow(value *contestpb.ScoreboardRow) types.ScoreboardRow {
	results := make([]types.ScoreboardProblemResult, 0, len(value.ProblemResults))
	for _, result := range value.ProblemResults {
		if result != nil {
			results = append(results, scoreboardProblemResult(result))
		}
	}
	row := types.ScoreboardRow{
		Rank: value.Rank, Ordinal: value.Ordinal, UserID: value.UserId,
		DisplayName: value.DisplayName, ProblemResults: results,
	}
	if score := value.GetIcpc(); score != nil {
		row.Icpc = &types.IcpcScore{SolvedCount: score.SolvedCount,
			TotalPenaltySeconds: httpbridge.DurationSeconds(score.TotalPenalty),
			LastSolvedAt:        httpbridge.Timestamp(score.LastSolvedAt)}
	}
	if score := value.GetIoi(); score != nil {
		row.Ioi = &types.IoiScore{TotalScore: score.TotalScore,
			LastEffectiveSubmissionAt: httpbridge.Timestamp(score.LastEffectiveSubmissionAt)}
	}
	if score := value.GetOi(); score != nil {
		row.Oi = &types.OiScore{TotalScore: score.TotalScore,
			LastEffectiveSubmissionAt: httpbridge.Timestamp(score.LastEffectiveSubmissionAt)}
	}
	return row
}

func scoreboardProblemResult(value *contestpb.ScoreboardProblemResult) types.ScoreboardProblemResult {
	result := types.ScoreboardProblemResult{ProblemID: value.ProblemId, Label: value.Label}
	if item := value.GetIcpc(); item != nil {
		result.Icpc = &types.IcpcProblemResult{
			Solved: item.Solved, WrongAttempts: item.WrongAttempts,
			PenaltySeconds: httpbridge.DurationSeconds(item.Penalty),
			AcceptedAt:     httpbridge.Timestamp(item.AcceptedAt), FirstToSolve: item.FirstToSolve,
		}
	}
	if item := value.GetIoi(); item != nil {
		result.Ioi = &types.IoiProblemResult{
			Score: item.Score, MaxScore: item.MaxScore, SubmissionCount: item.SubmissionCount,
			LastEffectiveSubmissionAt: httpbridge.Timestamp(item.LastEffectiveSubmissionAt),
		}
	}
	if item := value.GetOi(); item != nil {
		result.Oi = &types.OiProblemResult{
			Score: item.Score, MaxScore: item.MaxScore, SubmissionCount: item.SubmissionCount,
			LastEffectiveSubmissionAt: httpbridge.Timestamp(item.LastEffectiveSubmissionAt),
		}
	}
	return result
}

func SubmissionStatus(value string) (*submitpb.SubmissionStatus, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	values := map[string]submitpb.SubmissionStatus{
		"queued":   submitpb.SubmissionStatus_SUBMISSION_STATUS_QUEUED,
		"judging":  submitpb.SubmissionStatus_SUBMISSION_STATUS_JUDGING,
		"finished": submitpb.SubmissionStatus_SUBMISSION_STATUS_FINISHED,
	}
	parsed, ok := values[strings.ToLower(strings.TrimSpace(value))]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid submission status")
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
		SubmittedAt:      httpbridge.Timestamp(value.SubmittedAt),
		JudgingStartedAt: httpbridge.Timestamp(value.JudgingStartedAt), FinishedAt: httpbridge.Timestamp(value.FinishedAt),
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
				Verdict: httpbridge.EnumName(result.Verdict.String(), "VERDICT_"),
				Score:   result.Score, TimeUsedMs: result.TimeUsedMs, MemoryUsedKB: result.MemoryUsedKb,
			})
		}
	}
	return types.SubmissionDetail{
		Summary: SubmissionSummary(value.Summary), SourceCode: value.SourceCode,
		CompileOutput: value.CompileOutput, CaseResults: caseResults,
	}, nil
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
