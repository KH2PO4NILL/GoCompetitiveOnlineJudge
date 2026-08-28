package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteContestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteContestLogic {
	return &DeleteContestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteContestLogic) DeleteContest(in *pb.DeleteContestReq) (*pb.DeleteContestResp, error) {
	// todo: add your logic here and delete this line

	return &pb.DeleteContestResp{}, nil
}
