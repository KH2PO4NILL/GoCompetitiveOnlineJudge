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

type ListContestsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListContestsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListContestsLogic {
	return &ListContestsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 用户侧查询与自助操作。
func (l *ListContestsLogic) ListContests(in *pb.ListContestsReq) (*pb.ListContestsResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	page, pageSize, err := normalizePagination(in.Page, in.PageSize)
	if err != nil {
		return nil, err
	}
	if err := validateKeyword(in.Keyword); err != nil {
		return nil, err
	}
	var statusFilter *int64
	if in.Status != pb.ContestStatus_CONTEST_STATUS_UNSPECIFIED {
		if !validContestStatus(in.Status) {
			return nil, invalidEnum("status")
		}
		value := int64(in.Status)
		statusFilter = &value
	}
	var modeFilter *int64
	if in.Mode != pb.ContestMode_CONTEST_MODE_UNSPECIFIED {
		if !validContestMode(in.Mode) {
			return nil, invalidEnum("mode")
		}
		value := int64(in.Mode)
		modeFilter = &value
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	contests, total, err := repository.ListPublic(l.ctx, model.ContestListFilter{
		Page: page, PageSize: pageSize, Keyword: in.Keyword,
		Status: statusFilter, Mode: modeFilter, At: nowUTC(l.svcCtx),
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	result := make([]*pb.ContestSummary, 0, len(contests))
	for _, contest := range contests {
		converted, err := contestSummaryProto(contest)
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return &pb.ListContestsResp{Total: total, Contests: result}, nil
}
