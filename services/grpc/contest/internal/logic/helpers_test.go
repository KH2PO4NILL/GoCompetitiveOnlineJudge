package logic

import (
	"errors"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAccessDecisionMatrix(t *testing.T) {
	normal := int64(model.ParticipantStatusNormal)
	disqualified := int64(model.ParticipantStatusDisqualified)
	withdrawn := int64(model.ParticipantStatusWithdrawn)
	pending := int64(model.RegistrationStatusPending)
	base := model.ContestAccessSnapshot{
		LifecycleStatus: model.ContestStatusPublished, EffectiveStatus: model.ContestStatusRunning,
		ProblemAssigned: true, ParticipantStatus: &normal,
	}
	tests := []struct {
		name       string
		mutate     func(*model.ContestAccessSnapshot)
		submission bool
		allowed    bool
		reason     pb.SubmissionDenyReason
	}{
		{name: "running access", allowed: true, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNSPECIFIED},
		{name: "running submission", submission: true, allowed: true, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNSPECIFIED},
		{name: "not started", mutate: func(v *model.ContestAccessSnapshot) { v.EffectiveStatus = model.ContestStatusPublished }, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_CONTEST_NOT_STARTED},
		{name: "finished access", mutate: func(v *model.ContestAccessSnapshot) { v.EffectiveStatus = model.ContestStatusFinished }, allowed: true, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNSPECIFIED},
		{name: "finished submission", submission: true, mutate: func(v *model.ContestAccessSnapshot) { v.EffectiveStatus = model.ContestStatusFinished }, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_CONTEST_FINISHED},
		{name: "cancelled", mutate: func(v *model.ContestAccessSnapshot) { v.LifecycleStatus = model.ContestStatusCancelled }, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_CONTEST_CANCELLED},
		{name: "unassigned", mutate: func(v *model.ContestAccessSnapshot) { v.ProblemAssigned = false }, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_PROBLEM_NOT_ASSIGNED},
		{name: "manager access", mutate: func(v *model.ContestAccessSnapshot) { v.Manager = true; v.ParticipantStatus = nil }, allowed: true, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNSPECIFIED},
		{name: "manager submission", submission: true, mutate: func(v *model.ContestAccessSnapshot) { v.Manager = true; v.ParticipantStatus = nil }, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_NOT_REGISTERED},
		{name: "manager before start", mutate: func(v *model.ContestAccessSnapshot) {
			v.Manager = true
			v.ParticipantStatus = nil
			v.EffectiveStatus = model.ContestStatusPublished
		}, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_CONTEST_NOT_STARTED},
		{name: "pending", mutate: func(v *model.ContestAccessSnapshot) { v.RegistrationStatus = &pending; v.ParticipantStatus = nil }, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_REGISTRATION_PENDING},
		{name: "not registered", mutate: func(v *model.ContestAccessSnapshot) { v.ParticipantStatus = nil }, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_NOT_REGISTERED},
		{name: "disqualified", mutate: func(v *model.ContestAccessSnapshot) { v.ParticipantStatus = &disqualified }, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_PARTICIPANT_DISQUALIFIED},
		{name: "withdrawn", mutate: func(v *model.ContestAccessSnapshot) { v.ParticipantStatus = &withdrawn }, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_PARTICIPANT_WITHDRAWN},
		{name: "draft", mutate: func(v *model.ContestAccessSnapshot) { v.LifecycleStatus = model.ContestStatusDraft }, reason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNAUTHORIZED},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := base
			if test.mutate != nil {
				test.mutate(&value)
			}
			allowed, reason := accessDecision(&value, test.submission)
			if allowed != test.allowed || reason != test.reason {
				t.Fatalf("accessDecision() = (%t, %s), want (%t, %s)", allowed, reason, test.allowed, test.reason)
			}
		})
	}
}

func TestContestPatchFieldMaskAndNullableClears(t *testing.T) {
	fields := &pb.ContestEditableFields{Title: "new title"}
	patch, err := contestPatchFromProto(fields, &fieldmaskpb.FieldMask{Paths: []string{"title", "registration_end_at"}})
	if err != nil || patch.Title == nil || *patch.Title != "new title" || patch.RegistrationEndAt == nil || *patch.RegistrationEndAt != nil {
		t.Fatalf("contestPatchFromProto() = %+v, %v", patch, err)
	}
	_, err = contestPatchFromProto(fields, &fieldmaskpb.FieldMask{Paths: []string{"unknown"}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unknown field mask code = %s", status.Code(err))
	}
	_, err = contestPatchFromProto(fields, &fieldmaskpb.FieldMask{Paths: []string{"title", "title"}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("duplicate field mask code = %s", status.Code(err))
	}
	_, err = contestPatchFromProto(&pb.ContestEditableFields{
		ScoreboardFreezePolicy: &pb.ContestEditableFields_ScoreboardFreezeAt{ScoreboardFreezeAt: timestamppb.Now()},
	}, &fieldmaskpb.FieldMask{Paths: []string{"scoreboard_freeze_at", "scoreboard_freeze_before_end"}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("conflicting freeze mask code = %s", status.Code(err))
	}
}

func TestEditableFromProtoValidatesTimeAndFreezeDuration(t *testing.T) {
	now := time.Now().UTC()
	valid := editableRequest(now)
	valid.ScoreboardFreezePolicy = &pb.ContestEditableFields_ScoreboardFreezeBeforeEnd{
		ScoreboardFreezeBeforeEnd: durationpb.New(30 * time.Minute),
	}
	editable, err := editableFromProto(valid)
	if err != nil || editable.ScoreboardFreezeBeforeEnd == nil {
		t.Fatalf("editableFromProto(valid) = %+v, %v", editable, err)
	}
	invalid := editableRequest(now)
	invalid.StartAt = &timestamppb.Timestamp{Seconds: 1 << 62}
	_, err = editableFromProto(invalid)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid timestamp code = %s", status.Code(err))
	}
}

func TestModelErrorMapping(t *testing.T) {
	tests := []struct {
		err  error
		code codes.Code
	}{
		{model.ErrInvalidContestInput, codes.InvalidArgument},
		{model.ErrContestNotFound, codes.NotFound},
		{model.ErrContestRevisionConflict, codes.Aborted},
		{model.ErrContestProblemConflict, codes.AlreadyExists},
		{model.ErrInvalidInviteCode, codes.PermissionDenied},
		{model.ErrRegistrationClosed, codes.FailedPrecondition},
		{errors.New("database failed"), codes.Internal},
	}
	for _, test := range tests {
		if got := status.Code(mapModelError(test.err)); got != test.code {
			t.Errorf("mapModelError(%v) = %s, want %s", test.err, got, test.code)
		}
	}
}

func TestScoreboardProtoRejectsModeMismatches(t *testing.T) {
	_, err := scoreboardProto(&model.ScoreboardPage{
		ContestID: 1, Mode: model.ContestModeICPC, GeneratedAt: time.Now().UTC(),
		View: model.ScoreboardViewLive, Rows: []model.ScoreboardRow{{Points: &model.PointsScore{}}},
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mismatched scoreboard code = %s", status.Code(err))
	}
}
