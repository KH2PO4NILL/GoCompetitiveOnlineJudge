package sandbox

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestExecuteRequestValidate(t *testing.T) {
	valid := validExecuteRequest()
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*ExecuteRequest)
		want   string
	}{
		{name: "attempt", mutate: func(r *ExecuteRequest) { r.AttemptID = 0 }, want: "AttemptID"},
		{name: "submission", mutate: func(r *ExecuteRequest) { r.SubmissionID = 0 }, want: "SubmissionID"},
		{name: "relative program", mutate: func(r *ExecuteRequest) { r.Program = "solution" }, want: "Program"},
		{name: "shell program", mutate: func(r *ExecuteRequest) { r.Program = "/bin/bash" }, want: "command shell"},
		{name: "working directory escape", mutate: func(r *ExecuteRequest) { r.WorkingDir = "/tmp" }, want: "WorkingDir"},
		{name: "duplicate env", mutate: func(r *ExecuteRequest) { r.Env = []string{"PATH=/usr/bin", "PATH=/bin"} }, want: "duplicate"},
		{name: "invalid env", mutate: func(r *ExecuteRequest) { r.Env = []string{"9BAD=value"} }, want: "variable name"},
		{name: "duplicate mount", mutate: func(r *ExecuteRequest) {
			r.Mounts = []Mount{{Source: "/host/a", Target: "/sandbox/data", ReadOnly: true}, {Source: "/host/b", Target: "/sandbox/data", ReadOnly: true}}
		}, want: "duplicate target"},
		{name: "mount root", mutate: func(r *ExecuteRequest) {
			r.Mounts = []Mount{{Source: "/host/a", Target: "/sandbox", ReadOnly: true}}
		}, want: "below /sandbox"},
		{name: "invalid limits", mutate: func(r *ExecuteRequest) { r.Limits.WallTime = r.Limits.CPUTime - time.Nanosecond }, want: "WallTime"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := validExecuteRequest()
			test.mutate(&request)
			err := request.Validate()
			if err == nil || !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want ErrInvalidRequest containing %q", err, test.want)
			}
		})
	}
}

func TestLimitsValidateHardBounds(t *testing.T) {
	valid := validExecuteRequest().Limits
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*Limits)
	}{
		{name: "cpu", mutate: func(l *Limits) { l.CPUTime = MaxExecutionTime + 1 }},
		{name: "wall", mutate: func(l *Limits) { l.WallTime = 0 }},
		{name: "memory", mutate: func(l *Limits) { l.MemoryBytes = MaxMemoryBytes + 1 }},
		{name: "tmpfs negative", mutate: func(limits *Limits) { limits.TmpfsBytes = -1 }},
		{name: "tmpfs memory", mutate: func(limits *Limits) { limits.TmpfsBytes = limits.MemoryBytes + 1 }},
		{name: "tmpfs hard cap", mutate: func(limits *Limits) { limits.MemoryBytes = MaxMemoryBytes; limits.TmpfsBytes = MaxTmpfsBytes + 1 }},
		{name: "output", mutate: func(l *Limits) { l.OutputBytes = MaxOutputBytes + 1 }},
		{name: "pids", mutate: func(l *Limits) { l.PIDs = MaxPIDLimit + 1 }},
		{name: "files", mutate: func(l *Limits) { l.OpenFiles = MaxOpenFiles + 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			limits := valid
			test.mutate(&limits)
			if err := limits.Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func validExecuteRequest() ExecuteRequest {
	return ExecuteRequest{
		AttemptID: 1, SubmissionID: 2, Image: "judge-cpp:local",
		Program: "/sandbox/solution", Args: []string{"--case", "1"},
		Env: []string{"PATH=/usr/bin", "lower_case=allowed"}, WorkingDir: "/sandbox/work",
		Mounts: []Mount{{Source: "/host/data", Target: "/sandbox/data", ReadOnly: true}},
		Limits: Limits{
			CPUTime: time.Second, WallTime: 2 * time.Second,
			MemoryBytes: 128 << 20, OutputBytes: 1 << 20, PIDs: 32, OpenFiles: 64,
		},
	}
}
