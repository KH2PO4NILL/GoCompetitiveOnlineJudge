package workspace

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkspaceCrashProcess(t *testing.T) {
	root := os.Getenv("JUDGE_WORKSPACE_CRASH_TEST")
	if root == "" {
		return
	}
	work, err := Open(root, "crashed-worker")
	if err != nil {
		t.Fatal(err)
	}
	defer work.Close()
	artifact := filepath.Join(work.Dir, "attempt-1-123", "artifact")
	if err := os.MkdirAll(artifact, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifact, "main"), []byte("orphan"), 0600); err != nil {
		t.Fatal(err)
	}
	fmt.Println("ready")
	for {
		time.Sleep(time.Hour)
	}
}

func TestCrashReleasesOwnershipAndCleanupPreservesOtherWorkers(t *testing.T) {
	root := t.TempDir()
	peer, err := Open(root, "live-worker")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	live := filepath.Join(peer.Dir, "attempt-2-456")
	if err := os.Mkdir(live, 0700); err != nil {
		t.Fatal(err)
	}
	unowned := filepath.Join(root, "attempt-3-789")
	if err := os.Mkdir(unowned, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWorkspaceCrashProcess$")
	command.Env = append(os.Environ(), "JUDGE_WORKSPACE_CRASH_TEST="+root)
	pipe, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	ready, err := bufio.NewReader(pipe).ReadString('\n')
	if err != nil || ready != "ready\n" {
		t.Fatalf("child startup: %q %v", ready, err)
	}
	if duplicate, err := Open(root, "crashed-worker"); err == nil {
		_ = duplicate.Close()
		t.Fatal("allowed a live worker's workspace to be reclaimed")
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()
	restarted, err := Open(root, "crashed-worker")
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	orphan := filepath.Join(restarted.Dir, "attempt-1-123")
	if _, err := os.Stat(orphan); err != nil {
		t.Fatal("crash did not leave artifact", err)
	}
	if err := os.Symlink(live, filepath.Join(restarted.Dir, "attempt-4-123")); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("orphan remains: %v", err)
	}
	for _, path := range []string{live, unowned} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("removed unowned data %s: %v", path, err)
		}
	}
}

func TestWorkspaceRejectsSymlinkAndRetainsLockInode(t *testing.T) {
	root := t.TempDir()
	work, err := Open(root, "worker")
	if err != nil {
		t.Fatal(err)
	}
	path := work.Dir
	if err := work.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), path); err != nil {
		t.Fatal(err)
	}
	if work, err := Open(root, "worker"); err == nil {
		_ = work.Close()
		t.Fatal("accepted workspace symlink")
	}
	if _, err := os.Stat(path + ".lock"); err != nil {
		t.Fatal("lock inode was removed", err)
	}
}
