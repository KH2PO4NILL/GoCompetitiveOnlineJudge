package sandbox

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/client"
)

func TestDockerSandboxIntegration(t *testing.T) {
	if os.Getenv("JUDGE_DOCKER_INTEGRATION") != "1" {
		t.Skip("set JUDGE_DOCKER_INTEGRATION=1 to run Docker sandbox integration tests")
	}
	image := strings.TrimSpace(os.Getenv("JUDGE_SANDBOX_TEST_IMAGE"))
	if image == "" {
		t.Fatal("JUDGE_SANDBOX_TEST_IMAGE must name a preloaded Linux image")
	}
	endpoint := strings.TrimSpace(os.Getenv("JUDGE_DOCKER_ENDPOINT"))
	if endpoint == "" {
		endpoint = "unix:///var/run/docker.sock"
	}

	root := t.TempDir()
	helper := buildSandboxIntegrationHelper(t, root)
	docker, err := NewDocker(DockerOptions{
		Endpoint: endpoint, WorkerID: "sandbox-integration", User: "65532:65532",
		AllowedRoots: []AllowedRoot{{Path: root, Writable: true}},
		APITimeout:   10 * time.Second, StopTimeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewDocker() error = %v", err)
	}
	t.Cleanup(func() { _ = docker.Close() })
	if _, err := docker.Cleanup(context.Background()); err != nil {
		t.Fatalf("initial Cleanup() error = %v", err)
	}

	attemptID := int64(0)
	execute := func(t *testing.T, mode string, limits Limits) ExecuteResult {
		t.Helper()
		attemptID++
		result, err := docker.Execute(context.Background(), ExecuteRequest{
			AttemptID: attemptID, SubmissionID: 1, Image: image,
			Program: "/sandbox/helper", Args: []string{mode}, WorkingDir: "/sandbox",
			Mounts: []Mount{{Source: helper, Target: "/sandbox/helper", ReadOnly: true}},
			Limits: limits,
		})
		if err != nil {
			t.Fatalf("Execute(%s) error = %v; diagnostic=%q", mode, err, result.Diagnostic)
		}
		return result
	}
	baseLimits := Limits{
		CPUTime: time.Second, WallTime: 3 * time.Second,
		MemoryBytes: 64 << 20, OutputBytes: 64 << 10, PIDs: 32, OpenFiles: 64,
	}

	t.Run("normal and separate output streams", func(t *testing.T) {
		result := execute(t, "normal", baseLimits)
		if result.Reason != ExitNormal || result.ExitCode == nil || *result.ExitCode != 0 ||
			string(result.Stdout) != "stdout" || string(result.Stderr) != "stderr" {
			t.Fatalf("result = %#v", result)
		}
	})
	t.Run("nonzero exit", func(t *testing.T) {
		result := execute(t, "exit", baseLimits)
		if result.Reason != ExitNormal || result.ExitCode == nil || *result.ExitCode != 7 {
			t.Fatalf("result = %#v", result)
		}
	})
	t.Run("signal", func(t *testing.T) {
		result := execute(t, "signal", baseLimits)
		if result.Reason != ExitSignal || result.Signal != "SIGTERM" {
			t.Fatalf("result = %#v", result)
		}
	})
	t.Run("wall time limit", func(t *testing.T) {
		limits := baseLimits
		limits.CPUTime = 100 * time.Millisecond
		limits.WallTime = 150 * time.Millisecond
		result := execute(t, "sleep", limits)
		if result.Reason != ExitTLE {
			t.Fatalf("result = %#v", result)
		}
	})
	t.Run("CPU time limit", func(t *testing.T) {
		limits := baseLimits
		limits.CPUTime = 100 * time.Millisecond
		limits.WallTime = 3 * time.Second
		result := execute(t, "loop", limits)
		if result.Reason != ExitTLE || result.CPUTime < limits.CPUTime {
			t.Fatalf("result = %#v", result)
		}
	})
	t.Run("output limit", func(t *testing.T) {
		limits := baseLimits
		limits.OutputBytes = 4096
		result := execute(t, "output", limits)
		if result.Reason != ExitOLE || result.OutputBytes != limits.OutputBytes {
			t.Fatalf("result = %#v", result)
		}
	})
	t.Run("memory limit", func(t *testing.T) {
		limits := baseLimits
		limits.MemoryBytes = 32 << 20
		limits.WallTime = 5 * time.Second
		result := execute(t, "memory", limits)
		if result.Reason != ExitMLE {
			t.Fatalf("result = %#v", result)
		}
	})
	t.Run("security boundaries", func(t *testing.T) {
		result := execute(t, "security", baseLimits)
		if result.Reason != ExitNormal || result.ExitCode == nil || *result.ExitCode != 0 || string(result.Stdout) != "secured" {
			t.Fatalf("result = %#v", result)
		}
	})
	t.Run("file descriptor limit", func(t *testing.T) {
		limits := baseLimits
		limits.OpenFiles = 64
		result := execute(t, "files", limits)
		if result.Reason != ExitNormal || result.ExitCode == nil || *result.ExitCode != 0 {
			t.Fatalf("result = %#v", result)
		}
	})
	t.Run("process limit", func(t *testing.T) {
		limits := baseLimits
		limits.PIDs = 8
		result := execute(t, "processes", limits)
		if result.Reason != ExitNormal || result.ExitCode == nil || *result.ExitCode != 0 {
			t.Fatalf("result = %#v", result)
		}
	})
	t.Run("context cancellation removes container", func(t *testing.T) {
		attemptID++
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		result, err := docker.Execute(ctx, ExecuteRequest{
			AttemptID: attemptID, SubmissionID: 1, Image: image,
			Program: "/sandbox/helper", Args: []string{"sleep"}, WorkingDir: "/sandbox",
			Mounts: []Mount{{Source: helper, Target: "/sandbox/helper", ReadOnly: true}},
			Limits: baseLimits,
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Execute() error = %v, result=%#v", err, result)
		}
		_, inspectErr := docker.client.ContainerInspect(context.Background(), result.ContainerID, client.ContainerInspectOptions{})
		if !cerrdefs.IsNotFound(inspectErr) {
			t.Fatalf("container %q still exists: %v", result.ContainerID, inspectErr)
		}
	})
}

func buildSandboxIntegrationHelper(t *testing.T, root string) string {
	t.Helper()
	source := filepath.Join(root, "helper.go")
	binary := filepath.Join(root, "helper")
	if err := os.WriteFile(source, []byte(sandboxIntegrationHelperSource), 0o600); err != nil {
		t.Fatalf("write integration helper: %v", err)
	}
	command := exec.Command("go", "build", "-trimpath", "-o", binary, source)
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build integration helper: %v\n%s", err, output)
	}
	return binary
}

const sandboxIntegrationHelperSource = `package main

import (
    "fmt"
    "net"
    "os"
    "os/exec"
    "runtime"
    "syscall"
    "time"
)

func main() {
    if len(os.Args) != 2 { os.Exit(64) }
    switch os.Args[1] {
    case "normal":
        fmt.Fprint(os.Stdout, "stdout")
        fmt.Fprint(os.Stderr, "stderr")
    case "exit":
        os.Exit(7)
    case "signal":
        _ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
        time.Sleep(time.Second)
    case "sleep", "child":
        time.Sleep(10 * time.Second)
    case "loop":
        for {}
    case "output":
        block := make([]byte, 8192)
        for { _, _ = os.Stdout.Write(block) }
    case "memory":
        var blocks [][]byte
        for {
            block := make([]byte, 1<<20)
            for index := 0; index < len(block); index += 4096 { block[index] = 1 }
            blocks = append(blocks, block)
            runtime.KeepAlive(blocks)
        }
    case "security":
        if os.Geteuid() == 0 { os.Exit(10) }
        if err := os.WriteFile("/rootfs-write-test", []byte("bad"), 0600); err == nil { os.Exit(11) }
        connection, err := net.DialTimeout("tcp", "198.18.0.1:80", 250*time.Millisecond)
        if err == nil { _ = connection.Close(); os.Exit(12) }
        fmt.Fprint(os.Stdout, "secured")
    case "files":
        var files []*os.File
        for len(files) < 128 {
            file, err := os.Open("/dev/null")
            if err != nil { fmt.Fprint(os.Stdout, "limited"); return }
            files = append(files, file)
        }
        os.Exit(13)
    case "processes":
        var children []*exec.Cmd
        for len(children) < 64 {
            child := exec.Command(os.Args[0], "child")
            if err := child.Start(); err != nil {
                fmt.Fprint(os.Stdout, "limited")
                return
            }
            children = append(children, child)
        }
        os.Exit(14)
    default:
        os.Exit(65)
    }
}
`
