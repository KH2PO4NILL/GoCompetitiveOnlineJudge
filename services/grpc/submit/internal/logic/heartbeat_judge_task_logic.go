package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type HeartbeatJudgeTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHeartbeatJudgeTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HeartbeatJudgeTaskLogic {
	return &HeartbeatJudgeTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HeartbeatJudgeTaskLogic) HeartbeatJudgeTask(in *pb.HeartbeatJudgeTaskReq) (*pb.HeartbeatJudgeTaskResp, error) {
	// todo: add your logic here and delete this line

	return &pb.HeartbeatJudgeTaskResp{}, nil
}
