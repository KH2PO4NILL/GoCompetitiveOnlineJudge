package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRegistrationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRegistrationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRegistrationsLogic {
	return &ListRegistrationsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListRegistrationsLogic) ListRegistrations(in *pb.ListRegistrationsReq) (*pb.ListRegistrationsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ListRegistrationsResp{}, nil
}
