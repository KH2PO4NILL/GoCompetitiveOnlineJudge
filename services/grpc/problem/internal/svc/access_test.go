package svc

import (
	"context"
	"errors"
	"testing"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	contestpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeContestAccessClient struct {
	response *contestpb.CanAccessProblemResp
	err      error
	request  *contestpb.CanAccessProblemReq
	ctx      context.Context
}

func (f *fakeContestAccessClient) CanAccessProblem(
	ctx context.Context,
	in *contestpb.CanAccessProblemReq,
	_ ...grpc.CallOption,
) (*contestpb.CanAccessProblemResp, error) {
	f.ctx = ctx
	f.request = in
	return f.response, f.err
}

func TestContestAccessRPCCheckerForwardsIdentityAndDecision(t *testing.T) {
	client := &fakeContestAccessClient{response: &contestpb.CanAccessProblemResp{Allowed: true}}
	checker := &contestAccessRPCChecker{
		client:     client,
		credential: sharedauth.ServiceClientAuthConf{Name: "problem.rpc", Token: "secret"},
	}

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
	md, ok := metadata.FromOutgoingContext(client.ctx)
	if !ok || len(md.Get(sharedauth.ServiceNameMetadataKey)) != 1 || md.Get(sharedauth.ServiceNameMetadataKey)[0] != "problem.rpc" ||
		len(md.Get(sharedauth.ServiceTokenMetadataKey)) != 1 || md.Get(sharedauth.ServiceTokenMetadataKey)[0] != "secret" {
		t.Fatalf("outgoing service metadata = %v", md)
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
