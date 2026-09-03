// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RestoreContestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRestoreContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RestoreContestLogic {
	return &RestoreContestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RestoreContestLogic) RestoreContest(req *types.RestoreContestReq) (resp *types.RestoreContestResp, err error) {
	return restoreContest(l.ctx, l.svcCtx, req)
}
