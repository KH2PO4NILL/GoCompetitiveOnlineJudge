package main

import (
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
)

func TestSubmitMethodAuthenticationBoundary(t *testing.T) {
	workerMethods := []string{
		pb.SubmitService_AcquireJudgeTask_FullMethodName,
		pb.SubmitService_HeartbeatJudgeTask_FullMethodName,
		pb.SubmitService_ReportJudgeResult_FullMethodName,
	}
	for _, method := range workerMethods {
		if got := submitMethodAuthentication(method); got != interceptor.AuthenticationSkipped {
			t.Errorf("submitMethodAuthentication(%s) = %v, want skipped", method, got)
		}
	}
	if got := submitMethodAuthentication(pb.SubmitService_CreateSubmission_FullMethodName); got != interceptor.AuthenticationRequired {
		t.Fatalf("CreateSubmission authentication = %v, want required", got)
	}
}
