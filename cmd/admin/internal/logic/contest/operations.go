package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/mapper"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"google.golang.org/protobuf/types/known/fieldmaskpb"
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
	response, err := svcCtx.Contests.ListContestsForManagement(ctx, &pb.ListContestsReq{
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

func createContest(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreateContestReq) (*types.CreateContestResp, error) {
	editable, err := mapper.ContestEditable(req.Contest)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Contests.CreateContest(ctx, &pb.CreateContestReq{Contest: editable})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("contest response")
	}
	contest, err := mapper.Contest(response.Contest)
	if err != nil {
		return nil, err
	}
	return &types.CreateContestResp{Contest: contest}, nil
}

func getContestForEdit(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GetContestForEditReq) (*types.GetContestForEditResp, error) {
	response, err := svcCtx.Contests.GetContestForEdit(ctx, &pb.GetContestForEditReq{ContestId: req.ContestID})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("contest response")
	}
	contest, err := mapper.Contest(response.Contest)
	if err != nil {
		return nil, err
	}
	problems := make([]types.ContestProblem, 0, len(response.Problems))
	for _, item := range response.Problems {
		if item != nil {
			problems = append(problems, mapper.ContestProblem(item))
		}
	}
	return &types.GetContestForEditResp{Contest: contest, Problems: problems}, nil
}

func updateContest(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UpdateContestReq) (*types.UpdateContestResp, error) {
	// HTTP 的秒数字段名与 Proto Duration 字段名不同，先统一为 Proto 掩码路径。
	updateMask := mapper.ContestUpdateMask(req.UpdateMask)
	patch, err := mapper.ContestPatch(req.Contest, updateMask)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Contests.UpdateContest(ctx, &pb.UpdateContestReq{
		ContestId: req.ContestID, Contest: patch,
		UpdateMask:       &fieldmaskpb.FieldMask{Paths: updateMask},
		ExpectedRevision: req.ExpectedRevision,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("contest response")
	}
	contest, err := mapper.Contest(response.Contest)
	if err != nil {
		return nil, err
	}
	return &types.UpdateContestResp{Contest: contest}, nil
}

func publishContest(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PublishContestReq) (*types.PublishContestResp, error) {
	response, err := svcCtx.Contests.PublishContest(ctx, &pb.PublishContestReq{ContestId: req.ContestID, ExpectedRevision: req.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	contest, err := contestFromResponse(response)
	if err != nil {
		return nil, err
	}
	return &types.PublishContestResp{Contest: contest}, nil
}

func cancelContest(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CancelContestReq) (*types.CancelContestResp, error) {
	response, err := svcCtx.Contests.CancelContest(ctx, &pb.CancelContestReq{
		ContestId: req.ContestID, ExpectedRevision: req.ExpectedRevision, Reason: req.Reason,
	})
	if err != nil {
		return nil, err
	}
	contest, err := contestFromResponse(response)
	if err != nil {
		return nil, err
	}
	return &types.CancelContestResp{Contest: contest}, nil
}

func archiveContest(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ArchiveContestReq) (*types.ArchiveContestResp, error) {
	response, err := svcCtx.Contests.ArchiveContest(ctx, &pb.ArchiveContestReq{ContestId: req.ContestID, ExpectedRevision: req.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	contest, err := contestFromResponse(response)
	if err != nil {
		return nil, err
	}
	return &types.ArchiveContestResp{Contest: contest}, nil
}

func deleteContest(ctx context.Context, svcCtx *svc.ServiceContext, req *types.DeleteContestReq) (*types.DeleteContestResp, error) {
	response, err := svcCtx.Contests.DeleteContest(ctx, &pb.DeleteContestReq{ContestId: req.ContestID, ExpectedRevision: req.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("delete contest response")
	}
	return &types.DeleteContestResp{Revision: response.Revision}, nil
}

func restoreContest(ctx context.Context, svcCtx *svc.ServiceContext, req *types.RestoreContestReq) (*types.RestoreContestResp, error) {
	response, err := svcCtx.Contests.RestoreContest(ctx, &pb.RestoreContestReq{ContestId: req.ContestID, ExpectedRevision: req.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	contest, err := contestFromResponse(response)
	if err != nil {
		return nil, err
	}
	return &types.RestoreContestResp{Contest: contest}, nil
}

func setContestProblems(ctx context.Context, svcCtx *svc.ServiceContext, req *types.SetContestProblemsReq) (*types.SetContestProblemsResp, error) {
	problems := make([]*pb.ContestProblem, 0, len(req.Problems))
	for _, item := range req.Problems {
		problems = append(problems, mapper.ContestProblemRequest(item))
	}
	response, err := svcCtx.Contests.SetContestProblems(ctx, &pb.SetContestProblemsReq{
		ContestId: req.ContestID, Problems: problems, ExpectedRevision: req.ExpectedRevision,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("set contest problems response")
	}
	contest, err := mapper.Contest(response.Contest)
	if err != nil {
		return nil, err
	}
	resultProblems := make([]types.ContestProblem, 0, len(response.Problems))
	for _, item := range response.Problems {
		if item != nil {
			resultProblems = append(resultProblems, mapper.ContestProblem(item))
		}
	}
	return &types.SetContestProblemsResp{Contest: contest, Problems: resultProblems}, nil
}

func listRegistrations(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListRegistrationsReq) (*types.ListRegistrationsResp, error) {
	statusValue, err := mapper.RegistrationStatus(req.Status)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Contests.ListRegistrations(ctx, &pb.ListRegistrationsReq{
		ContestId: req.ContestID, Page: req.Page, PageSize: req.PageSize, Status: statusValue,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("registration list")
	}
	items := make([]types.ContestRegistration, 0, len(response.Registrations))
	for _, item := range response.Registrations {
		registration, err := mapper.ContestRegistration(item)
		if err != nil {
			return nil, err
		}
		items = append(items, registration)
	}
	return &types.ListRegistrationsResp{Total: response.Total, Registrations: items}, nil
}

func reviewRegistration(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ReviewRegistrationReq) (*types.ReviewRegistrationResp, error) {
	decision, err := mapper.ReviewDecision(req.Decision)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Contests.ReviewRegistration(ctx, &pb.ReviewRegistrationReq{
		ContestId: req.ContestID, RegistrationId: req.RegistrationID, Decision: decision,
		ReviewNote: req.ReviewNote, ExpectedRevision: req.ExpectedRevision,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("review registration response")
	}
	registration, err := mapper.ContestRegistration(response.Registration)
	if err != nil {
		return nil, err
	}
	// reject 时 participant 保持 nil，HTTP JSON 返回 null，不伪造零值参赛者。
	return &types.ReviewRegistrationResp{
		Registration: registration, Participant: mapper.ContestParticipant(response.Participant),
	}, nil
}

func listParticipants(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListParticipantsReq) (*types.ListParticipantsResp, error) {
	statusValue, err := mapper.ParticipantStatus(req.Status)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Contests.ListParticipants(ctx, &pb.ListParticipantsReq{
		ContestId: req.ContestID, Page: req.Page, PageSize: req.PageSize, Status: statusValue,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("participant list")
	}
	items := make([]types.ContestParticipant, 0, len(response.Participants))
	for _, item := range response.Participants {
		if converted := mapper.ContestParticipant(item); converted != nil {
			items = append(items, *converted)
		}
	}
	return &types.ListParticipantsResp{Total: response.Total, Participants: items}, nil
}

func freezeScoreboard(ctx context.Context, svcCtx *svc.ServiceContext, req *types.FreezeScoreboardReq) (*types.FreezeScoreboardResp, error) {
	response, err := svcCtx.Contests.FreezeScoreboard(ctx, &pb.FreezeScoreboardReq{ContestId: req.ContestID, ExpectedRevision: req.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	contest, err := contestFromResponse(response)
	if err != nil {
		return nil, err
	}
	return &types.FreezeScoreboardResp{Contest: contest}, nil
}

func unfreezeScoreboard(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UnfreezeScoreboardReq) (*types.UnfreezeScoreboardResp, error) {
	response, err := svcCtx.Contests.UnfreezeScoreboard(ctx, &pb.UnfreezeScoreboardReq{ContestId: req.ContestID, ExpectedRevision: req.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	contest, err := contestFromResponse(response)
	if err != nil {
		return nil, err
	}
	return &types.UnfreezeScoreboardResp{Contest: contest}, nil
}

type contestResponse interface {
	GetContest() *pb.Contest
}

func contestFromResponse(response contestResponse) (types.Contest, error) {
	if response == nil {
		return types.Contest{}, mapper.InvalidRPCResponse("contest response")
	}
	return mapper.Contest(response.GetContest())
}
