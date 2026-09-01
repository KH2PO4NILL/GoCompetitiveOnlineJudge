package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetContestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContestLogic {
	return &GetContestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetContestLogic) GetContest(in *pb.GetContestReq) (*pb.GetContestResp, error) {
	if in == nil || in.ContestId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id must be positive")
	}
	visible, err := loadVisibleContest(l.ctx, l.svcCtx, in.ContestId)
	if err != nil {
		return nil, err
	}
	contest, err := contestDetailProto(visible.Detail)
	if err != nil {
		return nil, err
	}
	return &pb.GetContestResp{Contest: contest}, nil
}
