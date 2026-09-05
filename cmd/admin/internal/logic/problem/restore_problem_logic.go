// Code scaffolded by goctl. No recover, Safe to edit.

package problem

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RestoreProblemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRestoreProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RestoreProblemLogic {
	return &RestoreProblemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RestoreProblemLogic) RestoreProblem(req *types.RestoreProblemReq) (resp *types.RestoreProblemResp, err error) {
	return restoreProblem(l.ctx, l.svcCtx, req)
}
