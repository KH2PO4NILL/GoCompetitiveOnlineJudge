// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FreezeScoreboardLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFreezeScoreboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FreezeScoreboardLogic {
	return &FreezeScoreboardLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FreezeScoreboardLogic) FreezeScoreboard(req *types.FreezeScoreboardReq) (resp *types.FreezeScoreboardResp, err error) {
	return freezeScoreboard(l.ctx, l.svcCtx, req)
}
