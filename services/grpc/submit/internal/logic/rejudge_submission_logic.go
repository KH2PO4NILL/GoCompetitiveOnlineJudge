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

type RejudgeSubmissionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRejudgeSubmissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RejudgeSubmissionLogic {
	return &RejudgeSubmissionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RejudgeSubmissionLogic) RejudgeSubmission(in *pb.RejudgeSubmissionReq) (*pb.RejudgeSubmissionResp, error) {
	principal, err := requireAdmin(l.ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.SubmissionId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "submission_id must be positive")
	}
	reason, err := validateUTF8Length(in.Reason, 500, "reason", false)
	if err != nil {
		return nil, err
	}
	repository, err := requireSubmitModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	result, err := repository.RejudgeSubmission(l.ctx, model.RejudgeSubmissionInput{
		SubmissionID: in.SubmissionId, RequestedBy: principal.UserID, Reason: reason,
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	if result == nil || result.Submission == nil || result.AttemptID <= 0 {
		return nil, status.Error(codes.Internal, "submit model returned an empty rejudge response")
	}
	summary, err := submissionSummaryProto(
		result.Submission.SubmissionSummary,
		pb.ResultVisibility_RESULT_VISIBILITY_FULL,
	)
	if err != nil {
		return nil, err
	}
	return &pb.RejudgeSubmissionResp{Submission: summary, AttemptId: result.AttemptID}, nil
}
