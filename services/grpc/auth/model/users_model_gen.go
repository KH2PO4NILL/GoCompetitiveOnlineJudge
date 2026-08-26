// Code scaffolded by goctl. Only init once, Safe to edit.
// gorm

package model

import (
	"context"
	"errors"
	"time"
	"database/sql"
	"gorm.io/gorm"
)

type (
	usersModel interface {
		CreateTable() error
		Insert(ctx context.Context, data []*Users) error
		InsertOne(ctx context.Context, data *Users) error
		FindOne(ctx context.Context, id int64) (*Users, error)
		List(ctx context.Context, req UsersListReq) ([]Users, int64, error)

		Update(ctx context.Context, data *Users) error
		Delete(ctx context.Context, id int64) error
	}

	defaultUsersModel struct {
		conn  *gorm.DB
		table string
	}

	Users struct {
		Id           int64         `json:"id" gorm:"id"`
		Username     string        `json:"username" gorm:"username"`
		Email        string        `json:"email" gorm:"email"`
		PasswordHash string        `json:"password_hash" gorm:"password_hash"`
		Role         int64         `json:"role" gorm:"role"`
		Status       int64         `json:"status" gorm:"status"`
		SchoolId     sql.NullInt64 `json:"school_id" gorm:"school_id"`
		CreatedAt    time.Time     `json:"created_at" gorm:"created_at"`
		UpdatedAt    time.Time     `json:"updated_at" gorm:"updated_at"`
		DeletedAt    sql.NullTime  `json:"deleted_at" gorm:"deleted_at"`
	}
)

func newUsersModel(conn *gorm.DB) *defaultUsersModel {
	return &defaultUsersModel{
		conn:  conn,
		table: "`users`",
	}
}

func (m *defaultUsersModel) Delete(ctx context.Context, id int64) error {
	return m.conn.WithContext(ctx).Where("id = ?", id).Delete(&Users{}).Error
}

func (m *defaultUsersModel) FindOne(ctx context.Context, id int64) (*Users, error) {
	model := &Users{}
	err := m.conn.WithContext(ctx).Where("id = ?", id).First(model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return model, nil
}

type UsersListReq struct {
	Page int `json:"page"` // Page number
	Size int `json:"size"` // Number of items per page
}

func (m *defaultUsersModel) List(ctx context.Context, req UsersListReq) ([]Users, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}

	total := int64(0)
	list := make([]Users, 0)
	session := m.conn.WithContext(ctx).Model(&Users{})
	err := session.Count(&total).Error
	if err != nil {
		return list, total, err
	}
	if total == 0 {
		return list, total, err
	}

	offset := (req.Page - 1) * req.Size
	err = session.Limit(req.Size).Offset(offset).Find(&list).Error

	return list, total, err
}

func (m *defaultUsersModel) CreateTable() error {
	return m.conn.AutoMigrate(&Users{})
}

func (m *defaultUsersModel) Insert(ctx context.Context, data []*Users) error {
	return m.conn.WithContext(ctx).Create(data).Error
}

func (m *defaultUsersModel) InsertOne(ctx context.Context, data *Users) error {
	return m.conn.WithContext(ctx).Create(data).Error
}

func (m *defaultUsersModel) Update(ctx context.Context, data *Users) error {
	return m.conn.WithContext(ctx).Model(&Users{}).Where("id = ?", data.Id).Updates(data).Error
}

func (m *defaultUsersModel) tableName() string {
	return m.table
}
