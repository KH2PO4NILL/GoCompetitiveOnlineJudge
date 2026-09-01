package svc

import (
	"context"
	"testing"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	problemservice "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/problemservice"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeProblemService struct {
	problemservice.ProblemService
	response *problempb.BatchGetProblemsResp
	err      error
	metadata metadata.MD
}

func (f *fakeProblemService) BatchGetProblems(ctx context.Context, _ *problempb.BatchGetProblemsReq, _ ...grpc.CallOption) (*problempb.BatchGetProblemsResp, error) {
	f.metadata, _ = metadata.FromOutgoingContext(ctx)
	return f.response, f.err
}

func TestRPCProblemCatalogAuthenticatesAndAcceptsContestOnly(t *testing.T) {
	client := &fakeProblemService{response: &problempb.BatchGetProblemsResp{Problems: []*problempb.ProblemSummary{
		{ProblemId: 1, Status: problempb.ProblemStatus_PROBLEM_STATUS_PUBLISHED, Visibility: problempb.ProblemVisibility_PROBLEM_VISIBILITY_PUBLIC},
		{ProblemId: 2, Status: problempb.ProblemStatus_PROBLEM_STATUS_PUBLISHED, Visibility: problempb.ProblemVisibility_PROBLEM_VISIBILITY_CONTEST_ONLY},
	}}}
	catalog := NewRPCProblemCatalog(client, sharedauth.ServiceClientAuthConf{Name: "contest.rpc", Token: "secret"})
	ids, err := catalog.ListPublishedForContest(context.Background(), []int64{1, 2})
	if err != nil || len(ids) != 2 {
		t.Fatalf("ListPublishedForContest() = (%v, %v)", ids, err)
	}
	if got := client.metadata.Get(sharedauth.ServiceNameMetadataKey); len(got) != 1 || got[0] != "contest.rpc" {
		t.Fatalf("service name metadata = %v", got)
	}
	if got := client.metadata.Get(sharedauth.ServiceTokenMetadataKey); len(got) != 1 || got[0] != "secret" {
		t.Fatalf("service token metadata = %v", got)
	}
}

func TestRPCProblemCatalogRejectsInvalidEntries(t *testing.T) {
	client := &fakeProblemService{response: &problempb.BatchGetProblemsResp{Problems: []*problempb.ProblemSummary{
		{ProblemId: 1, Status: problempb.ProblemStatus_PROBLEM_STATUS_DRAFT, Visibility: problempb.ProblemVisibility_PROBLEM_VISIBILITY_PUBLIC},
	}}}
	catalog := NewRPCProblemCatalog(client, sharedauth.ServiceClientAuthConf{Name: "contest.rpc", Token: "secret"})
	if _, err := catalog.ListPublishedForContest(context.Background(), []int64{1}); err == nil {
		t.Fatal("invalid problem catalog entry was accepted")
	}
}
