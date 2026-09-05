package processor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync/atomic"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/checker"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/engine"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/testdatacache"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	submitpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Action string

const (
	ACK        Action = "ack"
	Retry      Action = "retry"
	DeadLetter Action = "dead_letter"
)

type Result struct {
	Action Action
	Reason string
}
type DataCache interface {
	Prepare(context.Context, int64, *problempb.JudgeProblemSpec) (*testdatacache.Package, error)
}
type Judge interface {
	Judge(context.Context, engine.JudgeInput) (engine.Outcome, error)
}
type Options struct {
	WorkerID                        string
	Submit                          SubmitGateway
	Problem                         ProblemGateway
	Cache                           DataCache
	Judge                           Judge
	Logger                          *slog.Logger
	Profiles                        map[string]string
	Checker                         checker.Mode
	HeartbeatInterval, RetryBackoff time.Duration
	ReportAttempts                  int
}
type Processor struct{ options Options }

var ErrLeaseLost = errors.New("lease explicitly invalidated")

func New(options Options) (*Processor, error) {
	if options.WorkerID == "" || options.Submit == nil || options.Problem == nil || options.Cache == nil || options.Judge == nil || options.Logger == nil || len(options.Profiles) == 0 || options.HeartbeatInterval <= 0 || options.RetryBackoff <= 0 || options.ReportAttempts < 1 || options.ReportAttempts > 16 {
		return nil, errors.New("invalid processor dependencies or bounded retry configuration")
	}
	options.Profiles = maps.Clone(options.Profiles)
	return &Processor{options: options}, nil
}

func (processor *Processor) Process(parent context.Context, event *submitpb.JudgeTaskEvent) Result {
	if event == nil || event.SubmissionId <= 0 || event.AttemptId <= 0 || event.EventId == "" || len(event.EventId) > 128 {
		return Result{DeadLetter, "invalid_event"}
	}
	if parent.Err() != nil {
		return Result{Retry, "cancelled"}
	}
	options := processor.options
	logger := options.Logger.With("submission_id", event.SubmissionId, "attempt_id", event.AttemptId, "worker_id", options.WorkerID)
	logger.InfoContext(parent, "judge task acquire", "stage", "acquire")
	lease, err := options.Submit.Acquire(parent, &submitpb.AcquireJudgeTaskReq{WorkerId: options.WorkerID, SubmissionId: event.SubmissionId, AttemptId: event.AttemptId, DeliveryId: event.EventId})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return Result{ACK, "stale"}
		}
		return Result{Retry, "acquire_failed"}
	}
	if lease == nil {
		return Result{Retry, "invalid_acquire_response"}
	}
	if !lease.Acquired {
		switch lease.DenyReason {
		case submitpb.AcquireDenyReason_ACQUIRE_DENY_REASON_ALREADY_FINISHED:
			return Result{ACK, "finished"}
		case submitpb.AcquireDenyReason_ACQUIRE_DENY_REASON_STALE_ATTEMPT:
			return Result{ACK, "stale"}
		case submitpb.AcquireDenyReason_ACQUIRE_DENY_REASON_LEASED_BY_OTHER_WORKER:
			return Result{ACK, "leased_by_other"}
		default:
			return Result{Retry, "invalid_acquire_denial"}
		}
	}
	task := lease.Task
	if lease.DenyReason != submitpb.AcquireDenyReason_ACQUIRE_DENY_REASON_UNSPECIFIED || task == nil || task.SubmissionId != event.SubmissionId || task.AttemptId != event.AttemptId || task.ProblemId <= 0 || task.ProblemRevision <= 0 || lease.LeaseToken == "" || lease.LeaseExpiresAt == nil || lease.LeaseExpiresAt.CheckValid() != nil || !lease.LeaseExpiresAt.AsTime().After(time.Now()) {
		return Result{Retry, "invalid_lease"}
	}
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	var progress atomic.Value
	progress.Store(engine.Progress{Stage: "spec"})
	heartbeatDone := make(chan struct{})
	go func() { defer close(heartbeatDone); processor.heartbeat(ctx, cancel, lease, &progress, logger) }()
	defer func() { cancel(nil); <-heartbeatDone }()
	failure := func(reason string) Result {
		if errors.Is(context.Cause(ctx), ErrLeaseLost) {
			return Result{ACK, "stale"}
		}
		return Result{Retry, reason}
	}
	spec, err := options.Problem.Spec(ctx, task.ProblemId, task.ProblemRevision)
	if err != nil || spec == nil || spec.ProblemId != task.ProblemId || spec.Revision != task.ProblemRevision {
		return failure("problem_revision_failed")
	}
	profile, exists := options.Profiles[task.Language]
	if !exists {
		return Result{DeadLetter, "unsupported_language"}
	}
	if spec.JudgeConfig.GetSettings() == nil || !slices.Contains(spec.JudgeConfig.Settings.AllowedLanguages, task.Language) {
		return failure("invalid_problem_languages")
	}
	progress.Store(engine.Progress{Stage: "download"})
	data, err := options.Cache.Prepare(ctx, task.ProblemRevision, spec)
	if err != nil || data == nil {
		return failure("test_data_failed")
	}
	defer data.Release()
	logger.InfoContext(ctx, "judge task started", "stage", "judge", "revision", task.ProblemRevision, "profile_id", profile)
	outcome, err := options.Judge.Judge(ctx, engine.JudgeInput{Request: engine.Request{AttemptID: task.AttemptId, SubmissionID: task.SubmissionId, ProfileID: profile, SourceCode: task.SourceCode}, Spec: spec, Package: data, Checker: options.Checker, Progress: func(value engine.Progress) { progress.Store(value) }})
	if err != nil || ctx.Err() != nil {
		return failure("judge_failed")
	}
	report := makeReport(options.WorkerID, lease, outcome)
	progress.Store(engine.Progress{Stage: "report", Completed: int32(len(outcome.Cases)), Total: int32(len(outcome.Cases))})
	logger.InfoContext(ctx, "judge task reporting", "stage", "report", "revision", task.ProblemRevision, "report_id", report.ReportId, "policy", outcome.Policy)
	for attempt := 0; attempt < options.ReportAttempts; attempt++ {
		response, err := options.Submit.Report(ctx, report)
		if err == nil && response != nil {
			switch response.Disposition {
			case submitpb.ReportDisposition_REPORT_DISPOSITION_APPLIED:
				return Result{ACK, "applied"}
			case submitpb.ReportDisposition_REPORT_DISPOSITION_DUPLICATE:
				return Result{ACK, "duplicate"}
			case submitpb.ReportDisposition_REPORT_DISPOSITION_STALE_ATTEMPT:
				return Result{ACK, "stale"}
			}
		}
		if ctx.Err() != nil {
			return failure("report_cancelled")
		}
		if attempt+1 < options.ReportAttempts && !sleep(ctx, min(options.RetryBackoff*time.Duration(1<<min(attempt, 6)), 5*time.Second)) {
			return failure("report_cancelled")
		}
	}
	return failure("report_failed")
}

func (processor *Processor) heartbeat(ctx context.Context, cancel context.CancelCauseFunc, lease *submitpb.AcquireJudgeTaskResp, progress *atomic.Value, logger *slog.Logger) {
	expires := lease.LeaseExpiresAt.AsTime()
	delay := min(processor.options.HeartbeatInterval, time.Until(expires)/3)
	var failures int
	for {
		if !sleep(ctx, max(time.Millisecond, delay)) {
			return
		}
		if !expires.After(time.Now()) {
			cancel(context.DeadlineExceeded)
			return
		}
		current := progress.Load().(engine.Progress)
		callCtx, stop := context.WithDeadline(ctx, expires)
		response, err := processor.options.Submit.Heartbeat(callCtx, &submitpb.HeartbeatJudgeTaskReq{WorkerId: processor.options.WorkerID, SubmissionId: lease.Task.SubmissionId, AttemptId: lease.Task.AttemptId, LeaseToken: lease.LeaseToken, CompletedTestCases: current.Completed, CurrentStage: current.Stage})
		stop()
		if ctx.Err() != nil {
			return
		}
		if status.Code(err) == codes.FailedPrecondition || status.Code(err) == codes.NotFound {
			cancel(ErrLeaseLost)
			return
		}
		if err == nil && response != nil && response.LeaseExpiresAt != nil && response.LeaseExpiresAt.CheckValid() == nil && response.LeaseExpiresAt.AsTime().After(time.Now()) {
			expires, failures = response.LeaseExpiresAt.AsTime(), 0
			delay = min(processor.options.HeartbeatInterval, time.Until(expires)/3)
			continue
		}
		logger.WarnContext(ctx, "judge heartbeat retry", "stage", current.Stage, "completed_cases", current.Completed, "rpc_code", status.Code(err).String())
		failures++
		delay = min(processor.options.RetryBackoff*time.Duration(1<<min(failures-1, 6)), processor.options.HeartbeatInterval, max(time.Millisecond, time.Until(expires)/3))
	}
}

func sleep(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func makeReport(workerID string, lease *submitpb.AcquireJudgeTaskResp, outcome engine.Outcome) *submitpb.ReportJudgeResultReq {
	report := &submitpb.ReportJudgeResultReq{WorkerId: workerID, SubmissionId: lease.Task.SubmissionId, AttemptId: lease.Task.AttemptId, LeaseToken: lease.LeaseToken, ReportId: fmt.Sprintf("judge-%d-v1", lease.Task.AttemptId), Verdict: verdict(outcome.Verdict), Score: outcome.Score, MaxScore: outcome.MaxScore, CompileOutput: outcome.CompileOutput}
	for _, result := range outcome.Cases {
		report.CaseResults = append(report.CaseResults, &submitpb.JudgeTestCaseResult{TestCaseId: result.ID, Ordinal: result.Ordinal, GroupName: result.Group, Verdict: verdict(result.Verdict), Score: result.Score, TimeUsedMs: result.TimeMS, MemoryUsedKb: result.MemoryKB, JudgeMessage: result.Message})
	}
	return report
}
func verdict(value engine.Verdict) submitpb.Verdict {
	values := map[engine.Verdict]submitpb.Verdict{engine.AC: submitpb.Verdict_VERDICT_ACCEPTED, engine.WA: submitpb.Verdict_VERDICT_WRONG_ANSWER, engine.CE: submitpb.Verdict_VERDICT_COMPILE_ERROR, engine.RE: submitpb.Verdict_VERDICT_RUNTIME_ERROR, engine.TLE: submitpb.Verdict_VERDICT_TIME_LIMIT_EXCEEDED, engine.MLE: submitpb.Verdict_VERDICT_MEMORY_LIMIT_EXCEEDED, engine.OLE: submitpb.Verdict_VERDICT_OUTPUT_LIMIT_EXCEEDED, engine.SE: submitpb.Verdict_VERDICT_SYSTEM_ERROR}
	if result, exists := values[value]; exists {
		return result
	}
	return submitpb.Verdict_VERDICT_SYSTEM_ERROR
}
