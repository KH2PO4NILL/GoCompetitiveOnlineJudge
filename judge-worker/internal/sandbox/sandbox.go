package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxArguments       = 256
	MaxEnvironment     = 128
	MaxMounts          = 16
	MaxStringBytes     = 4096
	MaxExecutionTime   = 24 * time.Hour
	MaxMemoryBytes     = int64(64 << 30)
	MaxOutputBytes     = int64(256 << 20)
	MaxPIDLimit        = int64(4096)
	MaxOpenFiles       = uint64(65536)
	DefaultNanoCPUs    = int64(1_000_000_000)
	DefaultStopTimeout = 5 * time.Second
	MaxAPITimeout      = time.Minute
	MaxStopTimeout     = time.Minute
)

var (
	ErrInvalidRequest = errors.New("invalid sandbox execute request")
	ErrOutputLimit    = errors.New("sandbox output limit exceeded")
)

// ExitReason describes how an untrusted process stopped. It deliberately does
// not contain judge-level concepts such as Accepted, Wrong Answer or Compile
// Error; those belong to the judge engine that calls Sandbox.
type ExitReason string

const (
	ExitNormal       ExitReason = "normal"
	ExitSignal       ExitReason = "signal"
	ExitTLE          ExitReason = "time_limit_exceeded"
	ExitMLE          ExitReason = "memory_limit_exceeded"
	ExitOLE          ExitReason = "output_limit_exceeded"
	ExitSandboxError ExitReason = "sandbox_error"
)

type Limits struct {
	CPUTime     time.Duration
	WallTime    time.Duration
	MemoryBytes int64
	OutputBytes int64
	PIDs        int64
	OpenFiles   uint64
}

// Mount is an operator-controlled bind mount. Source is a host path below an
// allow-listed root; Target is a canonical path below /sandbox in the container.
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

type ExecuteRequest struct {
	AttemptID    int64
	SubmissionID int64
	Image        string
	Program      string
	Args         []string
	Env          []string
	WorkingDir   string
	Stdin        io.Reader
	Mounts       []Mount
	Limits       Limits
}

type ExecuteResult struct {
	Reason ExitReason

	ExitCode *int
	Signal   string

	WallTime    time.Duration
	CPUTime     time.Duration
	PeakMemory  int64
	OutputBytes int64

	Stdout []byte
	Stderr []byte

	ContainerID string
	Diagnostic  string
}

type CleanupResult struct {
	Found   int
	Removed int
	Failed  int
}

type Sandbox interface {
	Execute(context.Context, ExecuteRequest) (ExecuteResult, error)
	Cleanup(context.Context) (CleanupResult, error)
	Close() error
}

func (request ExecuteRequest) Validate() error {
	var errs []error
	if request.AttemptID <= 0 {
		errs = append(errs, errors.New("AttemptID must be positive"))
	}
	if request.SubmissionID <= 0 {
		errs = append(errs, errors.New("SubmissionID must be positive"))
	}
	if err := validatePlainString("Image", request.Image, false); err != nil {
		errs = append(errs, err)
	}
	if err := validateProgram(request.Program); err != nil {
		errs = append(errs, err)
	}
	if request.WorkingDir == "" {
		request.WorkingDir = "/sandbox"
	}
	if err := validateContainerPath("WorkingDir", request.WorkingDir, true); err != nil {
		errs = append(errs, err)
	}
	if len(request.Args) > MaxArguments {
		errs = append(errs, fmt.Errorf("Args must contain at most %d entries", MaxArguments))
	}
	for index, argument := range request.Args {
		if err := validatePlainString(fmt.Sprintf("Args[%d]", index), argument, true); err != nil {
			errs = append(errs, err)
		}
	}
	if len(request.Env) > MaxEnvironment {
		errs = append(errs, fmt.Errorf("Env must contain at most %d entries", MaxEnvironment))
	}
	seenEnv := make(map[string]struct{}, len(request.Env))
	for index, value := range request.Env {
		if err := validateEnvironment(index, value, seenEnv); err != nil {
			errs = append(errs, err)
		}
	}
	if len(request.Mounts) > MaxMounts {
		errs = append(errs, fmt.Errorf("Mounts must contain at most %d entries", MaxMounts))
	}
	seenTargets := make(map[string]struct{}, len(request.Mounts))
	for index, item := range request.Mounts {
		if strings.TrimSpace(item.Source) == "" || strings.ContainsRune(item.Source, '\x00') {
			errs = append(errs, fmt.Errorf("Mounts[%d].Source is invalid", index))
		}
		if err := validateContainerPath(fmt.Sprintf("Mounts[%d].Target", index), item.Target, false); err != nil {
			errs = append(errs, err)
		} else if item.Target == "/sandbox" {
			errs = append(errs, fmt.Errorf("Mounts[%d].Target must be below /sandbox", index))
		} else if _, exists := seenTargets[item.Target]; exists {
			errs = append(errs, fmt.Errorf("Mounts contains duplicate target %q", item.Target))
		} else {
			seenTargets[item.Target] = struct{}{}
		}
	}
	if err := request.Limits.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w: %w", ErrInvalidRequest, errors.Join(errs...))
	}
	return nil
}

func (limits Limits) Validate() error {
	var errs []error
	if limits.CPUTime <= 0 || limits.CPUTime > MaxExecutionTime {
		errs = append(errs, fmt.Errorf("CPUTime must be between 1ns and %s", MaxExecutionTime))
	}
	if limits.WallTime <= 0 || limits.WallTime > MaxExecutionTime {
		errs = append(errs, fmt.Errorf("WallTime must be between 1ns and %s", MaxExecutionTime))
	}
	if limits.WallTime < limits.CPUTime {
		errs = append(errs, errors.New("WallTime must not be less than CPUTime"))
	}
	if limits.MemoryBytes <= 0 || limits.MemoryBytes > MaxMemoryBytes {
		errs = append(errs, fmt.Errorf("MemoryBytes must be between 1 and %d", MaxMemoryBytes))
	}
	if limits.OutputBytes <= 0 || limits.OutputBytes > MaxOutputBytes {
		errs = append(errs, fmt.Errorf("OutputBytes must be between 1 and %d", MaxOutputBytes))
	}
	if limits.PIDs <= 0 || limits.PIDs > MaxPIDLimit {
		errs = append(errs, fmt.Errorf("PIDs must be between 1 and %d", MaxPIDLimit))
	}
	if limits.OpenFiles <= 2 || limits.OpenFiles > MaxOpenFiles {
		errs = append(errs, fmt.Errorf("OpenFiles must be between 3 and %d", MaxOpenFiles))
	}
	return errors.Join(errs...)
}

func validatePlainString(field, value string, allowEmpty bool) error {
	if (!allowEmpty && strings.TrimSpace(value) == "") || len(value) > MaxStringBytes ||
		!utf8.ValidString(value) || strings.ContainsRune(value, '\x00') || strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s is invalid", field)
	}
	return nil
}

func validateProgram(value string) error {
	if err := validatePlainString("Program", value, false); err != nil {
		return err
	}
	cleaned := path.Clean(value)
	if !strings.HasPrefix(cleaned, "/") || cleaned == "/" || cleaned != value {
		return errors.New("Program must be a canonical absolute container path")
	}
	base := path.Base(cleaned)
	switch strings.ToLower(base) {
	case "sh", "bash", "dash", "ash", "zsh", "ksh", "fish":
		return errors.New("Program must not invoke a command shell")
	}
	return nil
}

func validateContainerPath(field, value string, allowSandboxRoot bool) error {
	if err := validatePlainString(field, value, false); err != nil {
		return err
	}
	cleaned := path.Clean(value)
	if !strings.HasPrefix(cleaned, "/sandbox") || cleaned != value ||
		(cleaned != "/sandbox" && !strings.HasPrefix(cleaned, "/sandbox/")) ||
		(!allowSandboxRoot && cleaned == "/sandbox") {
		return fmt.Errorf("%s must be a canonical absolute path below /sandbox", field)
	}
	return nil
}

func validateEnvironment(index int, value string, seen map[string]struct{}) error {
	if err := validatePlainString(fmt.Sprintf("Env[%d]", index), value, false); err != nil {
		return err
	}
	name, _, ok := strings.Cut(value, "=")
	if !ok || name == "" || len(name) > 128 {
		return fmt.Errorf("Env[%d] must use NAME=value form", index)
	}
	for pos, character := range name {
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || character == '_' || (pos > 0 && character >= '0' && character <= '9') {
			continue
		}
		return fmt.Errorf("Env[%d] has an invalid variable name", index)
	}
	if _, exists := seen[name]; exists {
		return fmt.Errorf("Env contains duplicate variable %q", name)
	}
	seen[name] = struct{}{}
	return nil
}
