package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DeleteProblemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteProblemLogic {
	return &DeleteProblemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeleteProblem is intentionally administrator-only. Authors may archive their
// own problems, while deletion changes global visibility and recovery state.
func (l *DeleteProblemLogic) DeleteProblem(in *pb.DeleteProblemReq) (*pb.DeleteProblemResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validatePositiveProblemRevision(in.ProblemId, in.ExpectedRevision); err != nil {
		return nil, err
	}
	principal, err := requireAdmin(l.ctx)
	if err != nil {
		return nil, err
	}
	revision, err := l.svcCtx.ProblemModel.SoftDelete(l.ctx, in.ProblemId, in.ExpectedRevision, principal.UserID)
	if err != nil {
		return nil, mapModelError(err)
	}
	return &pb.DeleteProblemResp{Revision: revision}, nil
}
