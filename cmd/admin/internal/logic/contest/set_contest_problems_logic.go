// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetContestProblemsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSetContestProblemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetContestProblemsLogic {
	return &SetContestProblemsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SetContestProblemsLogic) SetContestProblems(req *types.SetContestProblemsReq) (resp *types.SetContestProblemsResp, err error) {
	return setContestProblems(l.ctx, l.svcCtx, req)
}
