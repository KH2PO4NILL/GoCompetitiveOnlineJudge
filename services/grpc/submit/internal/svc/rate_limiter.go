package svc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrSubmissionRateLimited = errors.New("submission rate limit exceeded")

type SubmissionRateLimiter interface {
	Allow(ctx context.Context, userID int64, clientRequestID string) error
}

type NoopSubmissionRateLimiter struct{}

func (NoopSubmissionRateLimiter) Allow(context.Context, int64, string) error { return nil }

type redisSubmissionRateLimiter struct {
	client  redis.UniversalClient
	window  time.Duration
	maximum int64
}

var allowSubmissionScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[2]) == 1 then
  return 1
end
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
if count > tonumber(ARGV[2]) then
  redis.call('DECR', KEYS[1])
  return 0
end
redis.call('SET', KEYS[2], '1', 'EX', ARGV[1])
return 1
`)

func NewRedisSubmissionRateLimiter(
	client redis.UniversalClient,
	window time.Duration,
	maximum int64,
) (SubmissionRateLimiter, error) {
	if client == nil || window <= 0 || maximum <= 0 {
		return nil, errors.New("submission rate limiter requires redis, a window and a maximum")
	}
	return &redisSubmissionRateLimiter{client: client, window: window, maximum: maximum}, nil
}

func (l *redisSubmissionRateLimiter) Allow(
	ctx context.Context,
	userID int64,
	clientRequestID string,
) error {
	if l == nil || l.client == nil {
		return errors.New("submission rate limiter is unavailable")
	}
	if userID <= 0 || clientRequestID == "" {
		return errors.New("invalid submission rate limit identity")
	}
	digest := sha256.Sum256([]byte(clientRequestID))
	user := strconv.FormatInt(userID, 10)
	keys := []string{
		"submit:create:rate:" + user,
		"submit:create:request:" + user + ":" + hex.EncodeToString(digest[:]),
	}
	seconds := int64(l.window / time.Second)
	if seconds <= 0 {
		seconds = 1
	}
	allowed, err := allowSubmissionScript.Run(ctx, l.client, keys, seconds, l.maximum).Int64()
	if err != nil {
		return fmt.Errorf("check submission rate limit: %w", err)
	}
	if allowed != 1 {
		return ErrSubmissionRateLimited
	}
	return nil
}
