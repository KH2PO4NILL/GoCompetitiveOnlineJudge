package logic

import (
	"context"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResolveSubmissionResultVisibilityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveSubmissionResultVisibilityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveSubmissionResultVisibilityLogic {
	return &ResolveSubmissionResultVisibilityLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResolveSubmissionResultVisibilityLogic) ResolveSubmissionResultVisibility(in *pb.ResolveSubmissionResultVisibilityReq) (*pb.ResolveSubmissionResultVisibilityResp, error) {
	if err := requireService(l.ctx, "submit.rpc"); err != nil {
		return nil, err
	}
	if in == nil || in.ContestId <= 0 || in.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "contest_id and user_id must be positive")
	}
	observedAt, err := timestampValue(in.ObservedAt, true, "observed_at")
	if err != nil {
		return nil, err
	}
	repository, err := requireContestModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	contest, err := repository.GetForEdit(l.ctx, in.ContestId)
	if err != nil {
		return nil, mapModelError(err)
	}
	if contest == nil || contest.ContestID != in.ContestId {
		return nil, status.Error(codes.Internal, "contest model returned invalid result visibility state")
	}
	visibility, err := submissionResultVisibility(contest, observedAt)
	if err != nil {
		return nil, err
	}
	return &pb.ResolveSubmissionResultVisibilityResp{Visibility: visibility}, nil
}

func submissionResultVisibility(
	contest *model.ContestAggregate,
	observedAt time.Time,
) (pb.SubmissionResultVisibility, error) {
	if contest == nil || contest.ContestID <= 0 || contest.Editable.EndAt.IsZero() || observedAt.IsZero() {
		return 0, status.Error(codes.Internal, "contest result visibility state is invalid")
	}
	switch contest.Editable.Mode {
	case model.ContestModeICPC, model.ContestModeIOI:
		return pb.SubmissionResultVisibility_SUBMISSION_RESULT_VISIBILITY_FULL, nil
	case model.ContestModeOI:
	default:
		return 0, status.Error(codes.Internal, "contest mode is invalid")
	}

	switch contest.Status {
	case model.ContestStatusDraft:
		return pb.SubmissionResultVisibility_SUBMISSION_RESULT_VISIBILITY_STATUS_ONLY, nil
	case model.ContestStatusPublished:
		if observedAt.UTC().Before(contest.Editable.EndAt.UTC()) {
			return pb.SubmissionResultVisibility_SUBMISSION_RESULT_VISIBILITY_STATUS_ONLY, nil
		}
		return pb.SubmissionResultVisibility_SUBMISSION_RESULT_VISIBILITY_FULL, nil
	case model.ContestStatusCancelled, model.ContestStatusArchived, model.ContestStatusDeleted:
		return pb.SubmissionResultVisibility_SUBMISSION_RESULT_VISIBILITY_FULL, nil
	default:
		return 0, status.Error(codes.Internal, "contest lifecycle status is invalid")
	}
}
