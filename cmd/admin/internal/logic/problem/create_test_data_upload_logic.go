// Code scaffolded by goctl. No recover, Safe to edit.

package problem

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateTestDataUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateTestDataUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTestDataUploadLogic {
	return &CreateTestDataUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateTestDataUploadLogic) CreateTestDataUpload(req *types.CreateTestDataUploadReq) (resp *types.CreateTestDataUploadResp, err error) {
	return createTestDataUpload(l.ctx, l.svcCtx, req)
}
