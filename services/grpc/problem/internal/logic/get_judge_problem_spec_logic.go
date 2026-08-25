package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetJudgeProblemSpecLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetJudgeProblemSpecLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetJudgeProblemSpecLogic {
	return &GetJudgeProblemSpecLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetJudgeProblemSpecLogic) GetJudgeProblemSpec(in *pb.GetJudgeProblemSpecReq) (*pb.GetJudgeProblemSpecResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetJudgeProblemSpecResp{}, nil
}
