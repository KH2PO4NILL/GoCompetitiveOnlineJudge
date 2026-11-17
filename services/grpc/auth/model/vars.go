package model

import (
	"errors"
	mysql "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var (
	ErrNotFound  = gorm.ErrRecordNotFound
	ErrDuplicate = errors.New("duplicate record")
	ErrConflict  = errors.New("record state changed")
)

func normalizeError(err error) error {
	var duplicate *mysql.MySQLError
	if errors.As(err, &duplicate) && duplicate.Number == 1062 {
		return errors.Join(ErrDuplicate, err)
	}
	return err
}
