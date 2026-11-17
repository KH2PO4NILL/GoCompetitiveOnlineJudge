package mapper

import (
	"encoding/json"
	authpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	"strings"
	"testing"
)

func TestBrowserCredentialIsExcludedFromHTTPTokenPair(t *testing.T) {
	tokens, err := TokenPair(&authpb.TokenPair{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 60, BrowserSessionToken: "private-browser-secret"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(tokens)
	if err != nil || strings.Contains(string(encoded), "private-browser") || strings.Contains(string(encoded), "browser_session") {
		t.Fatal("browser cookie credential leaked into HTTP JSON")
	}
}
