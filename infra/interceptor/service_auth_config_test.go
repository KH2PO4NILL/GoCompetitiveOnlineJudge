package interceptor

import "testing"

func TestBuildServiceAuthentication(t *testing.T) {
	credentials, callers, err := BuildServiceAuthentication(InternalAuthConf{Callers: []InternalCallerConf{
		{Name: "problem.rpc", Token: "secret", Methods: []string{"/contest.ContestService/CanAccessProblem"}},
	}}, "/contest.ContestService/CanAccessProblem")
	if err != nil {
		t.Fatalf("BuildServiceAuthentication() error = %v", err)
	}
	if credentials["problem.rpc"] != "secret" || len(callers["/contest.ContestService/CanAccessProblem"]) != 1 {
		t.Fatalf("credentials/callers = (%v, %v)", credentials, callers)
	}
}

func TestBuildServiceAuthenticationRequiresCoverage(t *testing.T) {
	_, _, err := BuildServiceAuthentication(InternalAuthConf{Callers: []InternalCallerConf{
		{Name: "problem.rpc", Token: "secret", Methods: []string{"/contest.ContestService/Other"}},
	}}, "/contest.ContestService/CanAccessProblem")
	if err == nil {
		t.Fatal("BuildServiceAuthentication() unexpectedly succeeded")
	}
}
