// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ArchiveContestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewArchiveContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ArchiveContestLogic {
	return &ArchiveContestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ArchiveContestLogic) ArchiveContest(req *types.ArchiveContestReq) (resp *types.ArchiveContestResp, err error) {
	return archiveContest(l.ctx, l.svcCtx, req)
}
