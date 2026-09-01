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

type ListParticipantsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListParticipantsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListParticipantsLogic {
	return &ListParticipantsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListParticipantsLogic) ListParticipants(in *pb.ListParticipantsReq) (*pb.ListParticipantsResp, error) {
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
	if in.Status != pb.ParticipantStatus_PARTICIPANT_STATUS_UNSPECIFIED {
		if !validParticipantStatus(in.Status) {
			return nil, invalidEnum("status")
		}
		value := int64(in.Status)
		statusFilter = &value
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	participants, total, err := repository.ListParticipants(l.ctx, model.ParticipantListFilter{
		ContestID: in.ContestId, Page: page, PageSize: pageSize, Status: statusFilter,
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	result := make([]*pb.ContestParticipant, 0, len(participants))
	for index := range participants {
		converted, err := participantProto(&participants[index])
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return &pb.ListParticipantsResp{Total: total, Participants: result}, nil
}
