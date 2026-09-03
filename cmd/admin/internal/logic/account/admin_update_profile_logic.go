// Code scaffolded by goctl. No recover, Safe to edit.

package account

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminUpdateProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUpdateProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUpdateProfileLogic {
	return &AdminUpdateProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminUpdateProfileLogic) AdminUpdateProfile(req *types.UpdateProfileReq) (resp *types.UpdateProfileResp, err error) {
	return updateProfile(l.ctx, l.svcCtx, req)
}
