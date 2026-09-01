package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	infraauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	authpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultContestPageSize = 20
	maxContestPageSize     = 100
	maxContestProblemBatch = 100
	maxContestKeywordRunes = 200
)

func nowUTC(svcCtx *svc.ServiceContext) time.Time {
	if svcCtx != nil && svcCtx.Now != nil {
		return svcCtx.Now().UTC()
	}
	return time.Now().UTC()
}

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

func optionalPrincipal(ctx context.Context) (infraauth.Principal, bool, error) {
	_, ok := infraauth.PrincipalFromContext(ctx)
	if !ok {
		return infraauth.Principal{}, false, nil
	}
	validated, err := requirePrincipal(ctx)
	return validated, true, err
}

func isAdmin(principal infraauth.Principal) bool {
	return principal.Role == authpb.UserRole_USER_ROLE_ADMIN ||
		principal.Role == authpb.UserRole_USER_ROLE_SUPER_ADMIN
}

func requireService(ctx context.Context, expected string) error {
	principal, ok := infraauth.ServicePrincipalFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "trusted service identity required")
	}
	if principal.Name != expected {
		return status.Error(codes.PermissionDenied, "service caller is not allowed")
	}
	return nil
}

func requireContestModel(svcCtx *svc.ServiceContext) (model.ContestModel, error) {
	if svcCtx == nil || svcCtx.ContestModel == nil {
		return nil, status.Error(codes.Internal, "contest model is unavailable")
	}
	return svcCtx.ContestModel, nil
}

func authorizeContest(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	contestID int64,
) (infraauth.Principal, *model.ContestAggregate, error) {
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return infraauth.Principal{}, nil, err
	}
	repository, err := requireContestModel(svcCtx)
	if err != nil {
		return infraauth.Principal{}, nil, err
	}
	contest, err := repository.GetForEdit(ctx, contestID)
	if err != nil {
		return infraauth.Principal{}, nil, mapModelError(err)
	}
	if contest == nil {
		return infraauth.Principal{}, nil, status.Error(codes.Internal, "contest model returned an empty response")
	}
	if contest.CreatorID != principal.UserID && !isAdmin(principal) {
		return infraauth.Principal{}, nil, status.Error(codes.PermissionDenied, "contest permission denied")
	}
	return principal, contest, nil
}

func mapModelError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "contest operation was canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "contest operation timed out")
	case errors.Is(err, model.ErrInvalidContestInput):
		return status.Error(codes.InvalidArgument, "invalid contest input")
	case errors.Is(err, model.ErrContestNotFound),
		errors.Is(err, model.ErrRegistrationNotFound),
		errors.Is(err, model.ErrParticipantNotFound):
		return status.Error(codes.NotFound, "contest resource not found")
	case errors.Is(err, model.ErrContestRevisionConflict),
		errors.Is(err, model.ErrRegistrationRevisionConflict):
		return status.Error(codes.Aborted, "contest revision conflict")
	case errors.Is(err, model.ErrContestProblemConflict),
		errors.Is(err, model.ErrRegistrationAlreadyExists):
		return status.Error(codes.AlreadyExists, "contest resource already exists")
	case errors.Is(err, model.ErrInvalidInviteCode):
		return status.Error(codes.PermissionDenied, "invalid contest invitation")
	case errors.Is(err, model.ErrContestStateConflict),
		errors.Is(err, model.ErrContestAlreadyDeleted),
		errors.Is(err, model.ErrContestNotDeleted),
		errors.Is(err, model.ErrContestNotPublishable),
		errors.Is(err, model.ErrRegistrationStateConflict),
		errors.Is(err, model.ErrRegistrationClosed),
		errors.Is(err, model.ErrParticipantStateConflict),
		errors.Is(err, model.ErrScoreboardNotReady),
		errors.Is(err, model.ErrScoreboardVersionConflict),
		errors.Is(err, model.ErrScoreboardAlreadyFrozen),
		errors.Is(err, model.ErrScoreboardNotFrozen),
		errors.Is(err, model.ErrScoreboardSnapshotConflict):
		return status.Error(codes.FailedPrecondition, "contest state does not allow this operation")
	default:
		return status.Error(codes.Internal, "contest service internal error")
	}
}

func mapDependencyError(err error, dependency string) error {
	if errors.Is(err, context.Canceled) {
		return status.Errorf(codes.Canceled, "%s request was canceled", dependency)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Errorf(codes.DeadlineExceeded, "%s request timed out", dependency)
	}
	return status.Errorf(codes.Unavailable, "%s is unavailable", dependency)
}

func normalizePagination(page, pageSize int32) (int, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultContestPageSize
	}
	if pageSize > maxContestPageSize {
		return 0, 0, status.Errorf(codes.ResourceExhausted, "page_size must not exceed %d", maxContestPageSize)
	}
	return int(page), int(pageSize), nil
}

func timestampValue(value *timestamppb.Timestamp, required bool, field string) (time.Time, error) {
	if value == nil {
		if required {
			return time.Time{}, status.Errorf(codes.InvalidArgument, "%s is required", field)
		}
		return time.Time{}, nil
	}
	if err := value.CheckValid(); err != nil {
		return time.Time{}, status.Errorf(codes.InvalidArgument, "%s is invalid", field)
	}
	return value.AsTime().UTC(), nil
}

func optionalTimestampValue(value *timestamppb.Timestamp, field string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := timestampValue(value, true, field)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func durationValue(value *durationpb.Duration, field string) (*time.Duration, error) {
	if value == nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s is required", field)
	}
	if err := value.CheckValid(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s is invalid", field)
	}
	duration := value.AsDuration()
	if duration <= 0 || duration%time.Millisecond != 0 {
		return nil, status.Errorf(codes.InvalidArgument, "%s must be positive millisecond precision", field)
	}
	return &duration, nil
}

func validateUTF8(value, field string, maxRunes int, required bool) error {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxRunes {
		return status.Errorf(codes.InvalidArgument, "%s must be valid UTF-8 and at most %d characters", field, maxRunes)
	}
	if required && strings.TrimSpace(value) == "" {
		return status.Errorf(codes.InvalidArgument, "%s is required", field)
	}
	return nil
}

func validContestMode(value pb.ContestMode) bool {
	return value >= pb.ContestMode_CONTEST_MODE_ICPC && value <= pb.ContestMode_CONTEST_MODE_OI
}

func validContestStatus(value pb.ContestStatus) bool {
	return value >= pb.ContestStatus_CONTEST_STATUS_DRAFT && value <= pb.ContestStatus_CONTEST_STATUS_DELETED
}

func validContestVisibility(value pb.ContestVisibility) bool {
	return value >= pb.ContestVisibility_CONTEST_VISIBILITY_PUBLIC &&
		value <= pb.ContestVisibility_CONTEST_VISIBILITY_PRIVATE
}

func validRegistrationPolicy(value pb.RegistrationPolicy) bool {
	return value >= pb.RegistrationPolicy_REGISTRATION_POLICY_OPEN &&
		value <= pb.RegistrationPolicy_REGISTRATION_POLICY_INVITE_ONLY
}

func validRegistrationStatus(value pb.RegistrationStatus) bool {
	return value >= pb.RegistrationStatus_REGISTRATION_STATUS_PENDING &&
		value <= pb.RegistrationStatus_REGISTRATION_STATUS_CANCELLED
}

func validParticipantStatus(value pb.ParticipantStatus) bool {
	return value >= pb.ParticipantStatus_PARTICIPANT_STATUS_NORMAL &&
		value <= pb.ParticipantStatus_PARTICIPANT_STATUS_WITHDRAWN
}

func validScoreboardVisibility(value pb.ScoreboardVisibility) bool {
	return value >= pb.ScoreboardVisibility_SCOREBOARD_VISIBILITY_PUBLIC &&
		value <= pb.ScoreboardVisibility_SCOREBOARD_VISIBILITY_HIDDEN
}

func editableFromProto(fields *pb.ContestEditableFields) (model.EditableContest, error) {
	if fields == nil {
		return model.EditableContest{}, status.Error(codes.InvalidArgument, "contest is required")
	}
	if err := validateUTF8(fields.Title, "title", 200, true); err != nil {
		return model.EditableContest{}, err
	}
	if err := validateUTF8(fields.Description, "description", 200_000, false); err != nil {
		return model.EditableContest{}, err
	}
	if !validContestMode(fields.Mode) || !validRegistrationPolicy(fields.RegistrationPolicy) ||
		!validContestVisibility(fields.Visibility) || !validScoreboardVisibility(fields.ScoreboardVisibility) {
		return model.EditableContest{}, status.Error(codes.InvalidArgument, "contest enum value is invalid")
	}
	startAt, err := timestampValue(fields.StartAt, true, "start_at")
	if err != nil {
		return model.EditableContest{}, err
	}
	endAt, err := timestampValue(fields.EndAt, true, "end_at")
	if err != nil {
		return model.EditableContest{}, err
	}
	registrationStart, err := optionalTimestampValue(fields.RegistrationStartAt, "registration_start_at")
	if err != nil {
		return model.EditableContest{}, err
	}
	registrationEnd, err := optionalTimestampValue(fields.RegistrationEndAt, "registration_end_at")
	if err != nil {
		return model.EditableContest{}, err
	}
	editable := model.EditableContest{
		Title: fields.Title, Description: fields.Description, Mode: int64(fields.Mode),
		StartAt: startAt, EndAt: endAt, RegistrationStartAt: registrationStart,
		RegistrationEndAt: registrationEnd, RegistrationPolicy: int64(fields.RegistrationPolicy),
		Visibility: int64(fields.Visibility), ScoreboardVisibility: int64(fields.ScoreboardVisibility),
	}
	switch freeze := fields.ScoreboardFreezePolicy.(type) {
	case nil:
	case *pb.ContestEditableFields_ScoreboardFreezeAt:
		value, err := optionalTimestampValue(freeze.ScoreboardFreezeAt, "scoreboard_freeze_at")
		if err != nil || value == nil {
			if err != nil {
				return model.EditableContest{}, err
			}
			return model.EditableContest{}, status.Error(codes.InvalidArgument, "scoreboard_freeze_at is required")
		}
		editable.ScoreboardFreezeAt = value
	case *pb.ContestEditableFields_ScoreboardFreezeBeforeEnd:
		value, err := durationValue(freeze.ScoreboardFreezeBeforeEnd, "scoreboard_freeze_before_end")
		if err != nil {
			return model.EditableContest{}, err
		}
		editable.ScoreboardFreezeBeforeEnd = value
	default:
		return model.EditableContest{}, status.Error(codes.InvalidArgument, "scoreboard freeze policy is invalid")
	}
	if fields.InviteCode != nil {
		if err := validateUTF8(*fields.InviteCode, "invite_code", 64, true); err != nil || len(*fields.InviteCode) > 72 {
			if err != nil {
				return model.EditableContest{}, err
			}
			return model.EditableContest{}, status.Error(codes.InvalidArgument, "invite_code must not exceed 72 bytes")
		}
		value := *fields.InviteCode
		editable.InviteCode = &value
	}
	return editable, nil
}

func contestPatchFromProto(fields *pb.ContestEditableFields, mask *fieldmaskpb.FieldMask) (model.ContestPatch, error) {
	if fields == nil || mask == nil || len(mask.Paths) == 0 {
		return model.ContestPatch{}, status.Error(codes.InvalidArgument, "contest and a non-empty update_mask are required")
	}
	patch := model.ContestPatch{}
	seen := make(map[string]struct{}, len(mask.Paths))
	for _, rawPath := range mask.Paths {
		path := strings.TrimSpace(rawPath)
		if path == "" {
			return model.ContestPatch{}, status.Error(codes.InvalidArgument, "update_mask contains an empty path")
		}
		if _, exists := seen[path]; exists {
			return model.ContestPatch{}, status.Errorf(codes.InvalidArgument, "update_mask contains duplicate path %q", path)
		}
		seen[path] = struct{}{}
		switch path {
		case "title":
			if err := validateUTF8(fields.Title, "title", 200, true); err != nil {
				return model.ContestPatch{}, err
			}
			patch.Title = &fields.Title
		case "description":
			if err := validateUTF8(fields.Description, "description", 200_000, false); err != nil {
				return model.ContestPatch{}, err
			}
			patch.Description = &fields.Description
		case "mode":
			if !validContestMode(fields.Mode) {
				return model.ContestPatch{}, status.Error(codes.InvalidArgument, "mode is invalid")
			}
			value := int64(fields.Mode)
			patch.Mode = &value
		case "start_at":
			value, err := timestampValue(fields.StartAt, true, "start_at")
			if err != nil {
				return model.ContestPatch{}, err
			}
			patch.StartAt = &value
		case "end_at":
			value, err := timestampValue(fields.EndAt, true, "end_at")
			if err != nil {
				return model.ContestPatch{}, err
			}
			patch.EndAt = &value
		case "registration_start_at":
			value, err := optionalTimestampValue(fields.RegistrationStartAt, "registration_start_at")
			if err != nil {
				return model.ContestPatch{}, err
			}
			patch.RegistrationStartAt = &value
		case "registration_end_at":
			value, err := optionalTimestampValue(fields.RegistrationEndAt, "registration_end_at")
			if err != nil {
				return model.ContestPatch{}, err
			}
			patch.RegistrationEndAt = &value
		case "registration_policy":
			if !validRegistrationPolicy(fields.RegistrationPolicy) {
				return model.ContestPatch{}, status.Error(codes.InvalidArgument, "registration_policy is invalid")
			}
			value := int64(fields.RegistrationPolicy)
			patch.RegistrationPolicy = &value
		case "visibility":
			if !validContestVisibility(fields.Visibility) {
				return model.ContestPatch{}, status.Error(codes.InvalidArgument, "visibility is invalid")
			}
			value := int64(fields.Visibility)
			patch.Visibility = &value
		case "scoreboard_visibility":
			if !validScoreboardVisibility(fields.ScoreboardVisibility) {
				return model.ContestPatch{}, status.Error(codes.InvalidArgument, "scoreboard_visibility is invalid")
			}
			value := int64(fields.ScoreboardVisibility)
			patch.ScoreboardVisibility = &value
		case "scoreboard_freeze_at":
			if _, conflict := seen["scoreboard_freeze_before_end"]; conflict {
				return model.ContestPatch{}, status.Error(codes.InvalidArgument, "scoreboard freeze paths conflict")
			}
			var value *time.Time
			if fields.ScoreboardFreezePolicy != nil {
				freeze, ok := fields.ScoreboardFreezePolicy.(*pb.ContestEditableFields_ScoreboardFreezeAt)
				if !ok {
					return model.ContestPatch{}, status.Error(codes.InvalidArgument, "scoreboard_freeze_at does not match the supplied oneof")
				}
				parsed, err := optionalTimestampValue(freeze.ScoreboardFreezeAt, "scoreboard_freeze_at")
				if err != nil || parsed == nil {
					if err != nil {
						return model.ContestPatch{}, err
					}
					return model.ContestPatch{}, status.Error(codes.InvalidArgument, "scoreboard_freeze_at is required")
				}
				value = parsed
			}
			patch.ScoreboardFreezeAt = &value
		case "scoreboard_freeze_before_end":
			if _, conflict := seen["scoreboard_freeze_at"]; conflict {
				return model.ContestPatch{}, status.Error(codes.InvalidArgument, "scoreboard freeze paths conflict")
			}
			var value *time.Duration
			if fields.ScoreboardFreezePolicy != nil {
				freeze, ok := fields.ScoreboardFreezePolicy.(*pb.ContestEditableFields_ScoreboardFreezeBeforeEnd)
				if !ok {
					return model.ContestPatch{}, status.Error(codes.InvalidArgument, "scoreboard_freeze_before_end does not match the supplied oneof")
				}
				parsed, err := durationValue(freeze.ScoreboardFreezeBeforeEnd, "scoreboard_freeze_before_end")
				if err != nil {
					return model.ContestPatch{}, err
				}
				value = parsed
			}
			patch.ScoreboardFreezeBeforeEnd = &value
		case "invite_code":
			var value *string
			if fields.InviteCode != nil {
				if err := validateUTF8(*fields.InviteCode, "invite_code", 64, true); err != nil || len(*fields.InviteCode) > 72 {
					if err != nil {
						return model.ContestPatch{}, err
					}
					return model.ContestPatch{}, status.Error(codes.InvalidArgument, "invite_code must not exceed 72 bytes")
				}
				copyValue := *fields.InviteCode
				value = &copyValue
			}
			patch.InviteCode = &value
		default:
			return model.ContestPatch{}, status.Errorf(codes.InvalidArgument, "unsupported update_mask path %q", path)
		}
	}
	return patch, nil
}

func contestProblemsFromProto(values []*pb.ContestProblem) ([]model.ContestProblem, error) {
	if len(values) > maxContestProblemBatch {
		return nil, status.Errorf(codes.ResourceExhausted, "contest problems must not exceed %d entries", maxContestProblemBatch)
	}
	result := make([]model.ContestProblem, 0, len(values))
	problemIDs := make(map[int64]struct{}, len(values))
	labels := make(map[string]struct{}, len(values))
	orders := make(map[int32]struct{}, len(values))
	for index, value := range values {
		if value == nil || value.ProblemId <= 0 || value.DisplayOrder < 0 {
			return nil, status.Errorf(codes.InvalidArgument, "contest problem at index %d is invalid", index)
		}
		label := strings.TrimSpace(value.Label)
		if err := validateUTF8(label, "problem label", 32, true); err != nil {
			return nil, err
		}
		if _, exists := problemIDs[value.ProblemId]; exists {
			return nil, status.Error(codes.AlreadyExists, "duplicate contest problem_id")
		}
		if _, exists := labels[label]; exists {
			return nil, status.Error(codes.AlreadyExists, "duplicate contest problem label")
		}
		if _, exists := orders[value.DisplayOrder]; exists {
			return nil, status.Error(codes.AlreadyExists, "duplicate contest problem display_order")
		}
		problemIDs[value.ProblemId] = struct{}{}
		labels[label] = struct{}{}
		orders[value.DisplayOrder] = struct{}{}
		result = append(result, model.ContestProblem{
			ProblemID: value.ProblemId, Label: label,
			DisplayOrder: int64(value.DisplayOrder), MaxScore: value.MaxScore,
		})
	}
	return result, nil
}

type visibleContest struct {
	Detail       *model.ContestDetailRecord
	Principal    infraauth.Principal
	HasPrincipal bool
	Manager      bool
	MyStatus     *model.MyContestStatus
}

func loadVisibleContest(ctx context.Context, svcCtx *svc.ServiceContext, contestID int64) (*visibleContest, error) {
	repository, err := requireContestModel(svcCtx)
	if err != nil {
		return nil, err
	}
	detail, err := repository.GetDetail(ctx, contestID, nowUTC(svcCtx))
	if err != nil {
		return nil, mapModelError(err)
	}
	if detail == nil {
		return nil, status.Error(codes.Internal, "contest model returned an empty response")
	}
	principal, hasPrincipal, err := optionalPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	result := &visibleContest{Detail: detail, Principal: principal, HasPrincipal: hasPrincipal}
	if hasPrincipal {
		if isAdmin(principal) {
			result.Manager = true
		} else {
			aggregate, aggregateErr := repository.GetForEdit(ctx, contestID)
			if aggregateErr != nil {
				return nil, mapModelError(aggregateErr)
			}
			result.Manager = aggregate != nil && aggregate.CreatorID == principal.UserID
		}
	}
	if detail.Status == model.ContestStatusDraft && !result.Manager {
		return nil, status.Error(codes.NotFound, "contest resource not found")
	}
	if detail.Visibility != model.ContestVisibilityPrivate {
		return result, nil
	}
	if result.Manager {
		return result, nil
	}
	if !hasPrincipal {
		return nil, status.Error(codes.NotFound, "contest resource not found")
	}
	myStatus, err := repository.GetMyStatus(ctx, contestID, principal.UserID)
	if err != nil {
		return nil, mapModelError(err)
	}
	result.MyStatus = myStatus
	if hasContestRelationship(myStatus) {
		return result, nil
	}
	return nil, status.Error(codes.NotFound, "contest resource not found")
}

func hasContestRelationship(value *model.MyContestStatus) bool {
	if value == nil {
		return false
	}
	if value.Participant != nil && value.Participant.Status == model.ParticipantStatusNormal {
		return true
	}
	return value.Registration != nil &&
		(value.Registration.Status == model.RegistrationStatusPending ||
			value.Registration.Status == model.RegistrationStatusApproved)
}

func loadMyStatus(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	contestID, userID int64,
) (*model.MyContestStatus, error) {
	repository, err := requireContestModel(svcCtx)
	if err != nil {
		return nil, err
	}
	value, err := repository.GetMyStatus(ctx, contestID, userID)
	if err != nil {
		return nil, mapModelError(err)
	}
	return value, nil
}

func normalParticipant(value *model.MyContestStatus) bool {
	return value != nil && value.Participant != nil && value.Participant.Status == model.ParticipantStatusNormal
}

func accessDecision(snapshot *model.ContestAccessSnapshot, submission bool) (bool, pb.SubmissionDenyReason) {
	if snapshot == nil {
		return false, pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_OTHER
	}
	switch snapshot.LifecycleStatus {
	case model.ContestStatusCancelled:
		return false, pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_CONTEST_CANCELLED
	case model.ContestStatusDraft, model.ContestStatusArchived, model.ContestStatusDeleted:
		return false, pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNAUTHORIZED
	}
	if snapshot.EffectiveStatus == model.ContestStatusPublished {
		return false, pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_CONTEST_NOT_STARTED
	}
	if submission && snapshot.EffectiveStatus == model.ContestStatusFinished {
		return false, pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_CONTEST_FINISHED
	}
	if snapshot.EffectiveStatus != model.ContestStatusRunning &&
		snapshot.EffectiveStatus != model.ContestStatusFinished {
		return false, pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNAUTHORIZED
	}
	if !snapshot.ProblemAssigned {
		return false, pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_PROBLEM_NOT_ASSIGNED
	}
	if !submission && snapshot.Manager {
		return true, pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNSPECIFIED
	}
	if snapshot.RegistrationStatus != nil && *snapshot.RegistrationStatus == model.RegistrationStatusPending {
		return false, pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_REGISTRATION_PENDING
	}
	if snapshot.ParticipantStatus == nil {
		return false, pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_NOT_REGISTERED
	}
	switch *snapshot.ParticipantStatus {
	case model.ParticipantStatusNormal:
		return true, pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNSPECIFIED
	case model.ParticipantStatusDisqualified:
		return false, pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_PARTICIPANT_DISQUALIFIED
	case model.ParticipantStatusWithdrawn:
		return false, pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_PARTICIPANT_WITHDRAWN
	default:
		return false, pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNAUTHORIZED
	}
}

func statusAccessSnapshot(
	detail *model.ContestDetailRecord,
	myStatus *model.MyContestStatus,
	problemAssigned bool,
	manager bool,
) *model.ContestAccessSnapshot {
	result := &model.ContestAccessSnapshot{ProblemAssigned: problemAssigned, Manager: manager}
	if detail != nil {
		result.ContestID = detail.ContestID
		result.Mode = detail.Mode
		result.LifecycleStatus = detail.Status
		result.EffectiveStatus = detail.Status
		result.Visibility = detail.Visibility
		result.StartAt = detail.StartAt
		result.EndAt = detail.EndAt
	}
	if myStatus != nil && myStatus.Registration != nil {
		value := myStatus.Registration.Status
		result.RegistrationStatus = &value
	}
	if myStatus != nil && myStatus.Participant != nil {
		value := myStatus.Participant.Status
		result.ParticipantStatus = &value
	}
	return result
}

func ensurePublishedProblems(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	problems []model.ContestProblem,
) error {
	if len(problems) == 0 {
		return nil
	}
	if svcCtx == nil || svcCtx.ProblemCatalog == nil {
		return status.Error(codes.Unavailable, "problem catalog is unavailable")
	}
	ids := make([]int64, 0, len(problems))
	for _, problem := range problems {
		ids = append(ids, problem.ProblemID)
	}
	available, err := svcCtx.ProblemCatalog.ListPublishedForContest(ctx, ids)
	if err != nil {
		return mapDependencyError(err, "problem catalog")
	}
	seen := make(map[int64]struct{}, len(available))
	for _, id := range available {
		seen[id] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := seen[id]; !ok {
			return status.Errorf(codes.FailedPrecondition, "problem %d is not published for contest use", id)
		}
	}
	if len(seen) != len(ids) {
		return status.Error(codes.Unavailable, "problem catalog returned an invalid response")
	}
	return nil
}

func validateKeyword(keyword string) error {
	return validateUTF8(keyword, "keyword", maxContestKeywordRunes, false)
}

func invalidEnum(field string) error {
	return status.Errorf(codes.InvalidArgument, "%s enum value is invalid", field)
}

func requirePositive(values ...struct {
	name  string
	value int64
}) error {
	for _, item := range values {
		if item.value <= 0 {
			return status.Errorf(codes.InvalidArgument, "%s must be positive", item.name)
		}
	}
	return nil
}

func internalResponseError(name string) error {
	return status.Error(codes.Internal, fmt.Sprintf("contest model returned an invalid %s", name))
}
