// Code scaffolded by goctl. No recover, Safe to edit.

package problem

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FinalizeTestDataUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFinalizeTestDataUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FinalizeTestDataUploadLogic {
	return &FinalizeTestDataUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FinalizeTestDataUploadLogic) FinalizeTestDataUpload(req *types.FinalizeTestDataUploadReq) (resp *types.FinalizeTestDataUploadResp, err error) {
	return finalizeTestDataUpload(l.ctx, l.svcCtx, req)
}
