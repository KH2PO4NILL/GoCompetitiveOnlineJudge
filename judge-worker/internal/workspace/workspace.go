package workspace

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"golang.org/x/sys/unix"
)

var attemptName = regexp.MustCompile(`^attempt-[1-9][0-9]*-[0-9]+$`)

type Workspace struct {
	Dir  string
	lock *os.File
	mu   sync.Mutex
}

func Open(root, workerID string) (_ *Workspace, resultErr error) {
	if workerID == "" || root == "" {
		return nil, errors.New("workspace root and worker ID are required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("worker-%x", sha256.Sum256([]byte(workerID)))
	lockPath := filepath.Join(root, name+".lock")
	descriptor, err := unix.Open(lockPath, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if err != nil {
		return nil, fmt.Errorf("open worker workspace lock: %w", err)
	}
	lock := os.NewFile(uintptr(descriptor), lockPath)
	defer func() {
		if resultErr != nil {
			_ = lock.Close()
		}
	}()
	info, err := lock.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("workspace lock must be a regular file")
	}
	if err := unix.Flock(descriptor, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return nil, fmt.Errorf("worker workspace is already owned by a live process: %w", err)
	}
	directory := filepath.Join(root, name)
	if err := os.Mkdir(directory, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	info, err = os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("worker workspace must be a real directory")
	}
	return &Workspace{Dir: directory, lock: lock}, nil
}

func (workspace *Workspace) Cleanup(ctx context.Context) error {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if workspace.lock == nil {
		return errors.New("worker workspace is closed")
	}
	entries, err := os.ReadDir(workspace.Dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !attemptName.MatchString(entry.Name()) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(workspace.Dir, entry.Name())); err != nil {
			return fmt.Errorf("remove orphan attempt directory: %w", err)
		}
	}
	return nil
}

func (workspace *Workspace) Close() error {
	if workspace == nil {
		return nil
	}
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if workspace.lock == nil {
		return nil
	}
	err := workspace.lock.Close()
	workspace.lock = nil
	return err
}
