package config

import (
	"path/filepath"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"
	"github.com/zeromicro/go-zero/core/conf"
)

func TestContestConfigLoadsMySQLDSNFromEnvironment(t *testing.T) {
	t.Setenv("CONTEST_MYSQL_DSN", "user:password@tcp(127.0.0.1:3306)/contest?parseTime=true&loc=UTC")
	t.Setenv("CONTEST_INTERNAL_PROBLEM_TOKEN", "problem-token")
	t.Setenv("CONTEST_INTERNAL_SUBMIT_TOKEN", "submit-token")
	t.Setenv("AUTH_INTERNAL_CONTEST_TOKEN", "auth-token")
	t.Setenv("PROBLEM_INTERNAL_CONTEST_TOKEN", "problem-catalog-token")
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
	if c.ProblemRPC.Etcd.Key != "problem.rpc" {
		t.Fatalf("ProblemRPC key = %q", c.ProblemRPC.Etcd.Key)
	}
	if c.ProblemRPCAuth.Name != "contest.rpc" || c.ProblemRPCAuth.Token != "problem-catalog-token" {
		t.Fatalf("ProblemRPCAuth = %+v", c.ProblemRPCAuth)
	}
	if len(c.InternalAuth.Callers) != 2 || c.InternalAuth.Callers[0].Token != "problem-token" {
		t.Fatalf("InternalAuth = %+v", c.InternalAuth)
	}
	if c.ScoreboardFreeze.IntervalSeconds != 10 || c.ScoreboardFreeze.BatchSize != 100 ||
		c.ScoreboardFreeze.OperationTimeoutSeconds != 5 {
		t.Fatalf("ScoreboardFreeze = %+v", c.ScoreboardFreeze)
	}
	ignored := make(map[string]struct{}, len(c.Middlewares.StatConf.IgnoreContentMethods))
	for _, method := range c.Middlewares.StatConf.IgnoreContentMethods {
		ignored[method] = struct{}{}
	}
	for _, method := range []string{
		pb.ContestService_CreateContest_FullMethodName,
		pb.ContestService_UpdateContest_FullMethodName,
		pb.ContestService_RegisterContest_FullMethodName,
		pb.ContestService_ReviewRegistration_FullMethodName,
	} {
		if _, ok := ignored[method]; !ok {
			t.Errorf("sensitive RPC %q is not excluded from content logging", method)
		}
	}
}
