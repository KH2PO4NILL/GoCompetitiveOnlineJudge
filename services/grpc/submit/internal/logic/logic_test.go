package logic

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	infraauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	authpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	contestpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeSubmitModel struct {
	lookupFn    func(context.Context, int64, string) (*model.SubmissionAggregate, error)
	createFn    func(context.Context, model.CreateSubmissionInput) (*model.CreateSubmissionResult, error)
	getFn       func(context.Context, int64) (*model.SubmissionAggregate, error)
	listFn      func(context.Context, model.SubmissionListFilter) ([]model.SubmissionSummary, int64, error)
	rejudgeFn   func(context.Context, model.RejudgeSubmissionInput) (*model.RejudgeResult, error)
	batchFn     func(context.Context, model.BatchRejudgeInput) ([]model.RejudgeResult, error)
	acquireFn   func(context.Context, model.AcquireAttemptInput) (*model.AcquireAttemptResult, error)
	heartbeatFn func(context.Context, model.HeartbeatAttemptInput) (*model.HeartbeatResult, error)
	applyFn     func(context.Context, model.ApplyJudgeResultInput) (*model.ApplyJudgeResultResult, error)
}

func (f *fakeSubmitModel) CreateSubmission(ctx context.Context, input model.CreateSubmissionInput) (*model.CreateSubmissionResult, error) {
	if f.createFn == nil {
		return nil, errors.New("unexpected CreateSubmission")
	}
	return f.createFn(ctx, input)
}
func (f *fakeSubmitModel) GetSubmissionByClientRequestID(ctx context.Context, userID int64, key string) (*model.SubmissionAggregate, error) {
	if f.lookupFn == nil {
		return nil, model.ErrSubmissionNotFound
	}
	return f.lookupFn(ctx, userID, key)
}
func (f *fakeSubmitModel) GetSubmission(ctx context.Context, id int64) (*model.SubmissionAggregate, error) {
	if f.getFn == nil {
		return nil, errors.New("unexpected GetSubmission")
	}
	return f.getFn(ctx, id)
}
func (f *fakeSubmitModel) ListSubmissions(ctx context.Context, filter model.SubmissionListFilter) ([]model.SubmissionSummary, int64, error) {
	if f.listFn == nil {
		return nil, 0, errors.New("unexpected ListSubmissions")
	}
	return f.listFn(ctx, filter)
}
func (f *fakeSubmitModel) RejudgeSubmission(ctx context.Context, input model.RejudgeSubmissionInput) (*model.RejudgeResult, error) {
	if f.rejudgeFn == nil {
		return nil, errors.New("unexpected RejudgeSubmission")
	}
	return f.rejudgeFn(ctx, input)
}
func (f *fakeSubmitModel) BatchRejudgeSubmissions(ctx context.Context, input model.BatchRejudgeInput) ([]model.RejudgeResult, error) {
	if f.batchFn == nil {
		return nil, errors.New("unexpected BatchRejudgeSubmissions")
	}
	return f.batchFn(ctx, input)
}
func (f *fakeSubmitModel) AcquireAttempt(ctx context.Context, input model.AcquireAttemptInput) (*model.AcquireAttemptResult, error) {
	if f.acquireFn == nil {
		return nil, errors.New("unexpected AcquireAttempt")
	}
	return f.acquireFn(ctx, input)
}
func (f *fakeSubmitModel) HeartbeatAttempt(ctx context.Context, input model.HeartbeatAttemptInput) (*model.HeartbeatResult, error) {
	if f.heartbeatFn == nil {
		return nil, errors.New("unexpected HeartbeatAttempt")
	}
	return f.heartbeatFn(ctx, input)
}
func (f *fakeSubmitModel) ApplyJudgeResult(ctx context.Context, input model.ApplyJudgeResultInput) (*model.ApplyJudgeResultResult, error) {
	if f.applyFn == nil {
		return nil, errors.New("unexpected ApplyJudgeResult")
	}
	return f.applyFn(ctx, input)
}
func (*fakeSubmitModel) ListExpiredLeases(context.Context, time.Time, int) ([]model.ExpiredLease, error) {
	return nil, errors.New("unexpected ListExpiredLeases")
}
func (*fakeSubmitModel) SupersedeExpiredAttempt(context.Context, model.SupersedeAttemptInput) (*model.Attempt, error) {
	return nil, errors.New("unexpected SupersedeExpiredAttempt")
}
func (*fakeSubmitModel) ClaimOutboxEvents(context.Context, model.ClaimOutboxInput) ([]model.OutboxEvent, error) {
	return nil, errors.New("unexpected ClaimOutboxEvents")
}
func (*fakeSubmitModel) MarkOutboxPublished(context.Context, model.MarkOutboxPublishedInput) error {
	return errors.New("unexpected MarkOutboxPublished")
}
func (*fakeSubmitModel) MarkOutboxFailed(context.Context, model.MarkOutboxFailedInput) error {
	return errors.New("unexpected MarkOutboxFailed")
}

type fakeProblemCatalog struct {
	fn func(context.Context, int64, int64) (*problempb.JudgeProblemSpec, error)
}

func (f fakeProblemCatalog) GetJudgeProblemSpec(ctx context.Context, problemID, revision int64) (*problempb.JudgeProblemSpec, error) {
	return f.fn(ctx, problemID, revision)
}

type fakeContestPolicy struct {
	validateFn func(context.Context, int64, int64, int64, time.Time) (*svc.ContestValidation, error)
	resolveFn  func(context.Context, []svc.SubmissionReference, time.Time) (map[int64]svc.ResultVisibility, error)
}

func (f fakeContestPolicy) ValidateSubmission(ctx context.Context, contestID, problemID, userID int64, at time.Time) (*svc.ContestValidation, error) {
	if f.validateFn == nil {
		return nil, errors.New("unexpected ValidateSubmission")
	}
	return f.validateFn(ctx, contestID, problemID, userID, at)
}
func (f fakeContestPolicy) ResolveResultVisibility(ctx context.Context, refs []svc.SubmissionReference, at time.Time) (map[int64]svc.ResultVisibility, error) {
	if f.resolveFn == nil {
		return nil, errors.New("unexpected ResolveResultVisibility")
	}
	return f.resolveFn(ctx, refs, at)
}

type fakeRateLimiter struct {
	fn func(context.Context, int64, string) error
}

func (f fakeRateLimiter) Allow(ctx context.Context, userID int64, key string) error {
	return f.fn(ctx, userID, key)
}

func userContext(userID int64, role authpb.UserRole) context.Context {
	return infraauth.WithPrincipal(context.Background(), infraauth.Principal{
		UserID: userID, Role: role, Status: authpb.UserStatus_USER_STATUS_NORMAL,
	})
}

func workerContext() context.Context {
	return infraauth.WithServicePrincipal(context.Background(), infraauth.ServicePrincipal{Name: judgeWorkerService})
}

func judgeSpec() *problempb.JudgeProblemSpec {
	return &problempb.JudgeProblemSpec{
		ProblemId: 9, Revision: 7,
		JudgeConfig: &problempb.JudgeConfig{
			Settings: &problempb.JudgeSettings{
				TimeLimitMs: 1000, MemoryLimitMb: 256,
				AllowedLanguages: []string{"go1.26", "cpp20"},
				CheckerType:      problempb.CheckerType_CHECKER_TYPE_STANDARD,
			},
			TestDataVersion: "test-data-v1",
		},
		TestDataPackageKey: "problem-test-data/9/test-data-v1.zip",
		TestDataChecksum:   strings.Repeat("a", 64),
		TestCases: []*problempb.TestCaseInfo{
			{TestCaseId: 101, Ordinal: 0, GroupName: "sample", Score: 40, Enabled: true},
			{TestCaseId: 102, Ordinal: 1, GroupName: "main", Score: 60, Enabled: true},
		},
	}
}

func aggregate(statusValue int64, contestID *int64) *model.SubmissionAggregate {
	at := time.Date(2026, 9, 2, 1, 2, 3, 0, time.UTC)
	result := &model.SubmissionAggregate{
		SubmissionSummary: model.SubmissionSummary{
			SubmissionID: 11, UserID: 7, ProblemID: 9, ContestID: contestID,
			ProblemRevision: 7, Language: "go1.26", Status: statusValue,
			AttemptCount: 1, CurrentAttemptID: 21, SubmittedAt: at,
		},
		SourceCode: "package main", CurrentAttempt: &model.Attempt{
			AttemptID: 21, SubmissionID: 11, AttemptNo: 1, Status: model.AttemptStatusQueued,
			CreatedAt: at, UpdatedAt: at, EnqueuedAt: at,
		},
	}
	if statusValue == model.SubmissionStatusFinished {
		verdict, score, maxScore := int64(model.VerdictWrongAnswer), int64(40), int64(100)
		result.Verdict, result.Score, result.MaxScore = &verdict, &score, &maxScore
		result.FinishedAt = &at
		result.CurrentAttempt.Status = model.AttemptStatusFinished
		result.CurrentAttempt.CompileOutput = "private compiler output"
		result.CaseResults = []model.CaseResult{{
			TestCaseID: 101, Ordinal: 0, GroupName: "sample", Verdict: model.VerdictWrongAnswer,
			Score: 0, TimeUsedMS: 3, MemoryUsedKB: 4, JudgeMessage: "private diagnostics",
		}}
	}
	return result
}

func TestCreateSubmissionBindsRevisionContestTimeAndVisibility(t *testing.T) {
	now := time.Date(2026, 9, 2, 2, 0, 0, 0, time.UTC)
	contestID := int64(30)
	var contestAt time.Time
	var created model.CreateSubmissionInput
	repository := &fakeSubmitModel{createFn: func(_ context.Context, input model.CreateSubmissionInput) (*model.CreateSubmissionResult, error) {
		created = input
		return &model.CreateSubmissionResult{Submission: aggregate(model.SubmissionStatusQueued, &contestID)}, nil
	}}
	ctx := &svc.ServiceContext{
		SubmitModel: repository, Now: func() time.Time { return now },
		ProblemCatalog: fakeProblemCatalog{fn: func(_ context.Context, problemID, revision int64) (*problempb.JudgeProblemSpec, error) {
			if problemID != 9 || revision != 0 {
				t.Fatalf("problem lookup = (%d, %d)", problemID, revision)
			}
			return judgeSpec(), nil
		}},
		ContestPolicy: fakeContestPolicy{validateFn: func(_ context.Context, gotContest, problemID, userID int64, at time.Time) (*svc.ContestValidation, error) {
			if gotContest != contestID || problemID != 9 || userID != 7 {
				t.Fatalf("contest validation = (%d, %d, %d)", gotContest, problemID, userID)
			}
			contestAt = at
			return &svc.ContestValidation{Allowed: true, Mode: contestpb.ContestMode_CONTEST_MODE_OI}, nil
		}},
		RateLimiter: fakeRateLimiter{fn: func(_ context.Context, userID int64, key string) error {
			if userID != 7 || key != "request-1" {
				t.Fatalf("rate limit identity = (%d, %q)", userID, key)
			}
			return nil
		}},
	}
	response, err := NewCreateSubmissionLogic(userContext(7, authpb.UserRole_USER_ROLE_USER), ctx).CreateSubmission(
		&pb.CreateSubmissionReq{ProblemId: 9, ContestId: &contestID, Language: " go1.26 ", SourceCode: "package main", ClientRequestId: "request-1"},
	)
	if err != nil {
		t.Fatalf("CreateSubmission() error = %v", err)
	}
	if created.UserID != 7 || created.ProblemRevision != 7 || created.Language != "go1.26" || !created.SubmittedAt.Equal(now) {
		t.Fatalf("created input = %+v", created)
	}
	if !contestAt.Equal(created.SubmittedAt) {
		t.Fatalf("contest time %v differs from persisted time %v", contestAt, created.SubmittedAt)
	}
	if response.Submission.Summary.ResultVisibility != pb.ResultVisibility_RESULT_VISIBILITY_STATUS_ONLY {
		t.Fatalf("visibility = %s", response.Submission.Summary.ResultVisibility)
	}
}

func TestCreateSubmissionRetryUsesStoredAggregateBeforeDependencies(t *testing.T) {
	existing := aggregate(model.SubmissionStatusQueued, nil)
	repository := &fakeSubmitModel{lookupFn: func(_ context.Context, userID int64, key string) (*model.SubmissionAggregate, error) {
		return existing, nil
	}}
	response, err := NewCreateSubmissionLogic(userContext(7, authpb.UserRole_USER_ROLE_USER), &svc.ServiceContext{
		SubmitModel: repository,
	}).CreateSubmission(&pb.CreateSubmissionReq{
		ProblemId: 9, Language: "go1.26", SourceCode: "package main", ClientRequestId: "same-key",
	})
	if err != nil {
		t.Fatalf("CreateSubmission() retry error = %v", err)
	}
	if !response.Deduplicated || response.Submission.Summary.SubmissionId != 11 {
		t.Fatalf("retry response = %+v", response)
	}
}

func TestCreateSubmissionRecoversConcurrentCommitAfterContestCloses(t *testing.T) {
	contestID := int64(30)
	existing := aggregate(model.SubmissionStatusQueued, &contestID)
	lookupCalls := 0
	repository := &fakeSubmitModel{lookupFn: func(_ context.Context, userID int64, key string) (*model.SubmissionAggregate, error) {
		lookupCalls++
		if lookupCalls == 1 {
			return nil, model.ErrSubmissionNotFound
		}
		return existing, nil
	}}
	response, err := NewCreateSubmissionLogic(userContext(7, authpb.UserRole_USER_ROLE_USER), &svc.ServiceContext{
		SubmitModel: repository,
		ProblemCatalog: fakeProblemCatalog{fn: func(context.Context, int64, int64) (*problempb.JudgeProblemSpec, error) {
			return judgeSpec(), nil
		}},
		ContestPolicy: fakeContestPolicy{
			validateFn: func(context.Context, int64, int64, int64, time.Time) (*svc.ContestValidation, error) {
				return &svc.ContestValidation{Allowed: false, Mode: contestpb.ContestMode_CONTEST_MODE_OI}, nil
			},
			resolveFn: func(_ context.Context, refs []svc.SubmissionReference, _ time.Time) (map[int64]svc.ResultVisibility, error) {
				return map[int64]svc.ResultVisibility{refs[0].SubmissionID: svc.ResultVisibilityFull}, nil
			},
		},
	}).CreateSubmission(&pb.CreateSubmissionReq{
		ProblemId: 9, ContestId: &contestID, Language: "go1.26", SourceCode: "package main", ClientRequestId: "same-key",
	})
	if err != nil {
		t.Fatalf("CreateSubmission() concurrent retry error = %v", err)
	}
	if lookupCalls != 2 || !response.Deduplicated || response.Submission.Summary.SubmissionId != existing.SubmissionID {
		t.Fatalf("lookup calls/response = %d / %+v", lookupCalls, response)
	}
}

func TestGetSubmissionHidesOIResultAndDiagnostics(t *testing.T) {
	contestID := int64(30)
	existing := aggregate(model.SubmissionStatusFinished, &contestID)
	response, err := NewGetSubmissionLogic(userContext(7, authpb.UserRole_USER_ROLE_USER), &svc.ServiceContext{
		SubmitModel: &fakeSubmitModel{getFn: func(context.Context, int64) (*model.SubmissionAggregate, error) {
			return existing, nil
		}},
		ContestPolicy: fakeContestPolicy{resolveFn: func(_ context.Context, refs []svc.SubmissionReference, _ time.Time) (map[int64]svc.ResultVisibility, error) {
			return map[int64]svc.ResultVisibility{refs[0].SubmissionID: svc.ResultVisibilityStatusOnly}, nil
		}},
	}).GetSubmission(&pb.GetSubmissionReq{SubmissionId: 11})
	if err != nil {
		t.Fatalf("GetSubmission() error = %v", err)
	}
	detail := response.Submission
	if detail.Summary.Verdict != pb.Verdict_VERDICT_UNSPECIFIED || detail.Summary.Score != 0 ||
		detail.CompileOutput != "" || len(detail.CaseResults) != 0 {
		t.Fatalf("hidden detail leaked result: %+v", detail)
	}
	if detail.SourceCode == "" {
		t.Fatal("owner source code was unexpectedly hidden")
	}
}

func TestListSubmissionsRejectsAnotherUserBeforeModel(t *testing.T) {
	other := int64(8)
	_, err := NewListSubmissionsLogic(userContext(7, authpb.UserRole_USER_ROLE_USER), &svc.ServiceContext{
		SubmitModel: &fakeSubmitModel{},
	}).ListSubmissions(&pb.ListSubmissionsReq{UserId: &other})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("ListSubmissions() code = %s, want PermissionDenied", status.Code(err))
	}
}

func TestListSubmissionsRejectsUserVerdictFilterToPreventHiddenResultInference(t *testing.T) {
	verdict := pb.Verdict_VERDICT_ACCEPTED
	_, err := NewListSubmissionsLogic(userContext(7, authpb.UserRole_USER_ROLE_USER), &svc.ServiceContext{
		SubmitModel: &fakeSubmitModel{},
	}).ListSubmissions(&pb.ListSubmissionsReq{Verdict: &verdict})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("ListSubmissions() verdict filter code = %s, want PermissionDenied", status.Code(err))
	}
}

func TestListSubmissionsUsesStableUserScopedFilter(t *testing.T) {
	existing := aggregate(model.SubmissionStatusQueued, nil)
	repository := &fakeSubmitModel{listFn: func(_ context.Context, filter model.SubmissionListFilter) ([]model.SubmissionSummary, int64, error) {
		if filter.UserID == nil || *filter.UserID != 7 || filter.Page != 1 || filter.PageSize != 20 {
			t.Fatalf("list filter = %+v", filter)
		}
		return []model.SubmissionSummary{existing.SubmissionSummary}, 1, nil
	}}
	response, err := NewListSubmissionsLogic(userContext(7, authpb.UserRole_USER_ROLE_USER), &svc.ServiceContext{SubmitModel: repository}).
		ListSubmissions(&pb.ListSubmissionsReq{})
	if err != nil {
		t.Fatalf("ListSubmissions() error = %v", err)
	}
	if response.Total != 1 || len(response.Submissions) != 1 ||
		response.Submissions[0].ResultVisibility != pb.ResultVisibility_RESULT_VISIBILITY_FULL {
		t.Fatalf("list response = %+v", response)
	}
}

func TestRejudgeUsesAuthenticatedAdministrator(t *testing.T) {
	var input model.RejudgeSubmissionInput
	repository := &fakeSubmitModel{rejudgeFn: func(_ context.Context, got model.RejudgeSubmissionInput) (*model.RejudgeResult, error) {
		input = got
		return &model.RejudgeResult{SubmissionID: 11, AttemptID: 22, Submission: aggregate(model.SubmissionStatusQueued, nil)}, nil
	}}
	response, err := NewRejudgeSubmissionLogic(userContext(99, authpb.UserRole_USER_ROLE_ADMIN), &svc.ServiceContext{SubmitModel: repository}).
		RejudgeSubmission(&pb.RejudgeSubmissionReq{SubmissionId: 11, Reason: " repair "})
	if err != nil {
		t.Fatalf("RejudgeSubmission() error = %v", err)
	}
	if input.RequestedBy != 99 || input.Reason != "repair" || response.AttemptId != 22 {
		t.Fatalf("rejudge input/response = %+v / %+v", input, response)
	}
}

func TestBatchRejudgeCallsOneAtomicModelOperation(t *testing.T) {
	calls := 0
	repository := &fakeSubmitModel{batchFn: func(_ context.Context, input model.BatchRejudgeInput) ([]model.RejudgeResult, error) {
		calls++
		if input.RequestedBy != 99 || len(input.SubmissionIDs) != 2 {
			t.Fatalf("batch input = %+v", input)
		}
		return []model.RejudgeResult{{SubmissionID: 11, AttemptID: 21}, {SubmissionID: 12, AttemptID: 22}}, nil
	}}
	response, err := NewBatchRejudgeSubmissionsLogic(userContext(99, authpb.UserRole_USER_ROLE_ADMIN), &svc.ServiceContext{SubmitModel: repository}).
		BatchRejudgeSubmissions(&pb.BatchRejudgeSubmissionsReq{SubmissionIds: []int64{11, 12}, Reason: "repair"})
	if err != nil {
		t.Fatalf("BatchRejudgeSubmissions() error = %v", err)
	}
	if calls != 1 || len(response.Accepted) != 2 || response.Accepted[1].AttemptId != 22 {
		t.Fatalf("batch calls/response = %d / %+v", calls, response)
	}
}

func TestAcquireJudgeTaskReturnsStructuredDenialWithoutSecrets(t *testing.T) {
	repository := &fakeSubmitModel{acquireFn: func(context.Context, model.AcquireAttemptInput) (*model.AcquireAttemptResult, error) {
		return &model.AcquireAttemptResult{Acquired: false, DenyReason: model.AcquireDenyStaleAttempt}, nil
	}}
	response, err := NewAcquireJudgeTaskLogic(workerContext(), &svc.ServiceContext{SubmitModel: repository}).AcquireJudgeTask(
		&pb.AcquireJudgeTaskReq{WorkerId: "worker-1", SubmissionId: 11, AttemptId: 21, DeliveryId: "delivery-1"},
	)
	if err != nil {
		t.Fatalf("AcquireJudgeTask() error = %v", err)
	}
	if response.Acquired || response.DenyReason != pb.AcquireDenyReason_ACQUIRE_DENY_REASON_STALE_ATTEMPT ||
		response.Task != nil || response.LeaseToken != "" || response.LeaseExpiresAt != nil {
		t.Fatalf("acquire denial leaked fields: %+v", response)
	}
}

func TestAcquireJudgeTaskReturnsLeaseOnlyOnSuccess(t *testing.T) {
	expires := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
	repository := &fakeSubmitModel{acquireFn: func(_ context.Context, input model.AcquireAttemptInput) (*model.AcquireAttemptResult, error) {
		return &model.AcquireAttemptResult{
			Acquired: true, SubmissionID: input.SubmissionID, AttemptID: input.AttemptID,
			ProblemID: 9, ProblemRevision: 7, Language: "go1.26", SourceCode: "package main",
			LeaseToken: "lease-secret", LeaseExpiresAt: &expires,
		}, nil
	}}
	response, err := NewAcquireJudgeTaskLogic(workerContext(), &svc.ServiceContext{SubmitModel: repository}).AcquireJudgeTask(
		&pb.AcquireJudgeTaskReq{WorkerId: "worker-1", SubmissionId: 11, AttemptId: 21},
	)
	if err != nil {
		t.Fatalf("AcquireJudgeTask() success error = %v", err)
	}
	if !response.Acquired || response.DenyReason != pb.AcquireDenyReason_ACQUIRE_DENY_REASON_UNSPECIFIED ||
		response.Task == nil || response.Task.ProblemRevision != 7 || response.LeaseToken != "lease-secret" {
		t.Fatalf("acquire success = %+v", response)
	}
}

func TestHeartbeatRenewsLeaseWithoutProblemDependency(t *testing.T) {
	var heartbeat model.HeartbeatAttemptInput
	expires := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
	repository := &fakeSubmitModel{
		heartbeatFn: func(_ context.Context, input model.HeartbeatAttemptInput) (*model.HeartbeatResult, error) {
			heartbeat = input
			return &model.HeartbeatResult{LeaseExpiresAt: expires, CompletedTestCases: input.CompletedTestCases}, nil
		},
	}
	_, err := NewHeartbeatJudgeTaskLogic(workerContext(), &svc.ServiceContext{
		SubmitModel: repository,
		ProblemCatalog: fakeProblemCatalog{fn: func(context.Context, int64, int64) (*problempb.JudgeProblemSpec, error) {
			t.Fatal("heartbeat unexpectedly called Problem RPC")
			return nil, nil
		}},
	}).HeartbeatJudgeTask(&pb.HeartbeatJudgeTaskReq{
		WorkerId: "worker-1", SubmissionId: 11, AttemptId: 21, LeaseToken: "lease", CompletedTestCases: 1,
	})
	if err != nil {
		t.Fatalf("HeartbeatJudgeTask() error = %v", err)
	}
	if heartbeat.CompletedTestCases != 1 {
		t.Fatalf("heartbeat input = %+v", heartbeat)
	}
}

func TestReportJudgeResultCanonicalizesAndRecomputesWorkerPayload(t *testing.T) {
	existing := aggregate(model.SubmissionStatusJudging, nil)
	existing.CurrentAttempt.Status = model.AttemptStatusJudging
	var applied model.ApplyJudgeResultInput
	repository := &fakeSubmitModel{
		getFn: func(context.Context, int64) (*model.SubmissionAggregate, error) { return existing, nil },
		applyFn: func(_ context.Context, input model.ApplyJudgeResultInput) (*model.ApplyJudgeResultResult, error) {
			applied = input
			finished := aggregate(model.SubmissionStatusFinished, nil)
			return &model.ApplyJudgeResultResult{Disposition: model.ReportDispositionApplied, Submission: finished}, nil
		},
	}
	response, err := NewReportJudgeResultLogic(workerContext(), &svc.ServiceContext{
		SubmitModel: repository,
		ProblemCatalog: fakeProblemCatalog{fn: func(context.Context, int64, int64) (*problempb.JudgeProblemSpec, error) {
			return judgeSpec(), nil
		}},
	}).ReportJudgeResult(&pb.ReportJudgeResultReq{
		WorkerId: "worker-1", SubmissionId: 11, AttemptId: 21, LeaseToken: "lease", ReportId: "report-1",
		Verdict: pb.Verdict_VERDICT_ACCEPTED, Score: 999, MaxScore: 999,
		CaseResults: []*pb.JudgeTestCaseResult{
			{TestCaseId: 102, Ordinal: 99, GroupName: "untrusted", Verdict: pb.Verdict_VERDICT_WRONG_ANSWER, Score: 0, JudgeMessage: "diag"},
			{TestCaseId: 101, Ordinal: 88, GroupName: "untrusted", Verdict: pb.Verdict_VERDICT_ACCEPTED, Score: 40},
		},
	})
	if err != nil {
		t.Fatalf("ReportJudgeResult() error = %v", err)
	}
	if applied.Verdict != model.VerdictWrongAnswer || applied.Score != 40 || applied.MaxScore != 100 {
		t.Fatalf("server aggregate = %+v", applied)
	}
	if len(applied.CaseResults) != 2 || applied.CaseResults[0].TestCaseID != 101 ||
		applied.CaseResults[0].Ordinal != 0 || applied.CaseResults[1].GroupName != "main" {
		t.Fatalf("canonical cases = %+v", applied.CaseResults)
	}
	if response.Disposition != pb.ReportDisposition_REPORT_DISPOSITION_APPLIED {
		t.Fatalf("disposition = %s", response.Disposition)
	}
}

func TestReportJudgeResultReturnsStaleDispositionBeforeProblemRPC(t *testing.T) {
	existing := aggregate(model.SubmissionStatusJudging, nil)
	existing.CurrentAttemptID = 999
	response, err := NewReportJudgeResultLogic(workerContext(), &svc.ServiceContext{
		SubmitModel: &fakeSubmitModel{getFn: func(context.Context, int64) (*model.SubmissionAggregate, error) {
			return existing, nil
		}},
	}).ReportJudgeResult(&pb.ReportJudgeResultReq{
		WorkerId: "worker-1", SubmissionId: 11, AttemptId: 21, LeaseToken: "lease", ReportId: "report-1",
	})
	if err != nil {
		t.Fatalf("ReportJudgeResult() stale error = %v", err)
	}
	if response.Disposition != pb.ReportDisposition_REPORT_DISPOSITION_STALE_ATTEMPT {
		t.Fatalf("stale disposition = %s", response.Disposition)
	}
}

func TestAggregateVerdictPriority(t *testing.T) {
	tests := []struct {
		name     string
		verdicts []pb.Verdict
		want     pb.Verdict
	}{
		{"accepted", []pb.Verdict{pb.Verdict_VERDICT_ACCEPTED}, pb.Verdict_VERDICT_ACCEPTED},
		{"wrong beats partial", []pb.Verdict{pb.Verdict_VERDICT_PARTIAL_ACCEPTED, pb.Verdict_VERDICT_WRONG_ANSWER}, pb.Verdict_VERDICT_WRONG_ANSWER},
		{"system beats runtime", []pb.Verdict{pb.Verdict_VERDICT_RUNTIME_ERROR, pb.Verdict_VERDICT_SYSTEM_ERROR}, pb.Verdict_VERDICT_SYSTEM_ERROR},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := aggregateVerdict(test.verdicts, 100, 100); got != test.want {
				t.Fatalf("aggregateVerdict() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestCompileErrorReportRequiresNoCasesAndUsesProblemMaxScore(t *testing.T) {
	spec, err := normalizeProblem(judgeSpec(), 9, 7)
	if err != nil {
		t.Fatalf("normalizeProblem() error = %v", err)
	}
	result, err := normalizeJudgeResult(&pb.ReportJudgeResultReq{
		Verdict: pb.Verdict_VERDICT_COMPILE_ERROR, CompileOutput: "compiler failed",
	}, spec, runtimeLimits(nil))
	if err != nil {
		t.Fatalf("normalizeJudgeResult() compile error = %v", err)
	}
	if result.Verdict != model.VerdictCompileError || result.Score != 0 || result.MaxScore != 100 || len(result.CaseResults) != 0 {
		t.Fatalf("compile result = %+v", result)
	}
}

func TestNormalizeProblemRejectsIncompleteRuntimeConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*problempb.JudgeProblemSpec)
	}{
		{"missing time limit", func(spec *problempb.JudgeProblemSpec) { spec.JudgeConfig.Settings.TimeLimitMs = 0 }},
		{"missing memory limit", func(spec *problempb.JudgeProblemSpec) { spec.JudgeConfig.Settings.MemoryLimitMb = 0 }},
		{"invalid checker", func(spec *problempb.JudgeProblemSpec) {
			spec.JudgeConfig.Settings.CheckerType = problempb.CheckerType_CHECKER_TYPE_UNSPECIFIED
		}},
		{"missing test data version", func(spec *problempb.JudgeProblemSpec) { spec.JudgeConfig.TestDataVersion = "" }},
		{"missing package key", func(spec *problempb.JudgeProblemSpec) { spec.TestDataPackageKey = "" }},
		{"invalid checksum", func(spec *problempb.JudgeProblemSpec) { spec.TestDataChecksum = "not-sha256" }},
		{"missing special checker", func(spec *problempb.JudgeProblemSpec) {
			spec.JudgeConfig.Settings.CheckerType = problempb.CheckerType_CHECKER_TYPE_SPECIAL_JUDGE
		}},
		{"missing interactor", func(spec *problempb.JudgeProblemSpec) {
			spec.JudgeConfig.Settings.CheckerType = problempb.CheckerType_CHECKER_TYPE_INTERACTIVE
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := judgeSpec()
			test.mutate(spec)
			if _, err := normalizeProblem(spec, 9, 7); status.Code(err) != codes.Unavailable {
				t.Fatalf("normalizeProblem() code = %s, want Unavailable", status.Code(err))
			}
		})
	}
}

func TestLogicMethodIdentityBoundary(t *testing.T) {
	_, err := NewCreateSubmissionLogic(workerContext(), &svc.ServiceContext{}).CreateSubmission(&pb.CreateSubmissionReq{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("worker calling user method code = %s", status.Code(err))
	}
	_, err = NewAcquireJudgeTaskLogic(userContext(7, authpb.UserRole_USER_ROLE_ADMIN), &svc.ServiceContext{}).
		AcquireJudgeTask(&pb.AcquireJudgeTaskReq{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("user calling worker method code = %s", status.Code(err))
	}
}
