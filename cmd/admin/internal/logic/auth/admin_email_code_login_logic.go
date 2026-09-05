// Code scaffolded by goctl. No recover, Safe to edit.

package auth

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminEmailCodeLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminEmailCodeLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminEmailCodeLoginLogic {
	return &AdminEmailCodeLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminEmailCodeLoginLogic) AdminEmailCodeLogin(req *types.EmailCodeLoginReq) (resp *types.LoginResp, err error) {
	return emailCodeLogin(l.ctx, l.svcCtx, req)
}
