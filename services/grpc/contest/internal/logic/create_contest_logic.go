package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	if in == nil || in.Contest == nil {
		return nil, status.Error(codes.InvalidArgument, "contest is required")
	}
	principal, err := requirePrincipal(l.ctx)
	if err != nil {
		return nil, err
	}
	editable, err := editableFromProto(in.Contest)
	if err != nil {
		return nil, err
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	contest, err := repository.Create(l.ctx, model.CreateContestInput{CreatorID: principal.UserID, Editable: editable})
	if err != nil {
		return nil, mapModelError(err)
	}
	converted, err := contestProto(contest)
	if err != nil {
		return nil, err
	}
	return &pb.CreateContestResp{Contest: converted}, nil
}
