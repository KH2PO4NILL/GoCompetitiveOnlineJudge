// Code scaffolded by goctl. Only init once, Safe to edit.
// gorm

package model

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var _ ProblemsModel = (*customProblemsModel)(nil)

type (
	// ProblemsModel 原生 DB 接口，不走缓存
	ProblemsModel interface {
		Insert(ctx context.Context, data []*Problems) error
		InsertOne(ctx context.Context, data *Problems) error
		FindOne(ctx context.Context, id int64) (*Problems, error)
		List(ctx context.Context, req ProblemsListReq) ([]Problems, int64, error)
	}

	customProblemsModel struct {
		*defaultProblemsModel
	}
)

// NewProblemsModel 创建纯 DB model
func NewProblemsModel(conn *gorm.DB) ProblemsModel {
	return &customProblemsModel{
		defaultProblemsModel: newProblemsModel(conn),
	}
}

// FindOne hides soft-deleted rows. Restore flows should use the aggregate
// ProblemModel, which deliberately queries deleted rows when required.
func (m *customProblemsModel) FindOne(ctx context.Context, id int64) (*Problems, error) {
	problem := new(Problems)
	err := m.conn.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		Take(problem).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return problem, nil
}

func (m *customProblemsModel) List(
	ctx context.Context,
	req ProblemsListReq,
) ([]Problems, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}
	if req.Size > 100 {
		req.Size = 100
	}

	query := m.conn.WithContext(ctx).
		Model(&Problems{}).
		Where("deleted_at IS NULL")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	problems := make([]Problems, 0)
	if total == 0 {
		return problems, 0, nil
	}
	err := query.
		Order("id DESC").
		Limit(req.Size).
		Offset((req.Page - 1) * req.Size).
		Find(&problems).Error
	return problems, total, err
}
