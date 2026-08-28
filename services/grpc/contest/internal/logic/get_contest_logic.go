package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetContestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContestLogic {
	return &GetContestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetContestLogic) GetContest(in *pb.GetContestReq) (*pb.GetContestResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetContestResp{}, nil
}
