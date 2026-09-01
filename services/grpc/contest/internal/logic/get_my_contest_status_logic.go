package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetMyContestStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMyContestStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMyContestStatusLogic {
	return &GetMyContestStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetMyContestStatusLogic) GetMyContestStatus(in *pb.GetMyContestStatusReq) (*pb.GetMyContestStatusResp, error) {
	if in == nil || in.ContestId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id must be positive")
	}
	principal, err := requirePrincipal(l.ctx)
	if err != nil {
		return nil, err
	}
	visible, err := loadVisibleContest(l.ctx, l.svcCtx, in.ContestId)
	if err != nil {
		return nil, err
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	myStatus := visible.MyStatus
	if myStatus == nil {
		myStatus, err = repository.GetMyStatus(l.ctx, in.ContestId, principal.UserID)
		if err != nil {
			return nil, mapModelError(err)
		}
	}
	problems, err := repository.ListProblems(l.ctx, in.ContestId)
	if err != nil {
		return nil, mapModelError(err)
	}
	snapshot := statusAccessSnapshot(visible.Detail, myStatus, len(problems) > 0, visible.Manager)
	canView, _ := accessDecision(snapshot, false)
	canSubmit, _ := accessDecision(snapshot, true)
	converted, err := myContestStatusProto(myStatus, canView, canSubmit)
	if err != nil {
		return nil, err
	}
	return &pb.GetMyContestStatusResp{Status: converted}, nil
}
