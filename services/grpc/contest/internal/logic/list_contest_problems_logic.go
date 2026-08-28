package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListContestProblemsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListContestProblemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListContestProblemsLogic {
	return &ListContestProblemsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListContestProblemsLogic) ListContestProblems(in *pb.ListContestProblemsReq) (*pb.ListContestProblemsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ListContestProblemsResp{}, nil
}
