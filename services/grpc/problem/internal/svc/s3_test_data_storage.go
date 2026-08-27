package svc

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"unicode/utf8"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"
)

const (
	awsSignatureAlgorithm = "AWS4-HMAC-SHA256"
	awsServiceName        = "s3"
	emptyPayloadSHA256    = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	unsignedPayload       = "UNSIGNED-PAYLOAD"
	manifestFileName      = "manifest.json"
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

type testDataManifest struct {
	TestCases []testDataManifestCase `json:"test_cases"`
}

type testDataManifestCase struct {
	Name       string `json:"name"`
	InputFile  string `json:"input_file"`
	OutputFile string `json:"output_file"`
	Score      int64  `json:"score"`
	Ordinal    int64  `json:"ordinal"`
	Enabled    *bool  `json:"enabled,omitempty"`
	GroupName  string `json:"group_name,omitempty"`
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
	var totalUncompressed uint64
	for _, entry := range archive.File {
		entryName, err := safeArchivePath(entry.Name)
		if err != nil {
			return nil, err
		}
		if _, exists := entries[entryName]; exists {
			return nil, fmt.Errorf("archive contains duplicate path %q", entryName)
		}
		mode := entry.Mode()
		if mode&os.ModeSymlink != 0 || (!mode.IsRegular() && !mode.IsDir()) {
			return nil, fmt.Errorf("archive entry %q is not a regular file", entryName)
		}
		if entry.UncompressedSize64 > uint64(policy.MaxUncompressedBytes)-totalUncompressed {
			return nil, errors.New("archive exceeds the uncompressed size limit")
		}
		totalUncompressed += entry.UncompressedSize64
		entries[entryName] = entry
	}
	manifestEntry, ok := entries[manifestFileName]
	if !ok || manifestEntry.FileInfo().IsDir() || manifestEntry.UncompressedSize64 > uint64(policy.MaxManifestBytes) {
		return nil, errors.New("archive requires a bounded root manifest.json")
	}
	if err := verifyArchiveContents(archive.File, policy.MaxUncompressedBytes); err != nil {
		return nil, err
	}
	manifestBytes, err := readZipEntry(manifestEntry, policy.MaxManifestBytes)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	if !utf8.Valid(manifestBytes) {
		return nil, errors.New("manifest must be valid UTF-8")
	}
	var manifest testDataManifest
	decoder := json.NewDecoder(bytes.NewReader(manifestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	return testCasesFromManifest(manifest, entries)
}

func safeArchivePath(value string) (string, error) {
	if value == "" || strings.ContainsRune(value, '\x00') || strings.ContainsAny(value, "\\:") || strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("archive contains unsafe path %q", value)
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != strings.TrimSuffix(value, "/") {
		return "", fmt.Errorf("archive contains unsafe path %q", value)
	}
	return cleaned, nil
}

func verifyArchiveContents(entries []*zip.File, limit int64) error {
	remaining := limit
	buffer := make([]byte, 32<<10)
	for _, entry := range entries {
		if entry.FileInfo().IsDir() {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open archive entry %q: %w", entry.Name, err)
		}
		count, copyErr := io.CopyBuffer(io.Discard, io.LimitReader(reader, remaining+1), buffer)
		closeErr := reader.Close()
		if copyErr != nil || closeErr != nil {
			return fmt.Errorf("verify archive entry %q: %w", entry.Name, errors.Join(copyErr, closeErr))
		}
		if count > remaining {
			return errors.New("archive exceeds the uncompressed size limit")
		}
		remaining -= count
	}
	return nil
}

func readZipEntry(entry *zip.File, limit int64) ([]byte, error) {
	reader, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	content, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, errors.New("entry exceeds its size limit")
	}
	return content, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("manifest must contain exactly one JSON value")
		}
		return fmt.Errorf("decode trailing manifest content: %w", err)
	}
	return nil
}

func testCasesFromManifest(manifest testDataManifest, entries map[string]*zip.File) ([]model.TestCaseInput, error) {
	if len(manifest.TestCases) == 0 {
		return nil, errors.New("manifest contains no test cases")
	}
	ordinals := make(map[int64]struct{}, len(manifest.TestCases))
	result := make([]model.TestCaseInput, 0, len(manifest.TestCases))
	for _, item := range manifest.TestCases {
		inputPath, err := safeArchivePath(item.InputFile)
		if err != nil {
			return nil, fmt.Errorf("invalid input_file for test case %q", item.Name)
		}
		outputPath, err := safeArchivePath(item.OutputFile)
		if err != nil {
			return nil, fmt.Errorf("invalid output_file for test case %q", item.Name)
		}
		if inputPath == outputPath {
			return nil, fmt.Errorf("test case %q uses the same input and output file", item.Name)
		}
		inputEntry, inputExists := entries[inputPath]
		outputEntry, outputExists := entries[outputPath]
		if !inputExists || !outputExists || inputEntry.FileInfo().IsDir() || outputEntry.FileInfo().IsDir() {
			return nil, fmt.Errorf("test case %q references a missing file", item.Name)
		}
		if _, exists := ordinals[item.Ordinal]; exists {
			return nil, fmt.Errorf("manifest contains duplicate ordinal %d", item.Ordinal)
		}
		ordinals[item.Ordinal] = struct{}{}
		enabled := true
		if item.Enabled != nil {
			enabled = *item.Enabled
		}
		result = append(result, model.TestCaseInput{
			Name: item.Name, Score: item.Score, Ordinal: item.Ordinal,
			Enabled: enabled, GroupName: item.GroupName,
		})
	}
	return result, nil
}
