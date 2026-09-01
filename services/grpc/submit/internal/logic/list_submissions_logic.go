package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSubmissionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSubmissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSubmissionsLogic {
	return &ListSubmissionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListSubmissionsLogic) ListSubmissions(in *pb.ListSubmissionsReq) (*pb.ListSubmissionsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ListSubmissionsResp{}, nil
}
