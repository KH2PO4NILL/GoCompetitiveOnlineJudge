package svc

import (
	"context"
	"errors"
	"testing"

	contestpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"google.golang.org/grpc"
)

type fakeContestAccessClient struct {
	response *contestpb.CanAccessProblemResp
	err      error
	request  *contestpb.CanAccessProblemReq
}

func (f *fakeContestAccessClient) CanAccessProblem(
	_ context.Context,
	in *contestpb.CanAccessProblemReq,
	_ ...grpc.CallOption,
) (*contestpb.CanAccessProblemResp, error) {
	f.request = in
	return f.response, f.err
}

func TestContestAccessRPCCheckerForwardsIdentityAndDecision(t *testing.T) {
	client := &fakeContestAccessClient{response: &contestpb.CanAccessProblemResp{Allowed: true}}
	checker := &contestAccessRPCChecker{client: client}

	allowed, err := checker.CanAccessProblem(context.Background(), 11, 22, 33)
	if err != nil {
		t.Fatalf("CanAccessProblem() error = %v", err)
	}
	if !allowed {
		t.Fatal("CanAccessProblem() denied an allowed request")
	}
	if client.request.GetContestId() != 11 || client.request.GetProblemId() != 22 || client.request.GetUserId() != 33 {
		t.Fatalf("forwarded request = %+v", client.request)
	}
}

func TestContestAccessRPCCheckerFailsClosed(t *testing.T) {
	expected := errors.New("contest unavailable")
	checker := &contestAccessRPCChecker{client: &fakeContestAccessClient{err: expected}}
	allowed, err := checker.CanAccessProblem(context.Background(), 1, 2, 3)
	if allowed || !errors.Is(err, expected) {
		t.Fatalf("allowed = %v, error = %v", allowed, err)
	}

	checker = &contestAccessRPCChecker{client: &fakeContestAccessClient{}}
	allowed, err = checker.CanAccessProblem(context.Background(), 1, 2, 3)
	if allowed || err == nil {
		t.Fatalf("empty response allowed = %v, error = %v", allowed, err)
	}
}
