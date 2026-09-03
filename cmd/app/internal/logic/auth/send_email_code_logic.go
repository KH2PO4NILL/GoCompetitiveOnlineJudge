// Code scaffolded by goctl. No recover, Safe to edit.

package auth

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SendEmailCodeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSendEmailCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendEmailCodeLogic {
	return &SendEmailCodeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SendEmailCodeLogic) SendEmailCode(req *types.SendEmailCodeReq) (resp *types.SendEmailCodeResp, err error) {
	return sendEmailCode(l.ctx, l.svcCtx, req)
}
