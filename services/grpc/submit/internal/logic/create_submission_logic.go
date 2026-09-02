package logic

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateSubmissionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateSubmissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSubmissionLogic {
	return &CreateSubmissionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateSubmissionLogic) CreateSubmission(in *pb.CreateSubmissionReq) (*pb.CreateSubmissionResp, error) {
	principal, err := requirePrincipal(l.ctx)
	if err != nil {
		return nil, err
	}
	repository, err := requireSubmitModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	limits := runtimeLimits(l.svcCtx)
	if in == nil || in.ProblemId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "problem_id must be positive")
	}
	contestID, err := positiveOptional(in.ContestId, "contest_id")
	if err != nil {
		return nil, err
	}
	language := strings.TrimSpace(in.Language)
	if language == "" || len(language) > 64 || !validASCII(language) {
		return nil, status.Error(codes.InvalidArgument, "language is invalid")
	}
	if len(in.SourceCode) > limits.MaxSourceCodeBytes {
		return nil, status.Error(codes.ResourceExhausted, "source_code exceeds the limit")
	}
	if in.SourceCode == "" || !utf8.ValidString(in.SourceCode) {
		return nil, status.Error(codes.InvalidArgument, "source_code is invalid")
	}
	clientRequestID := strings.TrimSpace(in.ClientRequestId)
	if clientRequestID == "" || len(clientRequestID) > limits.MaxClientRequestBytes || !validASCII(clientRequestID) {
		return nil, status.Error(codes.InvalidArgument, "client_request_id is invalid")
	}
	submittedAt := serviceNow(l.svcCtx)
	if submittedAt.IsZero() {
		return nil, status.Error(codes.Internal, "submit service clock is invalid")
	}

	responseFromExisting := func(existing *model.SubmissionAggregate) (*pb.CreateSubmissionResp, error) {
		if !sameCreatePayload(existing, in.ProblemId, contestID, language, in.SourceCode) {
			return nil, status.Error(codes.Aborted, "client_request_id was used for different input")
		}
		visibilities, visibilityErr := resolveVisibilities(
			l.ctx, l.svcCtx, principal, []model.SubmissionSummary{existing.SubmissionSummary},
		)
		if visibilityErr != nil {
			return nil, visibilityErr
		}
		detail, detailErr := submissionDetailProto(existing, visibilities[existing.SubmissionID], true)
		if detailErr != nil {
			return nil, detailErr
		}
		return &pb.CreateSubmissionResp{Submission: detail, Deduplicated: true}, nil
	}
	recoverConcurrentCreate := func(fallback error) (*pb.CreateSubmissionResp, error) {
		if l.ctx.Err() != nil {
			return nil, fallback
		}
		existing, lookupErr := repository.GetSubmissionByClientRequestID(l.ctx, principal.UserID, clientRequestID)
		switch {
		case lookupErr == nil:
			return responseFromExisting(existing)
		case errors.Is(lookupErr, model.ErrSubmissionNotFound):
			return nil, fallback
		default:
			return nil, mapModelError(lookupErr)
		}
	}

	existing, lookupErr := repository.GetSubmissionByClientRequestID(l.ctx, principal.UserID, clientRequestID)
	if lookupErr == nil {
		return responseFromExisting(existing)
	}
	if !errors.Is(lookupErr, model.ErrSubmissionNotFound) {
		return nil, mapModelError(lookupErr)
	}

	if l.svcCtx == nil || l.svcCtx.ProblemCatalog == nil {
		return recoverConcurrentCreate(status.Error(codes.Unavailable, "problem catalog is unavailable"))
	}
	if l.svcCtx.RateLimiter != nil {
		if err := l.svcCtx.RateLimiter.Allow(l.ctx, principal.UserID, clientRequestID); err != nil {
			if errors.Is(err, svc.ErrSubmissionRateLimited) {
				return recoverConcurrentCreate(status.Error(codes.ResourceExhausted, "submission rate limit exceeded"))
			}
			return recoverConcurrentCreate(status.Error(codes.Unavailable, "submission rate limiter is unavailable"))
		}
	}
	rawSpec, err := l.svcCtx.ProblemCatalog.GetJudgeProblemSpec(l.ctx, in.ProblemId, 0)
	if err != nil {
		return recoverConcurrentCreate(mapDependencyError(err, "problem"))
	}
	spec, err := normalizeProblem(rawSpec, in.ProblemId, 0)
	if err != nil {
		return recoverConcurrentCreate(err)
	}
	if err := validateProblemCaseLimit(spec, limits.MaxCaseResults); err != nil {
		return recoverConcurrentCreate(err)
	}
	language, err = validateLanguage(spec, language)
	if err != nil {
		return recoverConcurrentCreate(err)
	}

	visibility := pb.ResultVisibility_RESULT_VISIBILITY_FULL
	if contestID != nil {
		if l.svcCtx.ContestPolicy == nil {
			return recoverConcurrentCreate(status.Error(codes.Unavailable, "contest policy is unavailable"))
		}
		validation, validationErr := l.svcCtx.ContestPolicy.ValidateSubmission(
			l.ctx, *contestID, in.ProblemId, principal.UserID, submittedAt,
		)
		if validationErr != nil {
			return recoverConcurrentCreate(mapDependencyError(validationErr, "contest"))
		}
		if validation == nil || !validation.Allowed {
			return recoverConcurrentCreate(status.Error(codes.FailedPrecondition, "contest submission is not allowed"))
		}
		visibility, err = createVisibility(validation)
		if err != nil {
			return recoverConcurrentCreate(err)
		}
	}
	result, err := repository.CreateSubmission(l.ctx, model.CreateSubmissionInput{
		UserID: principal.UserID, ProblemID: in.ProblemId, ContestID: contestID,
		ProblemRevision: spec.Revision, Language: language, SourceCode: in.SourceCode,
		ClientRequestID: clientRequestID, SubmittedAt: submittedAt,
	})
	if err != nil {
		return recoverConcurrentCreate(mapModelError(err))
	}
	if result == nil || result.Submission == nil {
		return nil, status.Error(codes.Internal, "submit model returned an empty response")
	}
	detail, err := submissionDetailProto(result.Submission, visibility, true)
	if err != nil {
		return nil, err
	}
	return &pb.CreateSubmissionResp{Submission: detail, Deduplicated: result.Deduplicated}, nil
}

func sameCreatePayload(
	existing *model.SubmissionAggregate,
	problemID int64,
	contestID *int64,
	language, sourceCode string,
) bool {
	if existing == nil || existing.ProblemID != problemID || existing.Language != language ||
		existing.SourceCode != sourceCode {
		return false
	}
	if existing.ContestID == nil || contestID == nil {
		return existing.ContestID == nil && contestID == nil
	}
	return *existing.ContestID == *contestID
}
