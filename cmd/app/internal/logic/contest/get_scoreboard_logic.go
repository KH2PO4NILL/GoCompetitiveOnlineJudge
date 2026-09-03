// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetScoreboardLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetScoreboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetScoreboardLogic {
	return &GetScoreboardLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetScoreboardLogic) GetScoreboard(req *types.GetScoreboardReq) (resp *types.GetScoreboardResp, err error) {
	return getScoreboard(l.ctx, l.svcCtx, req)
}
