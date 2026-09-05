package language

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"

	"go.yaml.in/yaml/v3"
)

var (
	ErrUnknownProfile  = errors.New("unknown language profile")
	ErrDisabledProfile = errors.New("disabled language profile")
)

//go:embed profiles.yaml
var defaultProfiles []byte

type Registry interface {
	Snapshot(context.Context, string) (Policy, error)
}

type ImageResolver interface {
	ResolveImage(context.Context, string) (string, error)
}

type Audit struct {
	ProfileID      string `json:"profile_id"`
	SchemaVersion  int    `json:"schema_version"`
	CompilerImage  string `json:"compiler_image"`
	RuntimeImage   string `json:"runtime_image"`
	CompilerDigest string `json:"compiler_digest"`
	RuntimeDigest  string `json:"runtime_digest"`
	PolicyDigest   string `json:"policy_digest"`
}

type Policy struct {
	Profile Profile
	Audit   Audit
}

type StaticRegistry struct {
	profiles map[string]Profile
	resolver ImageResolver
}

func LoadProfiles(filename string) ([]Profile, error) {
	content := defaultProfiles
	if filename != "" {
		file, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("open language registry: %w", err)
		}
		defer file.Close()
		content, err = io.ReadAll(io.LimitReader(file, (1<<20)+1))
		if err != nil {
			return nil, err
		}
	}
	if len(content) > 1<<20 {
		return nil, errors.New("language registry exceeds 1MiB")
	}
	var document struct {
		Profiles []Profile `yaml:"profiles"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode language registry: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("language registry must contain exactly one YAML document")
	}
	if err := validateProfiles(document.Profiles); err != nil {
		return nil, err
	}
	return document.Profiles, nil
}

func validateProfiles(profiles []Profile) error {
	if len(profiles) == 0 || len(profiles) > 256 {
		return errors.New("registry must contain 1 to 256 profiles")
	}
	seen := map[string]bool{}
	for _, profile := range profiles {
		if err := profile.Validate(); err != nil {
			return fmt.Errorf("profile %q: %w", profile.ID, err)
		}
		if seen[profile.ID] {
			return fmt.Errorf("duplicate immutable profile id %q", profile.ID)
		}
		seen[profile.ID] = true
	}
	return nil
}

func NewRegistry(profiles []Profile, resolver ImageResolver) (*StaticRegistry, error) {
	if err := validateProfiles(profiles); err != nil {
		return nil, err
	}
	if resolver == nil {
		return nil, errors.New("image resolver is required")
	}
	registry := &StaticRegistry{profiles: make(map[string]Profile, len(profiles)), resolver: resolver}
	for _, profile := range profiles {
		registry.profiles[profile.ID] = cloneProfile(profile)
	}
	return registry, nil
}

func (registry *StaticRegistry) Snapshot(ctx context.Context, id string) (Policy, error) {
	profile, exists := registry.profiles[id]
	if !exists {
		return Policy{}, fmt.Errorf("%w: %s", ErrUnknownProfile, id)
	}
	if !profile.Enabled {
		return Policy{}, fmt.Errorf("%w: %s", ErrDisabledProfile, id)
	}
	profile = cloneProfile(profile)
	compiler, err := registry.resolver.ResolveImage(ctx, profile.Build.Image)
	if err != nil {
		return Policy{}, fmt.Errorf("resolve compiler image: %w", err)
	}
	runtime := compiler
	if profile.Run.Image != profile.Build.Image {
		runtime, err = registry.resolver.ResolveImage(ctx, profile.Run.Image)
		if err != nil {
			return Policy{}, fmt.Errorf("resolve runtime image: %w", err)
		}
	}
	if !imageIDPattern.MatchString(compiler) || !imageIDPattern.MatchString(runtime) {
		return Policy{}, errors.New("image resolver must return immutable sha256 image IDs")
	}
	audit := Audit{ProfileID: profile.ID, SchemaVersion: profile.SchemaVersion,
		CompilerImage: profile.Build.Image, RuntimeImage: profile.Run.Image,
		CompilerDigest: compiler, RuntimeDigest: runtime}
	encoded, err := json.Marshal(struct {
		Profile                       Profile
		CompilerDigest, RuntimeDigest string
	}{profile, compiler, runtime})
	if err != nil {
		return Policy{}, fmt.Errorf("encode policy: %w", err)
	}
	audit.PolicyDigest = fmt.Sprintf("sha256:%x", sha256.Sum256(encoded))
	profile.Build.Image, profile.Run.Image = compiler, runtime
	return Policy{Profile: profile, Audit: audit}, nil
}

func cloneProfile(profile Profile) Profile {
	profile.Source.Extensions = slices.Clone(profile.Source.Extensions)
	profile.Build.Args = slices.Clone(profile.Build.Args)
	profile.Run.Args = slices.Clone(profile.Run.Args)
	profile.Artifact.Files = slices.Clone(profile.Artifact.Files)
	profile.EnvironmentAllowlist = slices.Clone(profile.EnvironmentAllowlist)
	profile.Environment = maps.Clone(profile.Environment)
	return profile
}
