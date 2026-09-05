package processor

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/engine"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/testdatacache"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	submitpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeSubmit struct {
	mu             sync.Mutex
	lease          *submitpb.AcquireJudgeTaskResp
	acquireErr     error
	finished       bool
	reports        []*submitpb.ReportJudgeResultReq
	reportFailures int
	heartbeats     []*submitpb.HeartbeatJudgeTaskReq
	heartbeat      func(int) error
}

func (gateway *fakeSubmit) Acquire(context.Context, *submitpb.AcquireJudgeTaskReq) (*submitpb.AcquireJudgeTaskResp, error) {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	if gateway.finished {
		return &submitpb.AcquireJudgeTaskResp{DenyReason: submitpb.AcquireDenyReason_ACQUIRE_DENY_REASON_ALREADY_FINISHED}, nil
	}
	return gateway.lease, gateway.acquireErr
}
func (gateway *fakeSubmit) Heartbeat(_ context.Context, request *submitpb.HeartbeatJudgeTaskReq) (*submitpb.HeartbeatJudgeTaskResp, error) {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	gateway.heartbeats = append(gateway.heartbeats, proto.Clone(request).(*submitpb.HeartbeatJudgeTaskReq))
	if gateway.heartbeat != nil {
		if err := gateway.heartbeat(len(gateway.heartbeats)); err != nil {
			return nil, err
		}
	}
	return &submitpb.HeartbeatJudgeTaskResp{LeaseExpiresAt: timestamppb.New(time.Now().Add(time.Second))}, nil
}
func (gateway *fakeSubmit) Report(_ context.Context, request *submitpb.ReportJudgeResultReq) (*submitpb.ReportJudgeResultResp, error) {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	gateway.reports = append(gateway.reports, proto.Clone(request).(*submitpb.ReportJudgeResultReq))
	duplicate := gateway.finished
	gateway.finished = true
	if len(gateway.reports) <= gateway.reportFailures {
		return nil, status.Error(codes.Unavailable, "sensitive-token source-code")
	}
	disposition := submitpb.ReportDisposition_REPORT_DISPOSITION_APPLIED
	if duplicate {
		disposition = submitpb.ReportDisposition_REPORT_DISPOSITION_DUPLICATE
	}
	return &submitpb.ReportJudgeResultResp{Disposition: disposition}, nil
}

type fakeProblem struct {
	revision int64
	err      error
}

func (gateway fakeProblem) Spec(context.Context, int64, int64) (*problempb.JudgeProblemSpec, error) {
	return &problempb.JudgeProblemSpec{ProblemId: 3, Revision: gateway.revision, JudgeConfig: &problempb.JudgeConfig{Settings: &problempb.JudgeSettings{AllowedLanguages: []string{"go"}}}}, gateway.err
}

type fakeCache struct{ err error }

func (cache fakeCache) Prepare(context.Context, int64, *problempb.JudgeProblemSpec) (*testdatacache.Package, error) {
	return &testdatacache.Package{}, cache.err
}

type fakeJudge struct {
	calls int
	run   func(context.Context, engine.JudgeInput) error
}

func (judge *fakeJudge) Judge(ctx context.Context, input engine.JudgeInput) (engine.Outcome, error) {
	judge.calls++
	if judge.run != nil {
		if err := judge.run(ctx, input); err != nil {
			return engine.Outcome{}, err
		}
	}
	return engine.Outcome{Verdict: engine.AC, Score: 1, MaxScore: 1, Cases: []engine.CaseResult{{ID: 1, Verdict: engine.AC, Score: 1}}}, nil
}

func fixture(t *testing.T) (*Processor, *fakeSubmit, *fakeJudge, *bytes.Buffer, *submitpb.JudgeTaskEvent) {
	t.Helper()
	gateway := &fakeSubmit{lease: &submitpb.AcquireJudgeTaskResp{Acquired: true, Task: &submitpb.JudgeTask{SubmissionId: 1, AttemptId: 2, ProblemId: 3, ProblemRevision: 4, Language: "go", SourceCode: "source-code"}, LeaseToken: "sensitive-token", LeaseExpiresAt: timestamppb.New(time.Now().Add(time.Second))}}
	judge := &fakeJudge{}
	logs := &bytes.Buffer{}
	processor, err := New(Options{WorkerID: "test-worker", Submit: gateway, Problem: fakeProblem{revision: 4}, Cache: fakeCache{}, Judge: judge, Logger: slog.New(slog.NewJSONHandler(logs, nil)), Profiles: map[string]string{"go": "go-v2"}, HeartbeatInterval: 10 * time.Millisecond, RetryBackoff: time.Millisecond, ReportAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}
	return processor, gateway, judge, logs, &submitpb.JudgeTaskEvent{EventId: "event", SubmissionId: 1, AttemptId: 2}
}

func TestReportLostResponseAndRedeliveryConverge(t *testing.T) {
	processor, gateway, judge, logs, event := fixture(t)
	gateway.reportFailures = 1
	result := processor.Process(context.Background(), event)
	if result != (Result{ACK, "duplicate"}) || len(gateway.reports) != 2 || !proto.Equal(gateway.reports[0], gateway.reports[1]) {
		t.Fatalf("result %v reports %v", result, gateway.reports)
	}
	if result := processor.Process(context.Background(), event); result != (Result{ACK, "finished"}) || judge.calls != 1 {
		t.Fatalf("redelivery %v calls %d", result, judge.calls)
	}
	for _, forbidden := range []string{"sensitive-token", "source-code"} {
		if strings.Contains(logs.String(), forbidden) {
			t.Fatal("sensitive log content")
		}
	}
	for _, required := range []string{"submission_id", "attempt_id", "worker_id", "revision", "report_id", "stage"} {
		if !strings.Contains(logs.String(), required) {
			t.Fatalf("missing log field %s", required)
		}
	}
}

func TestAcquireDenialsDoNotJudge(t *testing.T) {
	for _, reason := range []submitpb.AcquireDenyReason{submitpb.AcquireDenyReason_ACQUIRE_DENY_REASON_ALREADY_FINISHED, submitpb.AcquireDenyReason_ACQUIRE_DENY_REASON_STALE_ATTEMPT, submitpb.AcquireDenyReason_ACQUIRE_DENY_REASON_LEASED_BY_OTHER_WORKER} {
		processor, gateway, judge, _, event := fixture(t)
		gateway.lease = &submitpb.AcquireJudgeTaskResp{DenyReason: reason}
		if result := processor.Process(context.Background(), event); result.Action != ACK || judge.calls != 0 {
			t.Fatalf("%v: %v", reason, result)
		}
	}
}

func TestHeartbeatTransientFailureAndProgress(t *testing.T) {
	processor, gateway, judge, _, event := fixture(t)
	healthy := make(chan struct{})
	gateway.heartbeat = func(count int) error {
		if count == 1 {
			return status.Error(codes.Unavailable, "transient")
		}
		if count == 2 {
			close(healthy)
		}
		return nil
	}
	judge.run = func(ctx context.Context, input engine.JudgeInput) error {
		input.Progress(engine.Progress{Stage: "run", Completed: 1, Total: 2})
		select {
		case <-healthy:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if result := processor.Process(context.Background(), event); result.Action != ACK || len(gateway.heartbeats) < 2 {
		t.Fatal(result)
	}
	if gateway.heartbeats[0].CompletedTestCases != 1 || gateway.heartbeats[0].CurrentStage != "run" {
		t.Fatal(gateway.heartbeats)
	}
}

func TestLeaseLossCancelsJudgeAndJoinsHeartbeat(t *testing.T) {
	for _, explicit := range []bool{true, false} {
		processor, gateway, judge, _, event := fixture(t)
		gateway.lease.LeaseExpiresAt = timestamppb.New(time.Now().Add(40 * time.Millisecond))
		gateway.heartbeat = func(int) error {
			if explicit {
				return status.Error(codes.FailedPrecondition, "expired")
			}
			return status.Error(codes.Unavailable, "down")
		}
		judge.run = func(ctx context.Context, _ engine.JudgeInput) error { <-ctx.Done(); return ctx.Err() }
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		result := processor.Process(ctx, event)
		cancel()
		want := Retry
		if explicit {
			want = ACK
		}
		if result.Action != want || len(gateway.reports) != 0 {
			t.Fatalf("explicit %v result %v", explicit, result)
		}
	}
}

func TestPermanentAndTransientFailures(t *testing.T) {
	for _, mode := range []string{"event", "revision", "acquire", "cache", "judge", "report"} {
		t.Run(mode, func(t *testing.T) {
			processor, gateway, judge, _, event := fixture(t)
			want := Retry
			switch mode {
			case "event":
				event.SubmissionId = 0
				want = DeadLetter
			case "revision":
				processor.options.Problem = fakeProblem{revision: 5}
			case "acquire":
				gateway.acquireErr = errors.New("unavailable")
			case "cache":
				processor.options.Cache = fakeCache{err: errors.New("unavailable")}
			case "judge":
				judge.run = func(context.Context, engine.JudgeInput) error { return errors.New("docker unavailable") }
			case "report":
				gateway.reportFailures = 100
			}
			if result := processor.Process(context.Background(), event); result.Action != want {
				t.Fatal(result)
			}
		})
	}
}
