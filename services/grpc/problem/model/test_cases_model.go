// Code scaffolded by goctl. Only init once, Safe to edit.
// gorm

package model

import (
	"context"

	"gorm.io/gorm"
)

var _ TestCasesModel = (*customTestCasesModel)(nil)

type (
	// TestCasesModel 原生 DB 接口，不走缓存
	TestCasesModel interface {
		Insert(ctx context.Context, data []*TestCases) error
		InsertOne(ctx context.Context, data *TestCases) error
		FindOne(ctx context.Context, id int64) (*TestCases, error)
		List(ctx context.Context, req TestCasesListReq) ([]TestCases, int64, error)
		FindByTestDataVersionID(ctx context.Context, testDataVersionID int64) ([]TestCases, error)
	}

	customTestCasesModel struct {
		*defaultTestCasesModel
	}
)

// NewTestCasesModel 创建纯 DB model
func NewTestCasesModel(conn *gorm.DB) TestCasesModel {
	return &customTestCasesModel{
		defaultTestCasesModel: newTestCasesModel(conn),
	}
}

func (m *customTestCasesModel) FindByTestDataVersionID(
	ctx context.Context,
	testDataVersionID int64,
) ([]TestCases, error) {
	testCases := make([]TestCases, 0)
	err := m.conn.WithContext(ctx).
		Where("test_data_version_id = ?", testDataVersionID).
		Order("ordinal ASC").
		Find(&testCases).Error
	return testCases, err
}
