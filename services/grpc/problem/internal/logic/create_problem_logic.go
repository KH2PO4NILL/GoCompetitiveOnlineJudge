package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateProblemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateProblemLogic {
	return &CreateProblemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 出题人或管理员接口，权限必须由 gRPC metadata 中的鉴权信息决定。

func (l *CreateProblemLogic) CreateProblem(in *pb.CreateProblemReq) (*pb.CreateProblemResp, error) {
	// todo: add your logic here and delete this line

	return &pb.CreateProblemResp{}, nil
}
