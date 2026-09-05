// Package testdatamanifest defines the versioned, shared schema for problem
// test-data archives. Both problem-rpc and judge-worker must use this package
// when accepting or consuming a package.
package testdatamanifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"strings"
	"unicode/utf8"
)

const (
	FileName       = "manifest.json"
	CurrentVersion = 1
	MaxPathBytes   = 1024
	MaxTestCases   = 10000
)

// Manifest is stored as manifest.json at the root of every test-data ZIP.
// Files contains every regular payload file in the archive, excluding the
// manifest itself. No undeclared payload files are allowed.
type Manifest struct {
	Version   int        `json:"version"`
	Files     []File     `json:"files"`
	TestCases []TestCase `json:"test_cases"`
}

type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type TestCase struct {
	Name       string `json:"name"`
	InputFile  string `json:"input_file"`
	OutputFile string `json:"output_file"`
	Score      int64  `json:"score"`
	Ordinal    int64  `json:"ordinal"`
	Enabled    *bool  `json:"enabled,omitempty"`
	GroupName  string `json:"group_name,omitempty"`
}

func (testCase TestCase) IsEnabled() bool {
	return testCase.Enabled == nil || *testCase.Enabled
}

// Parse reads at most maxBytes, rejects trailing JSON and unknown fields, and
// validates all schema invariants. Manifest size is deliberately bounded so
// this small control file may be held in memory.
func Parse(reader io.Reader, maxBytes int64) (*Manifest, error) {
	if reader == nil {
		return nil, errors.New("manifest reader is required")
	}
	if maxBytes <= 0 {
		return nil, errors.New("manifest size limit must be positive")
	}
	content, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	if int64(len(content)) > maxBytes {
		return nil, errors.New("manifest exceeds its size limit")
	}
	if !utf8.Valid(content) {
		return nil, errors.New("manifest must be valid UTF-8")
	}
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("manifest must contain exactly one JSON value")
		}
		return nil, fmt.Errorf("decode trailing manifest content: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// Validate normalizes SHA-256 strings to lowercase and checks that test cases
// reference a unique, fully declared payload file list.
func (manifest *Manifest) Validate() error {
	if manifest == nil {
		return errors.New("manifest is required")
	}
	if manifest.Version != CurrentVersion {
		return fmt.Errorf("unsupported manifest version %d", manifest.Version)
	}
	if len(manifest.Files) == 0 {
		return errors.New("manifest contains no payload files")
	}
	files := make(map[string]struct{}, len(manifest.Files))
	for index := range manifest.Files {
		item := &manifest.Files[index]
		cleaned, err := SafePath(item.Path)
		if err != nil {
			return fmt.Errorf("invalid manifest file path %q: %w", item.Path, err)
		}
		item.Path = cleaned
		if item.Path == FileName {
			return errors.New("manifest must not list itself as a payload file")
		}
		if _, exists := files[item.Path]; exists {
			return fmt.Errorf("manifest contains duplicate file path %q", item.Path)
		}
		files[item.Path] = struct{}{}
		if item.Size < 0 {
			return fmt.Errorf("manifest file %q has a negative size", item.Path)
		}
		checksum, err := NormalizeSHA256(item.SHA256)
		if err != nil {
			return fmt.Errorf("manifest file %q has an invalid SHA-256", item.Path)
		}
		item.SHA256 = checksum
	}
	for filePath := range files {
		for parent := path.Dir(filePath); parent != "."; parent = path.Dir(parent) {
			if _, exists := files[parent]; exists {
				return fmt.Errorf("manifest file %q has regular-file parent %q", filePath, parent)
			}
		}
	}
	if len(manifest.TestCases) == 0 {
		return errors.New("manifest contains no test cases")
	}
	if len(manifest.TestCases) > MaxTestCases {
		return fmt.Errorf("manifest contains more than %d test cases", MaxTestCases)
	}
	ordinals := make(map[int64]struct{}, len(manifest.TestCases))
	for index := range manifest.TestCases {
		item := &manifest.TestCases[index]
		item.Name = strings.TrimSpace(item.Name)
		if item.Name == "" || utf8.RuneCountInString(item.Name) > 128 {
			return errors.New("manifest contains an invalid test case name")
		}
		if utf8.RuneCountInString(item.GroupName) > 128 {
			return fmt.Errorf("test case %q has a group name longer than 128 characters", item.Name)
		}
		if item.Score < 0 || item.Score > math.MaxInt32 || item.Ordinal < 0 || item.Ordinal > math.MaxInt32 {
			return fmt.Errorf("test case %q has an out-of-range score or ordinal", item.Name)
		}
		if _, exists := ordinals[item.Ordinal]; exists {
			return fmt.Errorf("manifest contains duplicate ordinal %d", item.Ordinal)
		}
		ordinals[item.Ordinal] = struct{}{}
		inputPath, err := SafePath(item.InputFile)
		if err != nil {
			return fmt.Errorf("invalid input_file for test case %q", item.Name)
		}
		outputPath, err := SafePath(item.OutputFile)
		if err != nil {
			return fmt.Errorf("invalid output_file for test case %q", item.Name)
		}
		if inputPath == outputPath {
			return fmt.Errorf("test case %q uses the same input and output file", item.Name)
		}
		if _, exists := files[inputPath]; !exists {
			return fmt.Errorf("test case %q references undeclared input file %q", item.Name, inputPath)
		}
		if _, exists := files[outputPath]; !exists {
			return fmt.Errorf("test case %q references undeclared output file %q", item.Name, outputPath)
		}
		item.InputFile = inputPath
		item.OutputFile = outputPath
	}
	return nil
}

// PayloadDirectories returns the exact set of ancestor directories needed by
// the declared payload files. Explicit ZIP directory entries outside this set
// are not part of the package.
func (manifest *Manifest) PayloadDirectories() map[string]struct{} {
	result := make(map[string]struct{})
	if manifest == nil {
		return result
	}
	for _, file := range manifest.Files {
		for parent := path.Dir(file.Path); parent != "."; parent = path.Dir(parent) {
			result[parent] = struct{}{}
		}
	}
	return result
}

func (manifest *Manifest) FileMap() map[string]File {
	if manifest == nil {
		return nil
	}
	result := make(map[string]File, len(manifest.Files))
	for _, file := range manifest.Files {
		result[file.Path] = file
	}
	return result
}

// SafePath validates an untrusted ZIP entry or manifest path. It intentionally
// accepts only canonical slash-separated relative paths and is safe on both
// Unix and Windows hosts.
func SafePath(value string) (string, error) {
	if value == "" || len(value) > MaxPathBytes || !utf8.ValidString(value) ||
		strings.ContainsRune(value, '\x00') || strings.ContainsAny(value, "\\:") || strings.HasPrefix(value, "/") {
		return "", errors.New("path must be a canonical relative UTF-8 path")
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != value {
		return "", errors.New("path must not contain traversal or non-canonical components")
	}
	return cleaned, nil
}

func NormalizeSHA256(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) != sha256.Size*2 {
		return "", errors.New("SHA-256 must contain 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", errors.New("SHA-256 must contain 64 hexadecimal characters")
	}
	return strings.ToLower(value), nil
}
