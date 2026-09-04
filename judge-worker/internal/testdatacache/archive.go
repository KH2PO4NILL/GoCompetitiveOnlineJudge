package testdatacache

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/testdatamanifest"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
)

func extractArchive(archivePath, destination string, limits limits) (*testdatamanifest.Manifest, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, errors.New("test data object is not a valid ZIP archive")
	}
	defer archive.Close()
	if len(archive.File) == 0 || len(archive.File) > limits.maxFiles {
		return nil, errors.New("ZIP entry count is outside the allowed range")
	}

	entries := make(map[string]*zip.File, len(archive.File))
	var declaredUncompressed uint64
	var declaredCompressed uint64
	for _, entry := range archive.File {
		entryName, err := safeEntryPath(entry)
		if err != nil {
			return nil, err
		}
		if _, exists := entries[entryName]; exists {
			return nil, fmt.Errorf("ZIP contains duplicate path %q", entryName)
		}
		mode := entry.Mode()
		if mode&os.ModeSymlink != 0 || (!mode.IsRegular() && !mode.IsDir()) {
			return nil, fmt.Errorf("ZIP entry %q is a link or special file", entryName)
		}
		if !mode.IsDir() && entry.UncompressedSize64 > uint64(limits.maxFileBytes) {
			return nil, fmt.Errorf("ZIP entry %q exceeds the single-file size limit", entryName)
		}
		if entry.UncompressedSize64 > uint64(limits.maxUncompressed)-declaredUncompressed {
			return nil, errors.New("ZIP exceeds the total uncompressed-size limit")
		}
		if !mode.IsDir() && ratioExceeded(entry.UncompressedSize64, entry.CompressedSize64, limits.maxCompressionRatio) {
			return nil, fmt.Errorf("ZIP entry %q exceeds the compression-ratio limit", entryName)
		}
		declaredUncompressed += entry.UncompressedSize64
		declaredCompressed += entry.CompressedSize64
		entries[entryName] = entry
	}
	if ratioExceeded(declaredUncompressed, declaredCompressed, limits.maxCompressionRatio) {
		return nil, errors.New("ZIP exceeds the aggregate compression-ratio limit")
	}
	if err := rejectPathTypeConflicts(entries); err != nil {
		return nil, err
	}

	manifestEntry, exists := entries[testdatamanifest.FileName]
	if !exists || manifestEntry.FileInfo().IsDir() || manifestEntry.UncompressedSize64 > uint64(limits.maxManifestBytes) {
		return nil, errors.New("ZIP requires a bounded regular manifest.json at its root")
	}
	reader, err := manifestEntry.Open()
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	manifest, parseErr := testdatamanifest.Parse(reader, limits.maxManifestBytes)
	closeErr := reader.Close()
	if parseErr != nil || closeErr != nil {
		return nil, fmt.Errorf("parse manifest: %w", errors.Join(parseErr, closeErr))
	}
	declaredFiles := manifest.FileMap()
	allowedDirectories := manifest.PayloadDirectories()
	for entryName, entry := range entries {
		if entry.FileInfo().IsDir() {
			if _, allowed := allowedDirectories[entryName]; !allowed {
				return nil, fmt.Errorf("ZIP contains undeclared directory %q", entryName)
			}
			continue
		}
		if entryName == testdatamanifest.FileName {
			continue
		}
		declared, declaredExists := declaredFiles[entryName]
		if !declaredExists {
			return nil, fmt.Errorf("ZIP contains undeclared payload file %q", entryName)
		}
		if entry.UncompressedSize64 != uint64(declared.Size) {
			return nil, fmt.Errorf("ZIP entry %q size does not match manifest", entryName)
		}
	}
	for _, declared := range manifest.Files {
		entry, exists := entries[declared.Path]
		if !exists || entry.FileInfo().IsDir() {
			return nil, fmt.Errorf("manifest payload file %q is missing from ZIP", declared.Path)
		}
	}

	remaining := limits.maxUncompressed
	for _, entry := range archive.File {
		entryName, _ := safeEntryPath(entry)
		if entry.FileInfo().IsDir() {
			continue
		}
		destinationPath, err := secureDestination(destination, entryName)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(destinationPath), 0o700); err != nil {
			return nil, fmt.Errorf("create extraction directory: %w", err)
		}
		var declared *testdatamanifest.File
		limit := limits.maxManifestBytes
		if entryName != testdatamanifest.FileName {
			item := declaredFiles[entryName]
			declared = &item
			limit = limits.maxFileBytes
		}
		written, err := extractRegularFile(entry, destinationPath, limit, remaining, declared)
		if err != nil {
			return nil, err
		}
		remaining -= written
	}
	return manifest, nil
}

func safeEntryPath(entry *zip.File) (string, error) {
	if entry == nil {
		return "", errors.New("ZIP contains an invalid entry")
	}
	name := entry.Name
	if entry.FileInfo().IsDir() {
		if !strings.HasSuffix(name, "/") {
			return "", fmt.Errorf("ZIP directory %q does not use a canonical path", name)
		}
		name = strings.TrimSuffix(name, "/")
	}
	cleaned, err := testdatamanifest.SafePath(name)
	if err != nil {
		return "", fmt.Errorf("ZIP contains unsafe path %q", entry.Name)
	}
	return cleaned, nil
}

func ratioExceeded(uncompressed, compressed uint64, limit int64) bool {
	if uncompressed == 0 {
		return false
	}
	if compressed == 0 || limit <= 0 {
		return true
	}
	quotient := uncompressed / compressed
	return quotient > uint64(limit) || (quotient == uint64(limit) && uncompressed%compressed != 0)
}

func rejectPathTypeConflicts(entries map[string]*zip.File) error {
	for entryName := range entries {
		for parent := path.Dir(entryName); parent != "."; parent = path.Dir(parent) {
			if existing, exists := entries[parent]; exists && !existing.FileInfo().IsDir() {
				return fmt.Errorf("ZIP path %q has regular-file parent %q", entryName, parent)
			}
		}
	}
	return nil
}

func secureDestination(root, relative string) (string, error) {
	joined := filepath.Join(root, filepath.FromSlash(relative))
	rel, err := filepath.Rel(root, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("ZIP path %q escapes extraction root", relative)
	}
	return joined, nil
}

func extractRegularFile(entry *zip.File, destination string, perFileLimit, remaining int64, declared *testdatamanifest.File) (int64, error) {
	if remaining < 0 {
		return 0, errors.New("ZIP exceeds the total uncompressed-size limit")
	}
	limit := min(perFileLimit, remaining)
	if declared != nil && declared.Size < limit {
		limit = declared.Size
	}
	input, err := entry.Open()
	if err != nil {
		return 0, fmt.Errorf("open ZIP entry %q: %w", entry.Name, err)
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		input.Close()
		return 0, fmt.Errorf("create extracted file %q: %w", entry.Name, err)
	}
	digest := sha256.New()
	written, copyErr := io.CopyBuffer(io.MultiWriter(output, digest), io.LimitReader(input, limit+1), make([]byte, 64<<10))
	syncErr := output.Sync()
	outputCloseErr := output.Close()
	inputCloseErr := input.Close()
	if copyErr != nil || syncErr != nil || outputCloseErr != nil || inputCloseErr != nil {
		return 0, fmt.Errorf("extract ZIP entry %q: %w", entry.Name, errors.Join(copyErr, syncErr, outputCloseErr, inputCloseErr))
	}
	if written > limit || written != int64(entry.UncompressedSize64) {
		return 0, fmt.Errorf("ZIP entry %q exceeds its size limit or has a false declared size", entry.Name)
	}
	if declared != nil {
		if written != declared.Size {
			return 0, fmt.Errorf("ZIP entry %q size does not match manifest", entry.Name)
		}
		if hex.EncodeToString(digest.Sum(nil)) != declared.SHA256 {
			return 0, fmt.Errorf("ZIP entry %q checksum does not match manifest", entry.Name)
		}
	}
	return written, nil
}

func verifyExtractedDirectory(root string, limits limits) (*testdatamanifest.Manifest, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("extraction root is not a regular directory")
	}
	manifestPath := filepath.Join(root, testdatamanifest.FileName)
	manifestInfo, err := os.Lstat(manifestPath)
	if err != nil || !manifestInfo.Mode().IsRegular() || manifestInfo.Mode()&os.ModeSymlink != 0 || manifestInfo.Size() > limits.maxManifestBytes {
		return nil, errors.New("extracted manifest is not a bounded regular file")
	}
	manifestFile, err := os.Open(manifestPath)
	if err != nil {
		return nil, err
	}
	manifest, parseErr := testdatamanifest.Parse(manifestFile, limits.maxManifestBytes)
	closeErr := manifestFile.Close()
	if parseErr != nil || closeErr != nil {
		return nil, errors.Join(parseErr, closeErr)
	}
	files := manifest.FileMap()
	allowedDirectories := manifest.PayloadDirectories()
	seen := make(map[string]struct{}, len(files))
	fileCount := 0
	total := manifestInfo.Size()
	if total > limits.maxUncompressed {
		return nil, errors.New("extracted files exceed the total uncompressed-size limit")
	}
	err = filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root {
			return nil
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.Mode().IsRegular() && !info.IsDir()) {
			return fmt.Errorf("extracted path %q is a link or special file", relative)
		}
		if info.IsDir() {
			if _, allowed := allowedDirectories[relative]; !allowed {
				return fmt.Errorf("extracted directory %q is undeclared", relative)
			}
			return nil
		}
		if relative == testdatamanifest.FileName {
			return nil
		}
		fileCount++
		if fileCount+1 > limits.maxFiles {
			return errors.New("extracted file count exceeds its limit")
		}
		declared, exists := files[relative]
		if !exists {
			return fmt.Errorf("extracted payload file %q is undeclared", relative)
		}
		if info.Size() != declared.Size || info.Size() > limits.maxFileBytes || info.Size() > limits.maxUncompressed-total {
			return fmt.Errorf("extracted payload file %q has an invalid size", relative)
		}
		file, err := os.Open(current)
		if err != nil {
			return err
		}
		digest := sha256.New()
		written, copyErr := io.Copy(digest, io.LimitReader(file, limits.maxFileBytes+1))
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			return errors.Join(copyErr, closeErr)
		}
		if written != declared.Size || hex.EncodeToString(digest.Sum(nil)) != declared.SHA256 {
			return fmt.Errorf("extracted payload file %q does not match manifest", relative)
		}
		total += written
		seen[relative] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(seen) != len(files) {
		return nil, errors.New("extracted payload file list does not match manifest")
	}
	return manifest, nil
}

func compareManifestToSpec(manifest *testdatamanifest.Manifest, spec *problempb.JudgeProblemSpec) error {
	if manifest == nil || spec == nil || len(manifest.TestCases) != len(spec.TestCases) {
		return errors.New("manifest test cases do not match JudgeProblemSpec")
	}
	byOrdinal := make(map[int32]*problempb.TestCaseInfo, len(spec.TestCases))
	for _, item := range spec.TestCases {
		if item == nil || item.Ordinal < 0 {
			return errors.New("JudgeProblemSpec contains an invalid test case")
		}
		if _, exists := byOrdinal[item.Ordinal]; exists {
			return errors.New("JudgeProblemSpec contains duplicate test case ordinals")
		}
		byOrdinal[item.Ordinal] = item
	}
	for _, item := range manifest.TestCases {
		expected, exists := byOrdinal[int32(item.Ordinal)]
		if !exists || expected.Name != item.Name || int64(expected.Score) != item.Score ||
			expected.Enabled != item.IsEnabled() || expected.GroupName != item.GroupName {
			return fmt.Errorf("manifest test case ordinal %d does not match JudgeProblemSpec", item.Ordinal)
		}
	}
	return nil
}
