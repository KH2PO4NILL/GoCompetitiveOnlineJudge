package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateContestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateContestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateContestLogic {
	return &CreateContestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建者与管理员接口。发布前必须校验必填字段；比赛开始后不可修改赛制、时间和题目
func (l *CreateContestLogic) CreateContest(in *pb.CreateContestReq) (*pb.CreateContestResp, error) {
	// todo: add your logic here and delete this line

	return &pb.CreateContestResp{}, nil
}
