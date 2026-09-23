package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AllowedRoot struct {
	Path     string
	Writable bool
}

type allowedRoot struct {
	path     string
	writable bool
}

func resolveAllowedRoots(roots []AllowedRoot) ([]allowedRoot, error) {
	if len(roots) == 0 {
		return nil, errors.New("sandbox requires at least one allowed mount root")
	}
	result := make([]allowedRoot, 0, len(roots))
	for _, root := range roots {
		resolved, err := secureExistingPath(root.Path)
		if err != nil {
			return nil, fmt.Errorf("resolve allowed mount root %q: %w", root.Path, err)
		}
		if resolved == string(filepath.Separator) {
			return nil, errors.New("sandbox must not allow the host filesystem root")
		}
		for _, existing := range result {
			overlaps, err := rootsOverlap(existing.path, resolved)
			if err != nil {
				return nil, err
			}
			if overlaps {
				return nil, fmt.Errorf("sandbox allowed mount roots %q and %q overlap", existing.path, resolved)
			}
		}
		result = append(result, allowedRoot{path: resolved, writable: root.Writable})
	}
	return result, nil
}

func rootsOverlap(first, second string) (bool, error) {
	inside, err := pathWithin(first, second)
	if err != nil || inside {
		return inside, err
	}
	return pathWithin(second, first)
}

func secureMountSource(source string, readOnly bool, roots []allowedRoot) (string, error) {
	resolved, err := secureExistingPath(source)
	if err != nil {
		return "", err
	}
	for _, root := range roots {
		inside, err := pathWithin(root.path, resolved)
		if err != nil {
			return "", err
		}
		if inside {
			if !readOnly && !root.writable {
				return "", fmt.Errorf("mount source %q is below a read-only allowed root", source)
			}
			return resolved, nil
		}
	}
	return "", fmt.Errorf("mount source %q is outside configured sandbox roots", source)
}

func secureExistingPath(value string) (string, error) {
	if strings.TrimSpace(value) == "" || strings.ContainsRune(value, '\x00') {
		return "", errors.New("path is empty or contains NUL")
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return "", errors.New("path must be a regular file or directory")
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func pathWithin(root, candidate string) (bool, error) {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false, err
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))), nil
}
