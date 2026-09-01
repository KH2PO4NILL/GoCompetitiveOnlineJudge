package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SetContestProblemsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetContestProblemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetContestProblemsLogic {
	return &SetContestProblemsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SetContestProblemsLogic) SetContestProblems(in *pb.SetContestProblemsReq) (*pb.SetContestProblemsResp, error) {
	if in == nil || in.ContestId <= 0 || in.ExpectedRevision <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id and expected_revision must be positive")
	}
	_, contest, err := authorizeContest(l.ctx, l.svcCtx, in.ContestId)
	if err != nil {
		return nil, err
	}
	problems, err := contestProblemsFromProto(in.Problems)
	if err != nil {
		return nil, err
	}
	for _, problem := range problems {
		if contest.Editable.Mode == model.ContestModeICPC && problem.MaxScore != 0 {
			return nil, status.Error(codes.InvalidArgument, "ICPC problems must have max_score 0")
		}
		if (contest.Editable.Mode == model.ContestModeIOI || contest.Editable.Mode == model.ContestModeOI) && problem.MaxScore <= 0 {
			return nil, status.Error(codes.InvalidArgument, "IOI/OI problems require a positive max_score")
		}
	}
	if err := ensurePublishedProblems(l.ctx, l.svcCtx, problems); err != nil {
		return nil, err
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	updated, storedProblems, err := repository.SetProblems(l.ctx, model.SetContestProblemsInput{
		ContestID: in.ContestId, ExpectedRevision: in.ExpectedRevision, Problems: problems,
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	convertedContest, err := contestProto(updated)
	if err != nil {
		return nil, err
	}
	convertedProblems := make([]*pb.ContestProblem, 0, len(storedProblems))
	for _, problem := range storedProblems {
		converted, err := contestProblemProto(problem)
		if err != nil {
			return nil, err
		}
		convertedProblems = append(convertedProblems, converted)
	}
	return &pb.SetContestProblemsResp{Contest: convertedContest, Problems: convertedProblems}, nil
}
