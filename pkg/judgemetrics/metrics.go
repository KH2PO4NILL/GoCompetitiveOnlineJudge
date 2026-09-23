package judgemetrics

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type Metrics struct {
	mu     sync.RWMutex
	values map[string]float64
}

func New() *Metrics { return &Metrics{values: map[string]float64{}} }
func (metrics *Metrics) Add(name string, value float64) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.values[name] += value
}
func (metrics *Metrics) Set(name string, value float64) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.values[name] = value
}
func (metrics *Metrics) Get(name string) float64 {
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()
	return metrics.values[name]
}
func (metrics *Metrics) Observe(name string, duration time.Duration) {
	metrics.Add(name+"_sum", duration.Seconds())
	metrics.Add(name+"_count", 1)
}
func (metrics *Metrics) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()
	writer.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	keys := make([]string, 0, len(metrics.values))
	for key := range metrics.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		_, _ = fmt.Fprintf(writer, "%s %g\n", key, metrics.values[key])
	}
}
func (metrics *Metrics) Listen(address, path string) (*http.Server, error) {
	return metrics.ListenWithProbes(address, path, nil)
}

func ValidatePath(path string) error {
	if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "\r\n\t {}?#\\%") || path == "/healthz" || path == "/readyz" {
		return errors.New("metrics path must be a literal absolute path distinct from /healthz and /readyz")
	}
	if _, err := url.ParseRequestURI(path); err != nil {
		return errors.New("metrics path must be a valid request path")
	}
	return nil
}

func (metrics *Metrics) ListenWithProbes(address, path string, ready func() bool) (*http.Server, error) {
	if err := ValidatePath(path); err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle(path, metrics)
	if ready != nil {
		mux.HandleFunc("/healthz", probe(func() bool { return true }, "ok"))
		mux.HandleFunc("/readyz", probe(ready, "ready"))
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	server := &http.Server{Addr: listener.Addr().String(), Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	go func() { _ = server.Serve(listener) }()
	return server, nil
}

func probe(ready func() bool, healthyStatus string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			writer.Header().Set("Allow", "GET, HEAD")
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		statusCode, state := http.StatusOK, healthyStatus
		if !ready() {
			statusCode, state = http.StatusServiceUnavailable, "not_ready"
		}
		writer.WriteHeader(statusCode)
		if request.Method != http.MethodHead {
			_, _ = fmt.Fprintf(writer, "{\"status\":\"%s\"}\n", state)
		}
	}
}
func Close(ctx context.Context, server *http.Server) error {
	if server == nil {
		return nil
	}
	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
		return err
	}
	return nil
}
