// Code scaffolded by goctl. Only init once, Safe to edit.
// gorm

package model

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"gorm.io/gorm/clause"
	"time"

	"gorm.io/gorm"
)

var _ UsersModel = (*customUsersModel)(nil)

type (
	// UsersModel 原生 DB 接口，不走缓存
	UsersModel interface {
		InsertOne(context.Context, *Users) error
		FindActiveByID(context.Context, int64) (*Users, error)
		ListActiveUsers(context.Context, int, int) ([]Users, int64, error)
		UpdateProfile(context.Context, int64, UserProfilePatch) error
		SoftDeleteAndAnonymize(context.Context, int64) error
		FindForAuthenticationByUsername(ctx context.Context, username string) (*Users, error)
		FindForAuthenticationByEmail(ctx context.Context, email string) (*Users, error)
		ExistsByUsername(ctx context.Context, username string) (bool, error)
		ExistsByEmail(ctx context.Context, email string) (bool, error)
		GetUserPasswordHash(ctx context.Context, userID int64) (string, error)
	}

	customUsersModel struct {
		conn *gorm.DB
	}
)

// NewUsersModel 创建纯 DB model
func NewUsersModel(conn *gorm.DB) UsersModel {
	return &customUsersModel{
		conn: conn,
	}
}

func (m *customUsersModel) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var count int64
	err := m.conn.WithContext(ctx).
		Model(&Users{}).
		Where("username = ? AND deleted_at IS NULL", username).
		Limit(1).
		Count(&count).Error
	return count > 0, err
}

func (m *customUsersModel) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := m.conn.WithContext(ctx).
		Model(&Users{}).
		Where("email = ? AND deleted_at IS NULL", email).
		Limit(1).
		Count(&count).Error
	return count > 0, err
}

func (m *customUsersModel) FindForAuthenticationByUsername(ctx context.Context, username string) (*Users, error) {
	user := new(Users)
	err := m.conn.WithContext(ctx).
		Where("username = ? AND deleted_at IS NULL", username).
		First(user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}
func (m *customUsersModel) FindForAuthenticationByEmail(ctx context.Context, email string) (*Users, error) {
	user := new(Users)
	err := m.conn.WithContext(ctx).
		Where("email = ? AND deleted_at IS NULL", email).
		First(user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}
func (m *customUsersModel) GetUserPasswordHash(ctx context.Context, userID int64) (string, error) {
	var user Users
	err := m.conn.WithContext(ctx).
		Select("password_hash").
		Where("id = ? AND deleted_at IS NULL", userID).
		Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return user.PasswordHash, nil
}

const publicUserFields = "id, username, email, role, status, school_id, created_at, updated_at, deleted_at"

type UserProfilePatch struct {
	Username *string
	SchoolID *int64
}

func (m *customUsersModel) InsertOne(ctx context.Context, user *Users) error {
	return normalizeError(m.conn.WithContext(ctx).Create(user).Error)
}
func (m *customUsersModel) FindActiveByID(ctx context.Context, id int64) (*Users, error) {
	var user Users
	err := m.conn.WithContext(ctx).Select(publicUserFields).Where("id = ? AND deleted_at IS NULL", id).Take(&user).Error
	if err != nil {
		return nil, normalizeError(err)
	}
	return &user, nil
}
func (m *customUsersModel) ListActiveUsers(ctx context.Context, page, size int) ([]Users, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	q := m.conn.WithContext(ctx).Model(&Users{}).Where("deleted_at IS NULL")
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	users := make([]Users, 0)
	err := q.Select(publicUserFields).Order("id").Limit(size).Offset((page - 1) * size).Find(&users).Error
	return users, total, err
}
func (m *customUsersModel) UpdateProfile(ctx context.Context, id int64, patch UserProfilePatch) error {
	return m.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user Users
		if err := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND deleted_at IS NULL", id).Take(&user).Error; err != nil {
			return normalizeError(err)
		}
		updates := map[string]any{}
		if patch.Username != nil {
			updates["username"] = *patch.Username
		}
		if patch.SchoolID != nil {
			if *patch.SchoolID < 0 {
				return ErrConflict
			}
			if *patch.SchoolID == 0 {
				updates["school_id"] = nil
			} else {
				var school Schools
				if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ? AND status = 1", *patch.SchoolID).Take(&school).Error; err != nil {
					return normalizeError(err)
				}
				updates["school_id"] = *patch.SchoolID
			}
		}
		if len(updates) == 0 {
			return nil
		}
		return normalizeError(tx.Model(&Users{}).Where("id = ?", id).Updates(updates).Error)
	})
}
func (m *customUsersModel) SoftDeleteAndAnonymize(ctx context.Context, id int64) error {
	return m.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user Users
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ? AND deleted_at IS NULL", id).Take(&user).Error; err != nil {
			return normalizeError(err)
		}
		var random [12]byte
		if _, err := rand.Read(random[:]); err != nil {
			return err
		}
		name := "deleted_" + hex.EncodeToString(random[:])
		now := time.Now().UTC()
		if err := tx.Model(&Users{}).Where("id = ?", id).Updates(map[string]any{"username": name, "email": name + "@invalid.local", "password_hash": "!deleted", "school_id": nil, "status": 4, "deleted_at": now}).Error; err != nil {
			return normalizeError(err)
		}
		return NewUserSessionsModel(tx).RevokeAll(ctx, id, 0, "user_deleted", now)
	})
}
