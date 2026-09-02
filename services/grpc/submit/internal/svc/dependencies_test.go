package svc

import (
	"context"
	"errors"
	"testing"
	"time"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	contestpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeContestSubmissionClient struct {
	validationCalls int
	visibilityCalls int
	metadata        metadata.MD
	validateFn      func(*contestpb.ValidateContestSubmissionReq) *contestpb.ValidateContestSubmissionResp
	visibilityFn    func(*contestpb.ResolveSubmissionResultVisibilityReq) *contestpb.ResolveSubmissionResultVisibilityResp
}

func (f *fakeContestSubmissionClient) ValidateContestSubmission(
	ctx context.Context,
	in *contestpb.ValidateContestSubmissionReq,
	_ ...grpc.CallOption,
) (*contestpb.ValidateContestSubmissionResp, error) {
	f.validationCalls++
	f.metadata, _ = metadata.FromOutgoingContext(ctx)
	return f.validateFn(in), nil
}

func (f *fakeContestSubmissionClient) ResolveSubmissionResultVisibility(
	ctx context.Context,
	in *contestpb.ResolveSubmissionResultVisibilityReq,
	_ ...grpc.CallOption,
) (*contestpb.ResolveSubmissionResultVisibilityResp, error) {
	f.visibilityCalls++
	f.metadata, _ = metadata.FromOutgoingContext(ctx)
	return f.visibilityFn(in), nil
}

type fakeProblemSpecClient struct {
	metadata metadata.MD
}

func (f *fakeProblemSpecClient) GetJudgeProblemSpec(
	ctx context.Context,
	in *problempb.GetJudgeProblemSpecReq,
	_ ...grpc.CallOption,
) (*problempb.GetJudgeProblemSpecResp, error) {
	f.metadata, _ = metadata.FromOutgoingContext(ctx)
	return &problempb.GetJudgeProblemSpecResp{Spec: &problempb.JudgeProblemSpec{
		ProblemId: in.ProblemId, Revision: in.Revision,
	}}, nil
}

func TestContestPolicyResolvesOIVisibilityOncePerContestAndUser(t *testing.T) {
	client := &fakeContestSubmissionClient{visibilityFn: func(in *contestpb.ResolveSubmissionResultVisibilityReq) *contestpb.ResolveSubmissionResultVisibilityResp {
		if in.ContestId != 10 || in.UserId != 7 || in.ObservedAt == nil {
			t.Fatalf("visibility request = %+v", in)
		}
		return &contestpb.ResolveSubmissionResultVisibilityResp{
			Visibility: contestpb.SubmissionResultVisibility_SUBMISSION_RESULT_VISIBILITY_STATUS_ONLY,
		}
	}}
	policy := &rpcContestPolicy{client: client, credential: sharedauth.ServiceClientAuthConf{Name: "submit.rpc", Token: "secret"}}
	result, err := policy.ResolveResultVisibility(context.Background(), []SubmissionReference{
		{SubmissionID: 1, ContestID: 10, ProblemID: 100, UserID: 7},
		{SubmissionID: 2, ContestID: 10, ProblemID: 101, UserID: 7},
	}, time.Now())
	if err != nil {
		t.Fatalf("ResolveResultVisibility() error = %v", err)
	}
	if client.visibilityCalls != 1 || result[1] != ResultVisibilityStatusOnly || result[2] != ResultVisibilityStatusOnly {
		t.Fatalf("calls/result = %d / %+v", client.visibilityCalls, result)
	}
	if got := client.metadata.Get(sharedauth.ServiceNameMetadataKey); len(got) != 1 || got[0] != "submit.rpc" {
		t.Fatalf("service name metadata = %v", got)
	}
}

func TestContestPolicyRevealsFinishedOIResult(t *testing.T) {
	client := &fakeContestSubmissionClient{visibilityFn: func(*contestpb.ResolveSubmissionResultVisibilityReq) *contestpb.ResolveSubmissionResultVisibilityResp {
		return &contestpb.ResolveSubmissionResultVisibilityResp{
			Visibility: contestpb.SubmissionResultVisibility_SUBMISSION_RESULT_VISIBILITY_FULL,
		}
	}}
	policy := &rpcContestPolicy{client: client}
	result, err := policy.ResolveResultVisibility(context.Background(), []SubmissionReference{
		{SubmissionID: 1, ContestID: 10, ProblemID: 100, UserID: 7},
	}, time.Now())
	if err != nil || result[1] != ResultVisibilityFull {
		t.Fatalf("finished OI result = %+v, %v", result, err)
	}
}

func TestContestPolicyValidatesNewSubmissionWithDedicatedEndpoint(t *testing.T) {
	client := &fakeContestSubmissionClient{validateFn: func(in *contestpb.ValidateContestSubmissionReq) *contestpb.ValidateContestSubmissionResp {
		if in.ContestId != 10 || in.ProblemId != 100 || in.UserId != 7 || in.SubmittedAt == nil {
			t.Fatalf("validation request = %+v", in)
		}
		return &contestpb.ValidateContestSubmissionResp{Allowed: true, Mode: contestpb.ContestMode_CONTEST_MODE_OI}
	}}
	policy := &rpcContestPolicy{client: client}
	result, err := policy.ValidateSubmission(context.Background(), 10, 100, 7, time.Now())
	if err != nil || result == nil || !result.Allowed || client.validationCalls != 1 {
		t.Fatalf("ValidateSubmission() = %+v, %v, calls=%d", result, err, client.validationCalls)
	}
}

func TestProblemCatalogForwardsServiceCredentials(t *testing.T) {
	client := &fakeProblemSpecClient{}
	catalog := &rpcProblemSpecCatalog{client: client, credential: sharedauth.ServiceClientAuthConf{Name: "submit.rpc", Token: "secret"}}
	spec, err := catalog.GetJudgeProblemSpec(context.Background(), 9, 7)
	if err != nil || spec.ProblemId != 9 || spec.Revision != 7 {
		t.Fatalf("GetJudgeProblemSpec() = %+v, %v", spec, err)
	}
	if got := client.metadata.Get(sharedauth.ServiceTokenMetadataKey); len(got) != 1 || got[0] != "secret" {
		t.Fatalf("service token metadata = %v", got)
	}
}

type fakeLeaseRecoveryModel struct {
	leases     []model.ExpiredLease
	superseded []model.SupersedeAttemptInput
	errors     map[int64]error
}

func (f *fakeLeaseRecoveryModel) ListExpiredLeases(context.Context, time.Time, int) ([]model.ExpiredLease, error) {
	return append([]model.ExpiredLease(nil), f.leases...), nil
}

func (f *fakeLeaseRecoveryModel) SupersedeExpiredAttempt(_ context.Context, input model.SupersedeAttemptInput) (*model.Attempt, error) {
	f.superseded = append(f.superseded, input)
	if err := f.errors[input.AttemptID]; err != nil {
		return nil, err
	}
	return &model.Attempt{AttemptID: input.AttemptID + 100}, nil
}

func TestLeaseRecoveryContinuesAfterBenignAndRealConflicts(t *testing.T) {
	repository := &fakeLeaseRecoveryModel{
		leases: []model.ExpiredLease{
			{SubmissionID: 1, AttemptID: 10},
			{SubmissionID: 2, AttemptID: 20},
			{SubmissionID: 3, AttemptID: 30},
		},
		errors: map[int64]error{10: model.ErrStaleAttempt, 20: errors.New("database failed")},
	}
	worker, err := NewLeaseRecoveryWorker(repository, time.Second, 10, time.Second, time.Now)
	if err != nil {
		t.Fatalf("NewLeaseRecoveryWorker() error = %v", err)
	}
	err = worker.RunOnce(context.Background())
	if err == nil || len(repository.superseded) != 3 {
		t.Fatalf("RunOnce() error/calls = %v / %+v", err, repository.superseded)
	}
}

func TestSubmissionRateLimiterRejectsInvalidConfiguration(t *testing.T) {
	if _, err := NewRedisSubmissionRateLimiter(nil, time.Minute, 1); err == nil {
		t.Fatal("NewRedisSubmissionRateLimiter() accepted nil redis")
	}
}
