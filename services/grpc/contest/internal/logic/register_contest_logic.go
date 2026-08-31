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

type RegisterContestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterContestLogic {
	return &RegisterContestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RegisterContestLogic) RegisterContest(in *pb.RegisterContestReq) (*pb.RegisterContestResp, error) {
	if in == nil || in.ContestId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id must be positive")
	}
	if err := validateUTF8(in.ApplicationNote, "application_note", 2000, false); err != nil {
		return nil, err
	}
	if err := validateUTF8(in.InviteCode, "invite_code", 64, false); err != nil || len(in.InviteCode) > 72 {
		if err != nil {
			return nil, err
		}
		return nil, status.Error(codes.InvalidArgument, "invite_code must not exceed 72 bytes")
	}
	principal, err := requirePrincipal(l.ctx)
	if err != nil {
		return nil, err
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	registration, _, err := repository.Register(l.ctx, model.RegisterContestInput{
		ContestID: in.ContestId, UserID: principal.UserID, InviteCode: in.InviteCode,
		ApplicationNote: in.ApplicationNote, At: nowUTC(l.svcCtx),
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	converted, err := registrationProto(registration)
	if err != nil {
		return nil, err
	}
	if converted == nil {
		return nil, internalResponseError("registration")
	}
	return &pb.RegisterContestResp{Registration: converted}, nil
}
