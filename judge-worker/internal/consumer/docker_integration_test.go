package consumer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/checker"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/engine"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/language"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/processor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/sandbox"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/testdatacache"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/testdatamanifest"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	submitpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
	"github.com/google/uuid"
	"github.com/moby/moby/client"
	"go.uber.org/goleak"
)

func TestForcedShutdownCleansDockerIntegration(t *testing.T) {
	if os.Getenv("JUDGE_DOCKER_INTEGRATION") != "1" {
		t.Skip("requires Docker and official C++20 images")
	}
	baseline := goleak.IgnoreCurrent()
	t.Cleanup(func() { goleak.VerifyNone(t, baseline) })
	root := t.TempDir()
	data := filepath.Join(root, "data")
	if err := os.Mkdir(data, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"in", "out"} {
		if err := os.WriteFile(filepath.Join(data, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	endpoint := os.Getenv("JUDGE_DOCKER_ENDPOINT")
	if endpoint == "" {
		endpoint = "unix:///var/run/docker.sock"
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	workerID := "shutdown-" + uuid.NewString()
	executor, err := sandbox.NewDocker(sandbox.DockerOptions{Endpoint: endpoint, WorkerID: workerID, User: "65532:65532", AllowedRoots: []sandbox.AllowedRoot{{Path: root, Writable: true}}, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = executor.Close() })
	inspector, err := client.New(client.WithHost(endpoint), client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = inspector.Close() })
	profiles, err := language.LoadProfiles("")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := language.NewRegistry(profiles, executor)
	if err != nil {
		t.Fatal(err)
	}
	judgeEngine, err := engine.New(registry, executor, root, logger)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 1)
	finished := make(chan error, 1)
	driver, broker := fixture(t, func(ctx context.Context, _ *submitpb.JudgeTaskEvent) processor.Result {
		_, err := judgeEngine.Judge(ctx, engine.JudgeInput{
			Request: engine.Request{SubmissionID: 1, AttemptID: 2, ProfileID: "cpp20-v2", SourceCode: "int main(){for(;;){}}"},
			Spec:    &problempb.JudgeProblemSpec{ProblemId: 1, Revision: 1, JudgeConfig: &problempb.JudgeConfig{Settings: &problempb.JudgeSettings{TimeLimitMs: 10000, MemoryLimitMb: 64, CheckerType: problempb.CheckerType_CHECKER_TYPE_STANDARD}}, TestCases: []*problempb.TestCaseInfo{{TestCaseId: 1, Name: "one", Enabled: true}}},
			Package: &testdatacache.Package{Dir: data, Manifest: &testdatamanifest.Manifest{Version: 1, TestCases: []testdatamanifest.TestCase{{Name: "one", InputFile: "in", OutputFile: "out"}}}},
			Checker: checker.Exact,
			Progress: func(progress engine.Progress) {
				if progress.Stage == "run" {
					started <- struct{}{}
				}
			},
		})
		finished <- err
		return processor.Result{Action: processor.Retry}
	})
	broker.messages <- delivery(t)
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case err := <-finished:
		t.Fatal("judge did not enter run", err)
	case <-time.After(30 * time.Second):
		t.Fatal("compile did not finish")
	}
	runningCtx, runningCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer runningCancel()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	filters := make(client.Filters).Add("label", "gcoj.worker_id="+workerID).Add("status", "running")
	for {
		containers, err := inspector.ContainerList(runningCtx, client.ContainerListOptions{Filters: filters})
		if err != nil {
			t.Fatal("inspect running sandbox", err)
		}
		if len(containers.Items) == 1 {
			break
		}
		select {
		case <-poll.C:
		case err := <-finished:
			t.Fatal("judge finished before forced shutdown", err)
		case <-runningCtx.Done():
			t.Fatal("runtime container did not enter running state")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := driver.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("forced shutdown did not cancel", err)
	}
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("engine did not observe cancellation", err)
		}
	default:
		t.Fatal("engine goroutine outlived shutdown")
	}
	if broker.acks != 0 {
		t.Fatal("cancelled container was ACKed")
	}
	cleanup, err := executor.Cleanup(context.Background())
	if err != nil || cleanup.Found != 0 {
		t.Fatal("forced consumer shutdown left a container")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "data" {
		t.Fatal("forced consumer shutdown left an attempt directory")
	}
}
