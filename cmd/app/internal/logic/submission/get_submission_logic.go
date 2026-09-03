// Code scaffolded by goctl. No recover, Safe to edit.

package submission

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSubmissionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSubmissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSubmissionLogic {
	return &GetSubmissionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSubmissionLogic) GetSubmission(req *types.GetSubmissionReq) (resp *types.GetSubmissionResp, err error) {
	return getSubmission(l.ctx, l.svcCtx, req)
}
