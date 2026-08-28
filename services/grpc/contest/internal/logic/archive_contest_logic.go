package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ArchiveContestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewArchiveContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ArchiveContestLogic {
	return &ArchiveContestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ArchiveContestLogic) ArchiveContest(in *pb.ArchiveContestReq) (*pb.ArchiveContestResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ArchiveContestResp{}, nil
}
