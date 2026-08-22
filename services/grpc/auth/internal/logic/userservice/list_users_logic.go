package userservicelogic

import (
	"context"

	"auth/internal/svc"
	"auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListUsersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUsersLogic {
	return &ListUsersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListUsersLogic) ListUsers(in *pb.ListUsersReq) (*pb.ListUsersResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ListUsersResp{}, nil
}
