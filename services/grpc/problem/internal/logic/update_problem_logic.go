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

type UpdateProblemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProblemLogic {
	return &UpdateProblemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateProblemLogic) UpdateProblem(in *pb.UpdateProblemReq) (*pb.UpdateProblemResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validatePositiveProblemRevision(in.ProblemId, in.ExpectedRevision); err != nil {
		return nil, err
	}
	principal, _, err := authorizeProblem(l.ctx, l.svcCtx, in.ProblemId)
	if err != nil {
		return nil, err
	}
	patch, err := problemPatch(in.Problem, in.UpdateMask)
	if err != nil {
		return nil, err
	}
	problem, err := l.svcCtx.ProblemModel.Update(l.ctx, model.UpdateProblemInput{
		ProblemID: in.ProblemId, ExpectedRevision: in.ExpectedRevision,
		ActorID: principal.UserID, Patch: patch,
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	responseProblem, err := managementProblemProto(l.ctx, l.svcCtx, problem)
	if err != nil {
		return nil, err
	}
	return &pb.UpdateProblemResp{Problem: responseProblem}, nil
}
