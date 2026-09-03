// Code scaffolded by goctl. No recover, Safe to edit.

package auth

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminUsernameLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUsernameLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUsernameLoginLogic {
	return &AdminUsernameLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminUsernameLoginLogic) AdminUsernameLogin(req *types.UsernameLoginReq) (resp *types.LoginResp, err error) {
	return usernameLogin(l.ctx, l.svcCtx, req)
}
