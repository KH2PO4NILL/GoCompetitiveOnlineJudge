package config

import (
	"os"
	"strconv"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/outbox"
)

func (config *Config) ApplyEnvironment() error {
	if value := os.Getenv("SUBMIT_OUTBOX_ENABLED"); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		config.Outbox.Enabled = enabled
	}
	return nil
}

type OutboxConf struct {
	Enabled           bool   `json:",optional"`
	Endpoint          string `json:",optional"`
	TLS               bool   `json:",optional"`
	AccessKey         string `json:",optional"`
	SecretKey         string `json:",optional"`
	ID                string `json:",optional"`
	BatchSize         int    `json:",optional"`
	Concurrency       int    `json:",optional"`
	MaxAttempts       int32  `json:",optional"`
	IntervalMS        int64  `json:",optional"`
	ClaimSeconds      int64  `json:",optional"`
	TimeoutSeconds    int64  `json:",optional"`
	BackoffSeconds    int64  `json:",optional"`
	MaxBackoffSeconds int64  `json:",optional"`
	ShutdownSeconds   int64  `json:",optional"`
	MetricsListenOn   string `json:",optional"`
}

func (config OutboxConf) Options(id string) outbox.Options {
	return outbox.Options{ID: id, BatchSize: config.BatchSize, Concurrency: config.Concurrency, MaxAttempts: config.MaxAttempts, Interval: time.Duration(config.IntervalMS) * time.Millisecond, ClaimDuration: time.Duration(config.ClaimSeconds) * time.Second, Timeout: time.Duration(config.TimeoutSeconds) * time.Second, Backoff: time.Duration(config.BackoffSeconds) * time.Second, MaxBackoff: time.Duration(config.MaxBackoffSeconds) * time.Second}
}
