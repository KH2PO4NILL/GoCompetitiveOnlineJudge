// Code scaffolded by goctl. Only init once, Safe to edit.
// gorm

package model

import (
	"context"

	"gorm.io/gorm"
)

var _ ProblemTagsModel = (*customProblemTagsModel)(nil)

type (
	// ProblemTagsModel 原生 DB 接口，不走缓存
	ProblemTagsModel interface {
		Insert(ctx context.Context, data []*ProblemTags) error
		InsertOne(ctx context.Context, data *ProblemTags) error
		FindOne(ctx context.Context, id int64) (*ProblemTags, error)
		List(ctx context.Context, req ProblemTagsListReq) ([]ProblemTags, int64, error)
		FindByProblemID(ctx context.Context, problemID int64) ([]ProblemTags, error)
	}

	customProblemTagsModel struct {
		*defaultProblemTagsModel
	}
)

// NewProblemTagsModel 创建纯 DB model
func NewProblemTagsModel(conn *gorm.DB) ProblemTagsModel {
	return &customProblemTagsModel{
		defaultProblemTagsModel: newProblemTagsModel(conn),
	}
}

func (m *customProblemTagsModel) FindByProblemID(
	ctx context.Context,
	problemID int64,
) ([]ProblemTags, error) {
	tags := make([]ProblemTags, 0)
	err := m.conn.WithContext(ctx).
		Where("problem_id = ?", problemID).
		Order("id ASC").
		Find(&tags).Error
	return tags, err
}
