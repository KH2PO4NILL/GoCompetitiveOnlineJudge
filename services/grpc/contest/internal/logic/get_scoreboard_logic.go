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

type GetScoreboardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetScoreboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetScoreboardLogic {
	return &GetScoreboardLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetScoreboardLogic) GetScoreboard(in *pb.GetScoreboardReq) (*pb.GetScoreboardResp, error) {
	if in == nil || in.ContestId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id must be positive")
	}
	page, pageSize, err := normalizePagination(in.Page, in.PageSize)
	if err != nil {
		return nil, err
	}
	visible, err := loadVisibleContest(l.ctx, l.svcCtx, in.ContestId)
	if err != nil {
		return nil, err
	}
	var myStatus *model.MyContestStatus
	if visible.HasPrincipal {
		myStatus = visible.MyStatus
		if myStatus == nil && !visible.Manager {
			myStatus, err = loadMyStatus(l.ctx, l.svcCtx, in.ContestId, visible.Principal.UserID)
			if err != nil {
				return nil, err
			}
		}
	}
	switch visible.Detail.ScoreboardVisibility {
	case model.ScoreboardVisibilityPublic:
	case model.ScoreboardVisibilityParticipantsOnly:
		if !visible.Manager && !normalParticipant(myStatus) {
			return nil, status.Error(codes.PermissionDenied, "scoreboard participation is required")
		}
	case model.ScoreboardVisibilityHidden:
		if !visible.Manager {
			return nil, status.Error(codes.PermissionDenied, "scoreboard is hidden")
		}
	default:
		return nil, status.Error(codes.Internal, "scoreboard visibility is invalid")
	}
	var view int64
	switch {
	case visible.Detail.ScoreboardFrozen:
		view = model.ScoreboardViewFrozenPublic
	case visible.Detail.Status == model.ContestStatusFinished || visible.Detail.Status == model.ContestStatusArchived:
		view = model.ScoreboardViewFinal
	case visible.Detail.Status == model.ContestStatusRunning:
		view = model.ScoreboardViewLive
	default:
		return nil, status.Error(codes.FailedPrecondition, "scoreboard is not available")
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	scoreboard, err := repository.GetScoreboard(l.ctx, model.ScoreboardFilter{
		ContestID: in.ContestId, View: view, Page: page, PageSize: pageSize,
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	converted, err := scoreboardProto(scoreboard)
	if err != nil {
		return nil, err
	}
	return &pb.GetScoreboardResp{Scoreboard: converted}, nil
}
