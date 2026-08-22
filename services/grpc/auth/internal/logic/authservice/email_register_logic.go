package authservicelogic

import (
	"context"

	"auth/internal/svc"
	"auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type EmailRegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEmailRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EmailRegisterLogic {
	return &EmailRegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *EmailRegisterLogic) EmailRegister(in *pb.EmailRegisterReq) (*pb.RegisterResp, error) {
	// todo: add your logic here and delete this line

	return &pb.RegisterResp{}, nil
}
