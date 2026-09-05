package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadAppliesDefaultsAndExpandsEnvironment(t *testing.T) {
	t.Setenv("TEST_JUDGE_WORKER_ID", "  judge-local-1  ")
	t.Setenv("TEST_SUBMIT_TOKEN", "submit-secret")
	t.Setenv("TEST_PROBLEM_TOKEN", "problem-secret")

	path := writeConfig(t, `
Worker:
  ID: "${TEST_JUDGE_WORKER_ID}"
SubmitRPC:
  Target: "127.0.0.1:8004"
ProblemRPC:
  Target: "127.0.0.1:8006"
InternalAuth:
  SubmitToken: "${TEST_SUBMIT_TOKEN}"
  ProblemToken: "${TEST_PROBLEM_TOKEN}"
ObjectStorage:
  Endpoint: "http://127.0.0.1:9000"
  Bucket: "problem-test-data"
  AccessKey: "read-only-access"
  SecretKey: "read-only-secret"
`)

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if c.Worker.ID != "judge-local-1" {
		t.Fatalf("Worker.ID = %q", c.Worker.ID)
	}
	if c.Worker.Concurrency != DefaultConcurrency || c.Worker.QueueCapacity != DefaultQueueCapacity {
		t.Fatalf("worker defaults = concurrency %d queue %d", c.Worker.Concurrency, c.Worker.QueueCapacity)
	}
	if c.InternalAuth.Name != "judge.worker" {
		t.Fatalf("InternalAuth.Name = %q", c.InternalAuth.Name)
	}
	if c.TestDataCache.Dir != DefaultCacheDirectory || c.Sandbox.WorkDir != DefaultSandboxWorkDirectory {
		t.Fatalf("directory defaults = cache %q work %q", c.TestDataCache.Dir, c.Sandbox.WorkDir)
	}
	if c.Sandbox.Docker.Endpoint != DefaultDockerEndpoint || c.Sandbox.Docker.User != DefaultDockerUser ||
		c.Sandbox.Docker.NanoCPUs != DefaultDockerNanoCPUs || c.DockerAPITimeout() != DefaultDockerAPITimeout ||
		c.DockerStopTimeout() != DefaultDockerStopTimeout {
		t.Fatalf("Docker defaults = %#v", c.Sandbox.Docker)
	}
	if c.RPCTimeout() != DefaultRPCTimeout || c.TaskTimeout() != DefaultTaskTimeout ||
		c.ShutdownTimeout() != DefaultShutdownTimeout {
		t.Fatalf("timeout defaults = rpc %s task %s shutdown %s", c.RPCTimeout(), c.TaskTimeout(), c.ShutdownTimeout())
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	for _, test := range []struct {
		name    string
		unknown string
		want    string
	}{
		{name: "top level", unknown: "UnknownSection: true\n", want: "UnknownSection"},
		{name: "nested", unknown: "Timeouts:\n  TaskSecondz: 8\n", want: "TaskSecondz"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfig(t, minimalConfigYAML()+test.unknown)
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), "line") {
				t.Fatalf("Load() error = %v, want unknown field %q with source line", err, test.want)
			}
		})
	}
}

func TestValidateRejectsMissingRequiredFields(t *testing.T) {
	var c Config
	c.ApplyDefaults()
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	for _, field := range []string{"Worker.ID", "SubmitRPC.Target", "ProblemRPC.Target", "SubmitRPC internal authentication", "ProblemRPC internal authentication"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("Validate() error %q does not mention %q", err, field)
		}
	}
}

func TestValidateRejectsWorkerIDsOutsideSubmitRPCContract(t *testing.T) {
	for _, workerID := range []string{
		"评测机-1",
		"judge-\x7f",
		strings.Repeat("a", 129),
	} {
		t.Run(workerID, func(t *testing.T) {
			c := validConfig(t)
			c.Worker.ID = workerID
			if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "Worker.ID") {
				t.Fatalf("Validate() error = %v, want invalid Worker.ID", err)
			}
		})
	}
}

func TestValidateEnforcesWorkerHardLimits(t *testing.T) {
	for _, test := range []struct {
		name        string
		concurrency int
		queue       int
		want        string
	}{
		{name: "zero concurrency after explicit negative", concurrency: -1, queue: 1, want: "Worker.Concurrency"},
		{name: "too much concurrency", concurrency: MaxConcurrency + 1, queue: 1, want: "Worker.Concurrency"},
		{name: "negative queue", concurrency: 1, queue: -1, want: "Worker.QueueCapacity"},
		{name: "too much queue", concurrency: 1, queue: MaxQueueCapacity + 1, want: "Worker.QueueCapacity"},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := validConfig(t)
			c.Worker.Concurrency = test.concurrency
			c.Worker.QueueCapacity = test.queue
			if err := c.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want field %s", err, test.want)
			}
		})
	}
}

func TestValidateRejectsInvalidRuntimeConfiguration(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{name: "metrics address", mutate: func(c *Config) { c.Metrics.Enabled = true; c.Metrics.ListenOn = "bad" }, want: "Metrics.ListenOn"},
		{name: "metrics path", mutate: func(c *Config) { c.Metrics.Path = "metrics" }, want: "Metrics.Path"},
		{name: "docker endpoint", mutate: func(c *Config) { c.Sandbox.Docker.Endpoint = "file:///tmp/docker.sock" }, want: "Sandbox.Docker.Endpoint"},
		{name: "cache root", mutate: func(c *Config) { c.TestDataCache.Dir = string(filepath.Separator) }, want: "TestDataCache.Dir"},
		{name: "shutdown timeout", mutate: func(c *Config) { c.Timeouts.ShutdownSeconds = int64(MaxShutdownTimeout/time.Second) + 1 }, want: "Timeouts.ShutdownSeconds"},
		{name: "overflow timeout", mutate: func(c *Config) { c.Timeouts.TaskSeconds = int64(^uint64(0) >> 1) }, want: "Timeouts.TaskSeconds"},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := validConfig(t)
			test.mutate(&c)
			if err := c.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want field %s", err, test.want)
			}
		})
	}
}

func TestValidateRejectsUnsafeDockerRuntimeOptions(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*DockerConf)
		want   string
	}{
		{name: "root user", mutate: func(c *DockerConf) { c.User = "0:0" }, want: "Sandbox.Docker.User"},
		{name: "named user", mutate: func(c *DockerConf) { c.User = "nobody" }, want: "Sandbox.Docker.User"},
		{name: "API timeout", mutate: func(c *DockerConf) { c.APITimeoutSeconds = 0 }, want: "APITimeoutSeconds"},
		{name: "stop timeout", mutate: func(c *DockerConf) { c.StopTimeoutSeconds = int64(MaxDockerStopTimeout/time.Second) + 1 }, want: "StopTimeoutSeconds"},
		{name: "CPU quota", mutate: func(c *DockerConf) { c.NanoCPUs = MaxDockerNanoCPUs + 1 }, want: "NanoCPUs"},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := validConfig(t)
			c.ApplyDefaults()
			test.mutate(&c.Sandbox.Docker)
			if err := c.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want field %s", err, test.want)
			}
		})
	}
}

func TestValidateRejectsIncompleteDockerEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"unix://",
		"unix:///",
		"tcp://",
		"tcp://localhost",
		"tcp://localhost:0",
		"http://?version=1.0",
		"https://:2376",
	} {
		t.Run(endpoint, func(t *testing.T) {
			c := validConfig(t)
			c.Sandbox.Docker.Endpoint = endpoint
			if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "Sandbox.Docker.Endpoint") {
				t.Fatalf("Validate() error = %v for endpoint %q", err, endpoint)
			}
		})
	}
}

func TestValidationErrorsDoNotExposeTokens(t *testing.T) {
	c := validConfig(t)
	secret := strings.Repeat("sensitive-token-", 300)
	c.InternalAuth.SubmitToken = secret
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "sensitive-token") {
		t.Fatalf("validation error exposed token: %v", err)
	}
}

func TestLoadReportsConfigPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("Load() error = %v, want path %q", err, path)
	}
}

func validConfig(t *testing.T) Config {
	t.Helper()
	root := t.TempDir()
	c := Config{
		Worker:       WorkerConf{ID: "judge-1", Concurrency: 2, QueueCapacity: 8},
		SubmitRPC:    RPCConf{Target: "127.0.0.1:8004"},
		ProblemRPC:   RPCConf{Target: "127.0.0.1:8006"},
		InternalAuth: InternalAuthConf{Name: "judge.worker", SubmitToken: "submit-secret", ProblemToken: "problem-secret"},
		TestDataCache: TestDataCacheConf{
			Dir: filepath.Join(root, "cache"),
		},
		ObjectStorage: ObjectStorageConf{
			Endpoint: "http://127.0.0.1:9000", Region: "us-east-1", Bucket: "problem-test-data",
			AccessKey: "read-only-access", SecretKey: "read-only-secret", UsePathStyle: true, RequestTimeoutSeconds: 60,
		},
		Sandbox: SandboxConf{
			Driver: "docker", WorkDir: filepath.Join(root, "work"),
			Docker: DockerConf{Endpoint: DefaultDockerEndpoint},
		},
		Log:      LogConf{Level: "info", Format: "text"},
		Metrics:  MetricsConf{ListenOn: DefaultMetricsListenOn, Path: DefaultMetricsPath},
		Timeouts: TimeoutConf{RPCSeconds: 5, TaskSeconds: 60, ShutdownSeconds: 5},
	}
	return c
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "judge-worker.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func minimalConfigYAML() string {
	return `
Worker:
  ID: "judge-test"
SubmitRPC:
  Target: "127.0.0.1:8004"
ProblemRPC:
  Target: "127.0.0.1:8006"
InternalAuth:
  SubmitToken: "submit-secret"
  ProblemToken: "problem-secret"
ObjectStorage:
  Endpoint: "http://127.0.0.1:9000"
  Bucket: "problem-test-data"
  AccessKey: "read-only-access"
  SecretKey: "read-only-secret"
`
}
