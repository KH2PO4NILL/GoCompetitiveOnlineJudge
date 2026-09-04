package svc

import (
	"archive/zip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/testdatamanifest"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"
)

const (
	awsSignatureAlgorithm = "AWS4-HMAC-SHA256"
	awsServiceName        = "s3"
	emptyPayloadSHA256    = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	unsignedPayload       = "UNSIGNED-PAYLOAD"
)

type s3TestDataStorage struct {
	endpoint     *url.URL
	region       string
	bucket       string
	accessKey    string
	secretKey    string
	usePathStyle bool
	client       *http.Client
	policy       UploadPolicy
	now          func() time.Time
}

func NewS3TestDataStorage(c config.ObjectStorageConf, policy UploadPolicy) (TestDataStorage, error) {
	endpoint, err := url.Parse(strings.TrimSpace(c.Endpoint))
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" ||
		(endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.User != nil {
		return nil, errors.New("object storage endpoint must be an http(s) URL without credentials, query or fragment")
	}
	endpoint.Path = strings.TrimSuffix(endpoint.Path, "/")
	region := strings.TrimSpace(c.Region)
	if region == "" {
		region = "us-east-1"
	}
	bucket := strings.TrimSpace(c.Bucket)
	if !validBucketName(bucket) {
		return nil, errors.New("object storage bucket name is invalid")
	}
	if strings.TrimSpace(c.AccessKey) == "" || c.SecretKey == "" {
		return nil, errors.New("object storage access key and secret key are required")
	}
	timeout := time.Duration(c.RequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if timeout > 5*time.Minute {
		return nil, errors.New("object storage request timeout must not exceed 5 minutes")
	}
	return &s3TestDataStorage{
		endpoint: endpoint, region: region, bucket: bucket,
		accessKey: c.AccessKey, secretKey: c.SecretKey, usePathStyle: c.UsePathStyle,
		client: &http.Client{Timeout: timeout}, policy: policy.withDefaults(),
		now: func() time.Time { return time.Now().UTC() },
	}, nil
}

func (s *s3TestDataStorage) CreateUploadURL(_ context.Context, upload *model.TestDataUpload) (string, error) {
	if upload == nil || upload.ObjectKey == "" || upload.ExpiresAt.IsZero() {
		return "", errors.New("invalid upload session")
	}
	now := s.now().UTC()
	expires := upload.ExpiresAt.Sub(now)
	if expires > s.policy.URLTTL {
		expires = s.policy.URLTTL
	}
	seconds := int64(expires / time.Second)
	if seconds <= 0 {
		return "", errors.New("upload session is expired")
	}
	objectURL := s.objectURL(upload.ObjectKey)
	date := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	scope := date + "/" + s.region + "/" + awsServiceName + "/aws4_request"
	query := objectURL.Query()
	query.Set("X-Amz-Algorithm", awsSignatureAlgorithm)
	query.Set("X-Amz-Credential", s.accessKey+"/"+scope)
	query.Set("X-Amz-Date", amzDate)
	query.Set("X-Amz-Expires", strconv.FormatInt(seconds, 10))
	query.Set("X-Amz-SignedHeaders", "host")
	objectURL.RawQuery = canonicalQuery(query)
	canonicalRequest := strings.Join([]string{
		http.MethodPut,
		canonicalURI(objectURL),
		objectURL.RawQuery,
		"host:" + strings.ToLower(objectURL.Host) + "\n",
		"host",
		unsignedPayload,
	}, "\n")
	stringToSign := awsStringToSign(amzDate, scope, canonicalRequest)
	signature := hex.EncodeToString(s.signingHMAC(date, stringToSign))
	query.Set("X-Amz-Signature", signature)
	objectURL.RawQuery = canonicalQuery(query)
	return objectURL.String(), nil
}

func (s *s3TestDataStorage) InspectUpload(ctx context.Context, upload *model.TestDataUpload) (*TestDataObject, error) {
	if upload == nil || upload.ObjectKey == "" || upload.ContentLength <= 0 || upload.ContentLength > s.policy.MaxUploadBytes {
		return nil, fmt.Errorf("%w: invalid upload session", ErrTestDataObjectInvalid)
	}
	temporary, err := os.CreateTemp("", "problem-test-data-*.zip")
	if err != nil {
		return nil, fmt.Errorf("create temporary test data file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.objectURL(upload.ObjectKey).String(), nil)
	if err != nil {
		_ = temporary.Close()
		return nil, fmt.Errorf("create object request: %w", err)
	}
	s.signRequest(request, emptyPayloadSHA256, s.now().UTC())
	response, err := s.client.Do(request)
	if err != nil {
		_ = temporary.Close()
		return nil, fmt.Errorf("download test data object: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		_ = temporary.Close()
		return nil, ErrTestDataObjectNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		_ = temporary.Close()
		return nil, fmt.Errorf("object storage returned status %d", response.StatusCode)
	}
	if response.ContentLength > s.policy.MaxUploadBytes ||
		(response.ContentLength >= 0 && response.ContentLength != upload.ContentLength) {
		_ = temporary.Close()
		return nil, fmt.Errorf("%w: uploaded object size does not match", ErrTestDataObjectInvalid)
	}

	digest := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temporary, digest), io.LimitReader(response.Body, s.policy.MaxUploadBytes+1))
	closeErr := temporary.Close()
	if copyErr != nil || closeErr != nil {
		return nil, fmt.Errorf("store downloaded test data: %w", errors.Join(copyErr, closeErr))
	}
	if written != upload.ContentLength || written > s.policy.MaxUploadBytes {
		return nil, fmt.Errorf("%w: uploaded object size does not match", ErrTestDataObjectInvalid)
	}
	checksum := hex.EncodeToString(digest.Sum(nil))
	if !strings.EqualFold(checksum, strings.TrimSpace(upload.SHA256)) {
		return nil, fmt.Errorf("%w: uploaded object checksum does not match", ErrTestDataObjectInvalid)
	}
	testCases, err := inspectTestDataArchive(temporaryPath, s.policy)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTestDataObjectInvalid, err)
	}
	packageKey := fmt.Sprintf("problem-test-data/%d/versions/%s.zip", upload.ProblemID, upload.UploadID)
	if err := s.copyObject(ctx, upload.ObjectKey, packageKey); err != nil {
		return nil, fmt.Errorf("create immutable test data object: %w", err)
	}
	return &TestDataObject{
		ContentLength: written, SHA256: checksum, PackageKey: packageKey, TestCases: testCases,
	}, nil
}

func (s *s3TestDataStorage) copyObject(ctx context.Context, sourceKey, destinationKey string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, s.objectURL(destinationKey).String(), nil)
	if err != nil {
		return fmt.Errorf("create copy request: %w", err)
	}
	source := &url.URL{Path: "/" + s.bucket + "/" + strings.TrimPrefix(sourceKey, "/")}
	request.Header.Set("x-amz-copy-source", canonicalURI(source))
	s.signRequest(request, emptyPayloadSHA256, s.now().UTC())
	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("copy object: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("object storage copy returned status %d", response.StatusCode)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	return nil
}

func (s *s3TestDataStorage) objectURL(key string) *url.URL {
	result := *s.endpoint
	key = strings.TrimPrefix(key, "/")
	if s.usePathStyle {
		result.Path = path.Join(result.Path, s.bucket, key)
	} else {
		result.Host = s.bucket + "." + result.Host
		result.Path = path.Join(result.Path, key)
	}
	if !strings.HasPrefix(result.Path, "/") {
		result.Path = "/" + result.Path
	}
	return &result
}

func (s *s3TestDataStorage) signRequest(request *http.Request, payloadHash string, now time.Time) {
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	request.Header.Set("x-amz-content-sha256", payloadHash)
	request.Header.Set("x-amz-date", amzDate)
	headerValues := map[string]string{"host": strings.ToLower(request.URL.Host)}
	for name, values := range request.Header {
		name = strings.ToLower(name)
		if strings.HasPrefix(name, "x-amz-") && name != "x-amz-signature" {
			normalized := make([]string, 0, len(values))
			for _, value := range values {
				normalized = append(normalized, strings.Join(strings.Fields(value), " "))
			}
			headerValues[name] = strings.Join(normalized, ",")
		}
	}
	headerNames := make([]string, 0, len(headerValues))
	for name := range headerValues {
		headerNames = append(headerNames, name)
	}
	sort.Strings(headerNames)
	var canonicalHeaders strings.Builder
	for _, name := range headerNames {
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(headerValues[name])
		canonicalHeaders.WriteByte('\n')
	}
	signedHeaders := strings.Join(headerNames, ";")
	canonicalRequest := strings.Join([]string{
		request.Method, canonicalURI(request.URL), canonicalQuery(request.URL.Query()),
		canonicalHeaders.String(), signedHeaders, payloadHash,
	}, "\n")
	scope := date + "/" + s.region + "/" + awsServiceName + "/aws4_request"
	signature := hex.EncodeToString(s.signingHMAC(date, awsStringToSign(amzDate, scope, canonicalRequest)))
	request.Header.Set("Authorization", awsSignatureAlgorithm+" Credential="+s.accessKey+"/"+scope+
		", SignedHeaders="+signedHeaders+", Signature="+signature)
}

func (s *s3TestDataStorage) signingHMAC(date, value string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+s.secretKey), date)
	regionKey := hmacSHA256(dateKey, s.region)
	serviceKey := hmacSHA256(regionKey, awsServiceName)
	signingKey := hmacSHA256(serviceKey, "aws4_request")
	return hmacSHA256(signingKey, value)
}

func hmacSHA256(key []byte, value string) []byte {
	var digest hash.Hash = hmac.New(sha256.New, key)
	_, _ = digest.Write([]byte(value))
	return digest.Sum(nil)
}

func awsStringToSign(amzDate, scope, canonicalRequest string) string {
	digest := sha256.Sum256([]byte(canonicalRequest))
	return awsSignatureAlgorithm + "\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(digest[:])
}

func canonicalURI(value *url.URL) string {
	if value.Path == "" {
		return "/"
	}
	const hexadecimal = "0123456789ABCDEF"
	var result strings.Builder
	for _, character := range []byte(value.Path) {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' || character == '~' || character == '/' {
			result.WriteByte(character)
			continue
		}
		result.WriteByte('%')
		result.WriteByte(hexadecimal[character>>4])
		result.WriteByte(hexadecimal[character&0x0f])
	}
	return result.String()
}

func canonicalQuery(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var result []string
	for _, key := range keys {
		entries := append([]string(nil), values[key]...)
		sort.Strings(entries)
		for _, value := range entries {
			result = append(result, awsQueryEscape(key)+"="+awsQueryEscape(value))
		}
	}
	return strings.Join(result, "&")
}

func awsQueryEscape(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func validBucketName(value string) bool {
	if len(value) < 3 || len(value) > 63 || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") ||
		strings.HasPrefix(value, "-") || strings.HasSuffix(value, "-") || strings.Contains(value, "..") {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '.' && character != '-' {
			return false
		}
	}
	return true
}

func inspectTestDataArchive(fileName string, policy UploadPolicy) ([]model.TestCaseInput, error) {
	archive, err := zip.OpenReader(fileName)
	if err != nil {
		return nil, errors.New("test data package must be a valid ZIP archive")
	}
	defer archive.Close()
	if len(archive.File) == 0 || len(archive.File) > policy.MaxFileCount {
		return nil, errors.New("archive file count is outside the allowed range")
	}

	entries := make(map[string]*zip.File, len(archive.File))
	var declaredUncompressed uint64
	var declaredCompressed uint64
	for _, entry := range archive.File {
		entryName, err := safeZIPEntryPath(entry)
		if err != nil {
			return nil, err
		}
		if _, exists := entries[entryName]; exists {
			return nil, fmt.Errorf("archive contains duplicate path %q", entryName)
		}
		mode := entry.Mode()
		if mode&os.ModeSymlink != 0 || (!mode.IsRegular() && !mode.IsDir()) {
			return nil, fmt.Errorf("archive entry %q is not a regular file or directory", entryName)
		}
		if entry.UncompressedSize64 > uint64(policy.MaxFileBytes) && !mode.IsDir() {
			return nil, fmt.Errorf("archive entry %q exceeds the single-file size limit", entryName)
		}
		if entry.UncompressedSize64 > uint64(policy.MaxUncompressedBytes)-declaredUncompressed {
			return nil, errors.New("archive exceeds the uncompressed size limit")
		}
		if !mode.IsDir() && compressionRatioExceeded(entry.UncompressedSize64, entry.CompressedSize64, policy.MaxCompressionRatio) {
			return nil, fmt.Errorf("archive entry %q exceeds the compression-ratio limit", entryName)
		}
		declaredUncompressed += entry.UncompressedSize64
		declaredCompressed += entry.CompressedSize64
		entries[entryName] = entry
	}
	if compressionRatioExceeded(declaredUncompressed, declaredCompressed, policy.MaxCompressionRatio) {
		return nil, errors.New("archive exceeds the aggregate compression-ratio limit")
	}
	if err := rejectZIPPathTypeConflicts(entries); err != nil {
		return nil, err
	}

	manifestEntry, ok := entries[testdatamanifest.FileName]
	if !ok || manifestEntry.FileInfo().IsDir() || manifestEntry.UncompressedSize64 > uint64(policy.MaxManifestBytes) {
		return nil, errors.New("archive requires a bounded root manifest.json")
	}
	manifestReader, err := manifestEntry.Open()
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	manifestCounter := &countingReader{reader: manifestReader}
	manifest, parseErr := testdatamanifest.Parse(manifestCounter, policy.MaxManifestBytes)
	closeErr := manifestReader.Close()
	if parseErr != nil || closeErr != nil {
		return nil, fmt.Errorf("read manifest: %w", errors.Join(parseErr, closeErr))
	}
	if manifestCounter.read != int64(manifestEntry.UncompressedSize64) {
		return nil, errors.New("manifest has a false declared size")
	}

	files := manifest.FileMap()
	allowedDirectories := manifest.PayloadDirectories()
	for entryName, entry := range entries {
		if entry.FileInfo().IsDir() {
			if _, allowed := allowedDirectories[entryName]; !allowed {
				return nil, fmt.Errorf("archive contains undeclared directory %q", entryName)
			}
			continue
		}
		if entryName == testdatamanifest.FileName {
			continue
		}
		declared, exists := files[entryName]
		if !exists {
			return nil, fmt.Errorf("archive contains undeclared payload file %q", entryName)
		}
		if entry.UncompressedSize64 != uint64(declared.Size) {
			return nil, fmt.Errorf("archive entry %q size does not match manifest", entryName)
		}
	}
	for _, declared := range manifest.Files {
		entry, exists := entries[declared.Path]
		if !exists || entry.FileInfo().IsDir() {
			return nil, fmt.Errorf("manifest file %q is missing from archive", declared.Path)
		}
	}

	remaining := policy.MaxUncompressedBytes - manifestCounter.read
	for _, declared := range manifest.Files {
		entry := entries[declared.Path]
		if declared.Size > remaining {
			return nil, errors.New("archive exceeds the uncompressed size limit")
		}
		if err := verifyZIPFile(entry, declared, policy.MaxFileBytes); err != nil {
			return nil, err
		}
		remaining -= declared.Size
	}

	result := make([]model.TestCaseInput, 0, len(manifest.TestCases))
	for _, item := range manifest.TestCases {
		result = append(result, model.TestCaseInput{
			Name: item.Name, Score: item.Score, Ordinal: item.Ordinal,
			Enabled: item.IsEnabled(), GroupName: item.GroupName,
		})
	}
	return result, nil
}

type countingReader struct {
	reader io.Reader
	read   int64
}

func (reader *countingReader) Read(buffer []byte) (int, error) {
	count, err := reader.reader.Read(buffer)
	reader.read += int64(count)
	return count, err
}

func rejectZIPPathTypeConflicts(entries map[string]*zip.File) error {
	for entryName := range entries {
		for parent := path.Dir(entryName); parent != "."; parent = path.Dir(parent) {
			if existing, exists := entries[parent]; exists && !existing.FileInfo().IsDir() {
				return fmt.Errorf("archive path %q has regular-file parent %q", entryName, parent)
			}
		}
	}
	return nil
}

func safeZIPEntryPath(entry *zip.File) (string, error) {
	if entry == nil {
		return "", errors.New("archive contains an invalid entry")
	}
	name := entry.Name
	if entry.FileInfo().IsDir() {
		if !strings.HasSuffix(name, "/") {
			return "", fmt.Errorf("archive contains unsafe directory path %q", name)
		}
		name = strings.TrimSuffix(name, "/")
	}
	cleaned, err := testdatamanifest.SafePath(name)
	if err != nil {
		return "", fmt.Errorf("archive contains unsafe path %q", entry.Name)
	}
	return cleaned, nil
}

func compressionRatioExceeded(uncompressed, compressed uint64, limit int64) bool {
	if uncompressed == 0 {
		return false
	}
	if compressed == 0 || limit <= 0 {
		return true
	}
	return uncompressed/compressed > uint64(limit) ||
		(uncompressed/compressed == uint64(limit) && uncompressed%compressed != 0)
}

func verifyZIPFile(entry *zip.File, declared testdatamanifest.File, maxBytes int64) error {
	reader, err := entry.Open()
	if err != nil {
		return fmt.Errorf("open archive entry %q: %w", entry.Name, err)
	}
	digest := sha256.New()
	written, copyErr := io.Copy(digest, io.LimitReader(reader, maxBytes+1))
	closeErr := reader.Close()
	if copyErr != nil || closeErr != nil {
		return fmt.Errorf("verify archive entry %q: %w", entry.Name, errors.Join(copyErr, closeErr))
	}
	if written > maxBytes || written != declared.Size {
		return fmt.Errorf("archive entry %q size does not match manifest", entry.Name)
	}
	if hex.EncodeToString(digest.Sum(nil)) != declared.SHA256 {
		return fmt.Errorf("archive entry %q checksum does not match manifest", entry.Name)
	}
	return nil
}
