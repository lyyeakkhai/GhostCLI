# HTTP Server

> **Component**: Transport | **Layer**: 2 (HTTP) | **Related**: [translation-engine.md](./translation-engine.md)

The HTTP Server component handles all HTTP protocol concerns, including routing, middleware, and SSE streaming.

## Overview

```
┌──────────────────────────────────────────────────┐
│              HTTP Server                         │
│                                                  │
│  ┌────────────┐    ┌──────────────┐             │
│  │   Router   │───▶│  Middleware  │             │
│  └────────────┘    └──────────────┘             │
│         │                  │                     │
│         ▼                  ▼                     │
│  ┌────────────────────────────────┐             │
│  │     Request Handlers           │             │
│  │  - /v1/messages                │             │
│  │  - /health                     │             │
│  │  - /ping                       │             │
│  └────────────────────────────────┘             │
│                                                  │
└──────────────────────────────────────────────────┘
```

## Core Responsibilities

1. **Listen on configured port** (default: 3200)
2. **Route requests** to appropriate handlers
3. **Apply middleware** (CORS, logging, context)
4. **Stream SSE responses** with proper flushing
5. **Handle errors** and return appropriate status codes

## Implementation

### Server Initialization

```go
package api

import (
    "context"
    "net/http"
    "time"
)

type Server struct {
    router  *http.ServeMux
    engine  *engine.Engine
    port    string
    server  *http.Server
}

func NewServer(port string, eng *engine.Engine) *Server {
    s := &Server{
        router: http.NewServeMux(),
        engine: eng,
        port:   port,
    }
    
    s.registerRoutes()
    s.server = &http.Server{
        Addr:    "127.0.0.1:" + port,
        Handler: s.applyMiddleware(s.router),
    }
    
    return s
}

func (s *Server) Start() error {
    log.Printf("Starting server on http://127.0.0.1:%s", s.port)
    return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
    return s.server.Shutdown(ctx)
}
```

### Routing

```go
func (s *Server) registerRoutes() {
    s.router.HandleFunc("/v1/messages", s.handleMessages)
    s.router.HandleFunc("/health", s.handleHealth)
    s.router.HandleFunc("/ping", s.handlePing)
}
```

### Middleware Chain

```go
func (s *Server) applyMiddleware(handler http.Handler) http.Handler {
    // Apply middleware in reverse order (last applied = first executed)
    handler = s.corsMiddleware(handler)
    handler = s.loggingMiddleware(handler)
    handler = s.contextMiddleware(handler)
    return handler
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, anthropic-version, x-api-key")
        
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        log.Printf("[%s] %s %s", r.Method, r.URL.Path, r.RemoteAddr)
        
        next.ServeHTTP(w, r)
        
        log.Printf("[%s] %s completed in %v", r.Method, r.URL.Path, time.Since(start))
    })
}

func (s *Server) contextMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        // Add any context values here
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Messages Handler

```go
func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    // Parse request
    unifiedReq, err := s.engine.ParseRequest(r)
    if err != nil {
        s.writeError(w, "invalid_request_error", err.Error(), http.StatusBadRequest)
        return
    }
    
    // Route to provider
    events, err := s.engine.StreamChat(r.Context(), unifiedReq)
    if err != nil {
        s.writeError(w, "api_error", err.Error(), http.StatusBadGateway)
        return
    }
    
    // Stream response
    s.streamSSE(w, events)
}

func (s *Server) streamSSE(w http.ResponseWriter, events <-chan engine.UnifiedStreamEvent) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")
    
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming not supported", http.StatusInternalServerError)
        return
    }
    
    // Stream events
    if err := s.engine.FormatResponse(w, events); err != nil {
        log.Printf("Error streaming response: %v", err)
    }
}

func (s *Server) writeError(w http.ResponseWriter, errorType, message string, statusCode int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    
    errorResp := map[string]interface{}{
        "type": "error",
        "error": map[string]string{
            "type":    errorType,
            "message": message,
        },
    }
    
    json.NewEncoder(w).Encode(errorResp)
}
```

### Health Check Handler

```go
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    
    health := map[string]interface{}{
        "status":   "ok",
        "version":  "1.0.0",
        "provider": s.engine.GetProviderName(),
    }
    
    json.NewEncoder(w).Encode(health)
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("pong"))
}
```

## Socket Management

### Listening Socket

The server creates a **TCP listening socket** that stays open for the entire lifetime of the application:

```go
func (s *Server) Start() error {
    // Creates listening socket on 127.0.0.1:3200
    listener, err := net.Listen("tcp", "127.0.0.1:"+s.port)
    if err != nil {
        return err
    }
    
    // http.Serve handles Accept() loop automatically
    return http.Serve(listener, s.router)
}
```

### Connection Sockets

For each incoming request, the listening socket **accepts** a connection and spawns an **ephemeral connection socket**:

```
Listening Socket (127.0.0.1:3200)
    │
    ├─▶ Accept() → Connection Socket #1 (handles request #1)
    ├─▶ Accept() → Connection Socket #2 (handles request #2)
    └─▶ Accept() → Connection Socket #3 (handles request #3)
```

**Lifecycle**:
1. Claude Code connects → Connection socket created
2. Request processed → Data exchanged over connection socket
3. Response complete → Connection socket closed
4. Listening socket remains open for next request

## Graceful Shutdown

```go
func (s *Server) Shutdown(ctx context.Context) error {
    log.Println("Shutting down server...")
    
    // Stop accepting new connections
    if err := s.server.Shutdown(ctx); err != nil {
        return err
    }
    
    log.Println("Server stopped")
    return nil
}

// Usage in main.go
func main() {
    server := api.NewServer("3200", engine)
    
    // Handle shutdown signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        server.Shutdown(ctx)
    }()
    
    if err := server.Start(); err != nil && err != http.ErrServerClosed {
        log.Fatal(err)
    }
}
```

## Error Handling

### Client Errors (4xx)

```go
// Invalid JSON
s.writeError(w, "invalid_request_error", "Invalid JSON format", 400)

// Missing required field
s.writeError(w, "invalid_request_error", "Missing required field: model", 400)

// Unsupported model
s.writeError(w, "invalid_request_error", "Unsupported model: gpt-4", 400)
```

### Provider Errors (5xx)

```go
// Provider API down
s.writeError(w, "api_error", "Provider API unavailable", 502)

// Rate limiting
s.writeError(w, "rate_limit_error", "Rate limit exceeded", 429)

// Authentication failure
s.writeError(w, "authentication_error", "Invalid API key", 401)
```

## Performance Optimizations

### Connection Pooling

The HTTP client used by provider adapters should reuse connections:

```go
var httpClient = &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
    Timeout: 0,  // No timeout for streaming
}
```

### Buffering Strategy

```go
// Use buffered writer for SSE
writer := bufio.NewWriterSize(w, 4096)

for event := range events {
    fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event.Type, event.JSON)
    writer.Flush()  // Flush after each event
}
```

## Testing

### Handler Tests

```go
func TestHandleMessages(t *testing.T) {
    engine := &MockEngine{
        events: []UnifiedStreamEvent{
            {Type: "content_delta", Content: "Hello"},
            {Type: "stop", StopReason: "end_turn"},
        },
    }
    
    server := NewServer("3200", engine)
    
    req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{
        "model": "claude-3-7-sonnet",
        "messages": [{"role": "user", "content": "Hello"}],
        "max_tokens": 1024
    }`))
    req.Header.Set("anthropic-version", "2023-06-01")
    
    recorder := httptest.NewRecorder()
    server.handleMessages(recorder, req)
    
    assert.Equal(t, http.StatusOK, recorder.Code)
    assert.Contains(t, recorder.Body.String(), "Hello")
}
```

## Related Documentation

- [Translation Engine](./translation-engine.md) - Request/response translation
- [Communication Protocol](../architecture/communication-protocol.md) - HTTP/SSE details
- [Architecture Overview](../architecture/overview.md) - System architecture
