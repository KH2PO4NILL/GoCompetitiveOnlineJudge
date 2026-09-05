// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetContestForEditLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetContestForEditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContestForEditLogic {
	return &GetContestForEditLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetContestForEditLogic) GetContestForEdit(req *types.GetContestForEditReq) (resp *types.GetContestForEditResp, err error) {
	return getContestForEdit(l.ctx, l.svcCtx, req)
}
