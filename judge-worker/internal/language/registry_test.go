package language

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeResolver struct {
	digest string
	calls  int
	err    error
}

func (resolver *fakeResolver) ResolveImage(context.Context, string) (string, error) {
	resolver.calls++
	return resolver.digest, resolver.err
}

func testProfile(t *testing.T) Profile {
	t.Helper()
	profiles, err := LoadProfiles("")
	if err != nil {
		t.Fatal(err)
	}
	return profiles[0]
}

func TestProfileValidation(t *testing.T) {
	for name, mutate := range map[string]func(*Profile){
		"oversized id":        func(profile *Profile) { profile.ID = "fake-v" + strings.Repeat("1", 129) },
		"unversioned id":      func(profile *Profile) { profile.ID = "cpp20" },
		"schema version":      func(profile *Profile) { profile.SchemaVersion = 2 },
		"entry traversal":     func(profile *Profile) { profile.Source.EntryFile = "../main.cpp" },
		"entry extension":     func(profile *Profile) { profile.Source.EntryFile = "main.go" },
		"duplicate extension": func(profile *Profile) { profile.Source.Extensions = []string{".cpp", ".cpp"} },
		"extension path":      func(profile *Profile) { profile.Source.Extensions = []string{"./cpp"} },
		"source files":        func(profile *Profile) { profile.Source.MaxFiles = 0 },
		"source bytes":        func(profile *Profile) { profile.Source.MaxBytes = 1 << 30 },
		"latest":              func(profile *Profile) { profile.Build.Image = "gcc:latest" },
		"floating image":      func(profile *Profile) { profile.Run.Image = "gcc:14" },
		"shell": func(profile *Profile) {
			profile.Build.Program = "/bin/sh"
			profile.Build.Args = []string{"-c", "gcc main.cpp"}
		},
		"joined command":        func(profile *Profile) { profile.Run.Program = "/opt/judge/run --flag" },
		"nul args":              func(profile *Profile) { profile.Build.Args = []string{"bad\x00"} },
		"too many args":         func(profile *Profile) { profile.Build.Args = make([]string, 257) },
		"artifact directory":    func(profile *Profile) { profile.Artifact.Directory = "/tmp" },
		"artifact traversal":    func(profile *Profile) { profile.Artifact.Files = []string{"../main"} },
		"duplicate artifact":    func(profile *Profile) { profile.Artifact.Files = []string{"main", "main"} },
		"artifact bytes":        func(profile *Profile) { profile.Artifact.MaxBytes = 0 },
		"nan multiplier":        func(profile *Profile) { profile.TimeMultiplier = math.NaN() },
		"infinite multiplier":   func(profile *Profile) { profile.MemoryMultiplier = math.Inf(1) },
		"negative multiplier":   func(profile *Profile) { profile.TimeMultiplier = -1 },
		"zero multiplier":       func(profile *Profile) { profile.MemoryMultiplier = 0 },
		"pid limit":             func(profile *Profile) { profile.RunLimits.PIDs = 0 },
		"fd limit":              func(profile *Profile) { profile.BuildLimits.OpenFiles = 2 },
		"output limit":          func(profile *Profile) { profile.RunLimits.OutputBytes = 1 << 40 },
		"duration overflow":     func(profile *Profile) { profile.BuildLimits.CPUTimeMS = math.MaxInt64 },
		"wall time":             func(profile *Profile) { profile.BuildLimits.WallTimeMS = 1 },
		"environment whitelist": func(profile *Profile) { profile.Environment["SECRET"] = "value" },
		"invalid environment":   func(profile *Profile) { profile.EnvironmentAllowlist = []string{"NAME=value"} },
		"duplicate environment": func(profile *Profile) { profile.EnvironmentAllowlist = []string{"LANG", "LANG"} },
		"environment value":     func(profile *Profile) { profile.Environment["LANG"] = "bad\nvalue" },
	} {
		t.Run(name, func(t *testing.T) {
			profile := testProfile(t)
			mutate(&profile)
			if err := profile.Validate(); err == nil {
				t.Fatal("invalid profile accepted")
			}
			profile.Enabled = false
			if err := profile.Validate(); err == nil {
				t.Fatal("invalid disabled profile accepted")
			}
		})
	}
}

func TestImageReferences(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	for _, image := range []string{digest, "registry:5000/judge@" + digest, "judge:1.2.3", "judge:v1.2.3-bookworm"} {
		if err := ValidateImage(image); err != nil {
			t.Errorf("%s: %v", image, err)
		}
	}
	for _, image := range []string{"", "judge", "judge:latest", "judge:latest@" + digest, "judge:stable", "judge:1", "judge@sha256:abc", "judge:1.2.3\n"} {
		if err := ValidateImage(image); err == nil {
			t.Errorf("accepted %q", image)
		}
	}
}

func TestLoadStrictRegistry(t *testing.T) {
	for name, content := range map[string]string{
		"empty":              "",
		"empty list":         "profiles: []",
		"unknown field":      string(defaultProfiles) + "typo: true\n",
		"nested unknown":     strings.Replace(string(defaultProfiles), "entry_file:", "entry_flie:", 1),
		"multiple documents": string(defaultProfiles) + "---\nprofiles: []\n",
		"duplicate field":    strings.Replace(string(defaultProfiles), "schema_version: 1", "schema_version: 1\n    schema_version: 1", 1),
		"duplicate id":       strings.Replace(string(defaultProfiles), "id: go-v2", "id: cpp20-v2", 1),
		"oversized":          strings.Repeat(" ", (1<<20)+1),
	} {
		t.Run(name, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "profiles.yaml")
			if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadProfiles(filename); err == nil {
				t.Fatal("invalid registry accepted")
			}
		})
	}
}

func TestRegistrySnapshotIsImmutableAndAuditable(t *testing.T) {
	profile := testProfile(t)
	resolver := &fakeResolver{digest: "sha256:" + strings.Repeat("a", 64)}
	registry, err := NewRegistry([]Profile{profile}, resolver)
	if err != nil {
		t.Fatal(err)
	}
	profile.Environment["LANG"] = "mutated"
	profile.Source.Extensions[0] = ".go"
	first, err := registry.Snapshot(context.Background(), profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolver.calls != 2 || first.Profile.Environment["LANG"] != "C" || first.Profile.Source.Extensions[0] != ".cpp" {
		t.Fatalf("snapshot = %+v", first)
	}
	if first.Audit.CompilerDigest != resolver.digest || first.Audit.RuntimeDigest != resolver.digest || first.Profile.Build.Image != resolver.digest || first.Audit.ProfileID != profile.ID || !imageIDPattern.MatchString(first.Audit.PolicyDigest) {
		t.Fatalf("audit = %+v", first.Audit)
	}
	first.Profile.Environment["LANG"] = "changed"
	second, err := registry.Snapshot(context.Background(), profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Audit.PolicyDigest != first.Audit.PolicyDigest || second.Profile.Environment["LANG"] != "C" {
		t.Fatal("registry mutated through snapshot")
	}
	resolver.digest = "sha256:" + strings.Repeat("b", 64)
	third, err := registry.Snapshot(context.Background(), profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if third.Audit.PolicyDigest == first.Audit.PolicyDigest || first.Profile.Build.Image == resolver.digest {
		t.Fatal("image digest did not freeze policy")
	}
}

func TestRegistryErrors(t *testing.T) {
	profile := testProfile(t)
	profile.Enabled = false
	resolver := &fakeResolver{digest: "sha256:" + strings.Repeat("a", 64)}
	registry, err := NewRegistry([]Profile{profile}, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Snapshot(context.Background(), "missing"); !errors.Is(err, ErrUnknownProfile) {
		t.Fatal(err)
	}
	if _, err := registry.Snapshot(context.Background(), profile.ID); !errors.Is(err, ErrDisabledProfile) {
		t.Fatal(err)
	}
	if resolver.calls != 0 {
		t.Fatal("unknown or disabled profile resolved images")
	}
	profile.Enabled = true
	registry, err = NewRegistry([]Profile{profile}, resolver)
	if err != nil {
		t.Fatal(err)
	}
	resolver.err = errors.New("image not preloaded")
	if _, err := registry.Snapshot(context.Background(), profile.ID); !errors.Is(err, resolver.err) {
		t.Fatal(err)
	}
	resolver.err, resolver.digest = nil, "mutable:1.0.0"
	if _, err := registry.Snapshot(context.Background(), profile.ID); err == nil {
		t.Fatal("accepted mutable resolved image")
	}
}

func TestPolicyDigestIncludesProfileSettings(t *testing.T) {
	profile := testProfile(t)
	resolver := &fakeResolver{digest: "sha256:" + strings.Repeat("a", 64)}
	digest := func(profile Profile) string {
		registry, err := NewRegistry([]Profile{profile}, resolver)
		if err != nil {
			t.Fatal(err)
		}
		policy, err := registry.Snapshot(context.Background(), profile.ID)
		if err != nil {
			t.Fatal(err)
		}
		return policy.Audit.PolicyDigest
	}
	before := digest(profile)
	profile.RunLimits.PIDs++
	if digest(profile) == before {
		t.Fatal("resource change did not affect policy digest")
	}
	profile.Run.Image = "different-runtime:1.0.0"
	if digest(profile) == before {
		t.Fatal("runtime change did not affect policy digest")
	}
}
