package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReportJudgeResultLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReportJudgeResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReportJudgeResultLogic {
	return &ReportJudgeResultLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ReportJudgeResultLogic) ReportJudgeResult(in *pb.ReportJudgeResultReq) (*pb.ReportJudgeResultResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ReportJudgeResultResp{}, nil
}
