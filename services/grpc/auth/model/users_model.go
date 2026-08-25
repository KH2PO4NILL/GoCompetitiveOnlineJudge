// Code scaffolded by goctl. Only init once, Safe to edit.
// gorm

package model

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var _ UsersModel = (*customUsersModel)(nil)

type (
	// UsersModel 原生 DB 接口，不走缓存
	UsersModel interface {
		usersModel
		FindOneByUsername(ctx context.Context, username string) (*Users, error)
		FindOneByEmail(ctx context.Context, email string) (*Users, error)
		ExistsByUsername(ctx context.Context, username string) (bool, error)
		ExistsByEmail(ctx context.Context, email string) (bool, error)
		GetUserPasswordHash(ctx context.Context, userID int64) (string, error)
		UpdateUserPasswordHash(ctx context.Context, userID int64, newPasswordHash string) error
	}

	customUsersModel struct {
		*defaultUsersModel
	}
)

// NewUsersModel 创建纯 DB model
func NewUsersModel(conn *gorm.DB) UsersModel {
	return &customUsersModel{
		defaultUsersModel: newUsersModel(conn),
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

func (m *customUsersModel) FindOneByUsername(ctx context.Context, username string) (*Users, error) {
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
func (m *customUsersModel) FindOneByEmail(ctx context.Context, email string) (*Users, error) {
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

func (m *customUsersModel) UpdateUserPasswordHash(ctx context.Context, userID int64, newPasswordHash string) error {
	result := m.conn.WithContext(ctx).
		Model(&Users{}).
		Where("id = ? AND deleted_at IS NULL", userID).
		Update("password_hash", newPasswordHash)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}

	return nil
}
