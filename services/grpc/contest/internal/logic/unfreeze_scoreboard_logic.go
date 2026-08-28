package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnfreezeScoreboardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnfreezeScoreboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnfreezeScoreboardLogic {
	return &UnfreezeScoreboardLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UnfreezeScoreboardLogic) UnfreezeScoreboard(in *pb.UnfreezeScoreboardReq) (*pb.UnfreezeScoreboardResp, error) {
	// todo: add your logic here and delete this line

	return &pb.UnfreezeScoreboardResp{}, nil
}
