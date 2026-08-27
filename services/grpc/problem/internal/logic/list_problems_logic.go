package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListProblemsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProblemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProblemsLogic {
	return &ListProblemsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListProblemsLogic) ListProblems(in *pb.ListProblemsReq) (*pb.ListProblemsResp, error) {
	if err := validateListRequest(in); err != nil {
		return nil, err
	}
	filter := listFilter(in)
	var (
		problems []model.ProblemSummary
		total    int64
		err      error
	)
	if managementListRequested(in) {
		principal, principalErr := requirePrincipal(l.ctx)
		if principalErr != nil {
			return nil, principalErr
		}
		if !isAdmin(principal) {
			if filter.AuthorID != nil && *filter.AuthorID != principal.UserID {
				return nil, status.Error(codes.PermissionDenied, "cannot list another author's managed problems")
			}
			filter.AuthorID = int64Pointer(principal.UserID)
		}
		problems, total, err = l.svcCtx.ProblemModel.ListForManagement(l.ctx, filter)
	} else {
		filter.Status = nil
		filter.Visibility = nil
		filter.AuthorID = nil
		problems, total, err = l.svcCtx.ProblemModel.ListPublic(l.ctx, filter)
	}
	if err != nil {
		return nil, mapModelError(err)
	}
	items := make([]*pb.ProblemSummary, 0, len(problems))
	for _, problem := range problems {
		items = append(items, summaryProto(problem))
	}
	return &pb.ListProblemsResp{Total: total, Problems: items}, nil
}

func int64Pointer(value int64) *int64 {
	return &value
}
