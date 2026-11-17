package model

import (
	"context"
	"database/sql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// UserSessions maps to the user_sessions table. RefreshTokenHash contains a
// lowercase hexadecimal SHA-256 digest, never the plaintext refresh token.
type UserSessions struct {
	Id                 int64          `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	UserId             int64          `json:"user_id" gorm:"column:user_id;not null"`
	RefreshTokenHash   string         `json:"-" gorm:"column:refresh_token_hash;size:64;not null"`
	BrowserSessionHash sql.NullString `json:"-" gorm:"column:browser_session_hash;size:64"`
	DeviceId           sql.NullString `json:"device_id" gorm:"column:device_id"`
	DeviceName         sql.NullString `json:"device_name" gorm:"column:device_name"`
	IpAddress          sql.NullString `json:"ip_address" gorm:"column:ip_address"`
	UserAgent          sql.NullString `json:"user_agent" gorm:"column:user_agent"`
	ExpiresAt          time.Time      `json:"expires_at" gorm:"column:expires_at;not null"`
	LastUsedAt         sql.NullTime   `json:"last_used_at" gorm:"column:last_used_at"`
	RevokedAt          sql.NullTime   `json:"revoked_at" gorm:"column:revoked_at"`
	RevokedReason      sql.NullString `json:"revoked_reason" gorm:"column:revoked_reason"`
	CreatedAt          time.Time      `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time      `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (UserSessions) TableName() string {
	return "user_sessions"
}

// Construct on the caller's transaction to make account changes and revocation atomic.
type UserSessionsModel struct{ db *gorm.DB }

func NewUserSessionsModel(db *gorm.DB) *UserSessionsModel { return &UserSessionsModel{db: db} }
func (m *UserSessionsModel) Create(ctx context.Context, s *UserSessions, expected *Users) error {
	// Serialize login with account changes. A password verified before a reset
	// must not create a fresh, usable session after that reset commits.
	var user Users
	if err := m.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND deleted_at IS NULL", s.UserId).Take(&user).Error; err != nil {
		return normalizeError(err)
	}
	if user.Status != 1 || expected == nil || user.Username != expected.Username || user.Email != expected.Email || user.PasswordHash != expected.PasswordHash || user.Role != expected.Role || user.SchoolId != expected.SchoolId {
		return ErrConflict
	}
	return normalizeError(m.db.WithContext(ctx).Create(s).Error)
}
func (m *UserSessionsModel) LockRefresh(ctx context.Context, hash string, now time.Time) (*UserSessions, *Users, error) {
	return m.lockCredential(ctx, hash, now, "refresh_token_hash")
}
func (m *UserSessionsModel) LockBrowserSession(ctx context.Context, hash string, now time.Time) (*UserSessions, *Users, error) {
	return m.lockCredential(ctx, hash, now, "browser_session_hash")
}
func (m *UserSessionsModel) lockCredential(ctx context.Context, hash string, now time.Time, column string) (*UserSessions, *Users, error) {
	var candidate UserSessions
	if err := m.db.WithContext(ctx).Select("user_id").Where(column+" = ?", hash).Take(&candidate).Error; err != nil {
		return nil, nil, normalizeError(err)
	}
	// Lock order is always user, then session (also used by account updates/login).
	var user Users
	if err := m.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Select(publicUserFields).Where("id = ? AND deleted_at IS NULL", candidate.UserId).Take(&user).Error; err != nil {
		return nil, nil, normalizeError(err)
	}
	if user.Status != 1 {
		return nil, nil, ErrConflict
	}
	var session UserSessions
	if err := m.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(column+" = ? AND revoked_at IS NULL AND expires_at > ?", hash, now).Take(&session).Error; err != nil {
		return nil, nil, normalizeError(err)
	}
	return &session, &user, nil
}
func (m *UserSessionsModel) Rotate(ctx context.Context, id int64, oldHash, newHash string, now, expires time.Time) error {
	result := m.db.WithContext(ctx).Model(&UserSessions{}).Where("id = ? AND refresh_token_hash = ? AND revoked_at IS NULL AND expires_at > ?", id, oldHash, now).Updates(map[string]any{"refresh_token_hash": newHash, "last_used_at": now, "expires_at": expires})
	if result.Error != nil {
		return normalizeError(result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrConflict
	}
	return nil
}
func (m *UserSessionsModel) RevokeAll(ctx context.Context, userID, except int64, reason string, now time.Time) error {
	q := m.db.WithContext(ctx).Model(&UserSessions{}).Where("user_id = ? AND revoked_at IS NULL", userID)
	if except > 0 {
		q = q.Where("id <> ?", except)
	}
	return q.Updates(map[string]any{"revoked_at": now, "revoked_reason": reason}).Error
}
func (m *UserSessionsModel) RevokeByID(ctx context.Context, id, userID int64, reason string, now time.Time) (int64, error) {
	q := m.db.WithContext(ctx).Model(&UserSessions{}).Where("id = ? AND revoked_at IS NULL", id)
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	result := q.Updates(map[string]any{"revoked_at": now, "revoked_reason": reason})
	return result.RowsAffected, result.Error
}
func (m *UserSessionsModel) RevokeByRefreshHash(ctx context.Context, hash, reason string, now time.Time) (int64, error) {
	result := m.db.WithContext(ctx).Model(&UserSessions{}).Where("refresh_token_hash = ? AND revoked_at IS NULL", hash).Updates(map[string]any{"revoked_at": now, "revoked_reason": reason})
	return result.RowsAffected, result.Error
}
func (m *UserSessionsModel) Cleanup(ctx context.Context, now, revokedBefore time.Time) error {
	return m.db.WithContext(ctx).Where("expires_at <= ? OR (revoked_at IS NOT NULL AND revoked_at <= ?)", now, revokedBefore).Delete(&UserSessions{}).Error
}

type AccessState struct {
	UserID           int64         `gorm:"column:user_id"`
	Role             int64         `gorm:"column:user_role"`
	Status           int64         `gorm:"column:user_status"`
	SchoolID         sql.NullInt64 `gorm:"column:school_id"`
	SessionExpiresAt time.Time     `gorm:"column:session_expires_at"`
	SessionRevokedAt sql.NullTime  `gorm:"column:session_revoked_at"`
}

func (m *UserSessionsModel) AccessState(ctx context.Context, id, userID int64) (*AccessState, error) {
	var state AccessState
	err := m.db.WithContext(ctx).Model(&UserSessions{}).Select("user_sessions.user_id AS user_id, user_sessions.expires_at AS session_expires_at, user_sessions.revoked_at AS session_revoked_at, users.role AS user_role, users.status AS user_status, users.school_id AS school_id").Joins("JOIN users ON users.id = user_sessions.user_id AND users.deleted_at IS NULL").Where("user_sessions.id = ? AND user_sessions.user_id = ?", id, userID).Take(&state).Error
	if err != nil {
		return nil, normalizeError(err)
	}
	return &state, nil
}

func (m *UserSessionsModel) RevokeByBrowserHash(ctx context.Context, hash string, now time.Time) error {
	return m.db.WithContext(ctx).Model(&UserSessions{}).Where("browser_session_hash = ? AND revoked_at IS NULL", hash).Updates(map[string]any{"revoked_at": now, "revoked_reason": "logout"}).Error
}
