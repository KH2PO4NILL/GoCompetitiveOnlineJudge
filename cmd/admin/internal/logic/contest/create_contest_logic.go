// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateContestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateContestLogic {
	return &CreateContestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateContestLogic) CreateContest(req *types.CreateContestReq) (resp *types.CreateContestResp, err error) {
	return createContest(l.ctx, l.svcCtx, req)
}
