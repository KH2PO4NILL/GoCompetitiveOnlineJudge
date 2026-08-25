package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateTestDataUploadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTestDataUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTestDataUploadLogic {
	return &CreateTestDataUploadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateTestDataUploadLogic) CreateTestDataUpload(in *pb.CreateTestDataUploadReq) (*pb.CreateTestDataUploadResp, error) {
	// todo: add your logic here and delete this line

	return &pb.CreateTestDataUploadResp{}, nil
}
