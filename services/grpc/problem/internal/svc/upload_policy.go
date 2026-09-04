package svc

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/config"
)

const (
	defaultUploadURLTTL        = 15 * time.Minute
	defaultMaxUploadBytes      = int64(512 << 20)
	defaultMaxUncompressedSize = int64(2 << 30)
	defaultMaxFileBytes        = int64(512 << 20)
	defaultMaxArchiveFiles     = 20000
	defaultMaxManifestBytes    = int64(2 << 20)
	defaultMaxCompressionRatio = int64(200)
)

type UploadPolicy struct {
	URLTTL               time.Duration
	MaxUploadBytes       int64
	AllowedExtensions    []string
	MaxUncompressedBytes int64
	MaxFileBytes         int64
	MaxFileCount         int
	MaxManifestBytes     int64
	MaxCompressionRatio  int64
}

func NewUploadPolicy(c config.UploadConf) (UploadPolicy, error) {
	policy := defaultUploadPolicy()
	if c.URLTTLSeconds > 0 {
		policy.URLTTL = time.Duration(c.URLTTLSeconds) * time.Second
	}
	if c.MaxUploadBytes > 0 {
		policy.MaxUploadBytes = c.MaxUploadBytes
	}
	if len(c.AllowedExtensions) > 0 {
		policy.AllowedExtensions = append([]string(nil), c.AllowedExtensions...)
	}
	if c.MaxUncompressedBytes > 0 {
		policy.MaxUncompressedBytes = c.MaxUncompressedBytes
	}
	if c.MaxFileBytes > 0 {
		policy.MaxFileBytes = c.MaxFileBytes
	} else if policy.MaxFileBytes > policy.MaxUncompressedBytes {
		policy.MaxFileBytes = policy.MaxUncompressedBytes
	}
	if c.MaxFileCount > 0 {
		policy.MaxFileCount = c.MaxFileCount
	}
	if c.MaxManifestBytes > 0 {
		policy.MaxManifestBytes = c.MaxManifestBytes
	}
	if c.MaxCompressionRatio > 0 {
		policy.MaxCompressionRatio = c.MaxCompressionRatio
	}
	if err := policy.normalizeAndValidate(); err != nil {
		return UploadPolicy{}, err
	}
	return policy, nil
}

func defaultUploadPolicy() UploadPolicy {
	return UploadPolicy{
		URLTTL:               defaultUploadURLTTL,
		MaxUploadBytes:       defaultMaxUploadBytes,
		AllowedExtensions:    []string{".zip"},
		MaxUncompressedBytes: defaultMaxUncompressedSize,
		MaxFileBytes:         defaultMaxFileBytes,
		MaxFileCount:         defaultMaxArchiveFiles,
		MaxManifestBytes:     defaultMaxManifestBytes,
		MaxCompressionRatio:  defaultMaxCompressionRatio,
	}
}

func (p UploadPolicy) withDefaults() UploadPolicy {
	defaults := defaultUploadPolicy()
	if p.URLTTL <= 0 {
		p.URLTTL = defaults.URLTTL
	}
	if p.MaxUploadBytes <= 0 {
		p.MaxUploadBytes = defaults.MaxUploadBytes
	}
	if len(p.AllowedExtensions) == 0 {
		p.AllowedExtensions = defaults.AllowedExtensions
	}
	if p.MaxUncompressedBytes <= 0 {
		p.MaxUncompressedBytes = defaults.MaxUncompressedBytes
	}
	if p.MaxFileBytes <= 0 {
		p.MaxFileBytes = min(defaults.MaxFileBytes, p.MaxUncompressedBytes)
	}
	if p.MaxFileCount <= 0 {
		p.MaxFileCount = defaults.MaxFileCount
	}
	if p.MaxManifestBytes <= 0 {
		p.MaxManifestBytes = min(defaults.MaxManifestBytes, p.MaxFileBytes)
	}
	if p.MaxCompressionRatio <= 0 {
		p.MaxCompressionRatio = defaults.MaxCompressionRatio
	}
	_ = p.normalizeAndValidate()
	return p
}

func (p *UploadPolicy) normalizeAndValidate() error {
	if p.URLTTL <= 0 || p.URLTTL > 24*time.Hour {
		return errors.New("upload URL TTL must be between 1 second and 24 hours")
	}
	if p.MaxUploadBytes <= 0 || p.MaxUncompressedBytes < p.MaxUploadBytes {
		return errors.New("upload limits must be positive and uncompressed limit must not be smaller than upload limit")
	}
	if p.MaxFileBytes <= 0 || p.MaxFileBytes > p.MaxUncompressedBytes || p.MaxFileCount <= 0 ||
		p.MaxManifestBytes <= 0 || p.MaxManifestBytes > p.MaxFileBytes || p.MaxCompressionRatio <= 0 || p.MaxCompressionRatio > 100000 {
		return errors.New("archive file, manifest or compression-ratio limits are invalid")
	}
	seen := make(map[string]struct{}, len(p.AllowedExtensions))
	normalized := make([]string, 0, len(p.AllowedExtensions))
	for _, extension := range p.AllowedExtensions {
		extension = strings.ToLower(strings.TrimSpace(extension))
		if extension == "" || extension[0] != '.' || strings.ContainsAny(extension, `/\\\x00`) {
			return fmt.Errorf("invalid upload extension %q", extension)
		}
		if _, exists := seen[extension]; !exists {
			seen[extension] = struct{}{}
			normalized = append(normalized, extension)
		}
	}
	if len(normalized) == 0 {
		return errors.New("at least one upload extension is required")
	}
	p.AllowedExtensions = normalized
	return nil
}

func (p UploadPolicy) ValidateFileName(name string) error {
	p = p.withDefaults()
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 255 ||
		name != filepath.Base(name) || name == "." || strings.ContainsAny(name, `/\\\x00`) {
		return errors.New("file_name must be a valid base name of at most 255 characters")
	}
	extension := strings.ToLower(filepath.Ext(name))
	for _, allowed := range p.AllowedExtensions {
		if extension == allowed {
			return nil
		}
	}
	return fmt.Errorf("file extension %q is not allowed", extension)
}

func (s *ServiceContext) EffectiveUploadPolicy() UploadPolicy {
	if s == nil {
		return defaultUploadPolicy()
	}
	return s.UploadPolicy.withDefaults()
}
