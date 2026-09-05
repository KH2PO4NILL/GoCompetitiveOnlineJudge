// Code scaffolded by goctl. No recover, Safe to edit.

package auth

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EmailCodeLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEmailCodeLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EmailCodeLoginLogic {
	return &EmailCodeLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EmailCodeLoginLogic) EmailCodeLogin(req *types.EmailCodeLoginReq) (resp *types.LoginResp, err error) {
	return emailCodeLogin(l.ctx, l.svcCtx, req)
}
