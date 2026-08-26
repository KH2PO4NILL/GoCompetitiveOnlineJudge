// Code scaffolded by goctl. Only init once, Safe to edit.
// gorm

package model

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type (
	testCasesModel interface {
		CreateTable() error
		Insert(ctx context.Context, data []*TestCases) error
		InsertOne(ctx context.Context, data *TestCases) error

		FindOne(ctx context.Context, id int64) (*TestCases, error)
		List(ctx context.Context, req TestCasesListReq) ([]TestCases, int64, error)
		Update(ctx context.Context, data *TestCases) error

		Delete(ctx context.Context, id int64) error
	}

	defaultTestCasesModel struct {
		conn  *gorm.DB
		table string
	}

	TestCases struct {
		Id int64 `json:"id" gorm:"column:id"`

		TestDataVersionId int64 `json:"test_data_version_id" gorm:"column:test_data_version_id"`

		Name string `json:"name" gorm:"column:name"`

		Score int64 `json:"score" gorm:"column:score"`

		Ordinal int64 `json:"ordinal" gorm:"column:ordinal"`

		Enabled bool `json:"enabled" gorm:"column:enabled"`

		GroupName string `json:"group_name" gorm:"column:group_name"`
	}
)

func newTestCasesModel(conn *gorm.DB) *defaultTestCasesModel {
	return &defaultTestCasesModel{
		conn:  conn,
		table: "`test_cases`",
	}
}

func (m *defaultTestCasesModel) Delete(ctx context.Context, id int64) error {
	return m.conn.WithContext(ctx).Where("`id` = ?", id).Delete(&TestCases{}).Error
}

func (m *defaultTestCasesModel) FindOne(ctx context.Context, id int64) (*TestCases, error) {
	model := &TestCases{}
	err := m.conn.WithContext(ctx).Where("`id` = ?", id).First(model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return model, nil
}

type TestCasesListReq struct {
	Page int `json:"page"` // Page number
	Size int `json:"size"` // Number of items per page
}

func (m *defaultTestCasesModel) List(ctx context.Context, req TestCasesListReq) ([]TestCases, int64, error) {
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
	list := make([]TestCases, 0)
	session := m.conn.WithContext(ctx).Model(&TestCases{})
	err := session.Count(&total).Error
	if err != nil {
		return list, total, err
	}
	if total == 0 {
		return list, total, err
	}

	offset := (req.Page - 1) * req.Size
	err = session.Order("`id` DESC").Limit(req.Size).Offset(offset).Find(&list).Error

	return list, total, err
}

func (m *defaultTestCasesModel) CreateTable() error {
	return m.conn.AutoMigrate(&TestCases{})
}

func (m *defaultTestCasesModel) Insert(ctx context.Context, data []*TestCases) error {
	return m.conn.WithContext(ctx).Create(data).Error
}

func (m *defaultTestCasesModel) InsertOne(ctx context.Context, data *TestCases) error {
	return m.conn.WithContext(ctx).Create(data).Error
}

func (m *defaultTestCasesModel) Update(ctx context.Context, data *TestCases) error {
	return m.conn.WithContext(ctx).Model(&TestCases{}).Where("`id` = ?", data.Id).Updates(data).Error
}

func (m *defaultTestCasesModel) tableName() string {
	return m.table
}
