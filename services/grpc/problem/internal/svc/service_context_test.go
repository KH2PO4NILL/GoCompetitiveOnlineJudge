package svc

import (
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/config"
)

func TestInternalAuthConfigurationBuildsPerMethodAllowlist(t *testing.T) {
	credentials, callers, err := internalAuthConfiguration(config.InternalAuthConf{Callers: []config.InternalCallerConf{
		{Name: "submit.rpc", Token: "secret", Methods: []string{
			"/problem.ProblemService/BatchGetProblems",
			"/problem.ProblemService/GetJudgeProblemSpec",
		}},
	}})
	if err != nil {
		t.Fatalf("internalAuthConfiguration() error = %v", err)
	}
	if credentials["submit.rpc"] != "secret" || len(callers["/problem.ProblemService/BatchGetProblems"]) != 1 {
		t.Fatalf("credentials/callers = (%v, %v)", credentials, callers)
	}
}

func TestInternalAuthConfigurationRejectsMissingCoverage(t *testing.T) {
	_, _, err := internalAuthConfiguration(config.InternalAuthConf{Callers: []config.InternalCallerConf{
		{Name: "submit.rpc", Token: "secret", Methods: []string{"/problem.ProblemService/BatchGetProblems"}},
	}})
	if err == nil {
		t.Fatal("internalAuthConfiguration() unexpectedly succeeded")
	}
}
