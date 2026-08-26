package logic

import (
	"context"
	"errors"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"
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

//软删除题目也需要metadata的鉴权，仍需修改,软删除权限应为管理员级别

func (l *DeleteProblemLogic) DeleteProblem(in *pb.DeleteProblemReq) (*pb.DeleteProblemResp, error) {
	if in == nil || in.ProblemId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "problem_id must be positive")
	}
	if in.ExpectedRevision <= 0 {
		return nil, status.Error(codes.InvalidArgument, "expected_revision must be positive")
	}

	revision, err := l.svcCtx.ProblemModel.SoftDelete(l.ctx, in.ProblemId, in.ExpectedRevision)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrProblemNotFound):
			return nil, status.Error(codes.NotFound, "problem not found")
		case errors.Is(err, model.ErrProblemRevisionConflict):
			return nil, status.Error(codes.Aborted, "problem revision conflict")
		case errors.Is(err, model.ErrProblemAlreadyDeleted):
			return nil, status.Error(codes.FailedPrecondition, "problem is already deleted")
		default:
			l.Errorf("delete problem %d: %v", in.ProblemId, err)
			return nil, status.Error(codes.Internal, "failed to delete problem")
		}
	}
	return &pb.DeleteProblemResp{Revision: revision}, nil
}
