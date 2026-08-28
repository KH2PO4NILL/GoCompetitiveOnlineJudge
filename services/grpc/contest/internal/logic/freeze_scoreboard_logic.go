package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type FreezeScoreboardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFreezeScoreboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FreezeScoreboardLogic {
	return &FreezeScoreboardLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *FreezeScoreboardLogic) FreezeScoreboard(in *pb.FreezeScoreboardReq) (*pb.FreezeScoreboardResp, error) {
	// todo: add your logic here and delete this line

	return &pb.FreezeScoreboardResp{}, nil
}
