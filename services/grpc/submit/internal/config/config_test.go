package config

import (
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

func TestSubmitConfigLoadsMySQLDSNFromEnvironment(t *testing.T) {
	t.Setenv("SUBMIT_MYSQL_DSN", "user:password@tcp(127.0.0.1:3306)/submit?parseTime=true&loc=UTC")
	var c Config
	if err := conf.Load(filepath.Join("..", "..", "etc", "submit.yaml"), &c, conf.UseEnv()); err != nil {
		t.Fatalf("load submit config: %v", err)
	}
	if c.Mysql.DSN == "" {
		t.Fatal("Mysql.DSN was not expanded from SUBMIT_MYSQL_DSN")
	}
}
