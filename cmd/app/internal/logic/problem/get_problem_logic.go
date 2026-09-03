// Code scaffolded by goctl. No recover, Safe to edit.

package problem

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProblemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProblemLogic {
	return &GetProblemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProblemLogic) GetProblem(req *types.GetProblemReq) (resp *types.GetProblemResp, err error) {
	return getProblem(l.ctx, l.svcCtx, req)
}
