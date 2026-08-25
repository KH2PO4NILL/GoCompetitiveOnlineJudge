package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProblemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProblemLogic {
	return &GetProblemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 普通用户接口。服务端必须根据题目状态、可见性和比赛权限过滤结果。
func (l *GetProblemLogic) GetProblem(in *pb.GetProblemReq) (*pb.GetProblemResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetProblemResp{}, nil
}
