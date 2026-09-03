// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRegistrationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListRegistrationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRegistrationsLogic {
	return &ListRegistrationsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListRegistrationsLogic) ListRegistrations(req *types.ListRegistrationsReq) (resp *types.ListRegistrationsResp, err error) {
	return listRegistrations(l.ctx, l.svcCtx, req)
}
