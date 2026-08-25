package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ArchiveProblemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewArchiveProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ArchiveProblemLogic {
	return &ArchiveProblemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ArchiveProblemLogic) ArchiveProblem(in *pb.ArchiveProblemReq) (*pb.ArchiveProblemResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ArchiveProblemResp{}, nil
}
