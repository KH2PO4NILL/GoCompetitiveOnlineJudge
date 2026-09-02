package config

import (
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

func TestSubmitConfigLoadsMySQLDSNFromEnvironment(t *testing.T) {
	t.Setenv("SUBMIT_MYSQL_DSN", "user:password@tcp(127.0.0.1:3306)/submit?parseTime=true&loc=UTC")
	t.Setenv("AUTH_INTERNAL_SUBMIT_TOKEN", "auth-token")
	t.Setenv("PROBLEM_INTERNAL_SUBMIT_TOKEN", "problem-token")
	t.Setenv("CONTEST_INTERNAL_SUBMIT_TOKEN", "contest-token")
	t.Setenv("SUBMIT_INTERNAL_JUDGE_TOKEN", "judge-token")
	var c Config
	if err := conf.Load(filepath.Join("..", "..", "etc", "submit.yaml"), &c, conf.UseEnv()); err != nil {
		t.Fatalf("load submit config: %v", err)
	}
	if c.Mysql.DSN == "" {
		t.Fatal("Mysql.DSN was not expanded from SUBMIT_MYSQL_DSN")
	}
	if c.AuthRPCAuth.Token != "auth-token" || c.ProblemRPCAuth.Token != "problem-token" ||
		c.ContestRPCAuth.Token != "contest-token" || len(c.InternalAuth.Callers) != 1 ||
		c.InternalAuth.Callers[0].Token != "judge-token" {
		t.Fatalf("submit service credentials were not expanded: %+v", c)
	}
	if c.Lease.DurationSeconds <= 0 || c.Limits.MaxPageSize <= 0 || !c.RateLimit.Enabled {
		t.Fatalf("submit runtime policy was not loaded: %+v", c)
	}
}
