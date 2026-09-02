package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetSubmissionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSubmissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSubmissionLogic {
	return &GetSubmissionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSubmissionLogic) GetSubmission(in *pb.GetSubmissionReq) (*pb.GetSubmissionResp, error) {
	principal, err := requirePrincipal(l.ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.SubmissionId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "submission_id must be positive")
	}
	repository, err := requireSubmitModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	aggregate, err := repository.GetSubmission(l.ctx, in.SubmissionId)
	if err != nil {
		return nil, mapModelError(err)
	}
	if aggregate == nil {
		return nil, status.Error(codes.Internal, "submit model returned an empty response")
	}
	if aggregate.UserID != principal.UserID && !isAdmin(principal) {
		return nil, status.Error(codes.NotFound, "submission not found")
	}
	visibilities, err := resolveVisibilities(
		l.ctx, l.svcCtx, principal, []model.SubmissionSummary{aggregate.SubmissionSummary},
	)
	if err != nil {
		return nil, err
	}
	detail, err := submissionDetailProto(aggregate, visibilities[aggregate.SubmissionID], true)
	if err != nil {
		return nil, err
	}
	return &pb.GetSubmissionResp{Submission: detail}, nil
}
