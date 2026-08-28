package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterContestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterContestLogic {
	return &RegisterContestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RegisterContestLogic) RegisterContest(in *pb.RegisterContestReq) (*pb.RegisterContestResp, error) {
	// todo: add your logic here and delete this line

	return &pb.RegisterContestResp{}, nil
}
