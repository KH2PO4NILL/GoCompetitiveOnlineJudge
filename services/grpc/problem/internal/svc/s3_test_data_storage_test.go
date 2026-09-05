package svc

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/testdatamanifest"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"
)

const manifestFileName = testdatamanifest.FileName

type testDataManifest = testdatamanifest.Manifest
type testDataManifestCase = testdatamanifest.TestCase

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestUploadPolicyValidatesConfiguredLimitsAndFileNames(t *testing.T) {
	policy, err := NewUploadPolicy(config.UploadConf{
		URLTTLSeconds: 60, MaxUploadBytes: 1024, AllowedExtensions: []string{".ZIP", ".zip"},
		MaxUncompressedBytes: 2048, MaxFileCount: 10, MaxManifestBytes: 512,
	})
	if err != nil {
		t.Fatalf("NewUploadPolicy() error = %v", err)
	}
	if len(policy.AllowedExtensions) != 1 || policy.AllowedExtensions[0] != ".zip" {
		t.Fatalf("AllowedExtensions = %v", policy.AllowedExtensions)
	}
	if err := policy.ValidateFileName("cases.ZIP"); err != nil {
		t.Fatalf("ValidateFileName(cases.ZIP) error = %v", err)
	}
	for _, name := range []string{"../cases.zip", "cases.tar", "", strings.Repeat("a", 256) + ".zip"} {
		if err := policy.ValidateFileName(name); err == nil {
			t.Errorf("ValidateFileName(%q) unexpectedly succeeded", name)
		}
	}
}

func TestS3StorageCreatesBoundedPresignedUploadURL(t *testing.T) {
	policy := defaultUploadPolicy()
	storageValue, err := NewS3TestDataStorage(config.ObjectStorageConf{
		Endpoint: "https://storage.example.test", Region: "test-1", Bucket: "problem-data",
		AccessKey: "access", SecretKey: "secret", UsePathStyle: true,
	}, policy)
	if err != nil {
		t.Fatalf("NewS3TestDataStorage() error = %v", err)
	}
	storage := storageValue.(*s3TestDataStorage)
	now := time.Date(2026, 8, 27, 1, 2, 3, 0, time.UTC)
	storage.now = func() time.Time { return now }
	uploadURL, err := storage.CreateUploadURL(context.Background(), &model.TestDataUpload{
		ObjectKey: "problem-test-data/1/upload/data.zip", ExpiresAt: now.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateUploadURL() error = %v", err)
	}
	parsed, err := url.Parse(uploadURL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	query := parsed.Query()
	if parsed.Path != "/problem-data/problem-test-data/1/upload/data.zip" ||
		query.Get("X-Amz-Algorithm") != awsSignatureAlgorithm ||
		query.Get("X-Amz-Expires") != "900" || len(query.Get("X-Amz-Signature")) != 64 {
		t.Fatalf("presigned URL = %s", uploadURL)
	}
}

func TestCanonicalURIStrictlyEncodesObjectPath(t *testing.T) {
	value := &url.URL{Path: "/bucket/a b@/中文.zip"}
	if got, want := canonicalURI(value), "/bucket/a%20b%40/%E4%B8%AD%E6%96%87.zip"; got != want {
		t.Fatalf("canonicalURI() = %q, want %q", got, want)
	}
}

func TestS3StorageDownloadsAndInspectsValidArchive(t *testing.T) {
	archive := testArchive(t, map[string][]byte{
		manifestFileName: manifestJSON(t, []testDataManifestCase{{
			Name: "case-1", InputFile: "cases/1.in", OutputFile: "cases/1.out", Score: 100, Ordinal: 1,
		}}),
		"cases/1.in": []byte("1 2\n"), "cases/1.out": []byte("3\n"),
	})
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") == "" || request.Header.Get("x-amz-content-sha256") != emptyPayloadSHA256 {
			t.Error("request is missing SigV4 headers")
		}
		switch request.Method {
		case http.MethodGet:
			if request.URL.Path != "/problem-data/problem-test-data/1/upload/data.zip" {
				t.Errorf("GET path = %q", request.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(archive)),
				ContentLength: int64(len(archive)), Header: make(http.Header),
			}, nil
		case http.MethodPut:
			if request.URL.Path != "/problem-data/problem-test-data/1/versions/upload-1.zip" ||
				request.Header.Get("x-amz-copy-source") != "/problem-data/problem-test-data/1/upload/data.zip" ||
				!strings.Contains(request.Header.Get("Authorization"), "x-amz-copy-source") {
				t.Errorf("copy request = %s %s headers %v", request.Method, request.URL.Path, request.Header)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
		default:
			t.Errorf("unexpected request method %s", request.Method)
			return &http.Response{StatusCode: http.StatusMethodNotAllowed, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
		}
	})
	policy, err := NewUploadPolicy(config.UploadConf{
		MaxUploadBytes: 1 << 20, MaxUncompressedBytes: 2 << 20,
		MaxFileCount: 20, MaxManifestBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewUploadPolicy() error = %v", err)
	}
	storageValue, err := NewS3TestDataStorage(config.ObjectStorageConf{
		Endpoint: "https://storage.example.test", Bucket: "problem-data", AccessKey: "access", SecretKey: "secret", UsePathStyle: true,
	}, policy)
	if err != nil {
		t.Fatalf("NewS3TestDataStorage() error = %v", err)
	}
	storageValue.(*s3TestDataStorage).client.Transport = transport
	digest := sha256.Sum256(archive)
	object, err := storageValue.InspectUpload(context.Background(), &model.TestDataUpload{
		UploadID: "upload-1", ProblemID: 1, ObjectKey: "problem-test-data/1/upload/data.zip",
		ContentLength: int64(len(archive)), SHA256: hex.EncodeToString(digest[:]),
	})
	if err != nil {
		t.Fatalf("InspectUpload() error = %v", err)
	}
	if object.PackageKey != "problem-test-data/1/versions/upload-1.zip" ||
		len(object.TestCases) != 1 || object.TestCases[0].Name != "case-1" || !object.TestCases[0].Enabled {
		t.Fatalf("InspectUpload() object = %+v", object)
	}
}

func TestInspectTestDataArchiveRejectsUnsafePackages(t *testing.T) {
	validManifest := manifestJSON(t, []testDataManifestCase{{
		Name: "case", InputFile: "1.in", OutputFile: "1.out", Ordinal: 1,
	}})
	tests := []struct {
		name    string
		entries map[string][]byte
	}{
		{name: "path traversal", entries: map[string][]byte{manifestFileName: validManifest, "../escape": []byte("x"), "1.in": {}, "1.out": {}}},
		{name: "missing referenced file", entries: map[string][]byte{manifestFileName: validManifest, "1.in": {}}},
		{name: "undeclared directory", entries: map[string][]byte{manifestFileName: validManifest, "empty/": {}, "1.in": {}, "1.out": {}}},
		{name: "regular file parent", entries: map[string][]byte{manifestFileName: validManifest, "1.in": {}, "1.out": {}, "extra": {}, "extra/child": {}}},
		{name: "unknown manifest field", entries: map[string][]byte{manifestFileName: []byte(`{"test_cases":[],"unknown":true}`)}},
		{name: "duplicate ordinal", entries: map[string][]byte{
			manifestFileName: manifestJSON(t, []testDataManifestCase{
				{Name: "one", InputFile: "1.in", OutputFile: "1.out", Ordinal: 1},
				{Name: "two", InputFile: "2.in", OutputFile: "2.out", Ordinal: 1},
			}),
			"1.in": {}, "1.out": {}, "2.in": {}, "2.out": {},
		}},
	}
	policy := defaultUploadPolicy()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archive := testArchive(t, test.entries)
			fileName := filepath.Join(t.TempDir(), "test.zip")
			if err := os.WriteFile(fileName, archive, 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			if _, err := inspectTestDataArchive(fileName, policy); err == nil {
				t.Fatal("inspectTestDataArchive() unexpectedly succeeded")
			}
		})
	}
}

func TestInspectTestDataArchiveEnforcesUncompressedLimit(t *testing.T) {
	archive := testArchive(t, map[string][]byte{
		manifestFileName: manifestJSON(t, []testDataManifestCase{{Name: "case", InputFile: "1.in", OutputFile: "1.out", Ordinal: 1}}),
		"1.in":           bytes.Repeat([]byte("a"), 1024), "1.out": {},
	})
	fileName := filepath.Join(t.TempDir(), "large.zip")
	if err := os.WriteFile(fileName, archive, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	policy := defaultUploadPolicy()
	policy.MaxUncompressedBytes = 512
	if _, err := inspectTestDataArchive(fileName, policy); err == nil {
		t.Fatal("inspectTestDataArchive() unexpectedly succeeded")
	}
}

func testArchive(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	if raw, exists := entries[manifestFileName]; exists {
		var manifest testdatamanifest.Manifest
		if err := json.Unmarshal(raw, &manifest); err == nil {
			manifest.Version = testdatamanifest.CurrentVersion
			manifest.Files = nil
			for name, content := range entries {
				if name == manifestFileName || strings.HasSuffix(name, "/") {
					continue
				}
				digest := sha256.Sum256(content)
				manifest.Files = append(manifest.Files, testdatamanifest.File{
					Path: name, Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:]),
				})
			}
			encoded, err := json.Marshal(manifest)
			if err != nil {
				t.Fatalf("json.Marshal(manifest) error = %v", err)
			}
			entries[manifestFileName] = encoded
		}
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("zip.Create(%q) error = %v", name, err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatalf("zip.Write(%q) error = %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("zip.Close() error = %v", err)
	}
	return buffer.Bytes()
}

func manifestJSON(t *testing.T, testCases []testDataManifestCase) []byte {
	t.Helper()
	content, err := json.Marshal(testDataManifest{Version: testdatamanifest.CurrentVersion, TestCases: testCases})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return content
}
