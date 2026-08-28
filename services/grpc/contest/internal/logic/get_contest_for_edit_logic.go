package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetContestForEditLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetContestForEditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContestForEditLogic {
	return &GetContestForEditLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetContestForEditLogic) GetContestForEdit(in *pb.GetContestForEditReq) (*pb.GetContestForEditResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetContestForEditResp{}, nil
}
