package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AcquireJudgeTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAcquireJudgeTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AcquireJudgeTaskLogic {
	return &AcquireJudgeTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AcquireJudgeTaskLogic) AcquireJudgeTask(in *pb.AcquireJudgeTaskReq) (*pb.AcquireJudgeTaskResp, error) {
	// todo: add your logic here and delete this line

	return &pb.AcquireJudgeTaskResp{}, nil
}
