package engine

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/language"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/sandbox"
)

var ErrAttemptState = errors.New("invalid language attempt state")

type ProblemLimits struct {
	CPUTime     time.Duration
	WallTime    time.Duration
	MemoryBytes int64
}

type Request struct {
	AttemptID    int64
	SubmissionID int64
	ProfileID    string
	Sources      map[string][]byte
	Limits       ProblemLimits
}

type Result struct {
	Policy    language.Audit
	Execution sandbox.ExecuteResult
}

type Engine struct {
	registry language.Registry
	executor sandbox.Sandbox
	workDir  string
	logger   *slog.Logger
}

type Attempt struct {
	mu           sync.Mutex
	executor     sandbox.Sandbox
	logger       *slog.Logger
	policy       language.Policy
	attemptID    int64
	submissionID int64
	directory    string
	environment  []string
	runLimits    sandbox.Limits
	built        bool
	buildStarted bool
	closed       bool
}

func New(registry language.Registry, executor sandbox.Sandbox, workDir string, logger *slog.Logger) (*Engine, error) {
	if registry == nil || executor == nil || logger == nil || workDir == "" {
		return nil, errors.New("registry, sandbox, work directory and logger are required")
	}
	absolute, err := filepath.Abs(workDir)
	if err != nil {
		return nil, err
	}
	return &Engine{registry: registry, executor: executor, workDir: absolute, logger: logger}, nil
}

func (engine *Engine) Prepare(ctx context.Context, request Request) (*Attempt, error) {
	if request.AttemptID <= 0 || request.SubmissionID <= 0 {
		return nil, errors.New("attempt and submission IDs must be positive")
	}
	policy, err := engine.registry.Snapshot(ctx, request.ProfileID)
	if err != nil {
		return nil, err
	}
	if err := validateSources(policy.Profile.Source, request.Sources); err != nil {
		return nil, err
	}
	limits, err := effectiveLimits(policy.Profile, request.Limits)
	if err != nil {
		return nil, err
	}
	environment := make([]string, 0, len(policy.Profile.Environment))
	for name, value := range policy.Profile.Environment {
		environment = append(environment, name+"="+value)
	}
	sort.Strings(environment)
	encoded, err := json.Marshal(struct {
		RegistryDigest string
		RunLimits      sandbox.Limits
	}{policy.Audit.PolicyDigest, limits})
	if err != nil {
		return nil, err
	}
	policy.Audit.PolicyDigest = fmt.Sprintf("sha256:%x", sha256.Sum256(encoded))
	directory, err := os.MkdirTemp(engine.workDir, fmt.Sprintf("attempt-%d-", request.AttemptID))
	if err != nil {
		return nil, fmt.Errorf("create attempt directory: %w", err)
	}
	attempt := &Attempt{executor: engine.executor, logger: engine.logger, policy: policy,
		attemptID: request.AttemptID, submissionID: request.SubmissionID, directory: directory,
		environment: environment, runLimits: limits}
	if err := attempt.writeSources(request.Sources); err != nil {
		return nil, errors.Join(err, attempt.Close())
	}
	engine.logger.InfoContext(ctx, "language attempt policy frozen", "attempt_id", request.AttemptID,
		"submission_id", request.SubmissionID, "policy", policy.Audit)
	return attempt, nil
}

func validateSources(rules language.SourceRules, sources map[string][]byte) error {
	if len(sources) == 0 || len(sources) > rules.MaxFiles {
		return errors.New("source file count exceeds profile rules")
	}
	if _, exists := sources[rules.EntryFile]; !exists {
		return fmt.Errorf("missing source entry file %q", rules.EntryFile)
	}
	var total int64
	for name, content := range sources {
		if !validSourceName(name) || !slices.Contains(rules.Extensions, path.Ext(name)) {
			return fmt.Errorf("invalid source file %q", name)
		}
		if int64(len(content)) > rules.MaxBytes-total {
			return errors.New("source bytes exceed profile limit")
		}
		total += int64(len(content))
	}
	return nil
}

func validSourceName(name string) bool {
	if len(name) == 0 || len(name) > 128 || name == "." || name == ".." {
		return false
	}
	for index, character := range name {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' {
			continue
		}
		if index > 0 && (character == '.' || character == '-') {
			continue
		}
		return false
	}
	return true
}

func effectiveLimits(profile language.Profile, base ProblemLimits) (sandbox.Limits, error) {
	if base.CPUTime <= 0 || base.CPUTime > sandbox.MaxExecutionTime || base.WallTime < base.CPUTime || base.WallTime > sandbox.MaxExecutionTime || base.MemoryBytes <= 0 || base.MemoryBytes > sandbox.MaxMemoryBytes {
		return sandbox.Limits{}, errors.New("invalid problem time or memory limits")
	}
	limits := profile.RunLimits.SandboxLimits()
	cpu := math.Ceil(float64(base.CPUTime) * profile.TimeMultiplier)
	wall := math.Ceil(float64(base.WallTime) * profile.TimeMultiplier)
	memory := math.Ceil(float64(base.MemoryBytes) * profile.MemoryMultiplier)
	if cpu > float64(limits.CPUTime) || wall > float64(limits.WallTime) || memory > float64(limits.MemoryBytes) {
		return sandbox.Limits{}, errors.New("scaled problem limits exceed language profile caps")
	}
	limits.CPUTime, limits.WallTime, limits.MemoryBytes = time.Duration(cpu), time.Duration(wall), int64(memory)
	return limits, limits.Validate()
}

func (attempt *Attempt) writeSources(sources map[string][]byte) error {
	for _, name := range []string{"source", "artifact"} {
		if err := os.Mkdir(filepath.Join(attempt.directory, name), 0o755); err != nil {
			return err
		}
	}
	if err := os.Chmod(filepath.Join(attempt.directory, "source"), 0o755); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Join(attempt.directory, "artifact"), 0o777); err != nil {
		return err
	}
	for name, content := range sources {
		filename := filepath.Join(attempt.directory, "source", name)
		if err := os.WriteFile(filename, content, 0o444); err != nil {
			return err
		}
		if err := os.Chmod(filename, 0o444); err != nil {
			return err
		}
		if err := os.Chtimes(filename, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
			return err
		}
	}
	return nil
}

func (attempt *Attempt) Audit() language.Audit { return attempt.policy.Audit }

func (attempt *Attempt) Build(ctx context.Context) (Result, error) {
	attempt.mu.Lock()
	defer attempt.mu.Unlock()
	result := Result{Policy: attempt.policy.Audit}
	if attempt.closed || attempt.buildStarted {
		return result, fmt.Errorf("%w: build requires a fresh attempt", ErrAttemptState)
	}
	attempt.buildStarted = true
	request := attempt.executeRequest(attempt.policy.Profile.Build, attempt.policy.Profile.BuildLimits.SandboxLimits())
	request.Mounts = []sandbox.Mount{
		{Source: filepath.Join(attempt.directory, "source"), Target: language.SourceDirectory, ReadOnly: true},
		{Source: filepath.Join(attempt.directory, "artifact"), Target: language.ArtifactDirectory},
	}
	execution, err := attempt.executor.Execute(ctx, request)
	result.Execution = execution
	if err != nil {
		return result, err
	}
	if execution.Reason != sandbox.ExitNormal || execution.ExitCode == nil || *execution.ExitCode != 0 {
		return result, nil
	}
	if err := attempt.validateArtifacts(); err != nil {
		return result, fmt.Errorf("invalid build artifacts: %w", err)
	}
	attempt.built = true
	attempt.logger.InfoContext(ctx, "language build completed", "attempt_id", attempt.attemptID, "policy", attempt.policy.Audit)
	return result, nil
}

func (attempt *Attempt) Run(ctx context.Context, stdin io.Reader) (Result, error) {
	attempt.mu.Lock()
	defer attempt.mu.Unlock()
	result := Result{Policy: attempt.policy.Audit}
	if attempt.closed || !attempt.built {
		return result, fmt.Errorf("%w: run requires a successful build", ErrAttemptState)
	}
	request := attempt.executeRequest(attempt.policy.Profile.Run, attempt.runLimits)
	request.Stdin = stdin
	request.Mounts = []sandbox.Mount{{Source: filepath.Join(attempt.directory, "artifact"), Target: language.ArtifactDirectory, ReadOnly: true}}
	execution, err := attempt.executor.Execute(ctx, request)
	result.Execution = execution
	attempt.logger.InfoContext(ctx, "language run completed", "attempt_id", attempt.attemptID, "policy", attempt.policy.Audit, "reason", execution.Reason)
	return result, err
}

func (attempt *Attempt) executeRequest(command language.Command, limits sandbox.Limits) sandbox.ExecuteRequest {
	return sandbox.ExecuteRequest{AttemptID: attempt.attemptID, SubmissionID: attempt.submissionID,
		Image: command.Image, Program: command.Program, Args: slices.Clone(command.Args),
		Env: slices.Clone(attempt.environment), WorkingDir: language.ArtifactDirectory, Limits: limits}
}

func (attempt *Attempt) validateArtifacts() error {
	rules := attempt.policy.Profile.Artifact
	entries, err := os.ReadDir(filepath.Join(attempt.directory, "artifact"))
	if err != nil {
		return err
	}
	if len(entries) != len(rules.Files) {
		return errors.New("artifact files do not match declared outputs")
	}
	var total int64
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !slices.Contains(rules.Files, entry.Name()) || !info.Mode().IsRegular() || info.Size() <= 0 || info.Mode().Perm()&0o444 != 0o444 {
			return fmt.Errorf("artifact %q must be a declared, nonempty, readable regular file", entry.Name())
		}
		if info.Size() > rules.MaxBytes-total {
			return errors.New("artifact bytes exceed profile limit")
		}
		total += info.Size()
	}
	return nil
}

func (attempt *Attempt) Close() error {
	attempt.mu.Lock()
	defer attempt.mu.Unlock()
	attempt.closed = true
	return os.RemoveAll(attempt.directory)
}
