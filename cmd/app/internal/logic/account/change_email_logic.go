// Code scaffolded by goctl. No recover, Safe to edit.

package account

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChangeEmailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChangeEmailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeEmailLogic {
	return &ChangeEmailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChangeEmailLogic) ChangeEmail(req *types.ChangeEmailReq) (resp *types.ChangeEmailResp, err error) {
	return changeEmail(l.ctx, l.svcCtx, req)
}
