package judgemetrics

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsAreScrapable(t *testing.T) {
	metrics := New()
	metrics.Add("judge_ack_total", 1)
	metrics.Set("judge_inflight", 2)
	metrics.Observe("judge_latency_seconds", time.Second)
	response := httptest.NewRecorder()
	metrics.ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	for _, expected := range []string{"judge_ack_total 1", "judge_inflight 2", "judge_latency_seconds_count 1", "judge_latency_seconds_sum 1"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatal(response.Body.String())
		}
	}
}
