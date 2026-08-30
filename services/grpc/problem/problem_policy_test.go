package main

import (
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
)

func TestProblemMethodAuthentication(t *testing.T) {
	tests := []struct {
		method string
		want   interceptor.AuthenticationMode
	}{
		{pb.ProblemService_GetProblem_FullMethodName, interceptor.AuthenticationOptional},
		{pb.ProblemService_CreateProblem_FullMethodName, interceptor.AuthenticationRequired},
		{pb.ProblemService_GetJudgeProblemSpec_FullMethodName, interceptor.AuthenticationSkipped},
	}
	for _, test := range tests {
		if got := problemMethodAuthentication(test.method); got != test.want {
			t.Errorf("problemMethodAuthentication(%q) = %v, want %v", test.method, got, test.want)
		}
	}
}
