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

type CreateProblemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateProblemLogic {
	return &CreateProblemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 出题人或管理员接口，权限必须由 gRPC metadata 中的鉴权信息决定。

func (l *CreateProblemLogic) CreateProblem(in *pb.CreateProblemReq) (*pb.CreateProblemResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	principal, err := requirePrincipal(l.ctx)
	if err != nil {
		return nil, err
	}
	editable, err := editableFromProto(in.Problem)
	if err != nil {
		return nil, err
	}
	problem, err := l.svcCtx.ProblemModel.Create(l.ctx, model.CreateProblemInput{
		AuthorID: principal.UserID,
		Editable: editable,
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	responseProblem, err := managementProblemProto(l.ctx, l.svcCtx, problem)
	if err != nil {
		return nil, err
	}
	return &pb.CreateProblemResp{Problem: responseProblem}, nil
}
