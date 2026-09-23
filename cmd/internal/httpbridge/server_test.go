package httpbridge

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/core/logx"
)

func TestSafeAccessLogDoesNotDumpSensitiveContent(t *testing.T) {
	var logs bytes.Buffer
	previous := logx.Reset()
	logx.SetWriter(logx.NewWriter(&logs))
	t.Cleanup(func() { logx.SetWriter(previous) })
	for _, statusCode := range []int{http.StatusOK, http.StatusUnauthorized, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		logs.Reset()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login/username?secret-query-sentinel", strings.NewReader("secret-password-sentinel"))
		request.Header.Set("Authorization", "Bearer secret-token-sentinel")
		request.Header.Set("Cookie", "session=secret-cookie-sentinel")
		request.Header.Set("User-Agent", "secret-agent-sentinel")
		response := httptest.NewRecorder()
		SafeAccessLog(func(writer http.ResponseWriter, received *http.Request) {
			body, err := io.ReadAll(received.Body)
			if err != nil || string(body) != "secret-password-sentinel" || received.Header.Get("Authorization") != "Bearer secret-token-sentinel" {
				t.Fatal("logging middleware modified the real request")
			}
			writer.WriteHeader(statusCode)
			_, _ = writer.Write([]byte("secret-response-sentinel"))
		})(response, request)
		if strings.Contains(logs.String(), "secret-") || !strings.Contains(logs.String(), `"status":`) || !strings.Contains(logs.String(), `"method":"POST"`) {
			t.Fatal("access log leaked sensitive content or omitted safe status metadata")
		}
		if response.Code != statusCode || response.Body.String() != "secret-response-sentinel" {
			t.Fatal("logging middleware changed the response")
		}
	}
}

func TestStatusWriterPreservesFirstFinalStatus(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := &statusWriter{ResponseWriter: recorder}
	writer.WriteHeader(http.StatusAccepted)
	writer.WriteHeader(http.StatusInternalServerError)
	if _, err := writer.Write([]byte("ok")); err != nil {
		t.Fatal(err)
	}
	if writer.statusCode != http.StatusAccepted || recorder.Code != http.StatusAccepted || writer.Unwrap() != recorder {
		t.Fatal("response status or underlying writer changed")
	}
	implicit := &statusWriter{ResponseWriter: httptest.NewRecorder()}
	_, _ = implicit.Write([]byte("ok"))
	if implicit.statusCode != http.StatusOK {
		t.Fatal("implicit status was not captured")
	}
}
