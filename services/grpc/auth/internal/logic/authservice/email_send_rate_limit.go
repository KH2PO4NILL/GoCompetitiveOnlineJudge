package authservicelogic

import (
	"context"
	"strconv"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/rediskey"

	"github.com/redis/go-redis/v9"
)

const (
	emailSendIPLimit      = 20
	emailSendIPWindow     = 15 * time.Minute
	emailSendGlobalLimit  = 500
	emailSendGlobalWindow = time.Minute
)

type emailSendRateBucket struct {
	key    string
	limit  int
	window time.Duration
}

const reserveEmailSendRateScript = `
local retry_after = 0
for index, key in ipairs(KEYS) do
    local limit = tonumber(ARGV[(index - 1) * 2 + 1])
    local window = tonumber(ARGV[(index - 1) * 2 + 2])
    local attempts = redis.call("INCR", key)
    if attempts == 1 then
        redis.call("PEXPIRE", key, window)
    end
    if attempts > limit then
        local ttl = redis.call("PTTL", key)
        if ttl > retry_after then
            retry_after = ttl
        end
    end
end
return retry_after
`

func emailSendRateBuckets(info clientInfo) []emailSendRateBucket {
	buckets := []emailSendRateBucket{{
		key:    rediskey.EmailSendRateLimit("global", "all"),
		limit:  emailSendGlobalLimit,
		window: emailSendGlobalWindow,
	}}
	if info.ipAddress.Valid {
		buckets = append(buckets, emailSendRateBucket{
			key:    rediskey.EmailSendRateLimit("ip", info.ipAddress.String),
			limit:  emailSendIPLimit,
			window: emailSendIPWindow,
		})
	}
	return buckets
}

func reserveEmailSendRate(ctx context.Context, client *redis.Client, buckets []emailSendRateBucket) (bool, error) {
	keys := make([]string, 0, len(buckets))
	args := make([]any, 0, len(buckets)*2)
	for _, bucket := range buckets {
		keys = append(keys, bucket.key)
		args = append(
			args,
			strconv.Itoa(bucket.limit),
			bucket.window.Milliseconds(),
		)
	}
	retryAfter, err := client.Eval(ctx, reserveEmailSendRateScript, keys, args...).Int64()
	return retryAfter > 0, err
}
