package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/config"
)

func New(c config.LogConf, output io.Writer) (*slog.Logger, error) {
	if output == nil {
		return nil, fmt.Errorf("log output is required")
	}
	var level slog.Level
	switch strings.ToLower(strings.TrimSpace(c.Level)) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, fmt.Errorf("unsupported log level %q", c.Level)
	}
	options := &slog.HandlerOptions{AddSource: c.AddSource, Level: level}
	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(c.Format)) {
	case "text":
		handler = slog.NewTextHandler(output, options)
	case "json":
		handler = slog.NewJSONHandler(output, options)
	default:
		return nil, fmt.Errorf("unsupported log format %q", c.Format)
	}
	return slog.New(handler), nil
}
