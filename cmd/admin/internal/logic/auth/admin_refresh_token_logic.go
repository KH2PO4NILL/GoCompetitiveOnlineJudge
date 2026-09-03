// Code scaffolded by goctl. No recover, Safe to edit.

package auth

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminRefreshTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminRefreshTokenLogic {
	return &AdminRefreshTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminRefreshTokenLogic) AdminRefreshToken(req *types.RefreshTokenReq) (resp *types.RefreshTokenResp, err error) {
	return refreshToken(l.ctx, l.svcCtx, req)
}
