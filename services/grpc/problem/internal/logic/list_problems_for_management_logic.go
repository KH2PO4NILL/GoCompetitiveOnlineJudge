package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListProblemsForManagementLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProblemsForManagementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProblemsForManagementLogic {
	return &ListProblemsForManagementLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListProblemsForManagement 返回包含非公开状态的完整管理列表，仅允许管理员调用。
func (l *ListProblemsForManagementLogic) ListProblemsForManagement(in *pb.ListProblemsReq) (*pb.ListProblemsResp, error) {
	if err := validateListRequest(in); err != nil {
		return nil, err
	}
	// HTTP 中间件只做提前拦截，管理权限必须在 RPC 边界再次校验。
	if _, err := requireAdmin(l.ctx); err != nil {
		return nil, err
	}
	if l.svcCtx == nil || l.svcCtx.ProblemModel == nil {
		return nil, status.Error(codes.Internal, "problem model is unavailable")
	}
	problems, total, err := l.svcCtx.ProblemModel.ListForManagement(l.ctx, listFilter(in))
	if err != nil {
		return nil, mapModelError(err)
	}
	items := make([]*pb.ProblemSummary, 0, len(problems))
	for _, problem := range problems {
		items = append(items, summaryProto(problem))
	}
	return &pb.ListProblemsResp{Total: total, Problems: items}, nil
}
