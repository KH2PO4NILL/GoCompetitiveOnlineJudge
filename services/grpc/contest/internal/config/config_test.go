package config

import (
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

func TestContestConfigLoadsMySQLDSNFromEnvironment(t *testing.T) {
	t.Setenv("CONTEST_MYSQL_DSN", "user:password@tcp(127.0.0.1:3306)/contest?parseTime=true&loc=UTC")
	t.Setenv("CONTEST_INTERNAL_PROBLEM_TOKEN", "problem-token")
	t.Setenv("CONTEST_INTERNAL_SUBMIT_TOKEN", "submit-token")
	t.Setenv("AUTH_INTERNAL_CONTEST_TOKEN", "auth-token")
	var c Config
	if err := conf.Load(filepath.Join("..", "..", "etc", "contest.yaml"), &c, conf.UseEnv()); err != nil {
		t.Fatalf("load contest config: %v", err)
	}
	if c.Mysql.DSN == "" {
		t.Fatal("Mysql.DSN was not expanded from CONTEST_MYSQL_DSN")
	}
	if c.AuthRPC.Etcd.Key != "auth.rpc" {
		t.Fatalf("AuthRPC key = %q", c.AuthRPC.Etcd.Key)
	}
	if c.AuthRPCAuth.Name != "contest.rpc" || c.AuthRPCAuth.Token != "auth-token" {
		t.Fatalf("AuthRPCAuth = %+v", c.AuthRPCAuth)
	}
	if len(c.InternalAuth.Callers) != 2 || c.InternalAuth.Callers[0].Token != "problem-token" {
		t.Fatalf("InternalAuth = %+v", c.InternalAuth)
	}
}
