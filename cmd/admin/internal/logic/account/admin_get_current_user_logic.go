// Code scaffolded by goctl. No recover, Safe to edit.

package account

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminGetCurrentUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminGetCurrentUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetCurrentUserLogic {
	return &AdminGetCurrentUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGetCurrentUserLogic) AdminGetCurrentUser() (resp *types.GetCurrentUserResp, err error) {
	return getCurrentUser(l.ctx, l.svcCtx)
}
