package svc

import (
	"testing"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
)

func TestInternalAuthConfigurationBuildsPerMethodAllowlist(t *testing.T) {
	credentials, callers, err := sharedauth.BuildServiceAuthentication(sharedauth.InternalAuthConf{Callers: []sharedauth.InternalCallerConf{
		{Name: "submit.rpc", Token: "secret", Methods: []string{
			"/problem.ProblemService/BatchGetProblems",
			"/problem.ProblemService/GetJudgeProblemSpec",
		}},
	}}, "/problem.ProblemService/BatchGetProblems", "/problem.ProblemService/GetJudgeProblemSpec")
	if err != nil {
		t.Fatalf("internalAuthConfiguration() error = %v", err)
	}
	if credentials["submit.rpc"] != "secret" || len(callers["/problem.ProblemService/BatchGetProblems"]) != 1 {
		t.Fatalf("credentials/callers = (%v, %v)", credentials, callers)
	}
}

func TestInternalAuthConfigurationRejectsMissingCoverage(t *testing.T) {
	_, _, err := sharedauth.BuildServiceAuthentication(sharedauth.InternalAuthConf{Callers: []sharedauth.InternalCallerConf{
		{Name: "submit.rpc", Token: "secret", Methods: []string{"/problem.ProblemService/BatchGetProblems"}},
	}}, "/problem.ProblemService/BatchGetProblems", "/problem.ProblemService/GetJudgeProblemSpec")
	if err == nil {
		t.Fatal("internalAuthConfiguration() unexpectedly succeeded")
	}
}
