package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RestoreProblemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRestoreProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RestoreProblemLogic {
	return &RestoreProblemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RestoreProblemLogic) RestoreProblem(in *pb.RestoreProblemReq) (*pb.RestoreProblemResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validatePositiveProblemRevision(in.ProblemId, in.ExpectedRevision); err != nil {
		return nil, err
	}
	principal, err := requireAdmin(l.ctx)
	if err != nil {
		return nil, err
	}
	problem, err := l.svcCtx.ProblemModel.Restore(l.ctx, in.ProblemId, in.ExpectedRevision, principal.UserID)
	if err != nil {
		return nil, mapModelError(err)
	}
	responseProblem, err := managementProblemProto(l.ctx, l.svcCtx, problem)
	if err != nil {
		return nil, err
	}
	return &pb.RestoreProblemResp{Problem: responseProblem}, nil
}
