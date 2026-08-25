package authservicelogic

import (
	"context"
	"net/mail"
	"strings"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/rediskey"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/redis/go-redis/v9"
)

type emailCodePurpose string

const (
	emailCodePurposeRegister      emailCodePurpose = "register"
	emailCodePurposeLogin         emailCodePurpose = "login"
	emailCodePurposeChangeEmail   emailCodePurpose = "change_email"
	emailCodePurposeResetPassword emailCodePurpose = "reset_password"
)

const consumeEmailCodeScript = `
local saved = redis.call("GET", KEYS[1])
if not saved then
    return 0
end

local attempts = redis.call("INCR", KEYS[2])
if attempts == 1 then
    local ttl = redis.call("PTTL", KEYS[1])
    if ttl > 0 then
        redis.call("PEXPIRE", KEYS[2], ttl)
    else
        redis.call("EXPIRE", KEYS[2], 300)
    end
end

if attempts > 5 then
    redis.call("DEL", KEYS[1])
    redis.call("DEL", KEYS[2])
    return -1
end

if saved ~= ARGV[1] then
    return 0
end

redis.call("DEL", KEYS[1])
redis.call("DEL", KEYS[2])
return 1
`

const deleteEmailCodeIfMatchesScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    redis.call("DEL", KEYS[1])
    redis.call("DEL", KEYS[2])
    return 1
end
return 0
`

func parseEmailCodePurpose(value pb.EmailCodePurpose) (emailCodePurpose, bool) {
	switch value {
	case pb.EmailCodePurpose_EMAIL_CODE_PURPOSE_REGISTER:
		return emailCodePurposeRegister, true
	case pb.EmailCodePurpose_EMAIL_CODE_PURPOSE_LOGIN:
		return emailCodePurposeLogin, true
	case pb.EmailCodePurpose_EMAIL_CODE_PURPOSE_CHANGE_EMAIL:
		return emailCodePurposeChangeEmail, true
	case pb.EmailCodePurpose_EMAIL_CODE_PURPOSE_RESET_PASSWORD:
		return emailCodePurposeResetPassword, true
	default:
		return "", false
	}
}

func normalizeEmail(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 254 {
		return "", false
	}

	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value {
		return "", false
	}
	return value, true
}

func emailCodeKey(purpose emailCodePurpose, email string) string {
	return rediskey.EmailCode(string(purpose), email)
}

func emailCodeAttemptsKey(purpose emailCodePurpose, email string) string {
	return rediskey.EmailCodeAttempts(string(purpose), email)
}

func emailCodeCooldownKey(purpose emailCodePurpose, email string) string {
	return rediskey.EmailCodeCooldown(string(purpose), email)
}

func consumeEmailCode(
	ctx context.Context,
	client *redis.Client,
	purpose emailCodePurpose,
	email string,
	code string,
) (bool, error) {
	matched, err := client.Eval(
		ctx,
		consumeEmailCodeScript,
		[]string{
			emailCodeKey(purpose, email),
			emailCodeAttemptsKey(purpose, email),
		},
		code,
	).Int()
	return matched == 1, err
}

func deleteEmailCodeIfMatches(
	ctx context.Context,
	client *redis.Client,
	purpose emailCodePurpose,
	email string,
	code string,
) error {
	return client.Eval(
		ctx,
		deleteEmailCodeIfMatchesScript,
		[]string{
			emailCodeKey(purpose, email),
			emailCodeAttemptsKey(purpose, email),
		},
		code,
	).Err()
}

func isSixDigitCode(value string) bool {
	if len(value) != 6 {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}
