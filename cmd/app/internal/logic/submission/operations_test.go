package submission

import (
	"context"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/submitservice"
	"google.golang.org/grpc"
)

type submitClientStub struct {
	submitservice.SubmitService
	createReq *pb.CreateSubmissionReq
	listReq   *pb.ListSubmissionsReq
}

func (s *submitClientStub) CreateSubmission(
	_ context.Context, req *pb.CreateSubmissionReq, _ ...grpc.CallOption,
) (*pb.CreateSubmissionResp, error) {
	s.createReq = req
	return &pb.CreateSubmissionResp{Submission: &pb.SubmissionDetail{
		Summary: &pb.SubmissionSummary{SubmissionId: 11, ProblemId: req.ProblemId},
	}}, nil
}

func (s *submitClientStub) ListSubmissions(
	_ context.Context, req *pb.ListSubmissionsReq, _ ...grpc.CallOption,
) (*pb.ListSubmissionsResp, error) {
	s.listReq = req
	return &pb.ListSubmissionsResp{}, nil
}

func TestCreateSubmissionForwardsIdempotencyAndContest(t *testing.T) {
	client := new(submitClientStub)
	contestID := int64(3)
	response, err := createSubmission(context.Background(), &svc.ServiceContext{Submissions: client}, &types.CreateSubmissionReq{
		ProblemID: 7, ContestID: &contestID, Language: "go", SourceCode: "package main",
		ClientRequestID: "request-1",
	})
	if err != nil || response.Submission.Summary.SubmissionID != 11 {
		t.Fatalf("createSubmission() = (%+v, %v)", response, err)
	}
	if client.createReq == nil || client.createReq.GetContestId() != contestID || client.createReq.ClientRequestId != "request-1" {
		t.Fatalf("CreateSubmission request = %+v", client.createReq)
	}
}

func TestListSubmissionsMapsStatusFilter(t *testing.T) {
	client := new(submitClientStub)
	_, err := listSubmissions(context.Background(), &svc.ServiceContext{Submissions: client}, &types.ListSubmissionsReq{Status: "judging"})
	if err != nil {
		t.Fatalf("listSubmissions() error = %v", err)
	}
	if client.listReq == nil || client.listReq.Status == nil || *client.listReq.Status != pb.SubmissionStatus_SUBMISSION_STATUS_JUDGING {
		t.Fatalf("ListSubmissions request = %+v", client.listReq)
	}
}
