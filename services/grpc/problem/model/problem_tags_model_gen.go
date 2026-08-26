// Code scaffolded by goctl. Only init once, Safe to edit.
// gorm

package model

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type (
	problemTagsModel interface {
		CreateTable() error
		Insert(ctx context.Context, data []*ProblemTags) error
		InsertOne(ctx context.Context, data *ProblemTags) error

		FindOne(ctx context.Context, id int64) (*ProblemTags, error)
		List(ctx context.Context, req ProblemTagsListReq) ([]ProblemTags, int64, error)
		Update(ctx context.Context, data *ProblemTags) error

		Delete(ctx context.Context, id int64) error
	}

	defaultProblemTagsModel struct {
		conn  *gorm.DB
		table string
	}

	ProblemTags struct {
		Id int64 `json:"id" gorm:"column:id"`

		ProblemId int64 `json:"problem_id" gorm:"column:problem_id"`

		Tag string `json:"tag" gorm:"column:tag"`
	}
)

func newProblemTagsModel(conn *gorm.DB) *defaultProblemTagsModel {
	return &defaultProblemTagsModel{
		conn:  conn,
		table: "`problem_tags`",
	}
}

func (m *defaultProblemTagsModel) Delete(ctx context.Context, id int64) error {
	return m.conn.WithContext(ctx).Where("`id` = ?", id).Delete(&ProblemTags{}).Error
}

func (m *defaultProblemTagsModel) FindOne(ctx context.Context, id int64) (*ProblemTags, error) {
	model := &ProblemTags{}
	err := m.conn.WithContext(ctx).Where("`id` = ?", id).First(model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return model, nil
}

type ProblemTagsListReq struct {
	Page int `json:"page"` // Page number
	Size int `json:"size"` // Number of items per page
}

func (m *defaultProblemTagsModel) List(ctx context.Context, req ProblemTagsListReq) ([]ProblemTags, int64, error) {
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
	list := make([]ProblemTags, 0)
	session := m.conn.WithContext(ctx).Model(&ProblemTags{})
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

func (m *defaultProblemTagsModel) CreateTable() error {
	return m.conn.AutoMigrate(&ProblemTags{})
}

func (m *defaultProblemTagsModel) Insert(ctx context.Context, data []*ProblemTags) error {
	return m.conn.WithContext(ctx).Create(data).Error
}

func (m *defaultProblemTagsModel) InsertOne(ctx context.Context, data *ProblemTags) error {
	return m.conn.WithContext(ctx).Create(data).Error
}

func (m *defaultProblemTagsModel) Update(ctx context.Context, data *ProblemTags) error {
	return m.conn.WithContext(ctx).Model(&ProblemTags{}).Where("`id` = ?", data.Id).Updates(data).Error
}

func (m *defaultProblemTagsModel) tableName() string {
	return m.table
}
