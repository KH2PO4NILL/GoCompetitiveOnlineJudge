package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CancelRegistrationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelRegistrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelRegistrationLogic {
	return &CancelRegistrationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CancelRegistrationLogic) CancelRegistration(in *pb.CancelRegistrationReq) (*pb.CancelRegistrationResp, error) {
	if in == nil || in.ContestId <= 0 || in.ExpectedRevision <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id and expected_revision must be positive")
	}
	principal, err := requirePrincipal(l.ctx)
	if err != nil {
		return nil, err
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	registration, err := repository.CancelRegistration(l.ctx, in.ContestId, principal.UserID, in.ExpectedRevision)
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
	return &pb.CancelRegistrationResp{Registration: converted}, nil
}
