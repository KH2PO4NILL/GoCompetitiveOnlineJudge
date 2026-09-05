package testdatacache

import (
	"context"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestActiveLeaseSurvivesTTLReplacement(t *testing.T) {
	archive := buildArchive(t, validEntries(), nil)
	store := &fakeStore{content: archive}
	cache := newTestCache(t, store, func(*config.TestDataCacheConf) {})
	spec := specForArchive(archive)
	first, err := cache.Prepare(context.Background(), 11, spec)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	root, err := os.OpenRoot(first.Dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	cache.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	second, err := cache.Prepare(context.Background(), 11, spec)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release()
	input, err := root.Open("cases/1.in")
	if err != nil {
		t.Fatalf("active package was deleted by another Prepare: %v", err)
	}
	input.Close()
}

func TestCorruptActiveEntryCannotBeReinstalled(t *testing.T) {
	archive := buildArchive(t, validEntries(), nil)
	cache := newTestCache(t, &fakeStore{content: archive}, func(*config.TestDataCacheConf) {})
	spec := specForArchive(archive)
	pkg, err := cache.Prepare(context.Background(), 11, spec)
	if err != nil {
		t.Fatal(err)
	}
	defer pkg.Release()
	if err := os.WriteFile(filepath.Join(pkg.Dir, "cases/1.out"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if replacement, err := cache.Prepare(context.Background(), 11, spec); err == nil {
		replacement.Release()
		t.Fatal("replaced a leased corrupt entry")
	}
	if _, err := os.ReadFile(filepath.Join(pkg.Dir, "cases/1.in")); err != nil {
		t.Fatal("deleted the active input", err)
	}
	pkg.Release()
	repaired, err := cache.Prepare(context.Background(), 11, spec)
	if err != nil {
		t.Fatal(err)
	}
	repaired.Release()
}

func TestConcurrentExpiringPreparesKeepLeaseAcquisitionAtomic(t *testing.T) {
	archive := buildArchive(t, validEntries(), nil)
	cache := newTestCache(t, &fakeStore{content: archive}, func(*config.TestDataCacheConf) {})
	spec := specForArchive(archive)
	var clock atomic.Int64
	base := time.Now()
	cache.now = func() time.Time { return base.Add(time.Duration(clock.Add(7200)) * time.Second) }
	var group sync.WaitGroup
	for range 12 {
		group.Go(func() {
			for range 10 {
				pkg, err := cache.Prepare(context.Background(), 11, spec)
				if err != nil {
					t.Error(err)
					return
				}
				root, err := os.OpenRoot(pkg.Dir)
				if err == nil {
					time.Sleep(time.Millisecond)
					file, openErr := root.Open("cases/1.in")
					if openErr != nil {
						t.Error("active input disappeared", openErr)
					} else {
						_ = file.Close()
					}
					_ = root.Close()
				} else {
					t.Error(err)
				}
				pkg.Release()
			}
		})
	}
	group.Wait()
}
