package judgemetrics

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProbesSeparateLivenessReadinessAndMetrics(t *testing.T) {
	metrics := New()
	metrics.Set("judge_test_total", 1)
	var ready atomic.Bool
	server, err := metrics.ListenWithProbes("127.0.0.1:0", "/custom-metrics", ready.Load)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close(context.Background(), server) })
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	t.Cleanup(client.CloseIdleConnections)
	for _, test := range []struct {
		method string
		path   string
		ready  bool
		status int
		body   string
	}{
		{http.MethodGet, "/healthz", false, http.StatusOK, `{"status":"ok"}`},
		{http.MethodGet, "/readyz", false, http.StatusServiceUnavailable, `{"status":"not_ready"}`},
		{http.MethodGet, "/readyz", true, http.StatusOK, `{"status":"ready"}`},
		{http.MethodGet, "/readyz", false, http.StatusServiceUnavailable, `{"status":"not_ready"}`},
		{http.MethodHead, "/readyz", true, http.StatusOK, ""},
		{http.MethodHead, "/readyz", false, http.StatusServiceUnavailable, ""},
		{http.MethodPost, "/readyz", true, http.StatusMethodNotAllowed, ""},
		{http.MethodGet, "/custom-metrics", true, http.StatusOK, "judge_test_total 1"},
	} {
		ready.Store(test.ready)
		request, err := http.NewRequest(test.method, "http://"+server.Addr+test.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil || response.StatusCode != test.status || strings.TrimSpace(string(body)) != test.body {
			t.Fatalf("%s %s: status=%d body=%q error=%v", test.method, test.path, response.StatusCode, body, err)
		}
		if test.path != "/custom-metrics" && response.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("probe response must not be cached")
		}
	}
}

func TestInvalidMetricsPathsFailWithoutPanic(t *testing.T) {
	for _, path := range []string{"metrics", "/{", "/metrics/{tail...}", "/healthz", "/readyz", "/metrics?secret", "/metrics\x00", "/metrics name"} {
		t.Run(path, func(t *testing.T) {
			if server, err := New().ListenWithProbes("127.0.0.1:0", path, func() bool { return true }); err == nil || server != nil {
				_ = Close(context.Background(), server)
				t.Fatal("invalid metrics path opened a listener")
			}
		})
	}
}
