// Code scaffolded by goctl. No recover, Safe to edit.

package account

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminChangeEmailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminChangeEmailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminChangeEmailLogic {
	return &AdminChangeEmailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminChangeEmailLogic) AdminChangeEmail(req *types.ChangeEmailReq) (resp *types.ChangeEmailResp, err error) {
	return changeEmail(l.ctx, l.svcCtx, req)
}
