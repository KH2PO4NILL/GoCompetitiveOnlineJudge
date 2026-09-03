// Code scaffolded by goctl. No recover, Safe to edit.

package problem

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishProblemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPublishProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishProblemLogic {
	return &PublishProblemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PublishProblemLogic) PublishProblem(req *types.PublishProblemReq) (resp *types.PublishProblemResp, err error) {
	return publishProblem(l.ctx, l.svcCtx, req)
}
