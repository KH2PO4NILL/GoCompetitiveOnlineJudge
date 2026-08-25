package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProblemForEditLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProblemForEditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProblemForEditLogic {
	return &GetProblemForEditLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProblemForEditLogic) GetProblemForEdit(in *pb.GetProblemForEditReq) (*pb.GetProblemForEditResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetProblemForEditResp{}, nil
}
