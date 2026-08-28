package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMyContestStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMyContestStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMyContestStatusLogic {
	return &GetMyContestStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetMyContestStatusLogic) GetMyContestStatus(in *pb.GetMyContestStatusReq) (*pb.GetMyContestStatusResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetMyContestStatusResp{}, nil
}
