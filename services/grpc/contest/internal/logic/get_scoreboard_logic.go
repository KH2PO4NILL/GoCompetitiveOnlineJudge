package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetScoreboardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetScoreboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetScoreboardLogic {
	return &GetScoreboardLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetScoreboardLogic) GetScoreboard(in *pb.GetScoreboardReq) (*pb.GetScoreboardResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetScoreboardResp{}, nil
}
