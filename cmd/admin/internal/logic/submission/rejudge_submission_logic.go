// Code scaffolded by goctl. No recover, Safe to edit.

package submission

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RejudgeSubmissionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRejudgeSubmissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RejudgeSubmissionLogic {
	return &RejudgeSubmissionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RejudgeSubmissionLogic) RejudgeSubmission(req *types.RejudgeSubmissionReq) (resp *types.RejudgeSubmissionResp, err error) {
	return rejudgeSubmission(l.ctx, l.svcCtx, req)
}
