package problem

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/mapper"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/internal/httpbridge"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func listProblems(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListProblemsReq) (*types.ListProblemsResp, error) {
	difficulty, err := mapper.ProblemDifficulty(req.Difficulty)
	if err != nil {
		return nil, err
	}
	statusValue, err := mapper.ProblemStatus(req.Status)
	if err != nil {
		return nil, err
	}
	visibility, err := mapper.ProblemVisibility(req.Visibility)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Problems.ListProblemsForManagement(ctx, &pb.ListProblemsReq{
		Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword, Tags: append([]string(nil), req.Tags...),
		Difficulty: difficulty, Status: statusValue, Visibility: visibility, AuthorId: cloneInt64(req.AuthorID),
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

func createProblem(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreateProblemReq) (*types.CreateProblemResp, error) {
	editable, err := mapper.ProblemEditable(req.Problem)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Problems.CreateProblem(ctx, &pb.CreateProblemReq{Problem: editable})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("problem response")
	}
	problem, err := mapper.Problem(response.Problem)
	if err != nil {
		return nil, err
	}
	return &types.CreateProblemResp{Problem: problem}, nil
}

func getProblemForEdit(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GetProblemForEditReq) (*types.GetProblemForEditResp, error) {
	response, err := svcCtx.Problems.GetProblemForEdit(ctx, &pb.GetProblemForEditReq{ProblemId: req.ProblemID})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("problem response")
	}
	problem, err := mapper.Problem(response.Problem)
	if err != nil {
		return nil, err
	}
	testCases := make([]types.TestCaseInfo, 0, len(response.TestCases))
	for _, item := range response.TestCases {
		if item != nil {
			testCases = append(testCases, mapper.TestCase(item))
		}
	}
	return &types.GetProblemForEditResp{Problem: problem, TestCases: testCases}, nil
}

func updateProblem(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UpdateProblemReq) (*types.UpdateProblemResp, error) {
	// 只构造 update_mask 选中的字段，避免未选择的无效值影响本次 PATCH。
	patch, err := mapper.ProblemPatch(req.Problem, req.UpdateMask)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Problems.UpdateProblem(ctx, &pb.UpdateProblemReq{
		ProblemId: req.ProblemID, Problem: patch,
		UpdateMask:       &fieldmaskpb.FieldMask{Paths: append([]string(nil), req.UpdateMask...)},
		ExpectedRevision: req.ExpectedRevision,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("problem response")
	}
	problem, err := mapper.Problem(response.Problem)
	if err != nil {
		return nil, err
	}
	return &types.UpdateProblemResp{Problem: problem}, nil
}

func publishProblem(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PublishProblemReq) (*types.PublishProblemResp, error) {
	response, err := svcCtx.Problems.PublishProblem(ctx, &pb.PublishProblemReq{ProblemId: req.ProblemID, ExpectedRevision: req.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("problem response")
	}
	problem, err := mapper.Problem(response.Problem)
	if err != nil {
		return nil, err
	}
	return &types.PublishProblemResp{Problem: problem}, nil
}

func archiveProblem(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ArchiveProblemReq) (*types.ArchiveProblemResp, error) {
	response, err := svcCtx.Problems.ArchiveProblem(ctx, &pb.ArchiveProblemReq{ProblemId: req.ProblemID, ExpectedRevision: req.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("problem response")
	}
	problem, err := mapper.Problem(response.Problem)
	if err != nil {
		return nil, err
	}
	return &types.ArchiveProblemResp{Problem: problem}, nil
}

func deleteProblem(ctx context.Context, svcCtx *svc.ServiceContext, req *types.DeleteProblemReq) (*types.DeleteProblemResp, error) {
	response, err := svcCtx.Problems.DeleteProblem(ctx, &pb.DeleteProblemReq{ProblemId: req.ProblemID, ExpectedRevision: req.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("delete problem response")
	}
	return &types.DeleteProblemResp{Revision: response.Revision}, nil
}

func restoreProblem(ctx context.Context, svcCtx *svc.ServiceContext, req *types.RestoreProblemReq) (*types.RestoreProblemResp, error) {
	response, err := svcCtx.Problems.RestoreProblem(ctx, &pb.RestoreProblemReq{ProblemId: req.ProblemID, ExpectedRevision: req.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("problem response")
	}
	problem, err := mapper.Problem(response.Problem)
	if err != nil {
		return nil, err
	}
	return &types.RestoreProblemResp{Problem: problem}, nil
}

func createTestDataUpload(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreateTestDataUploadReq) (*types.CreateTestDataUploadResp, error) {
	response, err := svcCtx.Problems.CreateTestDataUpload(ctx, &pb.CreateTestDataUploadReq{
		ProblemId: req.ProblemID, FileName: req.FileName, ContentLength: req.ContentLength, Sha256: req.SHA256,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("test data upload response")
	}
	return &types.CreateTestDataUploadResp{
		UploadID: response.UploadId, UploadURL: response.UploadUrl, ExpiresAt: httpbridge.Timestamp(response.ExpiresAt),
	}, nil
}

func finalizeTestDataUpload(ctx context.Context, svcCtx *svc.ServiceContext, req *types.FinalizeTestDataUploadReq) (*types.FinalizeTestDataUploadResp, error) {
	response, err := svcCtx.Problems.FinalizeTestDataUpload(ctx, &pb.FinalizeTestDataUploadReq{
		ProblemId: req.ProblemID, UploadId: req.UploadID, Sha256: req.SHA256, ExpectedRevision: req.ExpectedRevision,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("finalize upload response")
	}
	testCases := make([]types.TestCaseInfo, 0, len(response.TestCases))
	for _, item := range response.TestCases {
		if item != nil {
			testCases = append(testCases, mapper.TestCase(item))
		}
	}
	return &types.FinalizeTestDataUploadResp{
		TestDataVersion: response.TestDataVersion, Revision: response.Revision, TestCases: testCases,
	}, nil
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
