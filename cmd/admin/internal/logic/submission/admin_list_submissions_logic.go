// Code scaffolded by goctl. No recover, Safe to edit.

package submission

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminListSubmissionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminListSubmissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListSubmissionsLogic {
	return &AdminListSubmissionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminListSubmissionsLogic) AdminListSubmissions(req *types.ListSubmissionsReq) (resp *types.ListSubmissionsResp, err error) {
	return listSubmissions(l.ctx, l.svcCtx, req)
}
