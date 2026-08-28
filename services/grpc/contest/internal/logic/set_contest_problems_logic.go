package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetContestProblemsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetContestProblemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetContestProblemsLogic {
	return &SetContestProblemsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SetContestProblemsLogic) SetContestProblems(in *pb.SetContestProblemsReq) (*pb.SetContestProblemsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.SetContestProblemsResp{}, nil
}
