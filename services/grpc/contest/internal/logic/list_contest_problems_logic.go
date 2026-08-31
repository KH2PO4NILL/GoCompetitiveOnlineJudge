package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListContestProblemsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListContestProblemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListContestProblemsLogic {
	return &ListContestProblemsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListContestProblemsLogic) ListContestProblems(in *pb.ListContestProblemsReq) (*pb.ListContestProblemsResp, error) {
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
	if visible.Detail.Status != int64(pb.ContestStatus_CONTEST_STATUS_RUNNING) &&
		visible.Detail.Status != int64(pb.ContestStatus_CONTEST_STATUS_FINISHED) {
		return nil, status.Error(codes.FailedPrecondition, "contest problems are not available")
	}
	if !visible.Manager {
		if !visible.HasPrincipal {
			return nil, status.Error(codes.PermissionDenied, "contest participation is required")
		}
		myStatus := visible.MyStatus
		if myStatus == nil {
			myStatus, err = loadMyStatus(l.ctx, l.svcCtx, in.ContestId, visible.Principal.UserID)
			if err != nil {
				return nil, err
			}
		}
		if !normalParticipant(myStatus) {
			return nil, status.Error(codes.PermissionDenied, "contest participation is required")
		}
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	problems, err := repository.ListProblems(l.ctx, in.ContestId)
	if err != nil {
		return nil, mapModelError(err)
	}
	total := int64(len(problems))
	start := (page - 1) * pageSize
	if start > len(problems) {
		start = len(problems)
	}
	end := start + pageSize
	if end > len(problems) {
		end = len(problems)
	}
	result := make([]*pb.ContestProblemSummary, 0, end-start)
	for _, problem := range problems[start:end] {
		converted, err := contestProblemSummaryProto(problem)
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return &pb.ListContestProblemsResp{Total: total, Problems: result}, nil
}
