package problem

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/mapper"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
)

func listProblems(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListProblemsReq) (*types.ListProblemsResp, error) {
	difficulty, err := mapper.ProblemDifficulty(req.Difficulty)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Problems.ListProblems(ctx, &pb.ListProblemsReq{
		Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword,
		Tags: append([]string(nil), req.Tags...), Difficulty: difficulty,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("problem list")
	}
	items := make([]types.ProblemSummary, 0, len(response.Problems))
	for _, item := range response.Problems {
		if item != nil {
			items = append(items, mapper.ProblemSummary(item))
		}
	}
	return &types.ListProblemsResp{Total: response.Total, Problems: items}, nil
}

func getProblem(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GetProblemReq) (*types.GetProblemResp, error) {
	response, err := svcCtx.Problems.GetProblem(ctx, &pb.GetProblemReq{
		ProblemId: req.ProblemID, ContestId: cloneInt64(req.ContestID),
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("problem response")
	}
	problem, err := mapper.PracticeProblem(response.Problem)
	if err != nil {
		return nil, err
	}
	return &types.GetProblemResp{Problem: problem}, nil
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
