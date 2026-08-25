package authservicelogic

import (
	"database/sql"
	"testing"
)

func TestPasswordLoginRateLimitBucketsSeparatePrincipalAndSharedIP(t *testing.T) {
	info := clientInfo{ipAddress: sql.NullString{String: "192.0.2.10", Valid: true}}
	first := passwordLoginRateLimitBuckets("email", "first@example.com", info)
	second := passwordLoginRateLimitBuckets("email", "second@example.com", info)

	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("bucket lengths = (%d, %d), want (2, 2)", len(first), len(second))
	}
	if first[0].key == second[0].key {
		t.Fatal("different accounts share the principal+IP rate-limit bucket")
	}
	if first[1].key != second[1].key {
		t.Fatal("accounts from the same IP do not share the IP rate-limit bucket")
	}
	if first[0].limit != passwordPrincipalFailureLimit {
		t.Fatalf("principal limit = %d, want %d", first[0].limit, passwordPrincipalFailureLimit)
	}
	if first[1].limit != passwordIPFailureLimit {
		t.Fatalf("IP limit = %d, want %d", first[1].limit, passwordIPFailureLimit)
	}
}

func TestPasswordLoginRateLimitBucketsWorkWithoutPeerIP(t *testing.T) {
	buckets := passwordLoginRateLimitBuckets("username", "alice", clientInfo{})
	if len(buckets) != 1 {
		t.Fatalf("bucket length = %d, want 1", len(buckets))
	}
}

func TestEmailSendRateBucketsUseHigherSharedIPLimit(t *testing.T) {
	info := clientInfo{ipAddress: sql.NullString{String: "192.0.2.10", Valid: true}}
	buckets := emailSendRateBuckets(info)
	if len(buckets) != 2 {
		t.Fatalf("bucket length = %d, want 2", len(buckets))
	}
	if buckets[1].limit != emailSendIPLimit {
		t.Fatalf("email IP limit = %d, want %d", buckets[1].limit, emailSendIPLimit)
	}
}
