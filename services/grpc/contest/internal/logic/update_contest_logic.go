package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateContestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateContestLogic {
	return &UpdateContestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateContestLogic) UpdateContest(in *pb.UpdateContestReq) (*pb.UpdateContestResp, error) {
	// todo: add your logic here and delete this line

	return &pb.UpdateContestResp{}, nil
}
