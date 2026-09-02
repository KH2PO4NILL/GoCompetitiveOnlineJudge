package main

import (
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"
)

func TestContestMethodAuthentication(t *testing.T) {
	tests := []struct {
		method string
		want   interceptor.AuthenticationMode
	}{
		{pb.ContestService_ListContests_FullMethodName, interceptor.AuthenticationOptional},
		{pb.ContestService_RegisterContest_FullMethodName, interceptor.AuthenticationRequired},
		{pb.ContestService_CreateContest_FullMethodName, interceptor.AuthenticationRequired},
		{pb.ContestService_CanAccessProblem_FullMethodName, interceptor.AuthenticationSkipped},
		{pb.ContestService_ResolveSubmissionResultVisibility_FullMethodName, interceptor.AuthenticationSkipped},
	}
	for _, test := range tests {
		if got := contestMethodAuthentication(test.method); got != test.want {
			t.Errorf("contestMethodAuthentication(%q) = %v, want %v", test.method, got, test.want)
		}
	}
}
