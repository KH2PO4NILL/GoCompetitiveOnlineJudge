package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"

	"go.yaml.in/yaml/v3"
)

const (
	DefaultConcurrency          = 4
	DefaultQueueCapacity        = 128
	MaxConcurrency              = 64
	MaxQueueCapacity            = 4096
	DefaultRPCTimeout           = 5 * time.Second
	DefaultTaskTimeout          = 5 * time.Minute
	DefaultShutdownTimeout      = 30 * time.Second
	MaxRPCTimeout               = time.Minute
	MaxTaskTimeout              = 24 * time.Hour
	MaxShutdownTimeout          = 5 * time.Minute
	DefaultCacheDirectory       = ".run/judge-worker/cache"
	DefaultCacheMaxBytes        = int64(20 << 30)
	DefaultCacheTTL             = 24 * time.Hour
	DefaultCleanupInterval      = 10 * time.Minute
	DefaultMaxDownloadBytes     = int64(512 << 20)
	DefaultMaxUncompressedBytes = int64(2 << 30)
	DefaultMaxFileBytes         = int64(512 << 20)
	DefaultMaxFileCount         = 20000
	DefaultMaxManifestBytes     = int64(2 << 20)
	DefaultMaxCompressionRatio  = int64(200)
	DefaultObjectRequestTimeout = 60 * time.Second
	DefaultSandboxWorkDirectory = ".run/judge-worker/work"
	DefaultDockerEndpoint       = "unix:///var/run/docker.sock"
	DefaultMetricsListenOn      = "127.0.0.1:9100"
	DefaultMetricsPath          = "/metrics"
)

type WorkerConf struct {
	ID            string `json:",optional" yaml:"ID"`
	Concurrency   int    `json:",optional" yaml:"Concurrency"`
	QueueCapacity int    `json:",optional" yaml:"QueueCapacity"`
}

type RPCConf struct {
	Target string `json:",optional" yaml:"Target"`
}

type InternalAuthConf struct {
	Name         string `json:",optional" yaml:"Name"`
	SubmitToken  string `json:",optional" yaml:"SubmitToken"`
	ProblemToken string `json:",optional" yaml:"ProblemToken"`
}

type TestDataCacheConf struct {
	Dir                    string `json:",optional" yaml:"Dir"`
	MaxBytes               int64  `json:",optional" yaml:"MaxBytes"`
	TTLSeconds             int64  `json:",optional" yaml:"TTLSeconds"`
	CleanupIntervalSeconds int64  `json:",optional" yaml:"CleanupIntervalSeconds"`
	MaxDownloadBytes       int64  `json:",optional" yaml:"MaxDownloadBytes"`
	MaxUncompressedBytes   int64  `json:",optional" yaml:"MaxUncompressedBytes"`
	MaxFileBytes           int64  `json:",optional" yaml:"MaxFileBytes"`
	MaxFileCount           int    `json:",optional" yaml:"MaxFileCount"`
	MaxManifestBytes       int64  `json:",optional" yaml:"MaxManifestBytes"`
	MaxCompressionRatio    int64  `json:",optional" yaml:"MaxCompressionRatio"`
}

type ObjectStorageConf struct {
	Endpoint              string `json:",optional" yaml:"Endpoint"`
	Region                string `json:",optional" yaml:"Region"`
	Bucket                string `json:",optional" yaml:"Bucket"`
	AccessKey             string `json:",optional" yaml:"AccessKey"`
	SecretKey             string `json:",optional" yaml:"SecretKey"`
	UsePathStyle          bool   `json:",optional" yaml:"UsePathStyle"`
	RequestTimeoutSeconds int64  `json:",optional" yaml:"RequestTimeoutSeconds"`
}

type DockerConf struct {
	Endpoint string `json:",optional" yaml:"Endpoint"`
}

type SandboxConf struct {
	Driver  string     `json:",optional" yaml:"Driver"`
	WorkDir string     `json:",optional" yaml:"WorkDir"`
	Docker  DockerConf `json:",optional" yaml:"Docker"`
}

type LogConf struct {
	Level     string `json:",optional" yaml:"Level"`
	Format    string `json:",optional" yaml:"Format"`
	AddSource bool   `json:",optional" yaml:"AddSource"`
}

type MetricsConf struct {
	Enabled  bool   `json:",optional" yaml:"Enabled"`
	ListenOn string `json:",optional" yaml:"ListenOn"`
	Path     string `json:",optional" yaml:"Path"`
}

type TimeoutConf struct {
	RPCSeconds      int64 `json:",optional" yaml:"RPCSeconds"`
	TaskSeconds     int64 `json:",optional" yaml:"TaskSeconds"`
	ShutdownSeconds int64 `json:",optional" yaml:"ShutdownSeconds"`
}

type Config struct {
	Worker        WorkerConf        `json:",optional" yaml:"Worker"`
	SubmitRPC     RPCConf           `json:",optional" yaml:"SubmitRPC"`
	ProblemRPC    RPCConf           `json:",optional" yaml:"ProblemRPC"`
	InternalAuth  InternalAuthConf  `json:",optional" yaml:"InternalAuth"`
	TestDataCache TestDataCacheConf `json:",optional" yaml:"TestDataCache"`
	ObjectStorage ObjectStorageConf `json:",optional" yaml:"ObjectStorage"`
	Sandbox       SandboxConf       `json:",optional" yaml:"Sandbox"`
	Log           LogConf           `json:",optional" yaml:"Log"`
	Metrics       MetricsConf       `json:",optional" yaml:"Metrics"`
	Timeouts      TimeoutConf       `json:",optional" yaml:"Timeouts"`
}

// Load reads the YAML configuration, expands environment variables, applies
// stable defaults and returns actionable validation errors instead of panicking.
func Load(path string) (Config, error) {
	var c Config
	if strings.TrimSpace(path) == "" {
		return c, errors.New("config file path is required")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("load judge worker config %q: %w", path, err)
	}
	// Use an alias without methods so decoding cannot invoke validation before
	// this package has applied defaults.
	type rawConfig Config
	var raw rawConfig
	decoder := yaml.NewDecoder(strings.NewReader(os.ExpandEnv(string(content))))
	decoder.KnownFields(true)
	if err := decoder.Decode(&raw); err != nil && !errors.Is(err, io.EOF) {
		return c, fmt.Errorf("load judge worker config %q: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return c, fmt.Errorf("load judge worker config %q: multiple YAML documents are not supported", path)
	} else if !errors.Is(err, io.EOF) {
		return c, fmt.Errorf("load judge worker config %q: %w", path, err)
	}
	c = Config(raw)
	c.ApplyDefaults()
	if err := c.Validate(); err != nil {
		return c, fmt.Errorf("validate judge worker config %q: %w", path, err)
	}
	return c, nil
}

func (c *Config) ApplyDefaults() {
	if c == nil {
		return
	}
	c.Worker.ID = strings.TrimSpace(c.Worker.ID)
	if c.Worker.Concurrency == 0 {
		c.Worker.Concurrency = DefaultConcurrency
	}
	if c.Worker.QueueCapacity == 0 {
		c.Worker.QueueCapacity = DefaultQueueCapacity
	}
	if strings.TrimSpace(c.InternalAuth.Name) == "" {
		c.InternalAuth.Name = "judge.worker"
	}
	if strings.TrimSpace(c.TestDataCache.Dir) == "" {
		c.TestDataCache.Dir = DefaultCacheDirectory
	}
	if c.TestDataCache.MaxBytes == 0 {
		c.TestDataCache.MaxBytes = DefaultCacheMaxBytes
	}
	if c.TestDataCache.TTLSeconds == 0 {
		c.TestDataCache.TTLSeconds = int64(DefaultCacheTTL / time.Second)
	}
	if c.TestDataCache.CleanupIntervalSeconds == 0 {
		c.TestDataCache.CleanupIntervalSeconds = int64(DefaultCleanupInterval / time.Second)
	}
	if c.TestDataCache.MaxDownloadBytes == 0 {
		c.TestDataCache.MaxDownloadBytes = DefaultMaxDownloadBytes
	}
	if c.TestDataCache.MaxUncompressedBytes == 0 {
		c.TestDataCache.MaxUncompressedBytes = DefaultMaxUncompressedBytes
	}
	if c.TestDataCache.MaxFileBytes == 0 {
		c.TestDataCache.MaxFileBytes = DefaultMaxFileBytes
	}
	if c.TestDataCache.MaxFileCount == 0 {
		c.TestDataCache.MaxFileCount = DefaultMaxFileCount
	}
	if c.TestDataCache.MaxManifestBytes == 0 {
		c.TestDataCache.MaxManifestBytes = DefaultMaxManifestBytes
	}
	if c.TestDataCache.MaxCompressionRatio == 0 {
		c.TestDataCache.MaxCompressionRatio = DefaultMaxCompressionRatio
	}
	if strings.TrimSpace(c.ObjectStorage.Region) == "" {
		c.ObjectStorage.Region = "us-east-1"
	}
	if c.ObjectStorage.RequestTimeoutSeconds == 0 {
		c.ObjectStorage.RequestTimeoutSeconds = int64(DefaultObjectRequestTimeout / time.Second)
	}
	if strings.TrimSpace(c.Sandbox.Driver) == "" {
		c.Sandbox.Driver = "docker"
	}
	if strings.TrimSpace(c.Sandbox.WorkDir) == "" {
		c.Sandbox.WorkDir = DefaultSandboxWorkDirectory
	}
	if strings.TrimSpace(c.Sandbox.Docker.Endpoint) == "" {
		c.Sandbox.Docker.Endpoint = DefaultDockerEndpoint
	}
	if strings.TrimSpace(c.Log.Level) == "" {
		c.Log.Level = "info"
	}
	if strings.TrimSpace(c.Log.Format) == "" {
		c.Log.Format = "text"
	}
	if strings.TrimSpace(c.Metrics.ListenOn) == "" {
		c.Metrics.ListenOn = DefaultMetricsListenOn
	}
	if strings.TrimSpace(c.Metrics.Path) == "" {
		c.Metrics.Path = DefaultMetricsPath
	}
	if c.Timeouts.RPCSeconds == 0 {
		c.Timeouts.RPCSeconds = int64(DefaultRPCTimeout / time.Second)
	}
	if c.Timeouts.TaskSeconds == 0 {
		c.Timeouts.TaskSeconds = int64(DefaultTaskTimeout / time.Second)
	}
	if c.Timeouts.ShutdownSeconds == 0 {
		c.Timeouts.ShutdownSeconds = int64(DefaultShutdownTimeout / time.Second)
	}
}

func (c Config) Validate() error {
	var errs []error
	workerID := strings.TrimSpace(c.Worker.ID)
	if workerID == "" {
		errs = append(errs, errors.New("Worker.ID is required"))
	} else if len(workerID) > 128 || !validPrintableASCII(workerID) {
		errs = append(errs, errors.New("Worker.ID must be at most 128 bytes of printable ASCII"))
	}
	if c.Worker.Concurrency < 1 || c.Worker.Concurrency > MaxConcurrency {
		errs = append(errs, fmt.Errorf("Worker.Concurrency must be between 1 and %d", MaxConcurrency))
	}
	if c.Worker.QueueCapacity < 1 || c.Worker.QueueCapacity > MaxQueueCapacity {
		errs = append(errs, fmt.Errorf("Worker.QueueCapacity must be between 1 and %d", MaxQueueCapacity))
	}
	if err := validateTarget("SubmitRPC.Target", c.SubmitRPC.Target); err != nil {
		errs = append(errs, err)
	}
	if err := validateTarget("ProblemRPC.Target", c.ProblemRPC.Target); err != nil {
		errs = append(errs, err)
	}
	for name, auth := range map[string]sharedauth.ServiceClientAuthConf{
		"SubmitRPC":  {Name: c.InternalAuth.Name, Token: c.InternalAuth.SubmitToken},
		"ProblemRPC": {Name: c.InternalAuth.Name, Token: c.InternalAuth.ProblemToken},
	} {
		if err := auth.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("%s internal authentication: %w", name, err))
		}
	}
	if err := validateDirectory("TestDataCache.Dir", c.TestDataCache.Dir); err != nil {
		errs = append(errs, err)
	}
	if err := validateTestDataCache(c.TestDataCache); err != nil {
		errs = append(errs, err)
	}
	if err := validateObjectStorage(c.ObjectStorage); err != nil {
		errs = append(errs, err)
	}
	if err := validateDirectory("Sandbox.WorkDir", c.Sandbox.WorkDir); err != nil {
		errs = append(errs, err)
	}
	if !strings.EqualFold(strings.TrimSpace(c.Sandbox.Driver), "docker") {
		errs = append(errs, errors.New("Sandbox.Driver must be docker"))
	}
	if err := validateDockerEndpoint(c.Sandbox.Docker.Endpoint); err != nil {
		errs = append(errs, err)
	}
	if err := validateLog(c.Log); err != nil {
		errs = append(errs, err)
	}
	if err := validateMetrics(c.Metrics); err != nil {
		errs = append(errs, err)
	}
	if err := validateTimeout("Timeouts.RPCSeconds", c.Timeouts.RPCSeconds, MaxRPCTimeout); err != nil {
		errs = append(errs, err)
	}
	if err := validateTimeout("Timeouts.TaskSeconds", c.Timeouts.TaskSeconds, MaxTaskTimeout); err != nil {
		errs = append(errs, err)
	}
	if err := validateTimeout("Timeouts.ShutdownSeconds", c.Timeouts.ShutdownSeconds, MaxShutdownTimeout); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (c Config) RPCTimeout() time.Duration {
	return time.Duration(c.Timeouts.RPCSeconds) * time.Second
}

func (c Config) TaskTimeout() time.Duration {
	return time.Duration(c.Timeouts.TaskSeconds) * time.Second
}

func (c Config) ShutdownTimeout() time.Duration {
	return time.Duration(c.Timeouts.ShutdownSeconds) * time.Second
}

func validPrintableASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] < 0x20 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func validateTarget(name, target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.ContainsAny(target, "\r\n\t ") {
		return fmt.Errorf("%s must not contain whitespace", name)
	}
	return nil
}

func validateDirectory(name, dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" || dir == "." {
		return fmt.Errorf("%s must identify a dedicated directory", name)
	}
	if strings.ContainsRune(dir, '\x00') || filepath.Clean(dir) == string(filepath.Separator) {
		return fmt.Errorf("%s is unsafe", name)
	}
	return nil
}

func validateTestDataCache(c TestDataCacheConf) error {
	if c.MaxBytes <= 0 || c.MaxDownloadBytes <= 0 || c.MaxUncompressedBytes <= 0 || c.MaxBytes < c.MaxUncompressedBytes || c.MaxFileBytes <= 0 ||
		c.MaxFileBytes > c.MaxUncompressedBytes || c.MaxFileCount <= 0 || c.MaxFileCount > 1000000 ||
		c.MaxManifestBytes <= 0 || c.MaxManifestBytes > c.MaxFileBytes || c.MaxCompressionRatio <= 0 || c.MaxCompressionRatio > 100000 {
		return errors.New("TestDataCache size, file-count or compression-ratio limits are invalid")
	}
	if c.TTLSeconds < 1 || c.TTLSeconds > int64((365*24*time.Hour)/time.Second) {
		return errors.New("TestDataCache.TTLSeconds must be between 1 and 31536000")
	}
	if c.CleanupIntervalSeconds < 1 || c.CleanupIntervalSeconds > int64((24*time.Hour)/time.Second) {
		return errors.New("TestDataCache.CleanupIntervalSeconds must be between 1 and 86400")
	}
	return nil
}

func validateObjectStorage(c ObjectStorageConf) error {
	endpoint, err := url.Parse(strings.TrimSpace(c.Endpoint))
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" ||
		(endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Opaque != "" ||
		endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.User != nil {
		return errors.New("ObjectStorage.Endpoint must be an http(s) URL without credentials, query or fragment")
	}
	if !validBucketName(strings.TrimSpace(c.Bucket)) {
		return errors.New("ObjectStorage.Bucket is invalid")
	}
	if strings.TrimSpace(c.AccessKey) == "" || c.SecretKey == "" || len(c.AccessKey) > 1024 || len(c.SecretKey) > 4096 {
		return errors.New("ObjectStorage read-only access key and secret key are required and must have bounded length")
	}
	if strings.TrimSpace(c.Region) == "" || strings.ContainsAny(c.Region, "\r\n\t ") {
		return errors.New("ObjectStorage.Region is invalid")
	}
	if c.RequestTimeoutSeconds < 1 || c.RequestTimeoutSeconds > 300 {
		return errors.New("ObjectStorage.RequestTimeoutSeconds must be between 1 and 300")
	}
	return nil
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

func validateDockerEndpoint(endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return errors.New("Sandbox.Docker.Endpoint is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Opaque != "" {
		return errors.New("Sandbox.Docker.Endpoint must be a valid unix, tcp, http or https URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("Sandbox.Docker.Endpoint must not contain credentials, query parameters or fragments")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "unix":
		if parsed.Host != "" || !filepath.IsAbs(parsed.Path) || filepath.Clean(parsed.Path) == string(filepath.Separator) {
			return errors.New("Sandbox.Docker.Endpoint unix URL must contain an absolute socket path")
		}
		return nil
	case "tcp":
		host, port, err := net.SplitHostPort(parsed.Host)
		if err != nil || strings.TrimSpace(host) == "" || !validPort(port) || parsed.Path != "" {
			return errors.New("Sandbox.Docker.Endpoint tcp URL must contain a valid host:port")
		}
		return nil
	case "http", "https":
		if strings.TrimSpace(parsed.Hostname()) == "" || (parsed.Port() != "" && !validPort(parsed.Port())) {
			return errors.New("Sandbox.Docker.Endpoint HTTP URL must contain a valid host and optional port")
		}
		return nil
	default:
		return errors.New("Sandbox.Docker.Endpoint must use unix, tcp, http or https")
	}
}

func validPort(port string) bool {
	value, err := strconv.Atoi(port)
	return err == nil && value >= 1 && value <= 65535
}

func validateLog(c LogConf) error {
	switch strings.ToLower(strings.TrimSpace(c.Level)) {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("Log.Level must be debug, info, warn or error")
	}
	switch strings.ToLower(strings.TrimSpace(c.Format)) {
	case "text", "json":
		return nil
	default:
		return errors.New("Log.Format must be text or json")
	}
}

func validateMetrics(c MetricsConf) error {
	if !strings.HasPrefix(c.Path, "/") || strings.ContainsAny(c.Path, "\r\n\t ") {
		return errors.New("Metrics.Path must be an absolute HTTP path without whitespace")
	}
	if _, _, err := net.SplitHostPort(strings.TrimSpace(c.ListenOn)); err != nil {
		return fmt.Errorf("Metrics.ListenOn must be a host:port address: %w", err)
	}
	return nil
}

func validateTimeout(name string, seconds int64, maximum time.Duration) error {
	maxSeconds := int64(maximum / time.Second)
	if seconds < 1 || seconds > maxSeconds {
		return fmt.Errorf("%s must be between 1 and %d", name, maxSeconds)
	}
	return nil
}
