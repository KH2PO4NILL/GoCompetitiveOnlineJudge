package logic

import (
	"context"
	"errors"

	infraauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetProblemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProblemLogic {
	return &GetProblemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProblemLogic) GetProblem(in *pb.GetProblemReq) (*pb.GetProblemResp, error) {
	if in == nil || in.ProblemId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "problem_id must be positive")
	}
	if in.ContestId != nil && *in.ContestId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id must be positive")
	}
	if in.ContestId == nil {
		problem, err := l.svcCtx.ProblemModel.GetPublic(l.ctx, in.ProblemId)
		if err != nil {
			return nil, mapModelError(err)
		}
		return &pb.GetProblemResp{Problem: practiceProblemProto(problem)}, nil
	}

	problem, err := l.svcCtx.ProblemModel.GetPublished(l.ctx, in.ProblemId)
	if err != nil {
		return nil, mapModelError(err)
	}
	switch problem.Editable.Visibility {
	case model.ProblemVisibilityPublic:
		return &pb.GetProblemResp{Problem: practiceProblemProto(problem)}, nil
	case model.ProblemVisibilityContestOnly:
	default:
		return nil, status.Error(codes.NotFound, "problem resource not found")
	}
	principal, ok := infraauth.PrincipalFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "problem resource not found")
	}
	if l.svcCtx.ContestAccess == nil {
		return nil, invalidDependency("contest access service")
	}
	allowed, err := l.svcCtx.ContestAccess.CanAccessProblem(l.ctx, *in.ContestId, in.ProblemId, principal.UserID)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, status.Error(codes.Canceled, "contest access check was canceled")
		}
		return nil, invalidDependency("contest access service")
	}
	if !allowed {
		return nil, status.Error(codes.NotFound, "problem resource not found")
	}
	return &pb.GetProblemResp{Problem: practiceProblemProto(problem)}, nil
}
