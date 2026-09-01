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

type ListRegistrationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRegistrationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRegistrationsLogic {
	return &ListRegistrationsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListRegistrationsLogic) ListRegistrations(in *pb.ListRegistrationsReq) (*pb.ListRegistrationsResp, error) {
	if in == nil || in.ContestId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id must be positive")
	}
	if _, _, err := authorizeContest(l.ctx, l.svcCtx, in.ContestId); err != nil {
		return nil, err
	}
	page, pageSize, err := normalizePagination(in.Page, in.PageSize)
	if err != nil {
		return nil, err
	}
	var statusFilter *int64
	if in.Status != pb.RegistrationStatus_REGISTRATION_STATUS_UNSPECIFIED {
		if !validRegistrationStatus(in.Status) {
			return nil, invalidEnum("status")
		}
		value := int64(in.Status)
		statusFilter = &value
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	registrations, total, err := repository.ListRegistrations(l.ctx, model.RegistrationListFilter{
		ContestID: in.ContestId, Page: page, PageSize: pageSize, Status: statusFilter,
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	result := make([]*pb.ContestRegistration, 0, len(registrations))
	for index := range registrations {
		converted, err := registrationProto(&registrations[index])
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return &pb.ListRegistrationsResp{Total: total, Registrations: result}, nil
}
