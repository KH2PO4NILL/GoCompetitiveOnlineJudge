// Code scaffolded by goctl. No recover, Safe to edit.

package auth

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminResetPasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminResetPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminResetPasswordLogic {
	return &AdminResetPasswordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminResetPasswordLogic) AdminResetPassword(req *types.ResetPasswordReq) (resp *types.EmptyResp, err error) {
	return resetPassword(l.ctx, l.svcCtx, req)
}
