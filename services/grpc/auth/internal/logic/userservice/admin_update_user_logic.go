package userservicelogic

import (
	"context"

	"auth/internal/svc"
	"auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminUpdateUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminUpdateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUpdateUserLogic {
	return &AdminUpdateUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminUpdateUserLogic) AdminUpdateUser(in *pb.AdminUpdateUserReq) (*pb.AdminUpdateUserResp, error) {
	// todo: add your logic here and delete this line

	return &pb.AdminUpdateUserResp{}, nil
}
