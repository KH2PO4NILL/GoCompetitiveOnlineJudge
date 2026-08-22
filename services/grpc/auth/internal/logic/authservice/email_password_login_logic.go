package authservicelogic

import (
	"context"

	"auth/internal/svc"
	"auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type EmailPasswordLoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEmailPasswordLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EmailPasswordLoginLogic {
	return &EmailPasswordLoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *EmailPasswordLoginLogic) EmailPasswordLogin(in *pb.EmailPasswordLoginReq) (*pb.LoginResp, error) {
	// todo: add your logic here and delete this line

	return &pb.LoginResp{}, nil
}
