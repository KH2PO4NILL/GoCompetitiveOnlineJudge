package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListContestsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListContestsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListContestsLogic {
	return &ListContestsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 用户侧查询与自助操作。
func (l *ListContestsLogic) ListContests(in *pb.ListContestsReq) (*pb.ListContestsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ListContestsResp{}, nil
}
