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

type CanAccessProblemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCanAccessProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CanAccessProblemLogic {
	return &CanAccessProblemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 受信任内部服务接口。预期业务拒绝使用响应字段，系统故障使用 gRPC status。
func (l *CanAccessProblemLogic) CanAccessProblem(in *pb.CanAccessProblemReq) (*pb.CanAccessProblemResp, error) {
	if err := requireService(l.ctx, "problem.rpc"); err != nil {
		return nil, err
	}
	if in == nil || in.ContestId <= 0 || in.ProblemId <= 0 || in.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id, problem_id and user_id must be positive")
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	snapshot, err := repository.GetAccessSnapshot(l.ctx, in.ContestId, in.ProblemId, in.UserId, nowUTC(l.svcCtx))
	if errors.Is(err, model.ErrContestNotFound) {
		return &pb.CanAccessProblemResp{
			Allowed: false, DenyReason: pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNAUTHORIZED,
		}, nil
	}
	if err != nil {
		return nil, mapModelError(err)
	}
	if snapshot == nil {
		return nil, internalResponseError("contest access snapshot")
	}
	if in.RequesterIsAdmin {
		adminSnapshot := *snapshot
		adminSnapshot.Manager = true
		snapshot = &adminSnapshot
	}
	allowed, reason := accessDecision(snapshot, false)
	return &pb.CanAccessProblemResp{Allowed: allowed, DenyReason: reason}, nil
}
