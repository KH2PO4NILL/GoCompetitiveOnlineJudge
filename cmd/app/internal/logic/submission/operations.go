package submission

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/mapper"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
)

func createSubmission(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreateSubmissionReq) (*types.CreateSubmissionResp, error) {
	response, err := svcCtx.Submissions.CreateSubmission(ctx, &pb.CreateSubmissionReq{
		ProblemId: req.ProblemID, ContestId: cloneInt64(req.ContestID), Language: req.Language,
		SourceCode: req.SourceCode, ClientRequestId: req.ClientRequestID,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("submission response")
	}
	submission, err := mapper.SubmissionDetail(response.Submission)
	if err != nil {
		return nil, err
	}
	return &types.CreateSubmissionResp{Submission: submission, Deduplicated: response.Deduplicated}, nil
}

func getSubmission(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GetSubmissionReq) (*types.GetSubmissionResp, error) {
	response, err := svcCtx.Submissions.GetSubmission(ctx, &pb.GetSubmissionReq{SubmissionId: req.SubmissionID})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("submission response")
	}
	submission, err := mapper.SubmissionDetail(response.Submission)
	if err != nil {
		return nil, err
	}
	return &types.GetSubmissionResp{Submission: submission}, nil
}

func listSubmissions(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListSubmissionsReq) (*types.ListSubmissionsResp, error) {
	statusValue, err := mapper.SubmissionStatus(req.Status)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Submissions.ListSubmissions(ctx, &pb.ListSubmissionsReq{
		Page: req.Page, PageSize: req.PageSize, ProblemId: cloneInt64(req.ProblemID),
		ContestId: cloneInt64(req.ContestID), Status: statusValue, Language: req.Language,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("submission list")
	}
	items := make([]types.SubmissionSummary, 0, len(response.Submissions))
	for _, item := range response.Submissions {
		if item != nil {
			items = append(items, mapper.SubmissionSummary(item))
		}
	}
	return &types.ListSubmissionsResp{Total: response.Total, Submissions: items}, nil
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
