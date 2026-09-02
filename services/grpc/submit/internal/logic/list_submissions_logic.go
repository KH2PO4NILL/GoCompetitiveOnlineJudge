package logic

import (
	"context"
	"strings"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListSubmissionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSubmissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSubmissionsLogic {
	return &ListSubmissionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListSubmissionsLogic) ListSubmissions(in *pb.ListSubmissionsReq) (*pb.ListSubmissionsResp, error) {
	principal, err := requirePrincipal(l.ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	limits := runtimeLimits(l.svcCtx)
	page := int(in.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(in.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > limits.MaxPageSize {
		return nil, status.Error(codes.ResourceExhausted, "page_size exceeds the limit")
	}
	userID := principal.UserID
	if in.UserId != nil {
		if *in.UserId <= 0 {
			return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
		}
		if *in.UserId != principal.UserID && !isAdmin(principal) {
			return nil, status.Error(codes.PermissionDenied, "cannot list another user's submissions")
		}
		userID = *in.UserId
	}
	problemID, err := positiveOptional(in.ProblemId, "problem_id")
	if err != nil {
		return nil, err
	}
	contestID, err := positiveOptional(in.ContestId, "contest_id")
	if err != nil {
		return nil, err
	}
	var statusFilter *int64
	if in.Status != nil {
		if *in.Status < pb.SubmissionStatus_SUBMISSION_STATUS_QUEUED ||
			*in.Status > pb.SubmissionStatus_SUBMISSION_STATUS_FINISHED {
			return nil, status.Error(codes.InvalidArgument, "status is invalid")
		}
		value := int64(*in.Status)
		statusFilter = &value
	}
	var verdictFilter *int64
	if in.Verdict != nil {
		if !isAdmin(principal) {
			return nil, status.Error(codes.PermissionDenied, "verdict filtering requires administrator permission")
		}
		if *in.Verdict < pb.Verdict_VERDICT_ACCEPTED || *in.Verdict > pb.Verdict_VERDICT_SYSTEM_ERROR {
			return nil, status.Error(codes.InvalidArgument, "verdict is invalid")
		}
		value := int64(*in.Verdict)
		verdictFilter = &value
	}
	language := strings.TrimSpace(in.Language)
	if len(language) > 64 || (language != "" && !validASCII(language)) {
		return nil, status.Error(codes.InvalidArgument, "language is invalid")
	}
	repository, err := requireSubmitModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	summaries, total, err := repository.ListSubmissions(l.ctx, model.SubmissionListFilter{
		Page: page, PageSize: pageSize, UserID: int64Pointer(userID), ProblemID: problemID,
		ContestID: contestID, Status: statusFilter, Verdict: verdictFilter, Language: language,
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	visibilities, err := resolveVisibilities(l.ctx, l.svcCtx, principal, summaries)
	if err != nil {
		return nil, err
	}
	response := &pb.ListSubmissionsResp{Total: total, Submissions: make([]*pb.SubmissionSummary, len(summaries))}
	for index, summary := range summaries {
		response.Submissions[index], err = submissionSummaryProto(summary, visibilities[summary.SubmissionID])
		if err != nil {
			return nil, err
		}
	}
	return response, nil
}
