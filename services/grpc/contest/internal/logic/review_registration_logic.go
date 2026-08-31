package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ReviewRegistrationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReviewRegistrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReviewRegistrationLogic {
	return &ReviewRegistrationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ReviewRegistrationLogic) ReviewRegistration(in *pb.ReviewRegistrationReq) (*pb.ReviewRegistrationResp, error) {
	if in == nil || in.ContestId <= 0 || in.RegistrationId <= 0 || in.ExpectedRevision <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id, registration_id and expected_revision must be positive")
	}
	if in.Decision != pb.RegistrationReviewDecision_REGISTRATION_REVIEW_DECISION_APPROVE &&
		in.Decision != pb.RegistrationReviewDecision_REGISTRATION_REVIEW_DECISION_REJECT {
		return nil, invalidEnum("decision")
	}
	if err := validateUTF8(in.ReviewNote, "review_note", 2000, false); err != nil {
		return nil, err
	}
	principal, _, err := authorizeContest(l.ctx, l.svcCtx, in.ContestId)
	if err != nil {
		return nil, err
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	registration, participant, err := repository.ReviewRegistration(l.ctx, model.ReviewRegistrationInput{
		ContestID: in.ContestId, RegistrationID: in.RegistrationId, ReviewerID: principal.UserID,
		Decision: int64(in.Decision), ReviewNote: in.ReviewNote, ExpectedRevision: in.ExpectedRevision,
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	convertedRegistration, err := registrationProto(registration)
	if err != nil {
		return nil, err
	}
	if convertedRegistration == nil {
		return nil, internalResponseError("registration")
	}
	convertedParticipant, err := participantProto(participant)
	if err != nil {
		return nil, err
	}
	return &pb.ReviewRegistrationResp{
		Registration: convertedRegistration, Participant: convertedParticipant,
	}, nil
}
