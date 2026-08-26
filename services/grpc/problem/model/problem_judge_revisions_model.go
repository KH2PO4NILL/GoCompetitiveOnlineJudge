// Code scaffolded by goctl. Only init once, Safe to edit.
// gorm

package model

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var _ ProblemJudgeRevisionsModel = (*customProblemJudgeRevisionsModel)(nil)

type (
	// ProblemJudgeRevisionsModel 原生 DB 接口，不走缓存
	ProblemJudgeRevisionsModel interface {
		Insert(ctx context.Context, data []*ProblemJudgeRevisions) error
		InsertOne(ctx context.Context, data *ProblemJudgeRevisions) error
		FindOne(ctx context.Context, id int64) (*ProblemJudgeRevisions, error)
		List(ctx context.Context, req ProblemJudgeRevisionsListReq) ([]ProblemJudgeRevisions, int64, error)
		FindByProblemRevision(ctx context.Context, problemID, revision int64) (*ProblemJudgeRevisions, error)
	}

	customProblemJudgeRevisionsModel struct {
		*defaultProblemJudgeRevisionsModel
	}
)

// NewProblemJudgeRevisionsModel 创建纯 DB model
func NewProblemJudgeRevisionsModel(conn *gorm.DB) ProblemJudgeRevisionsModel {
	return &customProblemJudgeRevisionsModel{
		defaultProblemJudgeRevisionsModel: newProblemJudgeRevisionsModel(conn),
	}
}

func (m *customProblemJudgeRevisionsModel) FindByProblemRevision(
	ctx context.Context,
	problemID int64,
	revision int64,
) (*ProblemJudgeRevisions, error) {
	snapshot := new(ProblemJudgeRevisions)
	err := m.conn.WithContext(ctx).
		Where("problem_id = ? AND revision = ?", problemID, revision).
		Take(snapshot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}
