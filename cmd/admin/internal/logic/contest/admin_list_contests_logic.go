// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminListContestsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminListContestsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListContestsLogic {
	return &AdminListContestsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminListContestsLogic) AdminListContests(req *types.ListContestsReq) (resp *types.ListContestsResp, err error) {
	return listContests(l.ctx, l.svcCtx, req)
}
