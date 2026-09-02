package logic

import (
	"context"
	"errors"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ReportJudgeResultLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReportJudgeResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReportJudgeResultLogic {
	return &ReportJudgeResultLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ReportJudgeResultLogic) ReportJudgeResult(in *pb.ReportJudgeResultReq) (*pb.ReportJudgeResultResp, error) {
	if err := requireJudgeWorker(l.ctx); err != nil {
		return nil, err
	}
	if in == nil || in.SubmissionId <= 0 || in.AttemptId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "submission_id and attempt_id must be positive")
	}
	workerID, err := normalizeWorkerIdentity(in.WorkerId, 128, "worker_id", true)
	if err != nil {
		return nil, err
	}
	leaseToken, err := normalizeWorkerIdentity(in.LeaseToken, 256, "lease_token", true)
	if err != nil {
		return nil, err
	}
	reportID, err := normalizeWorkerIdentity(in.ReportId, 128, "report_id", true)
	if err != nil {
		return nil, err
	}
	repository, err := requireSubmitModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	aggregate, err := repository.GetSubmission(l.ctx, in.SubmissionId)
	if err != nil {
		return nil, mapModelError(err)
	}
	if aggregate == nil || aggregate.CurrentAttempt == nil {
		return nil, status.Error(codes.Internal, "submit model returned an incomplete submission")
	}
	if aggregate.CurrentAttemptID != in.AttemptId {
		return &pb.ReportJudgeResultResp{
			Disposition: pb.ReportDisposition_REPORT_DISPOSITION_STALE_ATTEMPT,
		}, nil
	}
	if l.svcCtx == nil || l.svcCtx.ProblemCatalog == nil {
		return nil, status.Error(codes.Unavailable, "problem catalog is unavailable")
	}
	rawSpec, err := l.svcCtx.ProblemCatalog.GetJudgeProblemSpec(
		l.ctx, aggregate.ProblemID, aggregate.ProblemRevision,
	)
	if err != nil {
		return nil, mapDependencyError(err, "problem")
	}
	spec, err := normalizeProblem(rawSpec, aggregate.ProblemID, aggregate.ProblemRevision)
	if err != nil {
		return nil, err
	}
	limits := runtimeLimits(l.svcCtx)
	if err := validateProblemCaseLimit(spec, limits.MaxCaseResults); err != nil {
		return nil, err
	}
	normalized, err := normalizeJudgeResult(in, spec, limits)
	if err != nil {
		return nil, err
	}
	result, err := repository.ApplyJudgeResult(l.ctx, model.ApplyJudgeResultInput{
		WorkerID: workerID, SubmissionID: in.SubmissionId, AttemptID: in.AttemptId,
		LeaseToken: leaseToken, ReportID: reportID, Verdict: normalized.Verdict,
		Score: normalized.Score, MaxScore: normalized.MaxScore,
		CompileOutput: normalized.CompileOutput, CaseResults: normalized.CaseResults,
	})
	if errors.Is(err, model.ErrStaleAttempt) {
		return &pb.ReportJudgeResultResp{
			Disposition: pb.ReportDisposition_REPORT_DISPOSITION_STALE_ATTEMPT,
		}, nil
	}
	if err != nil {
		return nil, mapModelError(err)
	}
	if result == nil || result.Submission == nil {
		return nil, status.Error(codes.Internal, "submit model returned an empty report response")
	}
	var disposition pb.ReportDisposition
	switch result.Disposition {
	case model.ReportDispositionApplied:
		disposition = pb.ReportDisposition_REPORT_DISPOSITION_APPLIED
	case model.ReportDispositionDuplicate:
		disposition = pb.ReportDisposition_REPORT_DISPOSITION_DUPLICATE
	default:
		return nil, status.Error(codes.Internal, "submit model returned an invalid report disposition")
	}
	summary, err := submissionSummaryProto(
		result.Submission.SubmissionSummary,
		pb.ResultVisibility_RESULT_VISIBILITY_FULL,
	)
	if err != nil {
		return nil, err
	}
	return &pb.ReportJudgeResultResp{Disposition: disposition, Submission: summary}, nil
}
