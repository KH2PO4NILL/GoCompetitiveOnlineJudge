// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListContestProblemsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListContestProblemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListContestProblemsLogic {
	return &ListContestProblemsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListContestProblemsLogic) ListContestProblems(req *types.ListContestProblemsReq) (resp *types.ListContestProblemsResp, err error) {
	return listContestProblems(l.ctx, l.svcCtx, req)
}
