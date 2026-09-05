// Code scaffolded by goctl. No recover, Safe to edit.

package contestaccount

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterContestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterContestLogic {
	return &RegisterContestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterContestLogic) RegisterContest(req *types.RegisterContestReq) (resp *types.RegisterContestResp, err error) {
	return registerContest(l.ctx, l.svcCtx, req)
}
