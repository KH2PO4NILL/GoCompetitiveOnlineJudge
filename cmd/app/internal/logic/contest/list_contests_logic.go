// Code scaffolded by goctl. No recover, Safe to edit.

package contest

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListContestsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListContestsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListContestsLogic {
	return &ListContestsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListContestsLogic) ListContests(req *types.ListContestsReq) (resp *types.ListContestsResp, err error) {
	return listContests(l.ctx, l.svcCtx, req)
}
