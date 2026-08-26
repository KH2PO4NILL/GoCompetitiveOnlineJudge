// Code scaffolded by goctl. Only init once, Safe to edit.
// gorm

package model

import (
	"context"

	"gorm.io/gorm"
)

var _ TestDataUploadsModel = (*customTestDataUploadsModel)(nil)

type (
	// TestDataUploadsModel 原生 DB 接口，不走缓存
	TestDataUploadsModel interface {
		Insert(ctx context.Context, data []*TestDataUploads) error
		InsertOne(ctx context.Context, data *TestDataUploads) error
		FindOne(ctx context.Context, uploadID string) (*TestDataUploads, error)
		List(ctx context.Context, req TestDataUploadsListReq) ([]TestDataUploads, int64, error)
	}

	customTestDataUploadsModel struct {
		*defaultTestDataUploadsModel
	}
)

// NewTestDataUploadsModel 创建纯 DB model
func NewTestDataUploadsModel(conn *gorm.DB) TestDataUploadsModel {
	return &customTestDataUploadsModel{
		defaultTestDataUploadsModel: newTestDataUploadsModel(conn),
	}
}
