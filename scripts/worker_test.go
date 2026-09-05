package scripts_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

type workerState struct {
	PID      int
	Args     []string
	Consumer string
	Metrics  string
	Address  string
}

func TestMain(tests *testing.M) {
	if os.Getenv("GCOJ_WORKER_SCRIPT_HELPER") == "1" {
		shutdown := make(chan os.Signal, 1)
		signal.Notify(shutdown, syscall.SIGTERM)
		_ = json.NewEncoder(os.Stdout).Encode(workerState{
			PID: os.Getpid(), Args: os.Args[1:],
			Consumer: os.Getenv("JUDGE_CONSUMER_ENABLED"), Metrics: os.Getenv("JUDGE_METRICS_ENABLED"),
			Address: os.Getenv("JUDGE_METRICS_LISTEN_ON"),
		})
		<-shutdown
		fmt.Println("drained")
		os.Exit(0)
	}
	os.Exit(tests.Run())
}

func workerCommand(t *testing.T, directory, action string) *exec.Cmd {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("worker development script requires a Unix shell")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	command := exec.CommandContext(ctx, "bash", "dev-worker.sh", action)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "JUDGE_") || strings.HasPrefix(name, "GCOJ_WORKER_") || name == "ENV_FILE" || name == "RUNTIME_DIR" || name == "WORKER_CONFIG" || name == "GO" || name == "BASH_ENV" {
			continue
		}
		command.Env = append(command.Env, entry)
	}
	command.Env = append(command.Env, "ENV_FILE="+filepath.Join(directory, "worker.env"), "RUNTIME_DIR="+directory)
	return command
}

func writeWorkerFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerRunExecAndGracefulSignal(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			directory := t.TempDir()
			command := workerCommand(t, directory, "run")
			binaryDirectory := filepath.Join(directory, "bin")
			if err := os.MkdirAll(binaryDirectory, 0700); err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(executable)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(binaryDirectory, "judge-worker"), content, 0700); err != nil {
				t.Fatal(err)
			}
			environment := "GCOJ_WORKER_SCRIPT_HELPER=1\n"
			if explicit {
				environment += "JUDGE_CONSUMER_ENABLED=false\nJUDGE_METRICS_ENABLED=false\nJUDGE_METRICS_LISTEN_ON=127.0.0.1:19100\n"
			}
			environmentPath := filepath.Join(directory, "worker.env")
			writeWorkerFile(t, environmentPath, environment, 0600)
			configPath := filepath.Join(directory, "custom.yaml")
			writeWorkerFile(t, configPath, "", 0600)
			command.Env = append(command.Env, "WORKER_CONFIG="+configPath)
			stdout, err := command.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			var diagnostics bytes.Buffer
			command.Stderr = &diagnostics
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = command.Process.Kill() })
			reader := bufio.NewReader(stdout)
			line, err := reader.ReadBytes('\n')
			if err != nil {
				_ = command.Wait()
				t.Fatalf("worker helper did not start: %v %s", err, diagnostics.String())
			}
			var state workerState
			if err := json.Unmarshal(line, &state); err != nil {
				t.Fatal(err)
			}
			wantEnabled, wantAddress := "true", "127.0.0.1:9100"
			if explicit {
				wantEnabled, wantAddress = "false", "127.0.0.1:19100"
			}
			if state.PID != command.Process.Pid || strings.Join(state.Args, " ") != "-f "+configPath || state.Consumer != wantEnabled || state.Metrics != wantEnabled || state.Address != wantAddress {
				t.Fatalf("incorrect exec identity, flags or environment: %+v", state)
			}
			if err := command.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			remaining, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if err := command.Wait(); err != nil || string(remaining) != "drained\n" {
				t.Fatalf("signal failed to drain: %v %q %s", err, remaining, diagnostics.String())
			}
			unchanged, err := os.ReadFile(environmentPath)
			if err != nil || string(unchanged) != environment {
				t.Fatal("environment file changed", err)
			}
		})
	}
}

func TestWorkerStatusRequiresReadyHTTPResponse(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl is unavailable")
	}
	for _, status := range []int{200, 503, 301, 401, 404, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var unexpected atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/readyz" || request.Method != http.MethodGet {
					unexpected.Store(true)
				}
				writer.Header().Set("Location", "/must-not-follow")
				writer.WriteHeader(status)
				_, _ = writer.Write([]byte("private-response-value"))
			}))
			defer server.Close()
			directory := t.TempDir()
			writeWorkerFile(t, filepath.Join(directory, "worker.env"), "JUDGE_METRICS_LISTEN_ON="+strings.TrimPrefix(server.URL, "http://")+"\n", 0600)
			command := workerCommand(t, directory, "status")
			command.Env = append(command.Env, "http_proxy=http://127.0.0.1:1", "ALL_PROXY=http://127.0.0.1:1", "CURL_HOME="+directory)
			writeWorkerFile(t, filepath.Join(directory, ".curlrc"), "fail\nlocation\n", 0600)
			output, err := command.CombinedOutput()
			if (err == nil) != (status == 200) || unexpected.Load() || strings.Contains(string(output), "private-response-value") {
				t.Fatalf("unsafe or incorrect status: error=%v output=%q", err, output)
			}
		})
	}
}

func TestWorkerScriptRejectsInvalidInputs(t *testing.T) {
	for _, test := range []struct{ action, environment string }{
		{action: "run"},
		{action: "status"},
		{action: "stop", environment: ""},
		{action: "status", environment: "JUDGE_METRICS_LISTEN_ON=127.0.0.1:0"},
		{action: "status", environment: "JUDGE_METRICS_LISTEN_ON=127.0.0.1:65536"},
		{action: "status", environment: "JUDGE_METRICS_LISTEN_ON=private-address.example:9100"},
		{action: "status", environment: "JUDGE_METRICS_LISTEN_ON=127.0.0.1:9100/private-path"},
	} {
		directory := t.TempDir()
		if test.environment != "" {
			writeWorkerFile(t, filepath.Join(directory, "worker.env"), test.environment, 0600)
		}
		output, err := workerCommand(t, directory, test.action).CombinedOutput()
		if err == nil || strings.Contains(string(output), "private-") {
			t.Fatalf("invalid input was accepted or exposed: %v %q", err, output)
		}
	}
}

func TestWorkerStatusFailsOnUnavailableOrStalledEndpoint(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl is unavailable")
	}
	for _, stalled := range []bool{false, true} {
		t.Run(fmt.Sprint(stalled), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				<-request.Context().Done()
			}))
			defer server.Close()
			if !stalled {
				server.Close()
			}
			directory := t.TempDir()
			writeWorkerFile(t, filepath.Join(directory, "worker.env"), "JUDGE_METRICS_LISTEN_ON="+strings.TrimPrefix(server.URL, "http://")+"\n", 0600)
			started := time.Now()
			output, err := workerCommand(t, directory, "status").CombinedOutput()
			if err == nil || !strings.Contains(string(output), "endpoint is unreachable") || time.Since(started) > 5*time.Second {
				t.Fatalf("endpoint failure was not bounded and safely reported: %v %q", err, output)
			}
		})
	}
}

func TestWorkerBuildDoesNotLoadEnvironmentAndPreservesBinaryOnFailure(t *testing.T) {
	directory := t.TempDir()
	command := workerCommand(t, directory, "build")
	goStub := filepath.Join(directory, "go-stub")
	writeWorkerFile(t, goStub, "#!/usr/bin/env bash\nset -eu\n[[ $1 == build && $2 == -o && $4 == ./judge-worker ]]\nprintf 'new-binary' > \"$3\"\n", 0700)
	writeWorkerFile(t, filepath.Join(directory, "worker.env"), "exit 77\n", 0600)
	command.Env = append(command.Env, "GO="+goStub)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build loaded environment or used wrong flags: %v %s", err, output)
	}
	writeWorkerFile(t, goStub, "#!/usr/bin/env bash\nprintf 'partial-binary' > \"$3\"\nexit 1\n", 0700)
	command = workerCommand(t, directory, "build")
	command.Env = append(command.Env, "GO="+goStub)
	if _, err := command.CombinedOutput(); err == nil {
		t.Fatal("build failure was ignored")
	}
	content, err := os.ReadFile(filepath.Join(directory, "bin", "judge-worker"))
	if err != nil || string(content) != "new-binary" {
		t.Fatal("failed build replaced the existing binary", err)
	}
	leftovers, err := filepath.Glob(filepath.Join(directory, "bin", ".judge-worker.*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("build leaked temporary files", err)
	}
}
