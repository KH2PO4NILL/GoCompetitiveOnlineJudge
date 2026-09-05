// Code scaffolded by goctl. No recover, Safe to edit.

package auth

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminEmailPasswordLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminEmailPasswordLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminEmailPasswordLoginLogic {
	return &AdminEmailPasswordLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminEmailPasswordLoginLogic) AdminEmailPasswordLogin(req *types.EmailPasswordLoginReq) (resp *types.LoginResp, err error) {
	return emailPasswordLogin(l.ctx, l.svcCtx, req)
}
