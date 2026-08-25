package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListProblemsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProblemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProblemsLogic {
	return &ListProblemsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListProblemsLogic) ListProblems(in *pb.ListProblemsReq) (*pb.ListProblemsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ListProblemsResp{}, nil
}
