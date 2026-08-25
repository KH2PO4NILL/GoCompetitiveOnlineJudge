package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RestoreProblemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRestoreProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RestoreProblemLogic {
	return &RestoreProblemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RestoreProblemLogic) RestoreProblem(in *pb.RestoreProblemReq) (*pb.RestoreProblemResp, error) {
	// todo: add your logic here and delete this line

	return &pb.RestoreProblemResp{}, nil
}
