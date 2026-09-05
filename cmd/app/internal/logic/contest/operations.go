package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/mapper"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"
)

func listContests(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListContestsReq) (*types.ListContestsResp, error) {
	statusValue, err := mapper.ContestStatus(req.Status)
	if err != nil {
		return nil, err
	}
	mode, err := mapper.ContestMode(req.Mode)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Contests.ListContests(ctx, &pb.ListContestsReq{
		Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword, Status: statusValue, Mode: mode,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("contest list")
	}
	items := make([]types.ContestSummary, 0, len(response.Contests))
	for _, item := range response.Contests {
		if item != nil {
			items = append(items, mapper.ContestSummary(item))
		}
	}
	return &types.ListContestsResp{Total: response.Total, Contests: items}, nil
}

func getContest(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GetContestReq) (*types.GetContestResp, error) {
	response, err := svcCtx.Contests.GetContest(ctx, &pb.GetContestReq{ContestId: req.ContestID})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("contest response")
	}
	contest, err := mapper.ContestDetail(response.Contest)
	if err != nil {
		return nil, err
	}
	return &types.GetContestResp{Contest: contest}, nil
}

func listContestProblems(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListContestProblemsReq) (*types.ListContestProblemsResp, error) {
	response, err := svcCtx.Contests.ListContestProblems(ctx, &pb.ListContestProblemsReq{
		ContestId: req.ContestID, Page: req.Page, PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("contest problem list")
	}
	items := make([]types.ContestProblemSummary, 0, len(response.Problems))
	for _, item := range response.Problems {
		if item != nil {
			items = append(items, mapper.ContestProblemSummary(item))
		}
	}
	return &types.ListContestProblemsResp{Total: response.Total, Problems: items}, nil
}

func getScoreboard(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GetScoreboardReq) (*types.GetScoreboardResp, error) {
	response, err := svcCtx.Contests.GetScoreboard(ctx, &pb.GetScoreboardReq{
		ContestId: req.ContestID, Page: req.Page, PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("scoreboard response")
	}
	scoreboard, err := mapper.Scoreboard(response.Scoreboard)
	if err != nil {
		return nil, err
	}
	return &types.GetScoreboardResp{Scoreboard: scoreboard}, nil
}
