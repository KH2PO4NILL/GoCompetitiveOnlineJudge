// Code scaffolded by goctl. No recover, Safe to edit.

package problem

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminListProblemsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminListProblemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListProblemsLogic {
	return &AdminListProblemsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminListProblemsLogic) AdminListProblems(req *types.ListProblemsReq) (resp *types.ListProblemsResp, err error) {
	return listProblems(l.ctx, l.svcCtx, req)
}
