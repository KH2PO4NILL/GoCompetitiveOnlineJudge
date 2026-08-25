package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type FinalizeTestDataUploadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFinalizeTestDataUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FinalizeTestDataUploadLogic {
	return &FinalizeTestDataUploadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *FinalizeTestDataUploadLogic) FinalizeTestDataUpload(in *pb.FinalizeTestDataUploadReq) (*pb.FinalizeTestDataUploadResp, error) {
	// todo: add your logic here and delete this line

	return &pb.FinalizeTestDataUploadResp{}, nil
}
