// Code scaffolded by goctl. No recover, Safe to edit.

package health

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type HealthLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HealthLogic {
	return &HealthLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HealthLogic) Health() (resp *types.HealthResp, err error) {
	return &types.HealthResp{Status: "ok"}, nil
}
