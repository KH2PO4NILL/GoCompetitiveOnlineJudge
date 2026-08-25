// Code scaffolded by goctl. Only init once, Safe to edit.
// gorm

package model

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gorm.io/gorm"
)

type (
	schoolsModel interface {
		CreateTable() error
		Insert(ctx context.Context, data []*Schools) error
		InsertOne(ctx context.Context, data *Schools) error
		FindOne(ctx context.Context, id int64) (*Schools, error)
		List(ctx context.Context, req SchoolsListReq) ([]Schools, int64, error)
		Update(ctx context.Context, data *Schools) error
		Delete(ctx context.Context, id int64) error
	}

	defaultSchoolsModel struct {
		conn  *gorm.DB
		table string
	}

	Schools struct {
		Id        int64          `json:"id" gorm:"id"`
		Name      string         `json:"name" gorm:"name"`
		ShortName sql.NullString `json:"short_name" gorm:"short_name"`
		Country   sql.NullString `json:"country" gorm:"country"`
		Province  sql.NullString `json:"province" gorm:"province"`
		City      sql.NullString `json:"city" gorm:"city"`
		Status    int64          `json:"status" gorm:"status"`
		CreatedAt time.Time      `json:"created_at" gorm:"created_at"`
		UpdatedAt time.Time      `json:"updated_at" gorm:"updated_at"`
	}
)

func newSchoolsModel(conn *gorm.DB) *defaultSchoolsModel {
	return &defaultSchoolsModel{
		conn:  conn,
		table: "`schools`",
	}
}

func (m *defaultSchoolsModel) Delete(ctx context.Context, id int64) error {
	return m.conn.WithContext(ctx).Where("id = ?", id).Delete(&Schools{}).Error
}

func (m *defaultSchoolsModel) FindOne(ctx context.Context, id int64) (*Schools, error) {
	model := &Schools{}
	err := m.conn.WithContext(ctx).Where("id = ?", id).First(model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return model, nil
}

type SchoolsListReq struct {
	Page int `json:"page"` // Page number
	Size int `json:"size"` // Number of items per page
}

func (m *defaultSchoolsModel) List(ctx context.Context, req SchoolsListReq) ([]Schools, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}

	total := int64(0)
	list := make([]Schools, 0)
	session := m.conn.WithContext(ctx).Model(&Schools{})
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

func (m *defaultSchoolsModel) CreateTable() error {
	return m.conn.AutoMigrate(&Schools{})
}

func (m *defaultSchoolsModel) Insert(ctx context.Context, data []*Schools) error {
	return m.conn.WithContext(ctx).Create(data).Error
}

func (m *defaultSchoolsModel) InsertOne(ctx context.Context, data *Schools) error {
	return m.conn.WithContext(ctx).Create(data).Error
}

func (m *defaultSchoolsModel) Update(ctx context.Context, data *Schools) error {
	return m.conn.WithContext(ctx).Model(&Schools{}).Where("id = ?", data.Id).Updates(data).Error
}

func (m *defaultSchoolsModel) tableName() string {
	return m.table
}
