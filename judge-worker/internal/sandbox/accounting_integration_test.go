package sandbox

import (
	"context"
	"github.com/google/uuid"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func accountingSandbox(t *testing.T) (*Docker, ExecuteRequest, string) {
	t.Helper()
	if os.Getenv("JUDGE_DOCKER_INTEGRATION") != "1" {
		t.Skip("set JUDGE_DOCKER_INTEGRATION=1")
	}
	root := t.TempDir()
	helper := filepath.Join(root, "helper")
	if output, err := exec.Command("cc", "-static", "-O2", "testdata/accounting.c", "-o", helper).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	executor, err := NewDocker(DockerOptions{Endpoint: "unix:///var/run/docker.sock", WorkerID: "review-" + uuid.NewString(), User: "65532:65532", AllowedRoots: []AllowedRoot{{Path: root, Writable: true}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = executor.Cleanup(context.Background()); _ = executor.Close() })
	request := ExecuteRequest{AttemptID: 1, SubmissionID: 1, Image: "gcoj/judge-cpp20:1.2.0-runtime", Program: "/sandbox/helper", WorkingDir: "/sandbox", Mounts: []Mount{{Source: helper, Target: "/sandbox/helper", ReadOnly: true}}, Limits: Limits{CPUTime: time.Second, WallTime: 5 * time.Second, MemoryBytes: 64 << 20, OutputBytes: 4096, PIDs: 64, OpenFiles: 64}}
	return executor, request, root
}

func TestDockerCPUShortOverrun(t *testing.T) {
	executor, request, _ := accountingSandbox(t)
	request.Args = []string{"cpu"}
	request.Limits.CPUTime, request.Limits.WallTime = 100*time.Millisecond, 300*time.Millisecond
	result, err := executor.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("reason=%s cpu=%s wall=%s peak=%d stdout=%q", result.Reason, result.CPUTime, result.WallTime, result.PeakMemory, result.Stdout)
	if result.Reason != ExitTLE {
		t.Fatal("program used 200ms CPU but escaped 100ms CPU limit")
	}
}

func TestDockerSupervisorAccountingCannotBeForged(t *testing.T) {
	executor, request, _ := accountingSandbox(t)
	request.Args = []string{"tamper"}
	result, err := executor.Execute(context.Background(), request)
	if err != nil || result.Reason != ExitNormal || result.ExitCode == nil || *result.ExitCode != 0 || len(result.Stdout) != 29 || result.CPUTime == 0 {
		t.Fatalf("supervisor protection failed: %+v error=%v", result, err)
	}
}

func TestDockerSupervisorReservesOnlyItsOwnFileDescriptors(t *testing.T) {
	executor, request, _ := accountingSandbox(t)
	request.Args = []string{"minimal-fds"}
	request.Limits.OpenFiles = 3
	result, err := executor.Execute(context.Background(), request)
	if err != nil || result.Reason != ExitNormal || result.ExitCode == nil || *result.ExitCode != 0 || string(result.Stdout) != "limited\n" {
		t.Fatalf("child descriptor limit was not preserved: %+v error=%v", result, err)
	}
}

func TestDockerCPUIncludesAutoReapedDescendants(t *testing.T) {
	executor, request, _ := accountingSandbox(t)
	request.Args = []string{"cpu-descendants"}
	request.Limits.CPUTime, request.Limits.WallTime = 100*time.Millisecond, 300*time.Millisecond
	result, err := executor.Execute(context.Background(), request)
	if err != nil || result.Reason != ExitTLE || result.CPUTime < request.Limits.CPUTime {
		t.Fatalf("descendant CPU escaped accounting: %+v error=%v", result, err)
	}
}

func TestDockerNprocIsolation(t *testing.T) {
	executor, request, root := accountingSandbox(t)
	victim := request
	victim.Args, victim.Limits.PIDs = []string{"fork"}, 16
	baseline, err := executor.Execute(context.Background(), victim)
	if err != nil || baseline.ExitCode == nil || *baseline.ExitCode != 0 {
		t.Fatalf("baseline: %+v %v", baseline, err)
	}
	control := filepath.Join(root, "control")
	if err := os.Mkdir(control, 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(control, 0777); err != nil {
		t.Fatal(err)
	}
	request.Args = []string{"hold"}
	request.Mounts = append(request.Mounts, Mount{Source: control, Target: "/sandbox/control"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, err := executor.Execute(ctx, request); finished <- err }()
	defer func() { cancel(); <-finished }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(control, "ready")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("bounded holder did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	result, err := executor.Execute(context.Background(), victim)
	exitCode := -1
	if result.ExitCode != nil {
		exitCode = *result.ExitCode
	}
	t.Logf("concurrent victim: reason=%s exit=%d stdout=%q stderr=%q err=%v", result.Reason, exitCode, result.Stdout, result.Stderr, err)
	if err != nil || result.ExitCode == nil || *result.ExitCode != 0 {
		t.Fatal("another sandbox's processes broke the victim's fork")
	}
}
