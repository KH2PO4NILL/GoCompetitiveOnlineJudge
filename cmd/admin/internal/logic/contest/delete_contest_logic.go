// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteContestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteContestLogic {
	return &DeleteContestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteContestLogic) DeleteContest(req *types.DeleteContestReq) (resp *types.DeleteContestResp, err error) {
	return deleteContest(l.ctx, l.svcCtx, req)
}
