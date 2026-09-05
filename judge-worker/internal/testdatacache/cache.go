package testdatacache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/testdatamanifest"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
)

type limits struct {
	maxCacheBytes       int64
	ttl                 time.Duration
	cleanupInterval     time.Duration
	maxDownloadBytes    int64
	maxUncompressed     int64
	maxFileBytes        int64
	maxFiles            int
	maxManifestBytes    int64
	maxCompressionRatio int64
}

type Cache struct {
	root   string
	store  ObjectStore
	limits limits
	now    func() time.Time

	flights operationGroup
	opMu    sync.RWMutex
	leaseMu sync.Mutex
	active  map[string]int

	startOnce sync.Once
	stopOnce  sync.Once
	started   chan struct{}
	stop      chan struct{}
	done      chan struct{}
}

type Package struct {
	Dir      string
	Manifest *testdatamanifest.Manifest

	checksum string
	cache    *Cache
	once     sync.Once
}

// Release allows capacity and TTL cleanup to evict this package after the
// current judge attempt finishes.
func (pkg *Package) Release() {
	if pkg == nil || pkg.cache == nil {
		return
	}
	pkg.once.Do(func() { pkg.cache.release(pkg.checksum) })
}

func New(c config.TestDataCacheConf, store ObjectStore) (*Cache, error) {
	if store == nil {
		return nil, errors.New("test data object store is required")
	}
	directory := strings.TrimSpace(c.Dir)
	if directory == "" || directory == "." || strings.ContainsRune(directory, '\x00') ||
		filepath.Clean(directory) == string(filepath.Separator) {
		return nil, errors.New("test data cache requires a dedicated directory")
	}
	if c.MaxBytes < c.MaxUncompressedBytes || c.MaxDownloadBytes <= 0 || c.MaxUncompressedBytes <= 0 ||
		c.MaxFileBytes <= 0 || c.MaxFileBytes > c.MaxUncompressedBytes || c.MaxFileCount <= 0 || c.MaxFileCount > 1_000_000 ||
		c.MaxManifestBytes <= 0 || c.MaxManifestBytes > c.MaxFileBytes || c.MaxCompressionRatio <= 0 || c.MaxCompressionRatio > 100_000 ||
		c.TTLSeconds < 1 || c.TTLSeconds > int64((365*24*time.Hour)/time.Second) ||
		c.CleanupIntervalSeconds < 1 || c.CleanupIntervalSeconds > int64((24*time.Hour)/time.Second) {
		return nil, errors.New("test data cache limits are invalid")
	}
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve test data cache directory: %w", err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create test data cache directory: %w", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, fmt.Errorf("secure test data cache directory: %w", err)
	}
	cache := &Cache{
		root: root, store: store,
		limits: limits{
			maxCacheBytes: c.MaxBytes, ttl: time.Duration(c.TTLSeconds) * time.Second,
			cleanupInterval:  time.Duration(c.CleanupIntervalSeconds) * time.Second,
			maxDownloadBytes: c.MaxDownloadBytes, maxUncompressed: c.MaxUncompressedBytes,
			maxFileBytes: c.MaxFileBytes, maxFiles: c.MaxFileCount,
			maxManifestBytes: c.MaxManifestBytes, maxCompressionRatio: c.MaxCompressionRatio,
		},
		now:    func() time.Time { return time.Now().UTC() },
		active: make(map[string]int), started: make(chan struct{}), stop: make(chan struct{}), done: make(chan struct{}),
	}
	return cache, nil
}

func (cache *Cache) Start() {
	if cache == nil {
		return
	}
	cache.startOnce.Do(func() {
		close(cache.started)
		go func() {
			defer close(cache.done)
			ticker := time.NewTicker(cache.limits.cleanupInterval)
			defer ticker.Stop()
			for {
				select {
				case <-cache.stop:
					return
				case <-ticker.C:
					_ = cache.Cleanup(context.Background())
				}
			}
		}()
	})
}

func (cache *Cache) Close() {
	if cache == nil {
		return
	}
	cache.stopOnce.Do(func() { close(cache.stop) })
	select {
	case <-cache.started:
		<-cache.done
	default:
	}
}

// Prepare verifies that the RPC returned the exact requested revision, then
// obtains and validates the package identified by spec.TestDataChecksum.
func (cache *Cache) Prepare(ctx context.Context, requestedRevision int64, spec *problempb.JudgeProblemSpec) (*Package, error) {
	if cache == nil {
		return nil, errors.New("test data cache is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	checksum, err := validateSpec(requestedRevision, spec)
	if err != nil {
		return nil, err
	}

	cache.opMu.RLock()
	defer cache.opMu.RUnlock()
	data, err := cache.flights.Do(ctx, checksum, func() (*cachedPackage, error) {
		hit, hitErr := cache.loadCached(checksum)
		if hitErr != nil {
			hit, hitErr = cache.downloadAndInstall(ctx, spec, checksum)
		}
		if hitErr != nil {
			return nil, hitErr
		}
		if err := compareManifestToSpec(hit.manifest, spec); err != nil {
			return nil, err
		}
		cache.leaseMu.Lock()
		cache.active[checksum]++
		cache.leaseMu.Unlock()
		return hit, nil
	})
	if err != nil {
		return nil, err
	}
	return &Package{Dir: data.dir, Manifest: data.manifest, checksum: checksum, cache: cache}, nil
}

type cachedPackage struct {
	dir      string
	manifest *testdatamanifest.Manifest
}

func (cache *Cache) loadCached(checksum string) (*cachedPackage, error) {
	dir := cache.cachePath(checksum)
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("cache entry is not a directory")
	}
	if cache.now().Sub(info.ModTime()) > cache.limits.ttl && !cache.isActive(checksum) {
		return nil, errors.New("cache entry expired")
	}
	manifest, err := verifyExtractedDirectory(dir, cache.limits)
	if err != nil {
		return nil, fmt.Errorf("verify cached test data: %w", err)
	}
	now := cache.now()
	_ = os.Chtimes(dir, now, now)
	return &cachedPackage{dir: dir, manifest: manifest}, nil
}

func (cache *Cache) downloadAndInstall(ctx context.Context, spec *problempb.JudgeProblemSpec, checksum string) (_ *cachedPackage, resultErr error) {
	if err := cache.removeInvalidEntry(checksum); err != nil {
		return nil, err
	}
	archive, err := os.CreateTemp(cache.root, ".download-"+checksum[:12]+"-*.zip")
	if err != nil {
		return nil, fmt.Errorf("create temporary test data archive: %w", err)
	}
	archivePath := archive.Name()
	defer func() {
		_ = archive.Close()
		_ = os.Remove(archivePath)
	}()

	object, err := cache.store.Open(ctx, spec.TestDataPackageKey)
	if err != nil {
		return nil, err
	}
	if object == nil || object.Body == nil {
		return nil, errors.New("object storage returned an invalid response")
	}
	defer object.Body.Close()
	if object.ContentLength > cache.limits.maxDownloadBytes {
		return nil, errors.New("test data object exceeds the download size limit")
	}
	digest := sha256.New()
	written, copyErr := io.CopyBuffer(io.MultiWriter(archive, digest), io.LimitReader(object.Body, cache.limits.maxDownloadBytes+1), make([]byte, 64<<10))
	bodyCloseErr := object.Body.Close()
	fileCloseErr := archive.Close()
	if copyErr != nil || bodyCloseErr != nil || fileCloseErr != nil {
		return nil, fmt.Errorf("download test data object: %w", errors.Join(copyErr, bodyCloseErr, fileCloseErr))
	}
	if written > cache.limits.maxDownloadBytes || (object.ContentLength >= 0 && written != object.ContentLength) {
		return nil, errors.New("test data object size is invalid")
	}
	if actual := hex.EncodeToString(digest.Sum(nil)); actual != checksum {
		return nil, errors.New("test data object checksum does not match JudgeProblemSpec")
	}

	stage, err := os.MkdirTemp(cache.root, ".extract-"+checksum[:12]+"-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary extraction directory: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = os.RemoveAll(stage)
		}
	}()
	if err := os.Chmod(stage, 0o700); err != nil {
		return nil, fmt.Errorf("secure temporary extraction directory: %w", err)
	}
	manifest, err := extractArchive(archivePath, stage, cache.limits)
	if err != nil {
		return nil, err
	}
	if _, err := verifyExtractedDirectory(stage, cache.limits); err != nil {
		return nil, fmt.Errorf("verify extracted test data: %w", err)
	}
	if err := syncDirectory(stage); err != nil {
		return nil, err
	}
	finalPath := cache.cachePath(checksum)
	if err := os.Rename(stage, finalPath); err != nil {
		return nil, fmt.Errorf("publish test data cache entry atomically: %w", err)
	}
	if err := syncDirectory(cache.root); err != nil {
		return nil, err
	}
	return &cachedPackage{dir: finalPath, manifest: manifest}, nil
}

func (cache *Cache) removeInvalidEntry(checksum string) error {
	if cache.isActive(checksum) {
		return errors.New("cannot replace test data while a package lease is active")
	}
	path := cache.cachePath(checksum)
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect invalid cache entry: %w", err)
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove invalid cache entry: %w", err)
	}
	return nil
}

func (cache *Cache) cachePath(checksum string) string {
	return filepath.Join(cache.root, checksum)
}

func (cache *Cache) release(checksum string) {
	cache.leaseMu.Lock()
	defer cache.leaseMu.Unlock()
	if cache.active[checksum] <= 1 {
		delete(cache.active, checksum)
		return
	}
	cache.active[checksum]--
}

func validateSpec(requestedRevision int64, spec *problempb.JudgeProblemSpec) (string, error) {
	if spec == nil || spec.ProblemId <= 0 || spec.Revision <= 0 {
		return "", errors.New("JudgeProblemSpec problem_id and revision must be positive")
	}
	if requestedRevision <= 0 || spec.Revision != requestedRevision {
		return "", errors.New("JudgeProblemSpec revision does not match the requested revision")
	}
	if _, err := testdatamanifest.SafePath(spec.TestDataPackageKey); err != nil {
		return "", errors.New("JudgeProblemSpec test_data_package_key is unsafe")
	}
	checksum, err := testdatamanifest.NormalizeSHA256(spec.TestDataChecksum)
	if err != nil {
		return "", errors.New("JudgeProblemSpec test_data_checksum is invalid")
	}
	if len(spec.TestCases) == 0 || len(spec.TestCases) > testdatamanifest.MaxTestCases {
		return "", errors.New("JudgeProblemSpec contains an invalid number of test cases")
	}
	return checksum, nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory for sync: %w", err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil || closeErr != nil {
		return fmt.Errorf("sync directory: %w", errors.Join(syncErr, closeErr))
	}
	return nil
}

func (cache *Cache) Cleanup(ctx context.Context) error {
	if cache == nil {
		return nil
	}
	cache.opMu.Lock()
	defer cache.opMu.Unlock()
	entries, err := os.ReadDir(cache.root)
	if err != nil {
		return fmt.Errorf("scan test data cache: %w", err)
	}
	now := cache.now()
	var candidates []cleanupCandidate
	var total int64
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := entry.Name()
		path := filepath.Join(cache.root, name)
		if strings.HasPrefix(name, ".download-") || strings.HasPrefix(name, ".extract-") {
			if info, infoErr := entry.Info(); infoErr == nil && now.Sub(info.ModTime()) > cache.limits.ttl {
				_ = os.RemoveAll(path)
			}
			continue
		}
		checksum, checksumErr := testdatamanifest.NormalizeSHA256(name)
		if checksumErr != nil || checksum != name || !entry.IsDir() || cache.isActive(name) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		size, sizeErr := directorySize(path)
		if sizeErr != nil {
			_ = os.RemoveAll(path)
			continue
		}
		if now.Sub(info.ModTime()) > cache.limits.ttl {
			_ = os.RemoveAll(path)
			continue
		}
		total += size
		candidates = append(candidates, cleanupCandidate{path: path, size: size, lastUsed: info.ModTime()})
	}
	if total <= cache.limits.maxCacheBytes {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].lastUsed.Before(candidates[j].lastUsed) })
	for _, candidate := range candidates {
		if total <= cache.limits.maxCacheBytes {
			break
		}
		if err := os.RemoveAll(candidate.path); err != nil {
			return fmt.Errorf("evict test data cache entry: %w", err)
		}
		total -= candidate.size
	}
	return nil
}

type cleanupCandidate struct {
	path     string
	size     int64
	lastUsed time.Time
}

func (cache *Cache) isActive(checksum string) bool {
	cache.leaseMu.Lock()
	defer cache.leaseMu.Unlock()
	return cache.active[checksum] > 0
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.Mode().IsRegular() && !info.IsDir()) {
			return errors.New("cache contains a special file")
		}
		if info.Mode().IsRegular() {
			if info.Size() > int64(^uint64(0)>>1)-total {
				return errors.New("cache size overflow")
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

type operationGroup struct {
	mu    sync.Mutex
	calls map[string]*operationCall
}

type operationCall struct {
	done chan struct{}
}

func (group *operationGroup) Do(ctx context.Context, key string, function func() (*cachedPackage, error)) (*cachedPackage, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		group.mu.Lock()
		if group.calls == nil {
			group.calls = make(map[string]*operationCall)
		}
		if call, exists := group.calls[key]; exists {
			group.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-call.done:
				continue
			}
		}
		call := &operationCall{done: make(chan struct{})}
		group.calls[key] = call
		group.mu.Unlock()

		data, err := function()
		group.mu.Lock()
		delete(group.calls, key)
		close(call.done)
		group.mu.Unlock()
		return data, err
	}
}
