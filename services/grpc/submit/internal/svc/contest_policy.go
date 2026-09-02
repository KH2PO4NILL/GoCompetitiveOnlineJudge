package svc

import (
	"context"
	"errors"
	"time"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	contestpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ContestValidation struct {
	Allowed    bool
	DenyReason contestpb.SubmissionDenyReason
	Mode       contestpb.ContestMode
}

type SubmissionReference struct {
	SubmissionID int64
	ContestID    int64
	ProblemID    int64
	UserID       int64
}

type ResultVisibility uint8

const (
	ResultVisibilityStatusOnly ResultVisibility = iota + 1
	ResultVisibilityFull
)

// ContestPolicy owns the two Contest decisions Submit needs: whether a new
// contest submission is accepted and whether an existing result is currently
// visible. ResolveResultVisibility is batch-shaped so List logic can avoid one
// call per submission.
type ContestPolicy interface {
	ValidateSubmission(
		ctx context.Context,
		contestID, problemID, userID int64,
		submittedAt time.Time,
	) (*ContestValidation, error)
	ResolveResultVisibility(
		ctx context.Context,
		references []SubmissionReference,
		at time.Time,
	) (map[int64]ResultVisibility, error)
}

type contestPolicyClient interface {
	ValidateContestSubmission(
		ctx context.Context,
		in *contestpb.ValidateContestSubmissionReq,
		opts ...grpc.CallOption,
	) (*contestpb.ValidateContestSubmissionResp, error)
	ResolveSubmissionResultVisibility(
		ctx context.Context,
		in *contestpb.ResolveSubmissionResultVisibilityReq,
		opts ...grpc.CallOption,
	) (*contestpb.ResolveSubmissionResultVisibilityResp, error)
}

type rpcContestPolicy struct {
	client     contestPolicyClient
	credential sharedauth.ServiceClientAuthConf
}

func NewRPCContestPolicy(
	connection grpc.ClientConnInterface,
	credential sharedauth.ServiceClientAuthConf,
) ContestPolicy {
	return &rpcContestPolicy{
		client:     contestpb.NewContestServiceClient(connection),
		credential: credential,
	}
}

func (c *rpcContestPolicy) ValidateSubmission(
	ctx context.Context,
	contestID, problemID, userID int64,
	submittedAt time.Time,
) (*ContestValidation, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("contest RPC client is unavailable")
	}
	timestamp := timestamppb.New(submittedAt.UTC())
	if err := timestamp.CheckValid(); err != nil {
		return nil, errors.New("contest validation time is invalid")
	}
	ctx = sharedauth.WithOutgoingServiceCredentials(ctx, c.credential.Name, c.credential.Token)
	response, err := c.client.ValidateContestSubmission(ctx, &contestpb.ValidateContestSubmissionReq{
		ContestId:   contestID,
		ProblemId:   problemID,
		UserId:      userID,
		SubmittedAt: timestamp,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, errors.New("contest RPC returned an empty validation response")
	}
	if response.Allowed {
		if response.DenyReason != contestpb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNSPECIFIED ||
			response.Mode == contestpb.ContestMode_CONTEST_MODE_UNSPECIFIED {
			return nil, errors.New("contest RPC returned an inconsistent allow response")
		}
	} else if response.DenyReason == contestpb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNSPECIFIED {
		return nil, errors.New("contest RPC returned an unspecified rejection")
	}
	return &ContestValidation{
		Allowed: response.Allowed, DenyReason: response.DenyReason, Mode: response.Mode,
	}, nil
}

func (c *rpcContestPolicy) ResolveResultVisibility(
	ctx context.Context,
	references []SubmissionReference,
	at time.Time,
) (map[int64]ResultVisibility, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("contest RPC client is unavailable")
	}
	result := make(map[int64]ResultVisibility, len(references))
	type policyKey struct {
		contestID int64
		userID    int64
	}
	groups := make(map[policyKey][]SubmissionReference)
	for _, reference := range references {
		if reference.SubmissionID <= 0 || reference.ContestID <= 0 || reference.ProblemID <= 0 || reference.UserID <= 0 {
			return nil, errors.New("invalid submission visibility reference")
		}
		key := policyKey{contestID: reference.ContestID, userID: reference.UserID}
		groups[key] = append(groups[key], reference)
	}
	for _, group := range groups {
		representative := group[0]
		timestamp := timestamppb.New(at.UTC())
		if err := timestamp.CheckValid(); err != nil {
			return nil, errors.New("contest visibility time is invalid")
		}
		callCtx := sharedauth.WithOutgoingServiceCredentials(ctx, c.credential.Name, c.credential.Token)
		response, err := c.client.ResolveSubmissionResultVisibility(callCtx, &contestpb.ResolveSubmissionResultVisibilityReq{
			ContestId: representative.ContestID, UserId: representative.UserID, ObservedAt: timestamp,
		})
		if err != nil {
			return nil, err
		}
		if response == nil {
			return nil, errors.New("contest RPC returned an empty visibility response")
		}
		var visibility ResultVisibility
		switch response.Visibility {
		case contestpb.SubmissionResultVisibility_SUBMISSION_RESULT_VISIBILITY_STATUS_ONLY:
			visibility = ResultVisibilityStatusOnly
		case contestpb.SubmissionResultVisibility_SUBMISSION_RESULT_VISIBILITY_FULL:
			visibility = ResultVisibilityFull
		default:
			return nil, errors.New("contest RPC returned an invalid result visibility")
		}
		for _, reference := range group {
			result[reference.SubmissionID] = visibility
		}
	}
	return result, nil
}
