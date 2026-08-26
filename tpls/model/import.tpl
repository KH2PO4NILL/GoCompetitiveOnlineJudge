import (
	"context"
	"errors"
	{{if .time}}"time"{{end}}
	{{$needsSQL := false}}{{range .data.Fields}}{{if or (eq .DataType "sql.NullBool") (eq .DataType "sql.NullFloat64") (eq .DataType "sql.NullInt32") (eq .DataType "sql.NullInt64") (eq .DataType "sql.NullString") (eq .DataType "sql.NullTime")}}{{$needsSQL = true}}{{end}}{{end}}{{if $needsSQL}}"database/sql"{{end}}

	"gorm.io/gorm"
	{{.third}}
)
