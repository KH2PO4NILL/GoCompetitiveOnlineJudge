package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PublishContestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishContestLogic {
	return &PublishContestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PublishContestLogic) PublishContest(in *pb.PublishContestReq) (*pb.PublishContestResp, error) {
	if in == nil || in.ContestId <= 0 || in.ExpectedRevision <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id and expected_revision must be positive")
	}
	if _, _, err := authorizeContest(l.ctx, l.svcCtx, in.ContestId); err != nil {
		return nil, err
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	problems, err := repository.ListProblems(l.ctx, in.ContestId)
	if err != nil {
		return nil, mapModelError(err)
	}
	if len(problems) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "contest must contain at least one problem")
	}
	if err := ensurePublishedProblems(l.ctx, l.svcCtx, problems); err != nil {
		return nil, err
	}
	contest, err := repository.Publish(l.ctx, in.ContestId, in.ExpectedRevision)
	if err != nil {
		return nil, mapModelError(err)
	}
	converted, err := contestProto(contest)
	if err != nil {
		return nil, err
	}
	return &pb.PublishContestResp{Contest: converted}, nil
}
