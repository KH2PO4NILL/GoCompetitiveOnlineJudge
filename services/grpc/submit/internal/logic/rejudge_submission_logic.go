package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RejudgeSubmissionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRejudgeSubmissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RejudgeSubmissionLogic {
	return &RejudgeSubmissionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RejudgeSubmissionLogic) RejudgeSubmission(in *pb.RejudgeSubmissionReq) (*pb.RejudgeSubmissionResp, error) {
	// todo: add your logic here and delete this line

	return &pb.RejudgeSubmissionResp{}, nil
}
