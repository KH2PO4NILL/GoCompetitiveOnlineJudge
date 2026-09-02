package svc

import (
	"strings"
	"testing"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
)

func TestNewServiceContextRejectsEmptyMySQLDSN(t *testing.T) {
	if _, err := NewServiceContext(config.Config{}); err == nil {
		t.Fatal("NewServiceContext() unexpectedly accepted an empty MySQL DSN")
	}
}

func TestValidateRuntimeConfigRejectsUnsafeLimits(t *testing.T) {
	c := config.Config{}
	c.Limits.MaxSourceCodeBytes = 4*1024*1024 + 1
	if err := validateRuntimeConfig(c); err == nil {
		t.Fatal("validateRuntimeConfig() accepted an unsafe source limit")
	}
}

func TestNewServiceContextRejectsIncompleteWorkerAllowlistBeforeConnecting(t *testing.T) {
	c := config.Config{
		Mysql: config.MysqlConf{DSN: "not-used"},
		Limits: config.LimitsConf{
			MaxSourceCodeBytes: 1, MaxClientRequestBytes: 1, MaxPageSize: 1,
			MaxBatchRejudge: 1, MaxCaseResults: 1, MaxCompileOutputBytes: 1,
			MaxJudgeMessageBytes: 1,
		},
		Lease: config.LeaseConf{
			DurationSeconds: 1, RecoveryIntervalSeconds: 1,
			RecoveryBatchSize: 1, RecoveryTimeoutSeconds: 1,
		},
		AuthRPCAuth:    sharedauth.ServiceClientAuthConf{Name: "submit.rpc", Token: "auth"},
		ProblemRPCAuth: sharedauth.ServiceClientAuthConf{Name: "submit.rpc", Token: "problem"},
		ContestRPCAuth: sharedauth.ServiceClientAuthConf{Name: "submit.rpc", Token: "contest"},
		InternalAuth: sharedauth.InternalAuthConf{Callers: []sharedauth.InternalCallerConf{{
			Name: "judge.worker", Token: "judge",
			Methods: []string{pb.SubmitService_AcquireJudgeTask_FullMethodName},
		}}},
	}
	_, err := NewServiceContext(c)
	if err == nil || !strings.Contains(err.Error(), pb.SubmitService_HeartbeatJudgeTask_FullMethodName) {
		t.Fatalf("NewServiceContext() incomplete allowlist error = %v", err)
	}
}
