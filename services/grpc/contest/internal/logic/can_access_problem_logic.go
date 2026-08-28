package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CanAccessProblemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCanAccessProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CanAccessProblemLogic {
	return &CanAccessProblemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 受信任内部服务接口。预期业务拒绝使用响应字段，系统故障使用 gRPC status。
func (l *CanAccessProblemLogic) CanAccessProblem(in *pb.CanAccessProblemReq) (*pb.CanAccessProblemResp, error) {
	// todo: add your logic here and delete this line

	return &pb.CanAccessProblemResp{}, nil
}
