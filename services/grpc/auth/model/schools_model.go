package model

import (
	"context"
	"database/sql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SchoolsModel interface {
	FindOne(context.Context, int64) (*Schools, error)
	InsertOne(context.Context, *Schools) error
	UpdateDetails(context.Context, int64, SchoolPatch) error
	Disable(context.Context, int64) error
}
type customSchoolsModel struct{ conn *gorm.DB }
type SchoolPatch struct {
	Name                               *string
	ShortName, Country, Province, City *sql.NullString
}

func NewSchoolsModel(conn *gorm.DB) SchoolsModel { return &customSchoolsModel{conn: conn} }
func (m *customSchoolsModel) FindOne(ctx context.Context, id int64) (*Schools, error) {
	var school Schools
	err := m.conn.WithContext(ctx).Where("id = ?", id).Take(&school).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &school, nil
}
func (m *customSchoolsModel) InsertOne(ctx context.Context, s *Schools) error {
	return normalizeError(m.conn.WithContext(ctx).Create(s).Error)
}
func (m *customSchoolsModel) UpdateDetails(ctx context.Context, id int64, p SchoolPatch) error {
	return m.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var school Schools
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", id).Take(&school).Error; err != nil {
			return normalizeError(err)
		}
		fields := map[string]any{}
		if p.Name != nil {
			fields["name"] = *p.Name
		}
		for column, value := range map[string]*sql.NullString{"short_name": p.ShortName, "country": p.Country, "province": p.Province, "city": p.City} {
			if value != nil {
				fields[column] = *value
			}
		}
		if len(fields) == 0 {
			return nil
		}
		return normalizeError(tx.Model(&Schools{}).Where("id = ?", id).Updates(fields).Error)
	})
}
func (m *customSchoolsModel) Disable(ctx context.Context, id int64) error {
	return m.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var school Schools
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", id).Take(&school).Error; err != nil {
			return normalizeError(err)
		}
		return tx.Model(&Schools{}).Where("id = ?", id).Update("status", 2).Error
	})
}
