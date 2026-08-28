package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RestoreContestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRestoreContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RestoreContestLogic {
	return &RestoreContestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RestoreContestLogic) RestoreContest(in *pb.RestoreContestReq) (*pb.RestoreContestResp, error) {
	// todo: add your logic here and delete this line

	return &pb.RestoreContestResp{}, nil
}
