package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishProblemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishProblemLogic {
	return &PublishProblemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PublishProblemLogic) PublishProblem(in *pb.PublishProblemReq) (*pb.PublishProblemResp, error) {
	// todo: add your logic here and delete this line

	return &pb.PublishProblemResp{}, nil
}
