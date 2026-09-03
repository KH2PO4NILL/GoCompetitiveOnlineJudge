// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateContestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateContestLogic {
	return &UpdateContestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateContestLogic) UpdateContest(req *types.UpdateContestReq) (resp *types.UpdateContestResp, err error) {
	return updateContest(l.ctx, l.svcCtx, req)
}
