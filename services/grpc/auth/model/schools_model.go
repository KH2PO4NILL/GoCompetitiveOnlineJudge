// Code scaffolded by goctl. Only init once, Safe to edit.
// gorm

package model

import "gorm.io/gorm"

var _ SchoolsModel = (*customSchoolsModel)(nil)

type (
	// SchoolsModel 原生 DB 接口，不走缓存
	SchoolsModel interface {
		schoolsModel
	}

	customSchoolsModel struct {
		*defaultSchoolsModel
	}
)

// NewSchoolsModel 创建纯 DB model
func NewSchoolsModel(conn *gorm.DB) SchoolsModel {
	return &customSchoolsModel{
		defaultSchoolsModel: newSchoolsModel(conn),
	}
}
