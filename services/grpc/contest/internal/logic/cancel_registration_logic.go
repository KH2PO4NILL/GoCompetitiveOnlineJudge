package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CancelRegistrationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelRegistrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelRegistrationLogic {
	return &CancelRegistrationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CancelRegistrationLogic) CancelRegistration(in *pb.CancelRegistrationReq) (*pb.CancelRegistrationResp, error) {
	// todo: add your logic here and delete this line

	return &pb.CancelRegistrationResp{}, nil
}
