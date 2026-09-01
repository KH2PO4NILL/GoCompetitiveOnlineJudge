package logic

import (
	"context"
	"errors"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ValidateContestSubmissionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateContestSubmissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateContestSubmissionLogic {
	return &ValidateContestSubmissionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ValidateContestSubmissionLogic) ValidateContestSubmission(in *pb.ValidateContestSubmissionReq) (*pb.ValidateContestSubmissionResp, error) {
	if err := requireService(l.ctx, "submit.rpc"); err != nil {
		return nil, err
	}
	if in == nil || in.ContestId <= 0 || in.ProblemId <= 0 || in.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id, problem_id and user_id must be positive")
	}
	submittedAt, err := timestampValue(in.SubmittedAt, true, "submitted_at")
	if err != nil {
		return nil, err
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	snapshot, err := repository.GetAccessSnapshot(l.ctx, in.ContestId, in.ProblemId, in.UserId, submittedAt)
	if errors.Is(err, model.ErrContestNotFound) {
		return &pb.ValidateContestSubmissionResp{
			Allowed: false, DenyReason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNAUTHORIZED,
		}, nil
	}
	if err != nil {
		return nil, mapModelError(err)
	}
	if snapshot == nil {
		return nil, internalResponseError("contest access snapshot")
	}
	allowed, reason := accessDecision(snapshot, true)
	return &pb.ValidateContestSubmissionResp{
		Allowed: allowed, DenyReason: reason, Mode: pb.ContestMode(snapshot.Mode),
	}, nil
}
