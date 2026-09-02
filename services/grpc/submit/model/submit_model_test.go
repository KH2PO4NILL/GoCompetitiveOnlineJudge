package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCreateRequestFingerprintContainsOnlyStableRequestFields(t *testing.T) {
	contestID := int64(9)
	base := CreateSubmissionInput{
		UserID: 1, ProblemID: 2, ContestID: &contestID, ProblemRevision: 3,
		Language: "go1.26", SourceCode: "package main", ClientRequestID: "request-1",
		SubmittedAt: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
	}
	changedServerFields := base
	changedServerFields.ProblemRevision = 99
	changedServerFields.SubmittedAt = base.SubmittedAt.Add(time.Hour)
	if !bytes.Equal(createRequestFingerprint(base), createRequestFingerprint(changedServerFields)) {
		t.Fatal("fingerprint changed for server-owned revision or time")
	}
	changedPayload := base
	changedPayload.SourceCode = "package main\nfunc main() {}"
	if bytes.Equal(createRequestFingerprint(base), createRequestFingerprint(changedPayload)) {
		t.Fatal("fingerprint did not change with source code")
	}
}

func TestReportFingerprintIsOrderStableAndIncludesDiagnostics(t *testing.T) {
	base := ApplyJudgeResultInput{
		WorkerID: "worker-1", SubmissionID: 1, AttemptID: 2, LeaseToken: "lease", ReportID: "report-1",
		Verdict: VerdictWrongAnswer, Score: 40, MaxScore: 100, CompileOutput: "",
		CaseResults: []CaseResult{
			{TestCaseID: 2, Ordinal: 1, Verdict: VerdictWrongAnswer, JudgeMessage: "second"},
			{TestCaseID: 1, Ordinal: 0, Verdict: VerdictAccepted, Score: 40, JudgeMessage: "first"},
		},
	}
	first, normalized, err := encodeReportFingerprint(base)
	if err != nil {
		t.Fatalf("encodeReportFingerprint() error = %v", err)
	}
	if normalized[0].TestCaseID != 1 || normalized[1].TestCaseID != 2 {
		t.Fatalf("normalized cases = %+v", normalized)
	}
	reordered := base
	reordered.CaseResults = []CaseResult{base.CaseResults[1], base.CaseResults[0]}
	second, _, err := encodeReportFingerprint(reordered)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("reordered fingerprint changed: err=%v", err)
	}
	changedDiagnostic := reordered
	changedDiagnostic.CaseResults = append([]CaseResult(nil), reordered.CaseResults...)
	changedDiagnostic.CaseResults[0].JudgeMessage = "different"
	third, _, err := encodeReportFingerprint(changedDiagnostic)
	if err != nil || bytes.Equal(first, third) {
		t.Fatalf("diagnostic change was not fingerprinted: err=%v", err)
	}
}

func TestSensitiveModelFieldsAreNotJSONSerialized(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	aggregate := SubmissionAggregate{
		SubmissionSummary: SubmissionSummary{SubmissionID: 1, UserID: 2, ProblemID: 3, CurrentAttemptID: 4, SubmittedAt: now},
		SourceCode:        "sensitive-source",
		CurrentAttempt:    &Attempt{AttemptID: 4, CompileOutput: "sensitive-compiler", ReportID: stringPointer("sensitive-report")},
		CaseResults:       []CaseResult{{TestCaseID: 5, JudgeMessage: "sensitive-diagnostic"}},
	}
	encoded, err := json.Marshal(aggregate)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	for _, secret := range []string{"sensitive-source", "sensitive-compiler", "sensitive-report", "sensitive-diagnostic"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("JSON leaked %q: %s", secret, encoded)
		}
	}
	outbox := OutboxEvent{Payload: json.RawMessage(`{"source_code":"secret"}`)}
	encoded, err = json.Marshal(outbox)
	if err != nil || strings.Contains(string(encoded), "source_code") {
		t.Fatalf("outbox JSON leaked payload: %s err=%v", encoded, err)
	}
}

func TestValidationRejectsInvalidBoundaries(t *testing.T) {
	valid := CreateSubmissionInput{UserID: 1, ProblemID: 1, ProblemRevision: 1, Language: "go", SourceCode: "main", ClientRequestID: "request"}
	tests := []struct {
		name   string
		mutate func(*CreateSubmissionInput)
	}{
		{name: "missing user", mutate: func(value *CreateSubmissionInput) { value.UserID = 0 }},
		{name: "non ascii language", mutate: func(value *CreateSubmissionInput) { value.Language = "语言" }},
		{name: "empty source", mutate: func(value *CreateSubmissionInput) { value.SourceCode = "" }},
		{name: "invalid request id", mutate: func(value *CreateSubmissionInput) { value.ClientRequestID = "line\nbreak" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			test.mutate(&input)
			if err := validateCreateInput(&input); !errors.Is(err, ErrInvalidSubmissionInput) {
				t.Fatalf("validateCreateInput() error = %v", err)
			}
		})
	}
	filter := SubmissionListFilter{Page: 1, PageSize: maxListPageSize + 1}
	if err := normalizeListFilter(&filter); !errors.Is(err, ErrInvalidSubmissionInput) {
		t.Fatalf("normalizeListFilter() error = %v", err)
	}
}

func TestLeaseTokenStoresOnlyDigest(t *testing.T) {
	token, digest, err := randomLeaseToken()
	if err != nil {
		t.Fatalf("randomLeaseToken() error = %v", err)
	}
	if token == "" || len(digest) != 32 || bytes.Contains(digest, []byte(token)) {
		t.Fatalf("invalid token material: token=%q digest_len=%d", token, len(digest))
	}
	if !sameDigest(digest, leaseTokenDigest(token)) || sameDigest(digest, leaseTokenDigest(token+"x")) {
		t.Fatal("lease token digest comparison is incorrect")
	}
}

func TestOutboxErrorCodeRejectsArbitraryErrorText(t *testing.T) {
	for _, valid := range []string{"mq.timeout", "broker_unavailable", "publish-failed:retryable"} {
		if !validOutboxErrorCode(valid) {
			t.Fatalf("validOutboxErrorCode(%q) = false", valid)
		}
	}
	for _, invalid := range []string{"", "MQ.TIMEOUT", "password=secret", "payload: user source", strings.Repeat("x", 65)} {
		if validOutboxErrorCode(invalid) {
			t.Fatalf("validOutboxErrorCode(%q) = true", invalid)
		}
	}
}
