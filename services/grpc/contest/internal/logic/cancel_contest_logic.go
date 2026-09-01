package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CancelContestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelContestLogic {
	return &CancelContestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CancelContestLogic) CancelContest(in *pb.CancelContestReq) (*pb.CancelContestResp, error) {
	if in == nil || in.ContestId <= 0 || in.ExpectedRevision <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id and expected_revision must be positive")
	}
	if err := validateUTF8(in.Reason, "reason", 500, false); err != nil {
		return nil, err
	}
	if _, _, err := authorizeContest(l.ctx, l.svcCtx, in.ContestId); err != nil {
		return nil, err
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	contest, err := repository.Cancel(l.ctx, in.ContestId, in.ExpectedRevision, in.Reason)
	if err != nil {
		return nil, mapModelError(err)
	}
	converted, err := contestProto(contest)
	if err != nil {
		return nil, err
	}
	return &pb.CancelContestResp{Contest: converted}, nil
}
