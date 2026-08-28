package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListParticipantsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListParticipantsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListParticipantsLogic {
	return &ListParticipantsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListParticipantsLogic) ListParticipants(in *pb.ListParticipantsReq) (*pb.ListParticipantsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ListParticipantsResp{}, nil
}
