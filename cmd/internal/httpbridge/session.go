package httpbridge

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const browserSessionHeader = "X-GCOJ-Browser-Session"

// Browser sessions opt in explicitly. Cookies never replace credentials for API clients.
func CheckBrowserSession(r *http.Request) error {
	if r.Header.Get(browserSessionHeader) != "1" {
		return nil
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" || site == "same-site" {
		return status.Error(codes.PermissionDenied, "浏览器会话请求必须来自同一来源")
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host != r.Host || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return status.Error(codes.PermissionDenied, "浏览器会话请求必须来自同一来源")
		}
	}
	return nil
}
func sessionCookie(r *http.Request) *http.Cookie {
	name, path := "gcoj_app_refresh", "/api/v1/auth"
	if strings.HasPrefix(r.URL.Path, "/api/v1/admin/") {
		name, path = "gcoj_admin_refresh", "/api/v1/admin/auth"
	}
	return &http.Cookie{Name: name, Path: path, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")}
}
func BrowserRefreshToken(r *http.Request, fallback string) string {
	if r.Header.Get(browserSessionHeader) != "1" {
		return fallback
	}
	if cookie, err := r.Cookie(sessionCookie(r).Name); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return fallback
}
func SetBrowserSession(w http.ResponseWriter, r *http.Request, refresh string) {
	if r.Header.Get(browserSessionHeader) != "1" {
		return
	}
	cookie := sessionCookie(r)
	cookie.Value = refresh
	cookie.MaxAge = 30 * 24 * 60 * 60
	cookie.Expires = time.Now().Add(30 * 24 * time.Hour)
	if refresh == "" {
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(1, 0)
	}
	http.SetCookie(w, cookie)
}

type browserSessionContextKey struct{}
type browserSessionContext struct {
	set       func(string)
	hasCookie bool
}

func WithBrowserSession(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	if r.Header.Get(browserSessionHeader) != "1" {
		return ctx
	}
	cookie, err := r.Cookie(sessionCookie(r).Name)
	return context.WithValue(ctx, browserSessionContextKey{}, browserSessionContext{set: func(value string) { SetBrowserSession(w, r, value) }, hasCookie: err == nil && cookie.Value != ""})
}
func UsesBrowserSession(ctx context.Context) bool {
	state, _ := ctx.Value(browserSessionContextKey{}).(browserSessionContext)
	return state.hasCookie
}

// The reusable browser secret is never included in the public HTTP token DTO.
func CaptureBrowserSession(ctx context.Context, secret string) {
	state, ok := ctx.Value(browserSessionContextKey{}).(browserSessionContext)
	if ok && secret != "" {
		state.set(secret)
	}
}
