// Code scaffolded by goctl. No recover, Safe to edit.

package auth

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminSendEmailCodeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminSendEmailCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminSendEmailCodeLogic {
	return &AdminSendEmailCodeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminSendEmailCodeLogic) AdminSendEmailCode(req *types.SendEmailCodeReq) (resp *types.SendEmailCodeResp, err error) {
	return sendEmailCode(l.ctx, l.svcCtx, req)
}
