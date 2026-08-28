package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReviewRegistrationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReviewRegistrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReviewRegistrationLogic {
	return &ReviewRegistrationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ReviewRegistrationLogic) ReviewRegistration(in *pb.ReviewRegistrationReq) (*pb.ReviewRegistrationResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ReviewRegistrationResp{}, nil
}
