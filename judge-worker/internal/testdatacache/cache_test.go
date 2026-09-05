package testdatacache

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/testdatamanifest"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	"google.golang.org/protobuf/proto"
)

type fakeStore struct {
	content []byte
	delay   time.Duration
	opens   atomic.Int64
}

func (store *fakeStore) Open(ctx context.Context, _ string) (*Object, error) {
	store.opens.Add(1)
	if store.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(store.delay):
		}
	}
	return &Object{Body: io.NopCloser(bytes.NewReader(store.content)), ContentLength: int64(len(store.content))}, nil
}

type archiveEntry struct {
	name    string
	content []byte
	mode    os.FileMode
}

func TestCacheDownloadsOnceValidatesAndReusesChecksumEntry(t *testing.T) {
	archive := buildArchive(t, []archiveEntry{
		{name: "cases/1.in", content: []byte("1 2\n")},
		{name: "cases/1.out", content: []byte("3\n")},
	}, nil)
	store := &fakeStore{content: archive, delay: 30 * time.Millisecond}
	cache := newTestCache(t, store, func(c *config.TestDataCacheConf) {})
	spec := specForArchive(archive)

	const concurrency = 12
	packages := make(chan *Package, concurrency)
	errorsFound := make(chan error, concurrency)
	var wait sync.WaitGroup
	for range concurrency {
		wait.Add(1)
		go func() {
			defer wait.Done()
			pkg, err := cache.Prepare(context.Background(), spec.Revision, spec)
			if err != nil {
				errorsFound <- err
				return
			}
			packages <- pkg
		}()
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Fatalf("Prepare() error = %v", err)
	}
	close(packages)
	for pkg := range packages {
		if content, err := os.ReadFile(filepath.Join(pkg.Dir, "cases", "1.out")); err != nil || string(content) != "3\n" {
			t.Fatalf("cached output = %q, %v", content, err)
		}
		pkg.Release()
	}
	if got := store.opens.Load(); got != 1 {
		t.Fatalf("object downloads = %d, want 1", got)
	}

	pkg, err := cache.Prepare(context.Background(), spec.Revision, spec)
	if err != nil {
		t.Fatalf("cache hit Prepare() error = %v", err)
	}
	pkg.Release()
	if got := store.opens.Load(); got != 1 {
		t.Fatalf("downloads after cache hit = %d, want 1", got)
	}
}

func TestConcurrentCallersValidateManifestAgainstTheirOwnSpec(t *testing.T) {
	archive := buildArchive(t, validEntries(), nil)
	store := &fakeStore{content: archive, delay: 30 * time.Millisecond}
	cache := newTestCache(t, store, func(c *config.TestDataCacheConf) {})
	validSpec := specForArchive(archive)
	invalidSpec := proto.Clone(validSpec).(*problempb.JudgeProblemSpec)
	invalidSpec.TestCases[0].Score++

	start := make(chan struct{})
	validResult := make(chan error, 1)
	invalidResult := make(chan error, 1)
	go func() {
		<-start
		pkg, err := cache.Prepare(context.Background(), validSpec.Revision, validSpec)
		if pkg != nil {
			pkg.Release()
		}
		validResult <- err
	}()
	go func() {
		<-start
		pkg, err := cache.Prepare(context.Background(), invalidSpec.Revision, invalidSpec)
		if pkg != nil {
			pkg.Release()
		}
		invalidResult <- err
	}()
	close(start)
	if err := <-validResult; err != nil {
		t.Fatalf("valid Prepare() error = %v", err)
	}
	if err := <-invalidResult; err == nil || !strings.Contains(err.Error(), "JudgeProblemSpec") {
		t.Fatalf("invalid Prepare() error = %v", err)
	}
	if got := store.opens.Load(); got != 1 {
		t.Fatalf("object downloads = %d, want 1", got)
	}
}

func TestCacheRepairsCorruptPublishedEntryAtomically(t *testing.T) {
	archive := buildArchive(t, validEntries(), nil)
	store := &fakeStore{content: archive}
	cache := newTestCache(t, store, func(c *config.TestDataCacheConf) {})
	spec := specForArchive(archive)

	pkg, err := cache.Prepare(context.Background(), spec.Revision, spec)
	if err != nil {
		t.Fatalf("initial Prepare() error = %v", err)
	}
	payloadPath := filepath.Join(pkg.Dir, "cases", "1.out")
	pkg.Release()
	if err := os.WriteFile(payloadPath, []byte("corrupt"), 0o600); err != nil {
		t.Fatalf("corrupt cache entry: %v", err)
	}

	repaired, err := cache.Prepare(context.Background(), spec.Revision, spec)
	if err != nil {
		t.Fatalf("repair Prepare() error = %v", err)
	}
	defer repaired.Release()
	content, err := os.ReadFile(filepath.Join(repaired.Dir, "cases", "1.out"))
	if err != nil || string(content) != "3\n" {
		t.Fatalf("repaired payload = %q, %v", content, err)
	}
	if got := store.opens.Load(); got != 2 {
		t.Fatalf("object downloads = %d, want 2", got)
	}
	assertNoTemporaryEntries(t, cache.root)
}

func TestCacheRejectsObjectChecksumMismatchWithoutPublishingEntry(t *testing.T) {
	archive := buildArchive(t, validEntries(), nil)
	store := &fakeStore{content: archive}
	cache := newTestCache(t, store, func(c *config.TestDataCacheConf) {})
	spec := specForArchive(archive)
	spec.TestDataChecksum = strings.Repeat("0", 64)

	if _, err := cache.Prepare(context.Background(), spec.Revision, spec); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("Prepare() error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(cache.root, spec.TestDataChecksum)); !os.IsNotExist(err) {
		t.Fatalf("failed checksum left a published cache entry: %v", err)
	}
	assertNoTemporaryEntries(t, cache.root)
}

func TestCacheRejectsUnsafeZIPsAndLeavesNoAtomicHit(t *testing.T) {
	bomb := bytes.Repeat([]byte("a"), 64<<10)
	tests := []struct {
		name    string
		entries []archiveEntry
		mutate  func(*config.TestDataCacheConf)
	}{
		{name: "path traversal", entries: append(validEntries(), archiveEntry{name: "../escape", content: []byte("x")})},
		{name: "absolute path", entries: append(validEntries(), archiveEntry{name: "/escape", content: []byte("x")})},
		{name: "symlink", entries: append(validEntries(), archiveEntry{name: "link", content: []byte("target"), mode: os.ModeSymlink | 0o777})},
		{name: "device", entries: append(validEntries(), archiveEntry{name: "device", mode: os.ModeDevice | 0o600})},
		{name: "compression bomb", entries: append(validEntries(), archiveEntry{name: "bomb", content: bomb}), mutate: func(c *config.TestDataCacheConf) { c.MaxCompressionRatio = 2 }},
		{name: "undeclared directory", entries: append(validEntries(), archiveEntry{name: "empty/", mode: os.ModeDir | 0o700})},
		{name: "total uncompressed size", entries: []archiveEntry{{name: "cases/1.in", content: bytes.Repeat([]byte("x"), 700)}, {name: "cases/1.out", content: bytes.Repeat([]byte("y"), 700)}}, mutate: func(c *config.TestDataCacheConf) {
			c.MaxUncompressedBytes = 1200
			c.MaxFileBytes = 800
			c.MaxManifestBytes = 800
		}},
		{name: "too many entries", entries: append(validEntries(), archiveEntry{name: "extra", content: []byte("x")}), mutate: func(c *config.TestDataCacheConf) { c.MaxFileCount = 3 }},
		{name: "single file too large", entries: []archiveEntry{{name: "cases/1.in", content: bytes.Repeat([]byte("x"), 1024)}, {name: "cases/1.out", content: []byte("ok")}}, mutate: func(c *config.TestDataCacheConf) { c.MaxFileBytes = 512; c.MaxManifestBytes = 512 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archive := buildArchive(t, test.entries, nil)
			cache := newTestCache(t, &fakeStore{content: archive}, func(c *config.TestDataCacheConf) {
				if test.mutate != nil {
					test.mutate(c)
				}
			})
			spec := specForArchive(archive)
			if _, err := cache.Prepare(context.Background(), spec.Revision, spec); err == nil {
				t.Fatal("Prepare() unexpectedly succeeded")
			}
			if _, err := os.Lstat(filepath.Join(cache.root, spec.TestDataChecksum)); !os.IsNotExist(err) {
				t.Fatalf("unsafe archive left a cache hit: %v", err)
			}
			assertNoTemporaryEntries(t, cache.root)
		})
	}
}

func TestCacheRejectsManifestChecksumAndSpecMetadataMismatch(t *testing.T) {
	badChecksum := strings.Repeat("0", 64)
	archive := buildArchive(t, validEntries(), func(manifest *testdatamanifest.Manifest) {
		manifest.Files[0].SHA256 = badChecksum
	})
	cache := newTestCache(t, &fakeStore{content: archive}, func(c *config.TestDataCacheConf) {})
	spec := specForArchive(archive)
	if _, err := cache.Prepare(context.Background(), spec.Revision, spec); err == nil || !strings.Contains(err.Error(), "manifest") {
		t.Fatalf("manifest checksum Prepare() error = %v", err)
	}
	assertNoTemporaryEntries(t, cache.root)

	validArchive := buildArchive(t, validEntries(), nil)
	cache = newTestCache(t, &fakeStore{content: validArchive}, func(c *config.TestDataCacheConf) {})
	spec = specForArchive(validArchive)
	spec.TestCases[0].Score++
	if _, err := cache.Prepare(context.Background(), spec.Revision, spec); err == nil || !strings.Contains(err.Error(), "JudgeProblemSpec") {
		t.Fatalf("metadata mismatch Prepare() error = %v", err)
	}
}

func TestCacheValidatesRequestedRevisionBeforeDownload(t *testing.T) {
	archive := buildArchive(t, validEntries(), nil)
	store := &fakeStore{content: archive}
	cache := newTestCache(t, store, func(c *config.TestDataCacheConf) {})
	spec := specForArchive(archive)
	if _, err := cache.Prepare(context.Background(), spec.Revision+1, spec); err == nil {
		t.Fatal("Prepare() unexpectedly accepted mismatched revision")
	}
	if store.opens.Load() != 0 {
		t.Fatal("revision mismatch reached object storage")
	}
}

func TestCleanupHonorsLeaseAndThenEvictsExpiredEntry(t *testing.T) {
	archive := buildArchive(t, validEntries(), nil)
	cache := newTestCache(t, &fakeStore{content: archive}, func(c *config.TestDataCacheConf) { c.TTLSeconds = 1 })
	spec := specForArchive(archive)
	pkg, err := cache.Prepare(context.Background(), spec.Revision, spec)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	old := time.Now().Add(-2 * time.Second)
	if err := os.Chtimes(pkg.Dir, old, old); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}
	if err := cache.Cleanup(context.Background()); err != nil {
		t.Fatalf("Cleanup(active) error = %v", err)
	}
	if _, err := os.Stat(pkg.Dir); err != nil {
		t.Fatalf("active cache entry was removed: %v", err)
	}
	pkg.Release()
	if err := cache.Cleanup(context.Background()); err != nil {
		t.Fatalf("Cleanup(released) error = %v", err)
	}
	if _, err := os.Stat(pkg.Dir); !os.IsNotExist(err) {
		t.Fatalf("expired released cache entry still exists: %v", err)
	}
}

func TestS3StoreUsesSignedGETOnlyAndRejectsUnsafeObjectKey(t *testing.T) {
	store, err := NewS3Store(config.ObjectStorageConf{
		Endpoint: "https://storage.example.test", Region: "test-1", Bucket: "problem-data",
		AccessKey: "read-only", SecretKey: "secret", UsePathStyle: true, RequestTimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}
	store.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Path != "/problem-data/problem-test-data/1/version.zip" {
			t.Errorf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") == "" || request.Header.Get("x-amz-content-sha256") != emptyPayloadSHA256 {
			t.Error("request is missing SigV4 headers")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("zip")), ContentLength: 3, Header: make(http.Header)}, nil
	})
	object, err := store.Open(context.Background(), "problem-test-data/1/version.zip")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	object.Body.Close()
	if _, err := store.Open(context.Background(), "../secret"); err == nil {
		t.Fatal("Open() accepted an unsafe object key")
	}
}

func TestS3StoreDoesNotFollowRedirects(t *testing.T) {
	store, err := NewS3Store(config.ObjectStorageConf{
		Endpoint: "https://storage.example.test", Region: "test-1", Bucket: "problem-data",
		AccessKey: "read-only", SecretKey: "secret", UsePathStyle: true, RequestTimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}
	var requests atomic.Int64
	store.client.Transport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		requests.Add(1)
		return &http.Response{
			StatusCode: http.StatusTemporaryRedirect,
			Header:     http.Header{"Location": []string{"https://attacker.example.test/stolen"}},
			Body:       io.NopCloser(strings.NewReader("redirect")),
		}, nil
	})
	if _, err := store.Open(context.Background(), "problem-test-data/1/version.zip"); err == nil {
		t.Fatal("Open() unexpectedly accepted a redirect")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("redirect caused %d requests, want 1", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func newTestCache(t *testing.T, store ObjectStore, mutate func(*config.TestDataCacheConf)) *Cache {
	t.Helper()
	conf := config.TestDataCacheConf{
		Dir: t.TempDir(), MaxBytes: 8 << 20, TTLSeconds: 3600, CleanupIntervalSeconds: 3600,
		MaxDownloadBytes: 2 << 20, MaxUncompressedBytes: 4 << 20, MaxFileBytes: 1 << 20,
		MaxFileCount: 100, MaxManifestBytes: 64 << 10, MaxCompressionRatio: 200,
	}
	mutate(&conf)
	cache, err := New(conf, store)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(cache.Close)
	return cache
}

func validEntries() []archiveEntry {
	return []archiveEntry{
		{name: "cases/1.in", content: []byte("1 2\n")},
		{name: "cases/1.out", content: []byte("3\n")},
	}
}

func buildArchive(t *testing.T, entries []archiveEntry, mutate func(*testdatamanifest.Manifest)) []byte {
	t.Helper()
	manifest := testdatamanifest.Manifest{Version: testdatamanifest.CurrentVersion}
	for _, entry := range entries {
		if entry.mode&os.ModeType != 0 || strings.HasSuffix(entry.name, "/") || entry.name == testdatamanifest.FileName ||
			entry.name == "../escape" || entry.name == "/escape" {
			continue
		}
		digest := sha256.Sum256(entry.content)
		manifest.Files = append(manifest.Files, testdatamanifest.File{
			Path: entry.name, SHA256: hex.EncodeToString(digest[:]), Size: int64(len(entry.content)),
		})
	}
	manifest.TestCases = []testdatamanifest.TestCase{{
		Name: "case-1", InputFile: "cases/1.in", OutputFile: "cases/1.out", Score: 100, Ordinal: 1,
	}}
	if mutate != nil {
		mutate(&manifest)
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	manifestWriter, err := writer.Create(testdatamanifest.FileName)
	if err != nil {
		t.Fatalf("Create(manifest) error = %v", err)
	}
	if _, err := manifestWriter.Write(manifestJSON); err != nil {
		t.Fatalf("Write(manifest) error = %v", err)
	}
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		entryWriter, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("CreateHeader(%q) error = %v", entry.name, err)
		}
		if _, err := entryWriter.Write(entry.content); err != nil {
			t.Fatalf("Write(%q) error = %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("zip.Close() error = %v", err)
	}
	return buffer.Bytes()
}

func specForArchive(archive []byte) *problempb.JudgeProblemSpec {
	digest := sha256.Sum256(archive)
	return &problempb.JudgeProblemSpec{
		ProblemId: 7, Revision: 11, TestDataPackageKey: "problem-test-data/7/versions/version.zip",
		TestDataChecksum: hex.EncodeToString(digest[:]),
		TestCases:        []*problempb.TestCaseInfo{{Name: "case-1", Score: 100, Ordinal: 1, Enabled: true}},
	}
}

func assertNoTemporaryEntries(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".download-") || strings.HasPrefix(entry.Name(), ".extract-") {
			t.Fatalf("temporary cache entry was not removed: %s", entry.Name())
		}
	}
}
