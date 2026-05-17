# Module 06: Observability

## Overview

The Observability module provides structured logging, performance metrics, and error tracking for GhostCLI.

## Responsibilities

- Structured logging with configurable verbosity
- Performance metrics (TTFT, request duration)
- Error tracking and reporting
- Request/response logging
- Provider selection logging
- Model mapping logging

## Architecture

```
Observability
├── Structured Logging (slog)
│   ├── Log Levels (debug, info, warn, error)
│   ├── Contextual Fields
│   └── Timestamp Formatting (RFC3339)
├── Performance Metrics
│   ├── TTFT (Time-to-First-Token)
│   ├── Request Duration
│   └── Token Throughput
└── Error Tracking
    ├── Error Types
    ├── Stack Traces
    └── Error Context
```

## Related Requirements

- **Requirement 16**: Error Handling and Propagation
- **Requirement 17**: Structured Logging

## Key Components

### Structured Logging

**Log Levels**:
- **debug**: Detailed information (model mapping, internal state)
- **info**: Normal operations (requests, provider selection)
- **warn**: Recoverable issues (keyring fallback, missing usage)
- **error**: Errors affecting functionality (provider failures, parsing errors)

**Log Format**:
```json
{
  "time": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "msg": "request completed",
  "method": "POST",
  "path": "/v1/messages",
  "status": 200,
  "duration_ms": 1234,
  "provider": "deepseek",
  "model": "deepseek-chat"
}
```

**Verbosity Control**:
- `--verbose` flag → debug level
- Default → info level

### Performance Metrics

**TTFT (Time-to-First-Token)**:
```go
start := time.Now()
// ... wait for first event
ttft := time.Since(start)
logger.Info("first token received", "ttft_ms", ttft.Milliseconds())
```

**Request Duration**:
```go
start := time.Now()
// ... complete request
duration := time.Since(start)
logger.Info("request completed", "duration_ms", duration.Milliseconds())
```

**Token Throughput**:
```go
tokensPerSec := float64(totalTokens) / duration.Seconds()
logger.Info("streaming completed", "tokens_per_sec", tokensPerSec)
```

### Error Tracking

**Error Context**:
```go
logger.Error("provider request failed",
    "provider", provider,
    "status_code", statusCode,
    "error", err,
    "request_id", requestID,
)
```

**Error Types**:
- Configuration errors
- Network errors
- Provider errors
- Parsing errors
- Validation errors

## Logging Examples

### Request Logging
```go
logger.Info("request received",
    "method", r.Method,
    "path", r.URL.Path,
    "provider", provider,
)
```

### Provider Selection
```go
logger.Info("provider selected",
    "provider", provider,
    "model", anthropicModel,
    "mapped_model", providerModel,
)
```

### Model Mapping
```go
logger.Debug("model mapped",
    "anthropic_model", anthropicModel,
    "provider_model", providerModel,
)
```

### Error Logging
```go
logger.Error("failed to parse request",
    "error", err,
    "content_type", r.Header.Get("Content-Type"),
)
```

## Implementation Details

See [design.md](./design.md) for detailed implementation specifications.
