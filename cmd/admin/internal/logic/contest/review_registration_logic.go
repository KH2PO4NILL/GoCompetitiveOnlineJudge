// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReviewRegistrationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReviewRegistrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReviewRegistrationLogic {
	return &ReviewRegistrationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReviewRegistrationLogic) ReviewRegistration(req *types.ReviewRegistrationReq) (resp *types.ReviewRegistrationResp, err error) {
	return reviewRegistration(l.ctx, l.svcCtx, req)
}
