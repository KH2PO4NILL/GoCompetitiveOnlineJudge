package svc

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/sandbox"
)

func TestServiceProbesRemainLiveButNotReadyWithoutConsumer(t *testing.T) {
	settings := testConfig(t.TempDir())
	settings.Metrics.Enabled = true
	settings.Metrics.ListenOn = "127.0.0.1:0"
	service, err := newServiceContext(settings, func(config.Config, *slog.Logger) (sandbox.Sandbox, error) { return &fakeSandbox{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Shutdown(context.Background()) })
	if service.Ready() {
		t.Fatal("unstarted service is ready")
	}
	if err := service.Start(); err != nil {
		t.Fatal(err)
	}
	if service.Ready() {
		t.Fatal("disabled consumer must not advertise scheduling readiness")
	}
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	t.Cleanup(client.CloseIdleConnections)
	for _, path := range []string{"/healthz", "/readyz"} {
		service.lifecycleMu.Lock()
		response, err := client.Get("http://" + service.metrics.Addr + path)
		service.lifecycleMu.Unlock()
		if err != nil {
			t.Fatal("probe blocked on lifecycle lock:", err)
		}
		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		want := http.StatusOK
		if path == "/readyz" {
			want = http.StatusServiceUnavailable
		}
		if err != nil || response.StatusCode != want || strings.Contains(string(body), "secret") {
			t.Fatalf("unexpected probe: status=%d error=%v", response.StatusCode, err)
		}
	}
	if err := service.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if service.Ready() || !service.draining.Load() {
		t.Fatal("shutdown failed to withdraw readiness")
	}
}
