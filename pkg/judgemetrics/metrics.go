package judgemetrics

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sort"
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
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle(path, metrics)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	go func() { _ = server.Serve(listener) }()
	return server, nil
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
