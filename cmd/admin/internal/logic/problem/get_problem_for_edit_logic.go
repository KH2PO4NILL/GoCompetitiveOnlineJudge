// Code scaffolded by goctl. No recover, Safe to edit.

package problem

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProblemForEditLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetProblemForEditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProblemForEditLogic {
	return &GetProblemForEditLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProblemForEditLogic) GetProblemForEdit(req *types.GetProblemForEditReq) (resp *types.GetProblemForEditResp, err error) {
	return getProblemForEdit(l.ctx, l.svcCtx, req)
}
