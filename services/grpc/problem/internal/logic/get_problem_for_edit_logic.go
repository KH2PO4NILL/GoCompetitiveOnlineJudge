package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetProblemForEditLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProblemForEditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProblemForEditLogic {
	return &GetProblemForEditLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProblemForEditLogic) GetProblemForEdit(in *pb.GetProblemForEditReq) (*pb.GetProblemForEditResp, error) {
	if in == nil || in.ProblemId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "problem_id must be positive")
	}
	_, problem, err := authorizeProblem(l.ctx, l.svcCtx, in.ProblemId)
	if err != nil {
		return nil, err
	}
	testCaseRecords, err := l.svcCtx.ProblemModel.GetCurrentTestCases(l.ctx, in.ProblemId)
	if err != nil {
		return nil, mapModelError(err)
	}
	testCases := make([]*pb.TestCaseInfo, 0, len(testCaseRecords))
	for _, testCase := range testCaseRecords {
		testCases = append(testCases, testCaseProto(testCase))
	}
	responseProblem, err := managementProblemProto(l.ctx, l.svcCtx, problem)
	if err != nil {
		return nil, err
	}
	return &pb.GetProblemForEditResp{
		Problem:   responseProblem,
		TestCases: testCases,
	}, nil
}
