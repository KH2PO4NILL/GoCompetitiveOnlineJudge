// Code scaffolded by goctl. No recover, Safe to edit.

package submission

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminGetSubmissionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminGetSubmissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetSubmissionLogic {
	return &AdminGetSubmissionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGetSubmissionLogic) AdminGetSubmission(req *types.GetSubmissionReq) (resp *types.GetSubmissionResp, err error) {
	return getSubmission(l.ctx, l.svcCtx, req)
}
