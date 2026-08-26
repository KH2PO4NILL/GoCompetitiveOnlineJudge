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
	testDataUploadsModel interface {
		CreateTable() error
		Insert(ctx context.Context, data []*TestDataUploads) error
		InsertOne(ctx context.Context, data *TestDataUploads) error

		FindOne(ctx context.Context, uploadId string) (*TestDataUploads, error)
		List(ctx context.Context, req TestDataUploadsListReq) ([]TestDataUploads, int64, error)
		Update(ctx context.Context, data *TestDataUploads) error

		Delete(ctx context.Context, uploadId string) error
	}

	defaultTestDataUploadsModel struct {
		conn  *gorm.DB
		table string
	}

	TestDataUploads struct {
		UploadId string `json:"upload_id" gorm:"column:upload_id"`

		ProblemId int64 `json:"problem_id" gorm:"column:problem_id"`

		ObjectKey string `json:"object_key" gorm:"column:object_key"`

		FileName string `json:"file_name" gorm:"column:file_name"`

		ContentLength int64 `json:"content_length" gorm:"column:content_length"`

		Checksum string `json:"checksum" gorm:"column:checksum"`

		Status int64 `json:"status" gorm:"column:status"`

		ExpiresAt time.Time `json:"expires_at" gorm:"column:expires_at"`

		CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`

		FinalizedAt sql.NullTime `json:"finalized_at" gorm:"column:finalized_at"`
	}
)

func newTestDataUploadsModel(conn *gorm.DB) *defaultTestDataUploadsModel {
	return &defaultTestDataUploadsModel{
		conn:  conn,
		table: "`test_data_uploads`",
	}
}

func (m *defaultTestDataUploadsModel) Delete(ctx context.Context, uploadId string) error {
	return m.conn.WithContext(ctx).Where("`upload_id` = ?", uploadId).Delete(&TestDataUploads{}).Error
}

func (m *defaultTestDataUploadsModel) FindOne(ctx context.Context, uploadId string) (*TestDataUploads, error) {
	model := &TestDataUploads{}
	err := m.conn.WithContext(ctx).Where("`upload_id` = ?", uploadId).First(model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return model, nil
}

type TestDataUploadsListReq struct {
	Page int `json:"page"` // Page number
	Size int `json:"size"` // Number of items per page
}

func (m *defaultTestDataUploadsModel) List(ctx context.Context, req TestDataUploadsListReq) ([]TestDataUploads, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}
	if req.Size > 100 {
		req.Size = 100
	}

	total := int64(0)
	list := make([]TestDataUploads, 0)
	session := m.conn.WithContext(ctx).Model(&TestDataUploads{})
	err := session.Count(&total).Error
	if err != nil {
		return list, total, err
	}
	if total == 0 {
		return list, total, err
	}

	offset := (req.Page - 1) * req.Size
	err = session.Order("`upload_id` DESC").Limit(req.Size).Offset(offset).Find(&list).Error

	return list, total, err
}

func (m *defaultTestDataUploadsModel) CreateTable() error {
	return m.conn.AutoMigrate(&TestDataUploads{})
}

func (m *defaultTestDataUploadsModel) Insert(ctx context.Context, data []*TestDataUploads) error {
	return m.conn.WithContext(ctx).Create(data).Error
}

func (m *defaultTestDataUploadsModel) InsertOne(ctx context.Context, data *TestDataUploads) error {
	return m.conn.WithContext(ctx).Create(data).Error
}

func (m *defaultTestDataUploadsModel) Update(ctx context.Context, data *TestDataUploads) error {
	return m.conn.WithContext(ctx).Model(&TestDataUploads{}).Where("`upload_id` = ?", data.UploadId).Updates(data).Error
}

func (m *defaultTestDataUploadsModel) tableName() string {
	return m.table
}
