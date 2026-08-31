package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetContestForEditLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetContestForEditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContestForEditLogic {
	return &GetContestForEditLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetContestForEditLogic) GetContestForEdit(in *pb.GetContestForEditReq) (*pb.GetContestForEditResp, error) {
	if in == nil || in.ContestId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id must be positive")
	}
	_, contest, err := authorizeContest(l.ctx, l.svcCtx, in.ContestId)
	if err != nil {
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
	convertedContest, err := contestProto(contest)
	if err != nil {
		return nil, err
	}
	convertedProblems := make([]*pb.ContestProblem, 0, len(problems))
	for _, problem := range problems {
		converted, err := contestProblemProto(problem)
		if err != nil {
			return nil, err
		}
		convertedProblems = append(convertedProblems, converted)
	}
	return &pb.GetContestForEditResp{Contest: convertedContest, Problems: convertedProblems}, nil
}
