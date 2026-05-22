// Package telemetry provides structured logging and performance metrics
// for the GhostCLI proxy.
package telemetry

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	defaultLogger *slog.Logger
	loggerOnce    sync.Once
)

// LogLevel represents the severity of a log entry.
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// ParseLogLevel converts a string to a LogLevel.
func ParseLogLevel(level string) (LogLevel, error) {
	switch strings.ToLower(level) {
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("invalid log level: %s", level)
	}
}

// ToSlogLevel converts the telemetry LogLevel to slog.Level.
func (l LogLevel) ToSlogLevel() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// LoggerOptions configures the structured logger.
type LoggerOptions struct {
	Level       LogLevel
	Output      io.Writer
	AddSource   bool
	JSONFormat  bool
}

// DefaultLoggerOptions returns sensible defaults.
func DefaultLoggerOptions() LoggerOptions {
	return LoggerOptions{
		Level:      LevelInfo,
		Output:     os.Stderr,
		AddSource:  false,
		JSONFormat: false,
	}
}

// NewLogger creates a structured slog.Logger with the given options.
func NewLogger(opts LoggerOptions) *slog.Logger {
	var handler slog.Handler

	replaceAttr := func(groups []string, a slog.Attr) slog.Attr {
		// Rename "msg" to "message" for consistency
		if a.Key == slog.MessageKey {
			return slog.String("message", a.Value.String())
		}
		return a
	}

	handlerOpts := &slog.HandlerOptions{
		Level:       opts.Level.ToSlogLevel(),
		AddSource:   opts.AddSource,
		ReplaceAttr: replaceAttr,
	}

	if opts.JSONFormat {
		handler = slog.NewJSONHandler(opts.Output, handlerOpts)
	} else {
		handler = slog.NewTextHandler(opts.Output, handlerOpts)
	}

	return slog.New(handler)
}

// InitDefaultLogger initializes the package-level default logger once.
func InitDefaultLogger(opts LoggerOptions) {
	loggerOnce.Do(func() {
		defaultLogger = NewLogger(opts)
	})
}

// Default returns the package-level default logger.
// If InitDefaultLogger has not been called, it returns a logger writing to os.Stderr at Info level.
func Default() *slog.Logger {
	if defaultLogger == nil {
		InitDefaultLogger(DefaultLoggerOptions())
	}
	return defaultLogger
}

// RequestLogEntry holds fields for a single request/response cycle.
type RequestLogEntry struct {
	Method       string        `json:"method"`
	Path         string        `json:"path"`
	RemoteAddr   string        `json:"remote_addr"`
	StatusCode   int           `json:"status_code"`
	Duration     time.Duration `json:"duration"`
	DurationMs   float64       `json:"duration_ms"`
	Provider     string        `json:"provider"`
	Model        string        `json:"model,omitempty"`
	TTFT         time.Duration `json:"ttft,omitempty"`       // Time to first token
	TTFTMs       float64       `json:"ttft_ms,omitempty"`
	InputTokens  int           `json:"input_tokens,omitempty"`
	OutputTokens int           `json:"output_tokens,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// LogRequest logs a completed request with all telemetry fields.
func LogRequest(logger *slog.Logger, entry RequestLogEntry) {
	attrs := []any{
		slog.String("method", entry.Method),
		slog.String("path", entry.Path),
		slog.String("remote_addr", entry.RemoteAddr),
		slog.Int("status_code", entry.StatusCode),
		slog.Float64("duration_ms", entry.DurationMs),
		slog.String("provider", entry.Provider),
	}

	if entry.Model != "" {
		attrs = append(attrs, slog.String("model", entry.Model))
	}
	if entry.TTFT > 0 {
		attrs = append(attrs, slog.Float64("ttft_ms", entry.TTFTMs))
	}
	if entry.InputTokens > 0 {
		attrs = append(attrs, slog.Int("input_tokens", entry.InputTokens))
	}
	if entry.OutputTokens > 0 {
		attrs = append(attrs, slog.Int("output_tokens", entry.OutputTokens))
	}
	if entry.Error != "" {
		attrs = append(attrs, slog.String("error", entry.Error))
	}

	if entry.StatusCode >= 500 || entry.Error != "" {
		logger.Error("request failed", attrs...)
	} else if entry.StatusCode >= 400 {
		logger.Warn("request warning", attrs...)
	} else {
		logger.Info("request completed", attrs...)
	}
}

// LogStreamEvent logs a single streaming event for debugging.
func LogStreamEvent(logger *slog.Logger, eventType string, provider string) {
	logger.Debug("stream event",
		slog.String("event_type", eventType),
		slog.String("provider", provider),
	)
}

// LogProviderError logs a provider-level error with context.
func LogProviderError(logger *slog.Logger, provider string, statusCode int, err error) {
	logger.Error("provider error",
		slog.String("provider", provider),
		slog.Int("status_code", statusCode),
		slog.String("error", err.Error()),
	)
}
