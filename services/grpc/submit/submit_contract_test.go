package main

import (
	"os"
	"strings"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func messageDescriptor(t *testing.T, name protoreflect.Name) protoreflect.MessageDescriptor {
	t.Helper()
	descriptor := pb.File_proto_submit_proto.Messages().ByName(name)
	if descriptor == nil {
		t.Fatalf("message %s is missing from submit.proto", name)
	}
	return descriptor
}

func fieldNames(message protoreflect.MessageDescriptor) map[protoreflect.Name]protoreflect.FieldDescriptor {
	fields := make(map[protoreflect.Name]protoreflect.FieldDescriptor, message.Fields().Len())
	for index := 0; index < message.Fields().Len(); index++ {
		field := message.Fields().Get(index)
		fields[field.Name()] = field
	}
	return fields
}

func TestCreateSubmissionContractOwnsIdentityAndTime(t *testing.T) {
	fields := fieldNames(messageDescriptor(t, "CreateSubmissionReq"))
	for _, forbidden := range []protoreflect.Name{"user_id", "submitted_at", "problem_revision"} {
		if _, exists := fields[forbidden]; exists {
			t.Fatalf("CreateSubmissionReq must not expose server-owned field %s", forbidden)
		}
	}

	wantNumbers := map[protoreflect.Name]protoreflect.FieldNumber{
		"problem_id":        1,
		"contest_id":        2,
		"language":          3,
		"source_code":       4,
		"client_request_id": 5,
	}
	for name, number := range wantNumbers {
		field := fields[name]
		if field == nil || field.Number() != number {
			t.Fatalf("CreateSubmissionReq.%s number = %v, want %d", name, field, number)
		}
	}
	if !fields["contest_id"].HasOptionalKeyword() {
		t.Fatal("contest_id must preserve presence for practice submissions")
	}
	if fields["language"].Kind() != protoreflect.StringKind {
		t.Fatal("language must remain an extensible string identifier")
	}
}

func TestProblemRevisionIsPinnedAndCannotBeOverriddenByRejudge(t *testing.T) {
	summary := fieldNames(messageDescriptor(t, "SubmissionSummary"))
	if field := summary["problem_revision"]; field == nil || field.Number() != 10 || field.Kind() != protoreflect.Int64Kind {
		t.Fatalf("SubmissionSummary.problem_revision = %v, want int64 field 10", field)
	}
	task := fieldNames(messageDescriptor(t, "JudgeTask"))
	if field := task["problem_revision"]; field == nil || field.Number() != 4 {
		t.Fatalf("JudgeTask.problem_revision = %v, want field 4", field)
	}
	for _, messageName := range []protoreflect.Name{"RejudgeSubmissionReq", "BatchRejudgeSubmissionsReq"} {
		if _, exists := fieldNames(messageDescriptor(t, messageName))["problem_revision"]; exists {
			t.Fatalf("%s must not let callers replace the pinned problem revision", messageName)
		}
	}
}

func TestQueueEventContainsOnlyTaskLocators(t *testing.T) {
	fields := fieldNames(messageDescriptor(t, "JudgeTaskEvent"))
	want := map[protoreflect.Name]protoreflect.FieldNumber{
		"event_id":       1,
		"submission_id":  2,
		"attempt_id":     3,
		"enqueued_at":    4,
		"schema_version": 5,
	}
	if len(fields) != len(want) {
		t.Fatalf("JudgeTaskEvent has %d fields, want only %d locator fields", len(fields), len(want))
	}
	for name, number := range want {
		field := fields[name]
		if field == nil || field.Number() != number {
			t.Fatalf("JudgeTaskEvent.%s = %v, want field %d", name, field, number)
		}
	}
	for _, forbidden := range []protoreflect.Name{
		"source_code", "test_data_package_key", "checker_file_key", "interactor_file_key", "lease_token",
	} {
		if _, exists := fields[forbidden]; exists {
			t.Fatalf("JudgeTaskEvent must not contain sensitive field %s", forbidden)
		}
	}
}

func TestPublicTestCaseProjectionExcludesWorkerDiagnostics(t *testing.T) {
	publicFields := fieldNames(messageDescriptor(t, "TestCaseResult"))
	for _, forbidden := range []protoreflect.Name{
		"judge_message", "test_data_package_key", "checker_output", "sandbox_diagnostics",
	} {
		if _, exists := publicFields[forbidden]; exists {
			t.Fatalf("public TestCaseResult exposes %s", forbidden)
		}
	}
	workerFields := fieldNames(messageDescriptor(t, "JudgeTestCaseResult"))
	if field := workerFields["judge_message"]; field == nil || field.Number() != 8 {
		t.Fatalf("JudgeTestCaseResult.judge_message = %v, want internal field 8", field)
	}
}

func TestWorkerProtocolCarriesAttemptLeaseAndIdempotencyKeys(t *testing.T) {
	tests := []struct {
		message protoreflect.Name
		fields  []protoreflect.Name
	}{
		{message: "AcquireJudgeTaskReq", fields: []protoreflect.Name{"worker_id", "submission_id", "attempt_id", "delivery_id"}},
		{message: "AcquireJudgeTaskResp", fields: []protoreflect.Name{"acquired", "deny_reason", "task", "lease_token", "lease_expires_at"}},
		{message: "HeartbeatJudgeTaskReq", fields: []protoreflect.Name{"worker_id", "submission_id", "attempt_id", "lease_token"}},
		{message: "ReportJudgeResultReq", fields: []protoreflect.Name{"worker_id", "submission_id", "attempt_id", "lease_token", "report_id"}},
	}
	for _, test := range tests {
		fields := fieldNames(messageDescriptor(t, test.message))
		for _, name := range test.fields {
			if fields[name] == nil {
				t.Errorf("%s is missing %s", test.message, name)
			}
		}
	}
}

func TestSubmitServiceMethodBoundary(t *testing.T) {
	service := pb.File_proto_submit_proto.Services().ByName("SubmitService")
	if service == nil {
		t.Fatal("SubmitService descriptor is missing")
	}
	want := map[protoreflect.Name]bool{
		"CreateSubmission":        true,
		"GetSubmission":           true,
		"ListSubmissions":         true,
		"RejudgeSubmission":       true,
		"BatchRejudgeSubmissions": true,
		"AcquireJudgeTask":        true,
		"HeartbeatJudgeTask":      true,
		"ReportJudgeResult":       true,
	}
	if service.Methods().Len() != len(want) {
		t.Fatalf("SubmitService has %d methods, want %d", service.Methods().Len(), len(want))
	}
	for index := 0; index < service.Methods().Len(); index++ {
		method := service.Methods().Get(index)
		if !want[method.Name()] {
			t.Errorf("unexpected SubmitService method %s", method.Name())
		}
	}
}

func TestSensitiveRPCPayloadLoggingIsDisabled(t *testing.T) {
	contents, err := os.ReadFile("etc/submit.yaml")
	if err != nil {
		t.Fatalf("read submit.yaml: %v", err)
	}
	config := string(contents)
	if !strings.Contains(config, "ListenOn: 0.0.0.0:8004") {
		t.Fatal("submit RPC must use the documented port 8004")
	}
	for _, method := range []string{
		pb.SubmitService_CreateSubmission_FullMethodName,
		pb.SubmitService_GetSubmission_FullMethodName,
		pb.SubmitService_AcquireJudgeTask_FullMethodName,
		pb.SubmitService_HeartbeatJudgeTask_FullMethodName,
		pb.SubmitService_ReportJudgeResult_FullMethodName,
	} {
		if !strings.Contains(config, method) {
			t.Errorf("submit.yaml does not suppress content logging for %s", method)
		}
	}
}
