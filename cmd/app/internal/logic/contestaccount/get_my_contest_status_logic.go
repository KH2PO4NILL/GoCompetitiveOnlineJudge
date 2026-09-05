// Code scaffolded by goctl. No recover, Safe to edit.

package contestaccount

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMyContestStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetMyContestStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMyContestStatusLogic {
	return &GetMyContestStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetMyContestStatusLogic) GetMyContestStatus(req *types.GetMyContestStatusReq) (resp *types.GetMyContestStatusResp, err error) {
	return getMyContestStatus(l.ctx, l.svcCtx, req)
}
