package config

import (
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

func TestProblemConfigLoadsAuthAndContestRPCClients(t *testing.T) {
	t.Setenv("PROBLEM_MYSQL_DSN", "user:password@tcp(127.0.0.1:3306)/database")
	t.Setenv("PROBLEM_REDIS_PASSWORD", "redis-password")
	t.Setenv("PROBLEM_OBJECT_STORAGE_ENDPOINT", "http://127.0.0.1:9000")
	t.Setenv("PROBLEM_OBJECT_STORAGE_REGION", "us-east-1")
	t.Setenv("PROBLEM_OBJECT_STORAGE_BUCKET", "problem-test-data")
	t.Setenv("PROBLEM_OBJECT_STORAGE_ACCESS_KEY", "access-key")
	t.Setenv("PROBLEM_OBJECT_STORAGE_SECRET_KEY", "secret-key")
	t.Setenv("PROBLEM_INTERNAL_SUBMIT_TOKEN", "submit-token")
	t.Setenv("PROBLEM_INTERNAL_JUDGE_TOKEN", "judge-token")
	t.Setenv("CONTEST_INTERNAL_PROBLEM_TOKEN", "contest-token")
	t.Setenv("AUTH_INTERNAL_PROBLEM_TOKEN", "auth-token")

	var c Config
	if err := conf.Load(filepath.Join("..", "..", "etc", "problem.yaml"), &c, conf.UseEnv()); err != nil {
		t.Fatalf("load problem config: %v", err)
	}
	if c.AuthRPC.Etcd.Key != "auth.rpc" {
		t.Fatalf("AuthRPC key = %q", c.AuthRPC.Etcd.Key)
	}
	if c.AuthRPCAuth.Name != "problem.rpc" || c.AuthRPCAuth.Token != "auth-token" {
		t.Fatalf("AuthRPCAuth = %+v", c.AuthRPCAuth)
	}
	if c.ContestRPC.Etcd.Key != "contest.rpc" {
		t.Fatalf("ContestRPC key = %q", c.ContestRPC.Etcd.Key)
	}
	if c.ContestRPCAuth.Name != "problem.rpc" || c.ContestRPCAuth.Token != "contest-token" {
		t.Fatalf("ContestRPCAuth = %+v", c.ContestRPCAuth)
	}
}
