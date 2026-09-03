// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishContestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPublishContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishContestLogic {
	return &PublishContestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PublishContestLogic) PublishContest(req *types.PublishContestReq) (resp *types.PublishContestResp, err error) {
	return publishContest(l.ctx, l.svcCtx, req)
}
