// Code scaffolded by goctl. Only init once, Safe to edit.
// gorm

package model

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var _ TestDataVersionsModel = (*customTestDataVersionsModel)(nil)

type (
	// TestDataVersionsModel 原生 DB 接口，不走缓存
	TestDataVersionsModel interface {
		Insert(ctx context.Context, data []*TestDataVersions) error
		InsertOne(ctx context.Context, data *TestDataVersions) error
		FindOne(ctx context.Context, id int64) (*TestDataVersions, error)
		List(ctx context.Context, req TestDataVersionsListReq) ([]TestDataVersions, int64, error)
		FindByProblemVersion(ctx context.Context, problemID int64, version string) (*TestDataVersions, error)
	}

	customTestDataVersionsModel struct {
		*defaultTestDataVersionsModel
	}
)

// NewTestDataVersionsModel 创建纯 DB model
func NewTestDataVersionsModel(conn *gorm.DB) TestDataVersionsModel {
	return &customTestDataVersionsModel{
		defaultTestDataVersionsModel: newTestDataVersionsModel(conn),
	}
}

func (m *customTestDataVersionsModel) FindByProblemVersion(
	ctx context.Context,
	problemID int64,
	version string,
) (*TestDataVersions, error) {
	testDataVersion := new(TestDataVersions)
	err := m.conn.WithContext(ctx).
		Where("problem_id = ? AND version = ?", problemID, version).
		Take(testDataVersion).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return testDataVersion, nil
}
