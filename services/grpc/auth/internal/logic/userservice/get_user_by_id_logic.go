package userservicelogic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetUserByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserByIdLogic {
	return &GetUserByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUserByIdLogic) GetUserById(in *pb.GetUserByIdReq) (*pb.GetUserByIdResp, error) {
	if in == nil || in.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "用户 ID 不正确")
	}

	actor, err := authenticatedUser(l.ctx, l.svcCtx, l.Logger)
	if err != nil {
		return nil, err
	}
	// UserInfo 包含邮箱等敏感资料，普通用户只能查询自己，管理员才可以查询其他账号。
	if actor.Id != in.UserId && !isAdministrator(actor.Role) {
		return nil, status.Error(codes.PermissionDenied, "无权查看该用户信息")
	}

	target, err := findActiveUserByID(l.ctx, l.svcCtx.DB, in.UserId)
	if err != nil {
		l.Errorf("find user %d: %v", in.UserId, err)
		return nil, status.Error(codes.Internal, "读取用户信息失败")
	}
	if target == nil {
		return nil, status.Error(codes.NotFound, "用户不存在")
	}

	return &pb.GetUserByIdResp{User: userInfo(target)}, nil
}
