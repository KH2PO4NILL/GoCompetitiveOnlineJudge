// Code scaffolded by goctl. No recover, Safe to edit.

package account

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminChangePasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminChangePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminChangePasswordLogic {
	return &AdminChangePasswordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminChangePasswordLogic) AdminChangePassword(req *types.ChangePasswordReq) (resp *types.EmptyResp, err error) {
	return changePassword(l.ctx, l.svcCtx, req)
}
