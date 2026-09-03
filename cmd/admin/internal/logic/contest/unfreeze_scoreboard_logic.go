// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnfreezeScoreboardLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnfreezeScoreboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnfreezeScoreboardLogic {
	return &UnfreezeScoreboardLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UnfreezeScoreboardLogic) UnfreezeScoreboard(req *types.UnfreezeScoreboardReq) (resp *types.UnfreezeScoreboardResp, err error) {
	return unfreezeScoreboard(l.ctx, l.svcCtx, req)
}
