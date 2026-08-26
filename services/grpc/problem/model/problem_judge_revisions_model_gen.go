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
	problemJudgeRevisionsModel interface {
		CreateTable() error
		Insert(ctx context.Context, data []*ProblemJudgeRevisions) error
		InsertOne(ctx context.Context, data *ProblemJudgeRevisions) error

		FindOne(ctx context.Context, id int64) (*ProblemJudgeRevisions, error)
		List(ctx context.Context, req ProblemJudgeRevisionsListReq) ([]ProblemJudgeRevisions, int64, error)
		Update(ctx context.Context, data *ProblemJudgeRevisions) error

		Delete(ctx context.Context, id int64) error
	}

	defaultProblemJudgeRevisionsModel struct {
		conn  *gorm.DB
		table string
	}

	ProblemJudgeRevisions struct {
		Id int64 `json:"id" gorm:"column:id"`

		ProblemId int64 `json:"problem_id" gorm:"column:problem_id"`

		Revision int64 `json:"revision" gorm:"column:revision"`

		TimeLimitMs int64 `json:"time_limit_ms" gorm:"column:time_limit_ms"`

		MemoryLimitMb int64 `json:"memory_limit_mb" gorm:"column:memory_limit_mb"`

		AllowedLanguagesJson string `json:"allowed_languages_json" gorm:"column:allowed_languages_json"`

		CheckerType int64 `json:"checker_type" gorm:"column:checker_type"`

		CheckerFileKey sql.NullString `json:"checker_file_key" gorm:"column:checker_file_key"`

		InteractorFileKey sql.NullString `json:"interactor_file_key" gorm:"column:interactor_file_key"`

		TestDataVersionId int64 `json:"test_data_version_id" gorm:"column:test_data_version_id"`

		PublishedAt time.Time `json:"published_at" gorm:"column:published_at"`
	}
)

func newProblemJudgeRevisionsModel(conn *gorm.DB) *defaultProblemJudgeRevisionsModel {
	return &defaultProblemJudgeRevisionsModel{
		conn:  conn,
		table: "`problem_judge_revisions`",
	}
}

func (m *defaultProblemJudgeRevisionsModel) Delete(ctx context.Context, id int64) error {
	return m.conn.WithContext(ctx).Where("`id` = ?", id).Delete(&ProblemJudgeRevisions{}).Error
}

func (m *defaultProblemJudgeRevisionsModel) FindOne(ctx context.Context, id int64) (*ProblemJudgeRevisions, error) {
	model := &ProblemJudgeRevisions{}
	err := m.conn.WithContext(ctx).Where("`id` = ?", id).First(model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return model, nil
}

type ProblemJudgeRevisionsListReq struct {
	Page int `json:"page"` // Page number
	Size int `json:"size"` // Number of items per page
}

func (m *defaultProblemJudgeRevisionsModel) List(ctx context.Context, req ProblemJudgeRevisionsListReq) ([]ProblemJudgeRevisions, int64, error) {
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
	list := make([]ProblemJudgeRevisions, 0)
	session := m.conn.WithContext(ctx).Model(&ProblemJudgeRevisions{})
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

func (m *defaultProblemJudgeRevisionsModel) CreateTable() error {
	return m.conn.AutoMigrate(&ProblemJudgeRevisions{})
}

func (m *defaultProblemJudgeRevisionsModel) Insert(ctx context.Context, data []*ProblemJudgeRevisions) error {
	return m.conn.WithContext(ctx).Create(data).Error
}

func (m *defaultProblemJudgeRevisionsModel) InsertOne(ctx context.Context, data *ProblemJudgeRevisions) error {
	return m.conn.WithContext(ctx).Create(data).Error
}

func (m *defaultProblemJudgeRevisionsModel) Update(ctx context.Context, data *ProblemJudgeRevisions) error {
	return m.conn.WithContext(ctx).Model(&ProblemJudgeRevisions{}).Where("`id` = ?", data.Id).Updates(data).Error
}

func (m *defaultProblemJudgeRevisionsModel) tableName() string {
	return m.table
}
