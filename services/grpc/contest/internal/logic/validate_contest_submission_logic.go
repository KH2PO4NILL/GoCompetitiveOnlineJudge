package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ValidateContestSubmissionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateContestSubmissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateContestSubmissionLogic {
	return &ValidateContestSubmissionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ValidateContestSubmissionLogic) ValidateContestSubmission(in *pb.ValidateContestSubmissionReq) (*pb.ValidateContestSubmissionResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ValidateContestSubmissionResp{}, nil
}
