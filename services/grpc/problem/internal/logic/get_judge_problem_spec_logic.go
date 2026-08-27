package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetJudgeProblemSpecLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetJudgeProblemSpecLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetJudgeProblemSpecLogic {
	return &GetJudgeProblemSpecLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetJudgeProblemSpecLogic) GetJudgeProblemSpec(in *pb.GetJudgeProblemSpecReq) (*pb.GetJudgeProblemSpecResp, error) {
	if in == nil || in.ProblemId <= 0 || in.Revision < 0 {
		return nil, status.Error(codes.InvalidArgument, "problem_id must be positive and revision cannot be negative")
	}
	if _, err := requireInternalService(l.ctx); err != nil {
		return nil, err
	}
	spec, err := l.svcCtx.ProblemModel.GetJudgeSpec(l.ctx, in.ProblemId, in.Revision)
	if err != nil {
		return nil, mapModelError(err)
	}
	return &pb.GetJudgeProblemSpecResp{Spec: judgeSpecProto(spec)}, nil
}
