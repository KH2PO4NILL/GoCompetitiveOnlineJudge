package httpbridge

import (
	"crypto/tls"
	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrowserSessionCookiesAndCSRF(t *testing.T) {
	app := httptest.NewRequest("POST", "https://judge.example/api/v1/auth/login/username", nil)
	app.TLS = &tls.ConnectionState{}
	app.Header.Set(browserSessionHeader, "1")
	app.Header.Set("Origin", "https://judge.example")
	if err := CheckBrowserSession(app); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	SetBrowserSession(w, app, "private-refresh")
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing cookie")
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != 3 || cookie.Path != "/api/v1/auth" {
		t.Fatalf("unsafe cookie attributes: %+v", cookie)
	}
	app.AddCookie(cookie)
	if got := BrowserRefreshToken(app, "old-memory-token"); got != "private-refresh" {
		t.Fatal("cookie rotation not authoritative")
	}
	admin := httptest.NewRequest("POST", "https://judge.example/api/v1/admin/auth/token/refresh", nil)
	admin.Header.Set(browserSessionHeader, "1")
	admin.AddCookie(cookie)
	if BrowserRefreshToken(admin, "") != "" {
		t.Fatal("app cookie reused for admin")
	}
	admin.Header.Set("Origin", "https://other.example")
	if CheckBrowserSession(admin) == nil {
		t.Fatal("cross origin accepted")
	}
	admin.Header.Del("Origin")
	admin.Header.Set("Sec-Fetch-Site", "cross-site")
	if CheckBrowserSession(admin) == nil {
		t.Fatal("cross-site accepted")
	}
	w = httptest.NewRecorder()
	SetBrowserSession(w, app, "")
	if w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not delete cookie")
	}
}
func TestPreciseAuthenticationErrorSurvivesHTTPMapping(t *testing.T) {
	ConfigureErrorHandler()
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	httpx.ErrorCtx(req.Context(), w, sharedauth.AuthenticationError(codes.Unauthenticated, "access_token_invalid", "expired"))
	if w.Code != 401 || w.Body.String() != "{\"code\":\"access_token_invalid\",\"message\":\"expired\"}" {
		t.Fatalf("authentication mapping: %d %s", w.Code, w.Body.String())
	}
}

func TestBrowserCredentialContextIsScopedAndOptIn(t *testing.T) {
	request := httptest.NewRequest("POST", "https://judge.example/api/v1/auth/token/refresh", nil)
	writer := httptest.NewRecorder()
	ctx := WithBrowserSession(request.Context(), writer, request)
	CaptureBrowserSession(ctx, "hidden-credential")
	if len(writer.Result().Cookies()) != 0 || UsesBrowserSession(ctx) {
		t.Fatal("API client unexpectedly used cookies")
	}
	request.Header.Set(browserSessionHeader, "1")
	request.AddCookie(&http.Cookie{Name: "gcoj_app_refresh", Value: "hidden-credential"})
	writer = httptest.NewRecorder()
	ctx = WithBrowserSession(request.Context(), writer, request)
	CaptureBrowserSession(ctx, "hidden-credential")
	if !UsesBrowserSession(ctx) || len(writer.Result().Cookies()) != 1 {
		t.Fatal("browser credential not captured")
	}
}
