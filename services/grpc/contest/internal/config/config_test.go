package config

import (
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

func TestContestConfigLoadsMySQLDSNFromEnvironment(t *testing.T) {
	t.Setenv("CONTEST_MYSQL_DSN", "user:password@tcp(127.0.0.1:3306)/contest?parseTime=true&loc=UTC")
	var c Config
	if err := conf.Load(filepath.Join("..", "..", "etc", "contest.yaml"), &c, conf.UseEnv()); err != nil {
		t.Fatalf("load contest config: %v", err)
	}
	if c.Mysql.DSN == "" {
		t.Fatal("Mysql.DSN was not expanded from CONTEST_MYSQL_DSN")
	}
}
