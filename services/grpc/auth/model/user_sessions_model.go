package model

import (
	"database/sql"
	"time"
)

// UserSessions maps to the user_sessions table. RefreshTokenHash contains a
// lowercase hexadecimal SHA-256 digest, never the plaintext refresh token.
type UserSessions struct {
	Id               int64          `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	UserId           int64          `json:"user_id" gorm:"column:user_id;not null"`
	RefreshTokenHash string         `json:"-" gorm:"column:refresh_token_hash;size:64;not null"`
	DeviceId         sql.NullString `json:"device_id" gorm:"column:device_id"`
	DeviceName       sql.NullString `json:"device_name" gorm:"column:device_name"`
	IpAddress        sql.NullString `json:"ip_address" gorm:"column:ip_address"`
	UserAgent        sql.NullString `json:"user_agent" gorm:"column:user_agent"`
	ExpiresAt        time.Time      `json:"expires_at" gorm:"column:expires_at;not null"`
	LastUsedAt       sql.NullTime   `json:"last_used_at" gorm:"column:last_used_at"`
	RevokedAt        sql.NullTime   `json:"revoked_at" gorm:"column:revoked_at"`
	RevokedReason    sql.NullString `json:"revoked_reason" gorm:"column:revoked_reason"`
	CreatedAt        time.Time      `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time      `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (UserSessions) TableName() string {
	return "user_sessions"
}
