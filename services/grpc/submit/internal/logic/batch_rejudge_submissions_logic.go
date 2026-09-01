package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchRejudgeSubmissionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchRejudgeSubmissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchRejudgeSubmissionsLogic {
	return &BatchRejudgeSubmissionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BatchRejudgeSubmissionsLogic) BatchRejudgeSubmissions(in *pb.BatchRejudgeSubmissionsReq) (*pb.BatchRejudgeSubmissionsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.BatchRejudgeSubmissionsResp{}, nil
}
