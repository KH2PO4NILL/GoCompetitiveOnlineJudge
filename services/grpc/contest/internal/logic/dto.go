package logic

import (
	"math"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func timestampProto(value time.Time) (*timestamppb.Timestamp, error) {
	timestamp := timestamppb.New(value.UTC())
	if err := timestamp.CheckValid(); err != nil {
		return nil, status.Error(codes.Internal, "contest contains an invalid timestamp")
	}
	return timestamp, nil
}

func optionalTimestampProto(value *time.Time) (*timestamppb.Timestamp, error) {
	if value == nil {
		return nil, nil
	}
	return timestampProto(*value)
}

func durationProto(value time.Duration) (*durationpb.Duration, error) {
	duration := durationpb.New(value)
	if err := duration.CheckValid(); err != nil {
		return nil, status.Error(codes.Internal, "contest contains an invalid duration")
	}
	return duration, nil
}

func editableProto(value model.EditableContest) (*pb.ContestEditableFields, error) {
	startAt, err := timestampProto(value.StartAt)
	if err != nil {
		return nil, err
	}
	endAt, err := timestampProto(value.EndAt)
	if err != nil {
		return nil, err
	}
	registrationStart, err := optionalTimestampProto(value.RegistrationStartAt)
	if err != nil {
		return nil, err
	}
	registrationEnd, err := optionalTimestampProto(value.RegistrationEndAt)
	if err != nil {
		return nil, err
	}
	result := &pb.ContestEditableFields{
		Title: value.Title, Description: value.Description, Mode: pb.ContestMode(value.Mode),
		StartAt: startAt, EndAt: endAt, RegistrationStartAt: registrationStart,
		RegistrationEndAt: registrationEnd, RegistrationPolicy: pb.RegistrationPolicy(value.RegistrationPolicy),
		Visibility:           pb.ContestVisibility(value.Visibility),
		ScoreboardVisibility: pb.ScoreboardVisibility(value.ScoreboardVisibility),
	}
	if value.ScoreboardFreezeAt != nil {
		freezeAt, err := timestampProto(*value.ScoreboardFreezeAt)
		if err != nil {
			return nil, err
		}
		result.ScoreboardFreezePolicy = &pb.ContestEditableFields_ScoreboardFreezeAt{ScoreboardFreezeAt: freezeAt}
	} else if value.ScoreboardFreezeBeforeEnd != nil {
		freezeBefore, err := durationProto(*value.ScoreboardFreezeBeforeEnd)
		if err != nil {
			return nil, err
		}
		result.ScoreboardFreezePolicy = &pb.ContestEditableFields_ScoreboardFreezeBeforeEnd{
			ScoreboardFreezeBeforeEnd: freezeBefore,
		}
	}
	// InviteCode is write-only at the Model boundary. Never reconstruct or echo
	// a persisted bcrypt digest through the management DTO.
	return result, nil
}

func contestProto(value *model.ContestAggregate) (*pb.Contest, error) {
	if value == nil {
		return nil, internalResponseError("contest")
	}
	editable, err := editableProto(value.Editable)
	if err != nil {
		return nil, err
	}
	createdAt, err := timestampProto(value.CreatedAt)
	if err != nil {
		return nil, err
	}
	updatedAt, err := timestampProto(value.UpdatedAt)
	if err != nil {
		return nil, err
	}
	publishedAt, err := optionalTimestampProto(value.PublishedAt)
	if err != nil {
		return nil, err
	}
	cancelledAt, err := optionalTimestampProto(value.CancelledAt)
	if err != nil {
		return nil, err
	}
	archivedAt, err := optionalTimestampProto(value.ArchivedAt)
	if err != nil {
		return nil, err
	}
	deletedAt, err := optionalTimestampProto(value.DeletedAt)
	if err != nil {
		return nil, err
	}
	frozenAt, err := optionalTimestampProto(value.ScoreboardFrozenAt)
	if err != nil {
		return nil, err
	}
	statusValue := value.EffectiveStatus
	if statusValue == 0 {
		statusValue = value.Status
	}
	return &pb.Contest{
		ContestId: value.ContestID, CreatorId: value.CreatorID, Editable: editable,
		Status: pb.ContestStatus(statusValue), Revision: value.Revision,
		CreatedAt: createdAt, UpdatedAt: updatedAt, PublishedAt: publishedAt,
		CancelledAt: cancelledAt, ArchivedAt: archivedAt, DeletedAt: deletedAt,
		ScoreboardFrozen: value.ScoreboardFrozen, ScoreboardFrozenAt: frozenAt,
	}, nil
}

func contestDetailProto(value *model.ContestDetailRecord) (*pb.ContestDetail, error) {
	if value == nil {
		return nil, internalResponseError("contest detail")
	}
	startAt, err := timestampProto(value.StartAt)
	if err != nil {
		return nil, err
	}
	endAt, err := timestampProto(value.EndAt)
	if err != nil {
		return nil, err
	}
	registrationStart, err := optionalTimestampProto(value.RegistrationStartAt)
	if err != nil {
		return nil, err
	}
	registrationEnd, err := optionalTimestampProto(value.RegistrationEndAt)
	if err != nil {
		return nil, err
	}
	return &pb.ContestDetail{
		ContestId: value.ContestID, Title: value.Title, Description: value.Description,
		Mode: pb.ContestMode(value.Mode), Status: pb.ContestStatus(value.Status),
		StartAt: startAt, EndAt: endAt, RegistrationStartAt: registrationStart,
		RegistrationEndAt: registrationEnd, RegistrationPolicy: pb.RegistrationPolicy(value.RegistrationPolicy),
		Visibility:           pb.ContestVisibility(value.Visibility),
		ScoreboardVisibility: pb.ScoreboardVisibility(value.ScoreboardVisibility),
		RegistrationOpen:     value.RegistrationOpen, ScoreboardFrozen: value.ScoreboardFrozen,
		ParticipantCount: value.ParticipantCount,
	}, nil
}

func contestSummaryProto(value model.ContestSummary) (*pb.ContestSummary, error) {
	startAt, err := timestampProto(value.StartAt)
	if err != nil {
		return nil, err
	}
	endAt, err := timestampProto(value.EndAt)
	if err != nil {
		return nil, err
	}
	return &pb.ContestSummary{
		ContestId: value.ContestID, Title: value.Title, Mode: pb.ContestMode(value.Mode),
		Status: pb.ContestStatus(value.Status), StartAt: startAt, EndAt: endAt,
		RegistrationPolicy: pb.RegistrationPolicy(value.RegistrationPolicy),
		Visibility:         pb.ContestVisibility(value.Visibility), RegistrationOpen: value.RegistrationOpen,
		ParticipantCount: value.ParticipantCount,
	}, nil
}

func contestProblemProto(value model.ContestProblem) (*pb.ContestProblem, error) {
	if value.DisplayOrder < 0 || value.DisplayOrder > math.MaxInt32 {
		return nil, status.Error(codes.Internal, "contest problem display order is out of range")
	}
	return &pb.ContestProblem{
		ProblemId: value.ProblemID, Label: value.Label,
		DisplayOrder: int32(value.DisplayOrder), MaxScore: value.MaxScore,
	}, nil
}

func contestProblemSummaryProto(value model.ContestProblem) (*pb.ContestProblemSummary, error) {
	problem, err := contestProblemProto(value)
	if err != nil {
		return nil, err
	}
	return &pb.ContestProblemSummary{
		ProblemId: problem.ProblemId, Label: problem.Label,
		DisplayOrder: problem.DisplayOrder, MaxScore: problem.MaxScore,
	}, nil
}

func registrationProto(value *model.ContestRegistration) (*pb.ContestRegistration, error) {
	if value == nil {
		return nil, nil
	}
	createdAt, err := timestampProto(value.CreatedAt)
	if err != nil {
		return nil, err
	}
	updatedAt, err := timestampProto(value.UpdatedAt)
	if err != nil {
		return nil, err
	}
	reviewedAt, err := optionalTimestampProto(value.ReviewedAt)
	if err != nil {
		return nil, err
	}
	cancelledAt, err := optionalTimestampProto(value.CancelledAt)
	if err != nil {
		return nil, err
	}
	var reviewerID int64
	if value.ReviewerID != nil {
		reviewerID = *value.ReviewerID
	}
	return &pb.ContestRegistration{
		RegistrationId: value.RegistrationID, ContestId: value.ContestID, UserId: value.UserID,
		Status: pb.RegistrationStatus(value.Status), ApplicationNote: value.ApplicationNote,
		ReviewerId: reviewerID, ReviewNote: value.ReviewNote, Revision: value.Revision,
		CreatedAt: createdAt, ReviewedAt: reviewedAt, CancelledAt: cancelledAt, UpdatedAt: updatedAt,
	}, nil
}

func participantProto(value *model.ContestParticipant) (*pb.ContestParticipant, error) {
	if value == nil {
		return nil, nil
	}
	qualifiedAt, err := timestampProto(value.QualifiedAt)
	if err != nil {
		return nil, err
	}
	updatedAt, err := timestampProto(value.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &pb.ContestParticipant{
		ContestId: value.ContestID, UserId: value.UserID,
		Status: pb.ParticipantStatus(value.Status), QualifiedAt: qualifiedAt, UpdatedAt: updatedAt,
	}, nil
}

func myContestStatusProto(value *model.MyContestStatus, canView, canSubmit bool) (*pb.MyContestStatus, error) {
	if value == nil {
		return nil, internalResponseError("contest status")
	}
	registration, err := registrationProto(value.Registration)
	if err != nil {
		return nil, err
	}
	participant, err := participantProto(value.Participant)
	if err != nil {
		return nil, err
	}
	return &pb.MyContestStatus{
		ContestId: value.ContestID, Registration: registration, Participant: participant,
		CanViewProblems: canView, CanSubmit: canSubmit,
	}, nil
}

func scoreboardProto(value *model.ScoreboardPage) (*pb.Scoreboard, error) {
	if value == nil {
		return nil, internalResponseError("scoreboard")
	}
	if value.Mode < model.ContestModeICPC || value.Mode > model.ContestModeOI {
		return nil, status.Error(codes.Internal, "scoreboard mode is invalid")
	}
	if value.View < model.ScoreboardViewLive || value.View > model.ScoreboardViewFinal || value.Total < 0 {
		return nil, status.Error(codes.Internal, "scoreboard view or total is invalid")
	}
	generatedAt, err := timestampProto(value.GeneratedAt)
	if err != nil {
		return nil, err
	}
	result := &pb.Scoreboard{
		ContestId: value.ContestID, Mode: pb.ContestMode(value.Mode), GeneratedAt: generatedAt,
		Frozen: value.Frozen, View: pb.ScoreboardView(value.View), Total: value.Total,
		Rows: make([]*pb.ScoreboardRow, 0, len(value.Rows)),
	}
	for _, row := range value.Rows {
		converted, err := scoreboardRowProto(row, value.Mode)
		if err != nil {
			return nil, err
		}
		result.Rows = append(result.Rows, converted)
	}
	return result, nil
}

func scoreboardRowProto(value model.ScoreboardRow, mode int64) (*pb.ScoreboardRow, error) {
	result := &pb.ScoreboardRow{
		Rank: value.Rank, Ordinal: value.Ordinal, UserId: value.UserID,
		DisplayName:    value.DisplayName,
		ProblemResults: make([]*pb.ScoreboardProblemResult, 0, len(value.ProblemResults)),
	}
	switch mode {
	case model.ContestModeICPC:
		if value.ICPC == nil || value.Points != nil || value.ICPC.SolvedCount < 0 ||
			value.ICPC.SolvedCount > math.MaxInt32 || value.ICPC.TotalPenalty < 0 {
			return nil, status.Error(codes.Internal, "ICPC scoreboard row is invalid")
		}
		penalty, err := durationProto(value.ICPC.TotalPenalty)
		if err != nil {
			return nil, err
		}
		lastSolvedAt, err := optionalTimestampProto(value.ICPC.LastSolvedAt)
		if err != nil {
			return nil, err
		}
		result.Score = &pb.ScoreboardRow_Icpc{Icpc: &pb.IcpcScore{
			SolvedCount: int32(value.ICPC.SolvedCount), TotalPenalty: penalty, LastSolvedAt: lastSolvedAt,
		}}
	case model.ContestModeIOI, model.ContestModeOI:
		if value.Points == nil || value.ICPC != nil || value.Points.TotalScore < 0 {
			return nil, status.Error(codes.Internal, "points scoreboard row is invalid")
		}
		lastEffectiveAt, err := optionalTimestampProto(value.Points.LastEffectiveSubmissionAt)
		if err != nil {
			return nil, err
		}
		if mode == model.ContestModeIOI {
			result.Score = &pb.ScoreboardRow_Ioi{Ioi: &pb.IoiScore{
				TotalScore: value.Points.TotalScore, LastEffectiveSubmissionAt: lastEffectiveAt,
			}}
		} else {
			result.Score = &pb.ScoreboardRow_Oi{Oi: &pb.OiScore{
				TotalScore: value.Points.TotalScore, LastEffectiveSubmissionAt: lastEffectiveAt,
			}}
		}
	default:
		return nil, status.Error(codes.Internal, "scoreboard mode is invalid")
	}
	for _, problem := range value.ProblemResults {
		converted, err := scoreboardProblemResultProto(problem, mode)
		if err != nil {
			return nil, err
		}
		result.ProblemResults = append(result.ProblemResults, converted)
	}
	return result, nil
}

func scoreboardProblemResultProto(value model.ScoreboardProblemResult, mode int64) (*pb.ScoreboardProblemResult, error) {
	result := &pb.ScoreboardProblemResult{ProblemId: value.ProblemID, Label: value.Label}
	switch mode {
	case model.ContestModeICPC:
		if value.ICPC == nil || value.Points != nil || value.ICPC.WrongAttempts < 0 ||
			value.ICPC.WrongAttempts > math.MaxInt32 || value.ICPC.Penalty < 0 {
			return nil, status.Error(codes.Internal, "ICPC scoreboard problem result is invalid")
		}
		penalty, err := durationProto(value.ICPC.Penalty)
		if err != nil {
			return nil, err
		}
		acceptedAt, err := optionalTimestampProto(value.ICPC.AcceptedAt)
		if err != nil {
			return nil, err
		}
		result.Result = &pb.ScoreboardProblemResult_Icpc{Icpc: &pb.IcpcProblemResult{
			Solved: value.ICPC.Solved, WrongAttempts: int32(value.ICPC.WrongAttempts),
			Penalty: penalty, AcceptedAt: acceptedAt, FirstToSolve: value.ICPC.FirstToSolve,
		}}
	case model.ContestModeIOI, model.ContestModeOI:
		if value.Points == nil || value.ICPC != nil || value.Points.Score < 0 || value.Points.MaxScore <= 0 ||
			value.Points.SubmissionCount < 0 || value.Points.SubmissionCount > math.MaxInt32 {
			return nil, status.Error(codes.Internal, "points scoreboard problem result is invalid")
		}
		lastEffectiveAt, err := optionalTimestampProto(value.Points.LastEffectiveSubmissionAt)
		if err != nil {
			return nil, err
		}
		if mode == model.ContestModeIOI {
			result.Result = &pb.ScoreboardProblemResult_Ioi{Ioi: &pb.IoiProblemResult{
				Score: value.Points.Score, MaxScore: value.Points.MaxScore,
				SubmissionCount: int32(value.Points.SubmissionCount), LastEffectiveSubmissionAt: lastEffectiveAt,
			}}
		} else {
			result.Result = &pb.ScoreboardProblemResult_Oi{Oi: &pb.OiProblemResult{
				Score: value.Points.Score, MaxScore: value.Points.MaxScore,
				SubmissionCount: int32(value.Points.SubmissionCount), LastEffectiveSubmissionAt: lastEffectiveAt,
			}}
		}
	default:
		return nil, status.Error(codes.Internal, "scoreboard mode is invalid")
	}
	return result, nil
}
