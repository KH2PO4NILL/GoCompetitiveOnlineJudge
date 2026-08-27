package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ArchiveProblemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewArchiveProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ArchiveProblemLogic {
	return &ArchiveProblemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ArchiveProblemLogic) ArchiveProblem(in *pb.ArchiveProblemReq) (*pb.ArchiveProblemResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validatePositiveProblemRevision(in.ProblemId, in.ExpectedRevision); err != nil {
		return nil, err
	}
	principal, _, err := authorizeProblem(l.ctx, l.svcCtx, in.ProblemId)
	if err != nil {
		return nil, err
	}
	problem, err := l.svcCtx.ProblemModel.Archive(l.ctx, in.ProblemId, in.ExpectedRevision, principal.UserID)
	if err != nil {
		return nil, mapModelError(err)
	}
	responseProblem, err := managementProblemProto(l.ctx, l.svcCtx, problem)
	if err != nil {
		return nil, err
	}
	return &pb.ArchiveProblemResp{Problem: responseProblem}, nil
}
