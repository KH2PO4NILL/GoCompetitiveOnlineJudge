package submission

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/mapper"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
)

func listSubmissions(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListSubmissionsReq) (*types.ListSubmissionsResp, error) {
	statusValue, err := mapper.SubmissionStatus(req.Status)
	if err != nil {
		return nil, err
	}
	verdict, err := mapper.Verdict(req.Verdict)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Submissions.ListSubmissions(ctx, &pb.ListSubmissionsReq{
		Page: req.Page, PageSize: req.PageSize, UserId: cloneInt64(req.UserID),
		ProblemId: cloneInt64(req.ProblemID), ContestId: cloneInt64(req.ContestID),
		Status: statusValue, Verdict: verdict, Language: req.Language,
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

func rejudgeSubmission(ctx context.Context, svcCtx *svc.ServiceContext, req *types.RejudgeSubmissionReq) (*types.RejudgeSubmissionResp, error) {
	response, err := svcCtx.Submissions.RejudgeSubmission(ctx, &pb.RejudgeSubmissionReq{
		SubmissionId: req.SubmissionID, Reason: req.Reason,
	})
	if err != nil {
		return nil, err
	}
	if response == nil || response.Submission == nil {
		return nil, mapper.InvalidRPCResponse("rejudge response")
	}
	return &types.RejudgeSubmissionResp{
		Submission: mapper.SubmissionSummary(response.Submission), AttemptID: response.AttemptId,
	}, nil
}

func batchRejudgeSubmissions(ctx context.Context, svcCtx *svc.ServiceContext, req *types.BatchRejudgeSubmissionsReq) (*types.BatchRejudgeSubmissionsResp, error) {
	response, err := svcCtx.Submissions.BatchRejudgeSubmissions(ctx, &pb.BatchRejudgeSubmissionsReq{
		SubmissionIds: append([]int64(nil), req.SubmissionIDs...), Reason: req.Reason,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("batch rejudge response")
	}
	accepted := make([]types.BatchRejudgeItem, 0, len(response.Accepted))
	for _, item := range response.Accepted {
		if item != nil {
			accepted = append(accepted, types.BatchRejudgeItem{SubmissionID: item.SubmissionId, AttemptID: item.AttemptId})
		}
	}
	return &types.BatchRejudgeSubmissionsResp{Accepted: accepted}, nil
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
