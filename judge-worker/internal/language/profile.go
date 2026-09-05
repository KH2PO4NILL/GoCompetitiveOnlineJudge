package language

import (
	"errors"
	"fmt"
	"math"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/sandbox"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgepolicy"
	"github.com/distribution/reference"
)

const (
	SchemaVersion     = 1
	SourceDirectory   = "/sandbox/source"
	ArtifactDirectory = "/sandbox/artifact"
	BuildProgram      = "/opt/judge/build"
	RunProgram        = "/opt/judge/run"
)

var (
	profileIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}-v[1-9][0-9]*$`)
	filePattern      = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
	envPattern       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)
	versionPattern   = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9][A-Za-z0-9_.-]*)?$`)
	imageIDPattern   = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
)

type SourceRules struct {
	EntryFile  string   `yaml:"entry_file"`
	Extensions []string `yaml:"extensions"`
	MaxFiles   int      `yaml:"max_files"`
	MaxBytes   int64    `yaml:"max_bytes"`
}

type Command struct {
	Image   string   `yaml:"image"`
	Program string   `yaml:"program"`
	Args    []string `yaml:"args"`
}

type ArtifactRules struct {
	Directory string   `yaml:"directory"`
	Files     []string `yaml:"files"`
	MaxBytes  int64    `yaml:"max_bytes"`
}

type ResourceLimits struct {
	CPUTimeMS   int64  `yaml:"cpu_time_ms"`
	WallTimeMS  int64  `yaml:"wall_time_ms"`
	MemoryBytes int64  `yaml:"memory_bytes"`
	TmpfsBytes  int64  `yaml:"tmpfs_bytes"`
	OutputBytes int64  `yaml:"output_bytes"`
	PIDs        int64  `yaml:"pids"`
	OpenFiles   uint64 `yaml:"open_files"`
}

func (limits ResourceLimits) SandboxLimits() sandbox.Limits {
	return sandbox.Limits{
		CPUTime:     time.Duration(limits.CPUTimeMS) * time.Millisecond,
		WallTime:    time.Duration(limits.WallTimeMS) * time.Millisecond,
		MemoryBytes: limits.MemoryBytes, TmpfsBytes: limits.TmpfsBytes, OutputBytes: limits.OutputBytes,
		PIDs: limits.PIDs, OpenFiles: limits.OpenFiles,
	}
}

type Profile struct {
	ID                   string            `yaml:"id"`
	SchemaVersion        int               `yaml:"schema_version"`
	Enabled              bool              `yaml:"enabled"`
	Source               SourceRules       `yaml:"source"`
	Build                Command           `yaml:"build"`
	Run                  Command           `yaml:"run"`
	Artifact             ArtifactRules     `yaml:"artifact"`
	BuildLimits          ResourceLimits    `yaml:"build_limits"`
	RunLimits            ResourceLimits    `yaml:"run_limits"`
	TimeMultiplier       float64           `yaml:"time_multiplier"`
	MemoryMultiplier     float64           `yaml:"memory_multiplier"`
	EnvironmentAllowlist []string          `yaml:"environment_allowlist"`
	Environment          map[string]string `yaml:"environment"`
}

func (profile Profile) Validate() error {
	if len(profile.ID) > 128 || !profileIDPattern.MatchString(profile.ID) || profile.SchemaVersion != SchemaVersion {
		return errors.New("id must be a versioned lowercase name ending in -vN; schema_version must be 1")
	}
	if !validFile(profile.Source.EntryFile) || profile.Source.MaxFiles < 1 || profile.Source.MaxFiles > 64 || profile.Source.MaxBytes < 1 || profile.Source.MaxBytes > judgepolicy.MaxSourceCodeBytes {
		return errors.New("invalid source entry_file or source limits")
	}
	if len(profile.Source.Extensions) == 0 || len(profile.Source.Extensions) > 16 {
		return errors.New("source extensions must contain 1 to 16 entries")
	}
	seen := map[string]bool{}
	for _, extension := range profile.Source.Extensions {
		if !strings.HasPrefix(extension, ".") || !validFile("file"+extension) || path.Ext("file"+extension) != extension || seen[extension] {
			return fmt.Errorf("invalid or duplicate source extension %q", extension)
		}
		seen[extension] = true
	}
	if !seen[path.Ext(profile.Source.EntryFile)] {
		return errors.New("source entry_file must use an allowed extension")
	}
	if profile.Artifact.Directory != ArtifactDirectory || len(profile.Artifact.Files) == 0 || len(profile.Artifact.Files) > 64 || profile.Artifact.MaxBytes < 1 || profile.Artifact.MaxBytes > 256<<20 {
		return errors.New("invalid artifact directory, files or max_bytes")
	}
	seen = map[string]bool{}
	for _, file := range profile.Artifact.Files {
		if !validFile(file) || seen[file] {
			return fmt.Errorf("invalid or duplicate artifact file %q", file)
		}
		seen[file] = true
	}
	for _, multiplier := range []float64{profile.TimeMultiplier, profile.MemoryMultiplier} {
		if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier <= 0 || multiplier > 100 {
			return errors.New("time and memory multipliers must be finite, positive and at most 100")
		}
	}
	if len(profile.EnvironmentAllowlist) > sandbox.MaxEnvironment || len(profile.Environment) > sandbox.MaxEnvironment {
		return errors.New("too many environment variables")
	}
	seen = map[string]bool{}
	for _, name := range profile.EnvironmentAllowlist {
		if !envPattern.MatchString(name) || seen[name] {
			return fmt.Errorf("invalid or duplicate environment allowlist name %q", name)
		}
		seen[name] = true
	}
	environment := make([]string, 0, len(profile.Environment))
	for name, value := range profile.Environment {
		if !seen[name] {
			return fmt.Errorf("environment variable %q is not allowlisted", name)
		}
		environment = append(environment, name+"="+value)
	}
	for _, stage := range []struct {
		command Command
		limits  ResourceLimits
		program string
	}{
		{profile.Build, profile.BuildLimits, BuildProgram}, {profile.Run, profile.RunLimits, RunProgram},
	} {
		if stage.command.Program != stage.program {
			return fmt.Errorf("program must be %s; shell commands are forbidden", stage.program)
		}
		if err := ValidateImage(stage.command.Image); err != nil {
			return err
		}
		if stage.limits.CPUTimeMS <= 0 || stage.limits.CPUTimeMS > sandbox.MaxExecutionTime.Milliseconds() || stage.limits.WallTimeMS <= 0 || stage.limits.WallTimeMS > sandbox.MaxExecutionTime.Milliseconds() {
			return errors.New("CPU and wall time limits must be between 1ms and 24h")
		}
		request := sandbox.ExecuteRequest{
			AttemptID: 1, SubmissionID: 1, Image: stage.command.Image,
			Program: stage.command.Program, Args: stage.command.Args, Env: environment,
			WorkingDir: ArtifactDirectory, Limits: stage.limits.SandboxLimits(),
		}
		if err := request.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func ValidateImage(image string) error {
	if imageIDPattern.MatchString(image) {
		return nil
	}
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return fmt.Errorf("invalid image %q: %w", image, err)
	}
	if tagged, ok := named.(reference.Tagged); ok && strings.EqualFold(tagged.Tag(), "latest") {
		return errors.New("latest image tag is forbidden even with a digest")
	}
	if canonical, ok := named.(reference.Canonical); ok {
		if !imageIDPattern.MatchString(canonical.Digest().String()) {
			return errors.New("image digest must use sha256")
		}
		return nil
	}
	if tagged, ok := named.(reference.Tagged); ok && versionPattern.MatchString(tagged.Tag()) {
		return nil
	}
	return fmt.Errorf("image %q requires a sha256 digest or fixed major.minor.patch version", image)
}

func validFile(name string) bool {
	return filePattern.MatchString(name) && name != "." && name != ".."
}
