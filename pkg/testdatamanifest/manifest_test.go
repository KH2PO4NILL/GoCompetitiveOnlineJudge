package testdatamanifest

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestParseValidManifest(t *testing.T) {
	inputDigest := sha256.Sum256([]byte("input"))
	outputDigest := sha256.Sum256([]byte("output"))
	manifest, err := Parse(strings.NewReader(`{
		"version":1,
		"files":[
			{"path":"cases/1.in","sha256":"`+strings.ToUpper(hex.EncodeToString(inputDigest[:]))+`","size":5},
			{"path":"cases/1.out","sha256":"`+hex.EncodeToString(outputDigest[:])+`","size":6}
		],
		"test_cases":[{"name":"case-1","input_file":"cases/1.in","output_file":"cases/1.out","score":100,"ordinal":1}]
	}`), 4096)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if manifest.Version != CurrentVersion || !manifest.TestCases[0].IsEnabled() ||
		manifest.Files[0].SHA256 != strings.ToLower(manifest.Files[0].SHA256) {
		t.Fatalf("Parse() manifest = %+v", manifest)
	}
}

func TestParseRejectsUnsafeOrInconsistentManifest(t *testing.T) {
	digest := strings.Repeat("0", 64)
	for _, test := range []struct {
		name string
		json string
	}{
		{name: "unknown version", json: `{"version":2,"files":[],"test_cases":[]}`},
		{name: "unknown field", json: `{"version":1,"files":[],"test_cases":[],"extra":true}`},
		{name: "traversal", json: `{"version":1,"files":[{"path":"../x","sha256":"` + digest + `","size":0}],"test_cases":[]}`},
		{name: "self listed", json: `{"version":1,"files":[{"path":"manifest.json","sha256":"` + digest + `","size":0}],"test_cases":[]}`},
		{name: "undeclared reference", json: `{"version":1,"files":[{"path":"1.in","sha256":"` + digest + `","size":0}],"test_cases":[{"name":"x","input_file":"1.in","output_file":"1.out","score":0,"ordinal":0}]}`},
		{name: "duplicate ordinal", json: `{"version":1,"files":[{"path":"1.in","sha256":"` + digest + `","size":0},{"path":"1.out","sha256":"` + digest + `","size":0}],"test_cases":[{"name":"x","input_file":"1.in","output_file":"1.out","score":0,"ordinal":0},{"name":"y","input_file":"1.in","output_file":"1.out","score":0,"ordinal":0}]}`},
		{name: "regular file parent", json: `{"version":1,"files":[{"path":"cases","sha256":"` + digest + `","size":0},{"path":"cases/1.in","sha256":"` + digest + `","size":0},{"path":"1.out","sha256":"` + digest + `","size":0}],"test_cases":[{"name":"x","input_file":"cases/1.in","output_file":"1.out","score":0,"ordinal":0}]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(test.json), 4096); err == nil {
				t.Fatal("Parse() unexpectedly succeeded")
			}
		})
	}
}

func TestSafePathRejectsPlatformIndependentTraversal(t *testing.T) {
	for _, value := range []string{"", "/absolute", "../x", "a/../x", "a//x", `a\\x`, "C:x", "./x", "x/"} {
		if _, err := SafePath(value); err == nil {
			t.Errorf("SafePath(%q) unexpectedly succeeded", value)
		}
	}
	if got, err := SafePath("cases/1.in"); err != nil || got != "cases/1.in" {
		t.Fatalf("SafePath(valid) = %q, %v", got, err)
	}
}
