// User schema mirrors SQL migrations. Schema changes are applied by migrations.
package model

import (
	"database/sql"
	"time"
)

type (
	Users struct {
		Id           int64         `json:"id" gorm:"id"`
		Username     string        `json:"username" gorm:"username"`
		Email        string        `json:"email" gorm:"email"`
		PasswordHash string        `json:"-" gorm:"password_hash"`
		Role         int64         `json:"role" gorm:"role"`
		Status       int64         `json:"status" gorm:"status"`
		SchoolId     sql.NullInt64 `json:"school_id" gorm:"school_id"`
		CreatedAt    time.Time     `json:"created_at" gorm:"created_at"`
		UpdatedAt    time.Time     `json:"updated_at" gorm:"updated_at"`
		DeletedAt    sql.NullTime  `json:"deleted_at" gorm:"deleted_at"`
	}
)
