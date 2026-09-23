package e2e

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/checker"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/consumer"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/engine"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/language"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/processor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/sandbox"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/testdatacache"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/worker"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgeevent"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgemq"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/testdatamanifest"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
	submitpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/testsupport"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const secretSentinel = "e2e-secret-never-log-this"

type problemServer struct {
	problempb.UnimplementedProblemServiceServer
	spec *problempb.JudgeProblemSpec
	fail atomic.Bool
}

func (server *problemServer) GetJudgeProblemSpec(_ context.Context, request *problempb.GetJudgeProblemSpecReq) (*problempb.GetJudgeProblemSpecResp, error) {
	if server.fail.Swap(false) {
		return nil, status.Error(codes.Unavailable, secretSentinel)
	}
	if request.ProblemId != server.spec.ProblemId || request.Revision != server.spec.Revision {
		return nil, status.Error(codes.NotFound, "fixed revision not found")
	}
	return &problempb.GetJudgeProblemSpecResp{Spec: proto.Clone(server.spec).(*problempb.JudgeProblemSpec)}, nil
}

type faultSandbox struct {
	sandbox.Sandbox
	fail atomic.Bool
}

func (executor *faultSandbox) Execute(ctx context.Context, request sandbox.ExecuteRequest) (sandbox.ExecuteResult, error) {
	if executor.fail.Swap(false) {
		return sandbox.ExecuteResult{}, errors.New(secretSentinel)
	}
	return executor.Sandbox.Execute(ctx, request)
}

type lostAck struct {
	judgemq.Receiver
	lose     atomic.Bool
	lost     atomic.Int32
	received atomic.Int32
}

func (receiver *lostAck) Ack(ctx context.Context, message *judgemq.Delivery) error {
	if receiver.lose.Swap(false) {
		receiver.lost.Add(1)
		_ = receiver.Receiver.Visibility(ctx, message, time.Second)
		return errors.New("injected MQ ACK loss")
	}
	return receiver.Receiver.Ack(ctx, message)
}
func (receiver *lostAck) Receive(ctx context.Context, maximum int32, visibility time.Duration) ([]*judgemq.Delivery, error) {
	messages, err := receiver.Receiver.Receive(ctx, maximum, visibility)
	receiver.received.Add(int32(len(messages)))
	return messages, err
}

func TestJudgeEndToEndRecovery(t *testing.T) {
	if os.Getenv("JUDGE_E2E") != "1" {
		t.Skip("run scripts/test-judge-e2e.sh with isolated infrastructure")
	}
	if os.Getenv("JUDGE_E2E_ISOLATED") != "1" {
		t.Fatal("E2E requires explicit isolated broker/database/storage")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	archive, spec := testArchive(t)
	storageURL := os.Getenv("JUDGE_E2E_MINIO_ENDPOINT")
	bucket := "judge-e2e-" + uuid.NewString()
	objectURL := storageURL + "/" + bucket + "/" + spec.TestDataPackageKey
	storageRequest(t, "PUT", storageURL+"/"+bucket, nil)
	t.Cleanup(func() {
		storageRequest(t, "DELETE", objectURL, nil)
		storageRequest(t, "DELETE", storageURL+"/"+bucket, nil)
	})
	storageRequest(t, "PUT", objectURL, archive)
	target, err := url.Parse(storageURL)
	if err != nil {
		t.Fatal(err)
	}
	var failStorage atomic.Bool
	proxy := httputil.NewSingleHostReverseProxy(target)
	storageProxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Error("Worker attempted non-read-only object operation")
			writer.WriteHeader(http.StatusForbidden)
			return
		}
		if failStorage.Swap(false) {
			writer.WriteHeader(http.StatusServiceUnavailable)
			_, _ = writer.Write([]byte(secretSentinel))
			return
		}
		proxy.ServeHTTP(writer, request)
	}))
	defer storageProxy.Close()
	harness := testsupport.NewJudgeHarness(t, os.Getenv("SUBMIT_MODEL_TEST_DSN"), spec, secretSentinel)
	problem := &problemServer{spec: spec}
	auth := sharedauth.NewServiceAuthInterceptor(map[string]string{"judge.worker": secretSentinel}, func(string) []string { return []string{"judge.worker"} })
	problemRPC := grpc.NewServer(grpc.UnaryInterceptor(auth))
	problempb.RegisterProblemServiceServer(problemRPC, problem)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = problemRPC.Serve(listener) }()
	defer problemRPC.Stop()
	submit, problemGateway, closeRPC, err := processor.NewGateways(harness.Target, listener.Addr().String(), secretSentinel, secretSentinel, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer closeRPC()
	work := t.TempDir()
	dockerEndpoint := os.Getenv("JUDGE_DOCKER_ENDPOINT")
	if dockerEndpoint == "" {
		dockerEndpoint = "unix:///var/run/docker.sock"
	}
	docker, err := sandbox.NewDocker(sandbox.DockerOptions{Endpoint: dockerEndpoint, WorkerID: "e2e-" + uuid.NewString(), User: "65532:65532", AllowedRoots: []sandbox.AllowedRoot{{Path: work, Writable: true}}, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Close()
	executor := &faultSandbox{Sandbox: docker}
	profiles, err := language.LoadProfiles("")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := language.NewRegistry(profiles, docker)
	if err != nil {
		t.Fatal(err)
	}
	judgeEngine, err := engine.New(registry, executor, work, logger)
	if err != nil {
		t.Fatal(err)
	}
	settings := judgemq.Config{Endpoint: os.Getenv("ROCKETMQ_ENDPOINT"), Timeout: 5 * time.Second}
	mqConsumer, err := judgemq.NewConsumer(settings, judgeevent.Group)
	if err != nil {
		t.Fatal(err)
	}
	if err := mqConsumer.Start(); err != nil {
		t.Fatal(err)
	}
	defer mqConsumer.Close()
	sender, err := judgemq.NewProducer(settings)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	publisher := harness.Publisher(t, sender, logger)
	publisher.Start()
	defer publisher.Stop(context.Background())
	for _, scenario := range []string{"cpp20", "go", "submit_unavailable", "problem_unavailable", "minio_unavailable", "docker_unavailable", "crash_after_acquire", "source_size_boundary", "oversized_legacy_source"} {
		t.Run(scenario, func(t *testing.T) {
			cacheDir := filepath.Join(work, "cache-"+uuid.NewString())
			cacheConfig := config.Config{TestDataCache: config.TestDataCacheConf{Dir: cacheDir}}
			cacheConfig.ApplyDefaults()
			store, err := testdatacache.NewS3Store(config.ObjectStorageConf{Endpoint: storageProxy.URL, Region: "us-east-1", Bucket: bucket, AccessKey: os.Getenv("JUDGE_E2E_MINIO_ACCESS_KEY"), SecretKey: os.Getenv("JUDGE_E2E_MINIO_SECRET_KEY"), UsePathStyle: true, RequestTimeoutSeconds: 2})
			if err != nil {
				t.Fatal(err)
			}
			cache, err := testdatacache.New(cacheConfig.TestDataCache, store)
			if err != nil {
				t.Fatal(err)
			}
			defer cache.Close()
			taskProcessor, err := processor.New(processor.Options{WorkerID: "e2e-worker", Submit: submit, Problem: problemGateway, Cache: cache, Judge: judgeEngine, Logger: logger, Profiles: map[string]string{"go": "go-v3", "cpp20": "cpp20-v3"}, Checker: checker.Exact, HeartbeatInterval: 100 * time.Millisecond, RetryBackoff: 20 * time.Millisecond, ReportAttempts: 3})
			if err != nil {
				t.Fatal(err)
			}
			pool, err := worker.NewPool(2, 2, 30*time.Second, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := pool.Start(); err != nil {
				t.Fatal(err)
			}
			receiver := &lostAck{Receiver: mqConsumer}
			if scenario == "cpp20" {
				receiver.lose.Store(true)
				harness.DropReport.Store(true)
				harness.LoseMark.Store(true)
				harness.FailHeartbeat.Store(true)
			}
			harness.FailAcquire.Store(scenario == "submit_unavailable")
			problem.fail.Store(scenario == "problem_unavailable")
			failStorage.Store(scenario == "minio_unavailable")
			executor.fail.Store(scenario == "docker_unavailable")
			driver, err := consumer.New(receiver, sender, taskProcessor, pool, consumer.Options{WorkerID: "e2e-worker", Concurrency: 2, MaxAttempts: 16, Visibility: 15 * time.Second, Timeout: 5 * time.Second, Backoff: time.Second, MaxBackoff: 2 * time.Second}, logger)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Stop(context.Background())
			code, languageID := "package main\nimport \"fmt\"\nfunc main(){fmt.Println(42)}\n", "go"
			if scenario == "cpp20" {
				code, languageID = "#include <iostream>\nint main(){std::cout << 42 << '\\n';}\n", "cpp20"
			}
			if scenario == "source_size_boundary" {
				code += "//" + strings.Repeat("x", (1<<20)-len(code)-2)
			}
			before := harness.Applied.Load()
			created, err := harness.Model.CreateSubmission(ctx, model.CreateSubmissionInput{UserID: 1, ProblemID: 1, ProblemRevision: 1, Language: languageID, SourceCode: code, ClientRequestID: uuid.NewString(), SubmittedAt: time.Now()})
			if err != nil {
				t.Fatal(err)
			}
			submission := created.Submission
			if scenario == "oversized_legacy_source" {
				if err := harness.DB.Table("submissions").Where("id = ?", submission.SubmissionID).Update("source_code", strings.Repeat("x", 4<<20)).Error; err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "crash_after_acquire" {
				lease, err := submit.Acquire(ctx, &submitpb.AcquireJudgeTaskReq{WorkerId: "crashed-worker", SubmissionId: submission.SubmissionID, AttemptId: submission.CurrentAttemptID, DeliveryId: uuid.NewString()})
				if err != nil || !lease.Acquired {
					t.Fatal("crash fixture acquire failed", err)
				}
			}
			if err := driver.Start(); err != nil {
				t.Fatal(err)
			}
			await(t, ctx, func() bool {
				state, err := harness.Model.GetSubmission(ctx, submission.SubmissionID)
				return err == nil && state.Status == model.SubmissionStatusFinished
			})
			state, err := harness.Model.GetSubmission(ctx, submission.SubmissionID)
			wantVerdict, wantCases := model.VerdictAccepted, 1
			if scenario == "oversized_legacy_source" {
				wantVerdict, wantCases = model.VerdictCompileError, 0
			}
			if err != nil || state.Verdict == nil || *state.Verdict != wantVerdict || len(state.CaseResults) != wantCases {
				t.Fatalf("end-to-end result does not match verdict %d/cases %d: %+v %v", wantVerdict, wantCases, state, err)
			}
			if scenario == "oversized_legacy_source" && state.AttemptCount != 1 {
				t.Fatal("permanent source rejection created successor attempts")
			}
			if scenario == "crash_after_acquire" && state.AttemptCount < 2 {
				t.Fatal("expired lease did not create successor attempt")
			}
			harness.AssertAppliedOnce(t, submission.SubmissionID)
			await(t, ctx, func() bool {
				var pending int64
				err := harness.DB.Table("submit_outbox_events").Where("aggregate_id = ? AND event_type = ? AND status != ?", submission.SubmissionID, model.OutboxEventJudgeTask, model.OutboxStatusPublished).Count(&pending).Error
				return err == nil && pending == 0 && driver.Metrics.Get("judge_consumer_ack_total") > 0 && (scenario != "cpp20" || receiver.received.Load() >= 3)
			})
			if harness.Applied.Load() != before+1 {
				t.Fatal("duplicate delivery applied final result more than once")
			}
			if scenario == "cpp20" && (receiver.lost.Load() != 1 || harness.DropReport.Load() || harness.LoseMark.Load()) {
				t.Fatal("loss injection was not exercised")
			}
			shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
			defer stop()
			if err := driver.Stop(shutdown); err != nil {
				t.Fatal(err)
			}
			cleanup, err := docker.Cleanup(ctx)
			if err != nil || cleanup.Found != 0 {
				t.Fatalf("sandbox cleanup after %s: found=%d removed=%d failed=%d error=%v", scenario, cleanup.Found, cleanup.Removed, cleanup.Failed, err)
			}
			scrape := httptest.NewRecorder()
			driver.Metrics.ServeHTTP(scrape, httptest.NewRequest("GET", "/metrics", nil))
			for _, metric := range []string{"judge_consumer_ack_total", "judge_consumer_retry_total", "judge_consumer_inflight 0"} {
				if !strings.Contains(scrape.Body.String(), metric) {
					t.Fatal("metric missing", metric)
				}
			}
			cache.Close()
			if err := os.RemoveAll(cacheDir); err != nil {
				t.Fatal("test cache cleanup failed")
			}
		})
	}
	if err := publisher.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	cleanup, err := docker.Cleanup(ctx)
	if err != nil || cleanup.Found != 0 {
		t.Fatalf("final sandbox cleanup: found=%d removed=%d failed=%d error=%v", cleanup.Found, cleanup.Removed, cleanup.Failed, err)
	}
	entries, err := os.ReadDir(work)
	if err != nil || len(entries) != 0 {
		t.Fatal("attempt/cache directory leaked")
	}
	if strings.Contains(logs.String(), secretSentinel) || strings.Contains(logs.String(), "func main(){") || strings.Contains(logs.String(), "std::cout") {
		t.Fatal("logs leaked secret or full source")
	}
	for _, field := range []string{"submission_id", "attempt_id", "worker_id", "stage"} {
		if !strings.Contains(logs.String(), field) {
			t.Fatal("log context missing", field)
		}
	}
}

func await(t *testing.T, ctx context.Context, condition func() bool) {
	t.Helper()
	deadline := time.NewTimer(40 * time.Second)
	defer deadline.Stop()
	for !condition() {
		select {
		case <-ctx.Done():
			t.Fatal("E2E deadline exceeded")
		case <-deadline.C:
			t.Fatal("E2E condition did not converge")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func testArchive(t *testing.T) ([]byte, *problempb.JudgeProblemSpec) {
	t.Helper()
	manifest := testdatamanifest.Manifest{Version: 1, TestCases: []testdatamanifest.TestCase{{Name: "one", InputFile: "one.in", OutputFile: "one.out", Score: 100, Ordinal: 0}}}
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	for _, name := range []string{"one.in", "one.out"} {
		content := []byte("42\n")
		checksum := sha256.Sum256(content)
		manifest.Files = append(manifest.Files, testdatamanifest.File{Path: name, Size: int64(len(content)), SHA256: hex.EncodeToString(checksum[:])})
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	file, err := writer.Create(testdatamanifest.FileName)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(file).Encode(manifest); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	checksum := sha256.Sum256(body.Bytes())
	spec := &problempb.JudgeProblemSpec{ProblemId: 1, Revision: 1, TestDataPackageKey: "problem-test-data/1/e2e.zip", TestDataChecksum: hex.EncodeToString(checksum[:]), JudgeConfig: &problempb.JudgeConfig{TestDataVersion: "e2e-v1", Settings: &problempb.JudgeSettings{TimeLimitMs: 1000, MemoryLimitMb: 64, AllowedLanguages: []string{"go", "cpp20"}, CheckerType: problempb.CheckerType_CHECKER_TYPE_STANDARD}}, TestCases: []*problempb.TestCaseInfo{{TestCaseId: 1, Name: "one", Ordinal: 0, Score: 100, Enabled: true}}}
	return body.Bytes(), spec
}

func storageRequest(t *testing.T, method, address string, body []byte) {
	t.Helper()
	command := exec.Command("curl", "--fail", "--silent", "--show-error", "--connect-timeout", "3", "--max-time", "10", "--aws-sigv4", "aws:amz:us-east-1:s3", "--user", os.Getenv("JUDGE_E2E_MINIO_ACCESS_KEY")+":"+os.Getenv("JUDGE_E2E_MINIO_SECRET_KEY"), "-X", method, address)
	if body != nil {
		command.Args = append(command.Args, "--data-binary", "@-")
		command.Stdin = bytes.NewReader(body)
	}
	command.Stdout, command.Stderr = io.Discard, io.Discard
	if err := command.Run(); err != nil {
		t.Fatalf("isolated S3 fixture %s failed", method)
	}
}
