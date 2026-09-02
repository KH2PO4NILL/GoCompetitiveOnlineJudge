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

type BatchRejudgeSubmissionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchRejudgeSubmissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchRejudgeSubmissionsLogic {
	return &BatchRejudgeSubmissionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BatchRejudgeSubmissionsLogic) BatchRejudgeSubmissions(in *pb.BatchRejudgeSubmissionsReq) (*pb.BatchRejudgeSubmissionsResp, error) {
	principal, err := requireAdmin(l.ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || len(in.SubmissionIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "submission_ids is required")
	}
	if len(in.SubmissionIds) > runtimeLimits(l.svcCtx).MaxBatchRejudge {
		return nil, status.Error(codes.ResourceExhausted, "too many submissions to rejudge")
	}
	seen := make(map[int64]struct{}, len(in.SubmissionIds))
	for _, submissionID := range in.SubmissionIds {
		if submissionID <= 0 {
			return nil, status.Error(codes.InvalidArgument, "submission_ids must be positive")
		}
		if _, exists := seen[submissionID]; exists {
			return nil, status.Error(codes.AlreadyExists, "submission_ids contains a duplicate")
		}
		seen[submissionID] = struct{}{}
	}
	reason, err := validateUTF8Length(in.Reason, 500, "reason", false)
	if err != nil {
		return nil, err
	}
	repository, err := requireSubmitModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	results, err := repository.BatchRejudgeSubmissions(l.ctx, model.BatchRejudgeInput{
		SubmissionIDs: append([]int64(nil), in.SubmissionIds...),
		RequestedBy:   principal.UserID, Reason: reason,
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	if len(results) != len(in.SubmissionIds) {
		return nil, status.Error(codes.Internal, "submit model returned an incomplete batch response")
	}
	response := &pb.BatchRejudgeSubmissionsResp{Accepted: make([]*pb.BatchRejudgeItem, len(results))}
	for index, result := range results {
		if result.SubmissionID <= 0 || result.AttemptID <= 0 {
			return nil, status.Error(codes.Internal, "submit model returned an invalid batch response")
		}
		response.Accepted[index] = &pb.BatchRejudgeItem{
			SubmissionId: result.SubmissionID, AttemptId: result.AttemptID,
		}
	}
	return response, nil
}
