package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecureMountSourceEnforcesAllowedRootsAndWritePolicy(t *testing.T) {
	root := t.TempDir()
	readOnlyRoot := filepath.Join(root, "readonly")
	writableRoot := filepath.Join(root, "writable")
	outsideRoot := filepath.Join(root, "outside")
	for _, path := range []string{readOnlyRoot, writableRoot, outsideRoot} {
		if err := os.MkdirAll(path, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	roots, err := resolveAllowedRoots([]AllowedRoot{{Path: readOnlyRoot}, {Path: writableRoot, Writable: true}})
	if err != nil {
		t.Fatalf("resolveAllowedRoots() error = %v", err)
	}
	readOnlyFile := filepath.Join(readOnlyRoot, "case.in")
	if err := os.WriteFile(readOnlyFile, []byte("input"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := secureMountSource(readOnlyFile, true, roots); err != nil {
		t.Fatalf("read-only mount error = %v", err)
	}
	if _, err := secureMountSource(readOnlyFile, false, roots); err == nil || !strings.Contains(err.Error(), "read-only allowed root") {
		t.Fatalf("writable mount error = %v", err)
	}
	if _, err := secureMountSource(writableRoot, false, roots); err != nil {
		t.Fatalf("writable mount error = %v", err)
	}
	if _, err := secureMountSource(outsideRoot, true, roots); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("outside mount error = %v", err)
	}
}

func TestSecureMountSourceResolvesSymlinksBeforeAuthorization(t *testing.T) {
	root := t.TempDir()
	allowed := filepath.Join(root, "allowed")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(allowed, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(allowed, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	roots, err := resolveAllowedRoots([]AllowedRoot{{Path: allowed}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := secureMountSource(link, true, roots); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("symlink escape error = %v", err)
	}
}

func TestResolveAllowedRootsRejectsRootAndOverlap(t *testing.T) {
	if _, err := resolveAllowedRoots([]AllowedRoot{{Path: string(filepath.Separator)}}); err == nil {
		t.Fatal("host root was accepted")
	}
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveAllowedRoots([]AllowedRoot{{Path: root}, {Path: child, Writable: true}}); err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("overlapping roots error = %v", err)
	}
}
