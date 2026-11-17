// School schema mirrors SQL migrations.
package model

import (
	"database/sql"
	"time"
)

type (
	Schools struct {
		Id        int64          `json:"id" gorm:"id"`
		Name      string         `json:"name" gorm:"name"`
		ShortName sql.NullString `json:"short_name" gorm:"short_name"`
		Country   sql.NullString `json:"country" gorm:"country"`
		Province  sql.NullString `json:"province" gorm:"province"`
		City      sql.NullString `json:"city" gorm:"city"`
		Status    int64          `json:"status" gorm:"status"`
		CreatedAt time.Time      `json:"created_at" gorm:"created_at"`
		UpdatedAt time.Time      `json:"updated_at" gorm:"updated_at"`
	}
)
