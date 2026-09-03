// Code scaffolded by goctl. No recover, Safe to edit.

package auth

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UsernameLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUsernameLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UsernameLoginLogic {
	return &UsernameLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UsernameLoginLogic) UsernameLogin(req *types.UsernameLoginReq) (resp *types.LoginResp, err error) {
	return usernameLogin(l.ctx, l.svcCtx, req)
}
