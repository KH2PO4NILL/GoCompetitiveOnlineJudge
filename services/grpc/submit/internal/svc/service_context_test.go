package svc

import (
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/config"
)

func TestNewServiceContextRejectsEmptyMySQLDSN(t *testing.T) {
	if _, err := NewServiceContext(config.Config{}); err == nil {
		t.Fatal("NewServiceContext() unexpectedly accepted an empty MySQL DSN")
	}
}
