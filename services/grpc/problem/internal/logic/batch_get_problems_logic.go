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

type BatchGetProblemsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchGetProblemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchGetProblemsLogic {
	return &BatchGetProblemsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 仅允许受信任的内部服务调用。

func (l *BatchGetProblemsLogic) BatchGetProblems(in *pb.BatchGetProblemsReq) (*pb.BatchGetProblemsResp, error) {
	if in == nil || len(in.ProblemIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one problem_id is required")
	}
	if len(in.ProblemIds) > maxBatchSize {
		return nil, status.Errorf(codes.ResourceExhausted, "batch size must not exceed %d", maxBatchSize)
	}
	for _, problemID := range in.ProblemIds {
		if problemID <= 0 {
			return nil, status.Error(codes.InvalidArgument, "problem_ids must be positive")
		}
	}
	caller, err := requireInternalService(l.ctx)
	if err != nil {
		return nil, err
	}
	var problems []*model.ProblemSummary
	if caller.Name == "contest.rpc" {
		problems, err = l.svcCtx.ProblemModel.BatchGetPublishedSummaries(l.ctx, in.ProblemIds)
	} else {
		problems, err = l.svcCtx.ProblemModel.BatchGetPublicSummaries(l.ctx, in.ProblemIds)
	}
	if err != nil {
		return nil, mapModelError(err)
	}
	result := make([]*pb.ProblemSummary, 0, len(problems))
	for _, problem := range problems {
		if problem != nil {
			result = append(result, summaryProto(*problem))
		}
	}
	return &pb.BatchGetProblemsResp{Problems: result}, nil
}
