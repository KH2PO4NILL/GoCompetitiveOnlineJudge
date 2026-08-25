package rediskey

import (
	"crypto/sha256"
	"encoding/hex"
)

// Digest 对 Redis 键中的隐私数据做不可逆摘要，避免邮箱、用户名或 IP
// 直接出现在 Redis 快照、监控页面和运维命令输出中。
func Digest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func EmailCode(purpose, email string) string {
	return "auth:email_code:" + purpose + ":" + Digest(email)
}

func EmailCodeAttempts(purpose, email string) string {
	return EmailCode(purpose, email) + ":attempts"
}

func EmailCodeCooldown(purpose, email string) string {
	return "auth:email_code:cooldown:" + purpose + ":" + Digest(email)
}

func PasswordLogin(kind, value string) string {
	return "auth:password_login:failures:" + kind + ":" + Digest(value)
}

func EmailSendRateLimit(kind, value string) string {
	return "auth:email_send:rate:" + kind + ":" + Digest(value)
}

func UsedRefreshToken(refreshTokenHash string) string {
	return "auth:refresh_token:used:" + refreshTokenHash
}
