package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CancelContestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelContestLogic {
	return &CancelContestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CancelContestLogic) CancelContest(in *pb.CancelContestReq) (*pb.CancelContestResp, error) {
	// todo: add your logic here and delete this line

	return &pb.CancelContestResp{}, nil
}
