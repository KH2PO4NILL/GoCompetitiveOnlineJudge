package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchGetProblemsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchGetProblemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchGetProblemsLogic {
	return &BatchGetProblemsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 仅允许受信任的内部服务调用。
func (l *BatchGetProblemsLogic) BatchGetProblems(in *pb.BatchGetProblemsReq) (*pb.BatchGetProblemsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.BatchGetProblemsResp{}, nil
}
