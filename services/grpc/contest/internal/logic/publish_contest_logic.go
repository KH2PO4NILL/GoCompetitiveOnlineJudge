package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishContestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishContestLogic {
	return &PublishContestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PublishContestLogic) PublishContest(in *pb.PublishContestReq) (*pb.PublishContestResp, error) {
	// todo: add your logic here and delete this line

	return &pb.PublishContestResp{}, nil
}
