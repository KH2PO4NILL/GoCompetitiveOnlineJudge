package authservicelogic

import (
	"context"
	"strconv"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/rediskey"

	"github.com/redis/go-redis/v9"
)

const (
	passwordPrincipalFailureLimit = 5
	passwordIPFailureLimit        = 50
	passwordLoginFailureWindow    = 15 * time.Minute
)

type passwordLoginRateLimitBucket struct {
	key   string
	limit int
}

const checkPasswordLoginLimitScript = `
local retry_after = 0
for index, key in ipairs(KEYS) do
    local attempts = tonumber(redis.call("GET", key) or "0")
	local limit = tonumber(ARGV[index + 1])
    if attempts >= limit then
        local ttl = redis.call("PTTL", key)
        if ttl < 0 then
			redis.call("PEXPIRE", key, ARGV[1])
			ttl = tonumber(ARGV[1])
        end
        if ttl > retry_after then
            retry_after = ttl
        end
    end
end
return retry_after
`

const recordPasswordLoginFailureScript = `
local retry_after = 0
for index, key in ipairs(KEYS) do
    local attempts = redis.call("INCR", key)
    if attempts == 1 then
		redis.call("PEXPIRE", key, ARGV[1])
    end
	local limit = tonumber(ARGV[index + 1])
	if attempts >= limit then
        local ttl = redis.call("PTTL", key)
        if ttl > retry_after then
            retry_after = ttl
        end
    end
end
return retry_after
`

func passwordLoginRateLimitBuckets(principalKind, principal string, clientInfo clientInfo) []passwordLoginRateLimitBucket {
	principalValue := principalKind + ":" + principal
	if clientInfo.ipAddress.Valid {
		principalValue += ":" + clientInfo.ipAddress.String
	}
	buckets := []passwordLoginRateLimitBucket{{
		key:   passwordLoginRateLimitKey("principal", principalValue),
		limit: passwordPrincipalFailureLimit,
	}}
	if clientInfo.ipAddress.Valid {
		buckets = append(buckets, passwordLoginRateLimitBucket{
			key:   passwordLoginRateLimitKey("ip", clientInfo.ipAddress.String),
			limit: passwordIPFailureLimit,
		})
	}
	return buckets
}

func passwordLoginRateLimitKey(kind, value string) string {
	return rediskey.PasswordLogin(kind, value)
}

func isPasswordLoginRateLimited(ctx context.Context, client *redis.Client, buckets []passwordLoginRateLimitBucket) (bool, error) {
	keys, args := passwordLoginRateLimitScriptInput(buckets)
	retryAfter, err := client.Eval(
		ctx,
		checkPasswordLoginLimitScript,
		keys,
		args...,
	).Int64()
	return retryAfter > 0, err
}

func recordPasswordLoginFailure(ctx context.Context, client *redis.Client, buckets []passwordLoginRateLimitBucket) (bool, error) {
	keys, args := passwordLoginRateLimitScriptInput(buckets)
	retryAfter, err := client.Eval(
		ctx,
		recordPasswordLoginFailureScript,
		keys,
		args...,
	).Int64()
	return retryAfter > 0, err
}

func clearPasswordLoginFailures(ctx context.Context, client *redis.Client, buckets []passwordLoginRateLimitBucket) error {
	keys := make([]string, 0, len(buckets))
	// 登录成功只能清除该账号与当前 IP 的组合计数，不能清空共享 IP 的全局计数。
	if len(buckets) > 0 {
		keys = append(keys, buckets[0].key)
	}
	if len(keys) == 0 {
		return nil
	}
	return client.Del(ctx, keys...).Err()
}

func passwordLoginRateLimitScriptInput(buckets []passwordLoginRateLimitBucket) ([]string, []any) {
	keys := make([]string, 0, len(buckets))
	args := make([]any, 0, len(buckets)+1)
	args = append(args, passwordLoginFailureWindow.Milliseconds())
	for _, bucket := range buckets {
		keys = append(keys, bucket.key)
		args = append(args, strconv.Itoa(bucket.limit))
	}
	return keys, args
}
