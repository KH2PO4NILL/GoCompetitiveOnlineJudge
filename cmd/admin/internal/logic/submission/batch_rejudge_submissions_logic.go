// Code scaffolded by goctl. No recover, Safe to edit.

package submission

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchRejudgeSubmissionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBatchRejudgeSubmissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchRejudgeSubmissionsLogic {
	return &BatchRejudgeSubmissionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchRejudgeSubmissionsLogic) BatchRejudgeSubmissions(req *types.BatchRejudgeSubmissionsReq) (resp *types.BatchRejudgeSubmissionsResp, err error) {
	return batchRejudgeSubmissions(l.ctx, l.svcCtx, req)
}
