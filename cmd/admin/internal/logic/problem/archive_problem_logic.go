// Code scaffolded by goctl. No recover, Safe to edit.

package problem

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ArchiveProblemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewArchiveProblemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ArchiveProblemLogic {
	return &ArchiveProblemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ArchiveProblemLogic) ArchiveProblem(req *types.ArchiveProblemReq) (resp *types.ArchiveProblemResp, err error) {
	return archiveProblem(l.ctx, l.svcCtx, req)
}
