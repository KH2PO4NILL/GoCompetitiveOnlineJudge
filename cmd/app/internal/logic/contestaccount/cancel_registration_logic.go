// Code scaffolded by goctl. No recover, Safe to edit.

package contestaccount

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CancelRegistrationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCancelRegistrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelRegistrationLogic {
	return &CancelRegistrationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CancelRegistrationLogic) CancelRegistration(req *types.CancelRegistrationReq) (resp *types.CancelRegistrationResp, err error) {
	return cancelRegistration(l.ctx, l.svcCtx, req)
}
