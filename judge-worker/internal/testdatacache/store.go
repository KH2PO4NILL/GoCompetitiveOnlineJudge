package testdatacache

import (
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
	"path"
	"sort"
	"strings"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/testdatamanifest"
)

const (
	awsSignatureAlgorithm = "AWS4-HMAC-SHA256"
	awsServiceName        = "s3"
	emptyPayloadSHA256    = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

var ErrObjectNotFound = errors.New("test data object not found")

type Object struct {
	Body          io.ReadCloser
	ContentLength int64
}

// ObjectStore deliberately exposes only GET. The worker's object-store
// credentials must additionally be restricted to read-only access by the S3
// or MinIO bucket policy.
type ObjectStore interface {
	Open(ctx context.Context, key string) (*Object, error)
}

type S3Store struct {
	endpoint     *url.URL
	region       string
	bucket       string
	accessKey    string
	secretKey    string
	usePathStyle bool
	client       *http.Client
	now          func() time.Time
}

func NewS3Store(c config.ObjectStorageConf) (*S3Store, error) {
	endpoint, err := url.Parse(strings.TrimSpace(c.Endpoint))
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" ||
		(endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Opaque != "" || endpoint.User != nil ||
		endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, errors.New("object storage endpoint is invalid")
	}
	endpoint.Path = strings.TrimSuffix(endpoint.Path, "/")
	accessKey := strings.TrimSpace(c.AccessKey)
	if accessKey == "" || len(accessKey) > 1024 || c.SecretKey == "" || len(c.SecretKey) > 4096 {
		return nil, errors.New("object storage read-only credentials are required and must have bounded length")
	}
	if c.RequestTimeoutSeconds < 0 || c.RequestTimeoutSeconds > 300 {
		return nil, errors.New("object storage request timeout must not exceed 5 minutes")
	}
	timeout := config.DefaultObjectRequestTimeout
	if c.RequestTimeoutSeconds > 0 {
		timeout = time.Duration(c.RequestTimeoutSeconds) * time.Second
	}
	region := strings.TrimSpace(c.Region)
	if region == "" {
		region = "us-east-1"
	}
	if strings.ContainsAny(region, "\r\n\t ") {
		return nil, errors.New("object storage region is invalid")
	}
	bucket := strings.TrimSpace(c.Bucket)
	if !validS3BucketName(bucket) {
		return nil, errors.New("object storage bucket is invalid")
	}
	return &S3Store{
		endpoint: endpoint, region: region, bucket: bucket,
		accessKey: accessKey, secretKey: c.SecretKey, usePathStyle: c.UsePathStyle,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		now: func() time.Time { return time.Now().UTC() },
	}, nil
}

func validS3BucketName(value string) bool {
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

func (store *S3Store) Open(ctx context.Context, key string) (*Object, error) {
	if store == nil {
		return nil, errors.New("object storage is unavailable")
	}
	cleaned, err := testdatamanifest.SafePath(key)
	if err != nil || cleaned != key {
		return nil, errors.New("test data object key is unsafe")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, store.objectURL(key).String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create object request: %w", err)
	}
	store.signRequest(request, store.now().UTC())
	response, err := store.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download test data object: %w", err)
	}
	if response.StatusCode == http.StatusNotFound {
		response.Body.Close()
		return nil, ErrObjectNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		response.Body.Close()
		return nil, fmt.Errorf("object storage returned status %d", response.StatusCode)
	}
	return &Object{Body: response.Body, ContentLength: response.ContentLength}, nil
}

func (store *S3Store) objectURL(key string) *url.URL {
	result := *store.endpoint
	if store.usePathStyle {
		result.Path = path.Join(result.Path, store.bucket, key)
	} else {
		result.Host = store.bucket + "." + result.Host
		result.Path = path.Join(result.Path, key)
	}
	if !strings.HasPrefix(result.Path, "/") {
		result.Path = "/" + result.Path
	}
	return &result
}

func (store *S3Store) signRequest(request *http.Request, now time.Time) {
	date := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	request.Header.Set("x-amz-date", amzDate)
	request.Header.Set("x-amz-content-sha256", emptyPayloadSHA256)
	headerNames := make([]string, 0, len(request.Header)+1)
	headers := map[string][]string{"host": {strings.ToLower(request.URL.Host)}}
	for name, values := range request.Header {
		lower := strings.ToLower(name)
		headers[lower] = values
		headerNames = append(headerNames, lower)
	}
	headerNames = append(headerNames, "host")
	sort.Strings(headerNames)
	canonicalHeaders := strings.Builder{}
	for _, name := range headerNames {
		values := headers[name]
		for index := range values {
			values[index] = strings.Join(strings.Fields(values[index]), " ")
		}
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(strings.Join(values, ","))
		canonicalHeaders.WriteByte('\n')
	}
	signedHeaders := strings.Join(headerNames, ";")
	canonicalRequest := strings.Join([]string{
		request.Method,
		canonicalURI(request.URL),
		canonicalQuery(request.URL.Query()),
		canonicalHeaders.String(),
		signedHeaders,
		emptyPayloadSHA256,
	}, "\n")
	scope := date + "/" + store.region + "/" + awsServiceName + "/aws4_request"
	stringToSign := strings.Join([]string{
		awsSignatureAlgorithm,
		amzDate,
		scope,
		hexDigest(strings.NewReader(canonicalRequest)),
	}, "\n")
	signature := hex.EncodeToString(store.signingHMAC(date, stringToSign))
	request.Header.Set("Authorization", awsSignatureAlgorithm+" Credential="+store.accessKey+"/"+scope+
		", SignedHeaders="+signedHeaders+", Signature="+signature)
}

func (store *S3Store) signingHMAC(date, value string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+store.secretKey), date)
	regionKey := hmacSHA256(dateKey, store.region)
	serviceKey := hmacSHA256(regionKey, awsServiceName)
	requestKey := hmacSHA256(serviceKey, "aws4_request")
	return hmacSHA256(requestKey, value)
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func hexDigest(reader io.Reader) string {
	var digest hash.Hash = sha256.New()
	_, _ = io.Copy(digest, reader)
	return hex.EncodeToString(digest.Sum(nil))
}

func canonicalURI(value *url.URL) string {
	segments := strings.Split(value.EscapedPath(), "/")
	for index := range segments {
		if decoded, err := url.PathUnescape(segments[index]); err == nil {
			segments[index] = awsQueryEscape(decoded)
		}
	}
	result := strings.Join(segments, "/")
	if result == "" {
		return "/"
	}
	if !strings.HasPrefix(result, "/") {
		return "/" + result
	}
	return result
}

func canonicalQuery(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		items := append([]string(nil), values[key]...)
		sort.Strings(items)
		if len(items) == 0 {
			parts = append(parts, awsQueryEscape(key)+"=")
			continue
		}
		for _, item := range items {
			parts = append(parts, awsQueryEscape(key)+"="+awsQueryEscape(item))
		}
	}
	return strings.Join(parts, "&")
}

func awsQueryEscape(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}
