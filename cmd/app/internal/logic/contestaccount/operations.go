package contestaccount

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/mapper"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"
)

func registerContest(ctx context.Context, svcCtx *svc.ServiceContext, req *types.RegisterContestReq) (*types.RegisterContestResp, error) {
	response, err := svcCtx.Contests.RegisterContest(ctx, &pb.RegisterContestReq{
		ContestId: req.ContestID, InviteCode: req.InviteCode, ApplicationNote: req.ApplicationNote,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("registration response")
	}
	registration, err := mapper.ContestRegistration(response.Registration)
	if err != nil {
		return nil, err
	}
	return &types.RegisterContestResp{Registration: registration}, nil
}

func cancelRegistration(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CancelRegistrationReq) (*types.CancelRegistrationResp, error) {
	response, err := svcCtx.Contests.CancelRegistration(ctx, &pb.CancelRegistrationReq{
		ContestId: req.ContestID, ExpectedRevision: req.ExpectedRevision,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("registration response")
	}
	registration, err := mapper.ContestRegistration(response.Registration)
	if err != nil {
		return nil, err
	}
	return &types.CancelRegistrationResp{Registration: registration}, nil
}

func getMyContestStatus(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GetMyContestStatusReq) (*types.GetMyContestStatusResp, error) {
	response, err := svcCtx.Contests.GetMyContestStatus(ctx, &pb.GetMyContestStatusReq{ContestId: req.ContestID})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("contest status response")
	}
	statusValue, err := mapper.MyContestStatus(response.Status)
	if err != nil {
		return nil, err
	}
	return &types.GetMyContestStatusResp{Status: statusValue}, nil
}
