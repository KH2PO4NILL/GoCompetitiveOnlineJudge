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
	problemsModel interface {
		CreateTable() error
		Insert(ctx context.Context, data []*Problems) error
		InsertOne(ctx context.Context, data *Problems) error

		FindOne(ctx context.Context, id int64) (*Problems, error)
		List(ctx context.Context, req ProblemsListReq) ([]Problems, int64, error)
		Update(ctx context.Context, data *Problems) error

		Delete(ctx context.Context, id int64) error
	}

	defaultProblemsModel struct {
		conn  *gorm.DB
		table string
	}

	Problems struct {
		Id int64 `json:"id" gorm:"column:id"`

		AuthorId int64 `json:"author_id" gorm:"column:author_id"`

		Title string `json:"title" gorm:"column:title"`

		Description string `json:"description" gorm:"column:description"`

		InputDescription string `json:"input_description" gorm:"column:input_description"`

		OutputDescription string `json:"output_description" gorm:"column:output_description"`

		SamplesJson string `json:"samples_json" gorm:"column:samples_json"`

		Hint string `json:"hint" gorm:"column:hint"`

		Difficulty int64 `json:"difficulty" gorm:"column:difficulty"`

		Source string `json:"source" gorm:"column:source"`

		TimeLimitMs int64 `json:"time_limit_ms" gorm:"column:time_limit_ms"`

		MemoryLimitMb int64 `json:"memory_limit_mb" gorm:"column:memory_limit_mb"`

		AllowedLanguagesJson string `json:"allowed_languages_json" gorm:"column:allowed_languages_json"`

		CheckerType int64 `json:"checker_type" gorm:"column:checker_type"`

		CheckerFileKey sql.NullString `json:"checker_file_key" gorm:"column:checker_file_key"`

		InteractorFileKey sql.NullString `json:"interactor_file_key" gorm:"column:interactor_file_key"`

		CurrentTestDataVersionId sql.NullInt64 `json:"current_test_data_version_id" gorm:"column:current_test_data_version_id"`

		LatestPublishedRevision sql.NullInt64 `json:"latest_published_revision" gorm:"column:latest_published_revision"`

		Status int64 `json:"status" gorm:"column:status"`

		Visibility int64 `json:"visibility" gorm:"column:visibility"`

		Revision int64 `json:"revision" gorm:"column:revision"`

		PublishedAt sql.NullTime `json:"published_at" gorm:"column:published_at"`

		CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`

		UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`

		DeletedAt sql.NullTime `json:"deleted_at" gorm:"column:deleted_at"`

		DeletedFromStatus sql.NullInt64 `json:"deleted_from_status" gorm:"column:deleted_from_status"`
	}
)

func newProblemsModel(conn *gorm.DB) *defaultProblemsModel {
	return &defaultProblemsModel{
		conn:  conn,
		table: "`problems`",
	}
}

func (m *defaultProblemsModel) Delete(ctx context.Context, id int64) error {
	return m.conn.WithContext(ctx).Where("`id` = ?", id).Delete(&Problems{}).Error
}

func (m *defaultProblemsModel) FindOne(ctx context.Context, id int64) (*Problems, error) {
	model := &Problems{}
	err := m.conn.WithContext(ctx).Where("`id` = ?", id).First(model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return model, nil
}

type ProblemsListReq struct {
	Page int `json:"page"` // Page number
	Size int `json:"size"` // Number of items per page
}

func (m *defaultProblemsModel) List(ctx context.Context, req ProblemsListReq) ([]Problems, int64, error) {
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
	list := make([]Problems, 0)
	session := m.conn.WithContext(ctx).Model(&Problems{})
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

func (m *defaultProblemsModel) CreateTable() error {
	return m.conn.AutoMigrate(&Problems{})
}

func (m *defaultProblemsModel) Insert(ctx context.Context, data []*Problems) error {
	return m.conn.WithContext(ctx).Create(data).Error
}

func (m *defaultProblemsModel) InsertOne(ctx context.Context, data *Problems) error {
	return m.conn.WithContext(ctx).Create(data).Error
}

func (m *defaultProblemsModel) Update(ctx context.Context, data *Problems) error {
	return m.conn.WithContext(ctx).Model(&Problems{}).Where("`id` = ?", data.Id).Updates(data).Error
}

func (m *defaultProblemsModel) tableName() string {
	return m.table
}
