# Observability

> **Component**: Cross-Cutting | **Layer**: 5 (Telemetry) | **Related**: [http-server.md](./http-server.md)

The Observability component provides structured logging, metrics, and telemetry for monitoring and debugging GhostCLI.

## Overview

```
┌──────────────────────────────────────────────────┐
│           Observability Layer                    │
│                                                  │
│  ┌──────────────┐    ┌──────────────┐           │
│  │   Logging    │    │   Metrics    │           │
│  │    (slog)    │    │  (optional)  │           │
│  └──────────────┘    └──────────────┘           │
│                                                  │
└──────────────────────────────────────────────────┘
```

## Structured Logging

GhostCLI uses Go's `log/slog` package for structured, leveled logging.

### Logger Initialization

```go
package telemetry

import (
    "log/slog"
    "os"
)

func InitLogger(verbose bool) {
    level := slog.LevelInfo
    if verbose {
        level = slog.LevelDebug
    }
    
    handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: level,
        AddSource: verbose,  // Include source file/line in debug mode
    })
    
    logger := slog.New(handler)
    slog.SetDefault(logger)
}
```

### Log Levels

```go
// Debug: Detailed information for debugging
slog.Debug("Parsing request", "model", req.Model, "tokens", req.MaxTokens)

// Info: General informational messages
slog.Info("Server started", "port", 3200, "provider", "deepseek")

// Warn: Warning messages (non-critical issues)
slog.Warn("Keyring unavailable, using encrypted file", "provider", "deepseek")

// Error: Error messages (operation failed)
slog.Error("Provider request failed", "error", err, "provider", "deepseek")
```

### Structured Fields

```go
// ✅ Good: Structured fields
slog.Info("Request completed",
    "method", r.Method,
    "path", r.URL.Path,
    "status", 200,
    "duration_ms", duration.Milliseconds(),
    "provider", "deepseek",
)

// ❌ Bad: String interpolation
log.Printf("Request %s %s completed with status %d in %v", 
    r.Method, r.URL.Path, 200, duration)
```

### Context-Aware Logging

```go
// Add logger to context
ctx := context.WithValue(r.Context(), "logger", logger.With(
    "request_id", requestID,
    "provider", provider,
))

// Use logger from context
logger := ctx.Value("logger").(*slog.Logger)
logger.Info("Processing request")
```

## Logging Examples

### Server Startup

```go
slog.Info("Starting GhostCLI",
    "version", "1.0.0",
    "port", 3200,
    "provider", "deepseek",
)
```

Output:
```json
{
  "time": "2025-05-17T10:30:00Z",
  "level": "INFO",
  "msg": "Starting GhostCLI",
  "version": "1.0.0",
  "port": 3200,
  "provider": "deepseek"
}
```

### Request Processing

```go
slog.Debug("Parsing Anthropic request",
    "model", req.Model,
    "messages", len(req.Messages),
    "max_tokens", req.MaxTokens,
    "stream", req.Stream,
)

slog.Info("Request routed",
    "provider", "deepseek",
    "model", "deepseek-v4-pro",
)

slog.Info("Request completed",
    "duration_ms", duration.Milliseconds(),
    "input_tokens", 150,
    "output_tokens", 75,
)
```

### Error Logging

```go
slog.Error("Provider request failed",
    "error", err,
    "provider", "deepseek",
    "status_code", resp.StatusCode,
    "retry_attempt", 1,
)
```

## Metrics (Optional)

For production deployments, GhostCLI can export metrics for monitoring.

### Metric Types

```go
package telemetry

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Request counter
    requestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ghostcli_requests_total",
            Help: "Total number of requests",
        },
        []string{"provider", "status"},
    )
    
    // Request duration histogram
    requestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "ghostcli_request_duration_seconds",
            Help:    "Request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"provider"},
    )
    
    // Token usage counter
    tokensTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ghostcli_tokens_total",
            Help: "Total number of tokens processed",
        },
        []string{"provider", "type"},  // type: input/output
    )
    
    // Active connections gauge
    activeConnections = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "ghostcli_active_connections",
            Help: "Number of active connections",
        },
        []string{"provider"},
    )
)
```

### Recording Metrics

```go
func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    provider := s.engine.GetProviderName()
    
    // Increment active connections
    activeConnections.WithLabelValues(provider).Inc()
    defer activeConnections.WithLabelValues(provider).Dec()
    
    // Process request...
    
    // Record duration
    duration := time.Since(start)
    requestDuration.WithLabelValues(provider).Observe(duration.Seconds())
    
    // Record request count
    requestsTotal.WithLabelValues(provider, "success").Inc()
    
    // Record token usage
    tokensTotal.WithLabelValues(provider, "input").Add(float64(inputTokens))
    tokensTotal.WithLabelValues(provider, "output").Add(float64(outputTokens))
}
```

### Metrics Endpoint

```go
import (
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) registerRoutes() {
    s.router.HandleFunc("/v1/messages", s.handleMessages)
    s.router.HandleFunc("/health", s.handleHealth)
    s.router.Handle("/metrics", promhttp.Handler())  // Prometheus metrics
}
```

### Example Metrics Output

```
# HELP ghostcli_requests_total Total number of requests
# TYPE ghostcli_requests_total counter
ghostcli_requests_total{provider="deepseek",status="success"} 1523

# HELP ghostcli_request_duration_seconds Request duration in seconds
# TYPE ghostcli_request_duration_seconds histogram
ghostcli_request_duration_seconds_bucket{provider="deepseek",le="0.5"} 1200
ghostcli_request_duration_seconds_bucket{provider="deepseek",le="1"} 1450
ghostcli_request_duration_seconds_sum{provider="deepseek"} 1234.5
ghostcli_request_duration_seconds_count{provider="deepseek"} 1523

# HELP ghostcli_tokens_total Total number of tokens processed
# TYPE ghostcli_tokens_total counter
ghostcli_tokens_total{provider="deepseek",type="input"} 125000
ghostcli_tokens_total{provider="deepseek",type="output"} 75000

# HELP ghostcli_active_connections Number of active connections
# TYPE ghostcli_active_connections gauge
ghostcli_active_connections{provider="deepseek"} 3
```

## Performance Tracking

### Request Timing

```go
type RequestTimer struct {
    start time.Time
    stages map[string]time.Duration
}

func NewRequestTimer() *RequestTimer {
    return &RequestTimer{
        start: time.Now(),
        stages: make(map[string]time.Duration),
    }
}

func (t *RequestTimer) Mark(stage string) {
    t.stages[stage] = time.Since(t.start)
}

func (t *RequestTimer) Log() {
    slog.Debug("Request timing breakdown",
        "total_ms", time.Since(t.start).Milliseconds(),
        "parse_ms", t.stages["parse"].Milliseconds(),
        "route_ms", t.stages["route"].Milliseconds(),
        "provider_ms", t.stages["provider"].Milliseconds(),
        "format_ms", t.stages["format"].Milliseconds(),
    )
}
```

Usage:
```go
timer := NewRequestTimer()

// Parse request
req, _ := parseRequest(r)
timer.Mark("parse")

// Route to provider
provider, _ := router.Route(req)
timer.Mark("route")

// Call provider
events, _ := provider.StreamChat(ctx, req)
timer.Mark("provider")

// Format response
formatResponse(w, events)
timer.Mark("format")

timer.Log()
```

## Error Tracking

### Error Context

```go
type ErrorContext struct {
    Provider    string
    Model       string
    StatusCode  int
    ErrorType   string
    Message     string
    Timestamp   time.Time
}

func LogError(ctx *ErrorContext) {
    slog.Error("Request failed",
        "provider", ctx.Provider,
        "model", ctx.Model,
        "status_code", ctx.StatusCode,
        "error_type", ctx.ErrorType,
        "message", ctx.Message,
        "timestamp", ctx.Timestamp,
    )
}
```

### Error Aggregation

```go
var (
    errorCounts = make(map[string]int)
    errorMutex  sync.Mutex
)

func TrackError(provider, errorType string) {
    errorMutex.Lock()
    defer errorMutex.Unlock()
    
    key := fmt.Sprintf("%s:%s", provider, errorType)
    errorCounts[key]++
}

func GetErrorStats() map[string]int {
    errorMutex.Lock()
    defer errorMutex.Unlock()
    
    stats := make(map[string]int)
    for k, v := range errorCounts {
        stats[k] = v
    }
    return stats
}
```

## Debug Mode

### Enabling Debug Logging

```bash
# Via flag
ghostcli --provider deepseek --verbose

# Via environment
export GHOSTCLI_VERBOSE=true
ghostcli --provider deepseek
```

### Debug Output

```json
{
  "time": "2025-05-17T10:30:00Z",
  "level": "DEBUG",
  "source": {
    "function": "github.com/ghostcli/internal/engine.(*Engine).ParseRequest",
    "file": "/app/internal/engine/parser.go",
    "line": 45
  },
  "msg": "Parsing Anthropic request",
  "model": "claude-3-7-sonnet",
  "messages": 5,
  "max_tokens": 4096,
  "stream": true
}
```

## Log Rotation (Production)

For production deployments, use log rotation:

```go
import (
    "gopkg.in/natefinch/lumberjack.v2"
)

func InitProductionLogger() {
    logFile := &lumberjack.Logger{
        Filename:   "/var/log/ghostcli/app.log",
        MaxSize:    100,  // MB
        MaxBackups: 3,
        MaxAge:     28,   // days
        Compress:   true,
    }
    
    handler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })
    
    logger := slog.New(handler)
    slog.SetDefault(logger)
}
```

## Monitoring Dashboard

Example Grafana dashboard queries:

```promql
# Request rate
rate(ghostcli_requests_total[5m])

# Average request duration
rate(ghostcli_request_duration_seconds_sum[5m]) / 
rate(ghostcli_request_duration_seconds_count[5m])

# Token usage rate
rate(ghostcli_tokens_total[5m])

# Error rate
rate(ghostcli_requests_total{status="error"}[5m])

# Active connections
ghostcli_active_connections
```

## Related Documentation

- [HTTP Server](./http-server.md) - Request handling
- [Architecture Overview](../architecture/overview.md) - System architecture
