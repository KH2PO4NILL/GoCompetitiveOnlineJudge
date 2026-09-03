// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CancelContestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCancelContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelContestLogic {
	return &CancelContestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CancelContestLogic) CancelContest(req *types.CancelContestReq) (resp *types.CancelContestResp, err error) {
	return cancelContest(l.ctx, l.svcCtx, req)
}
