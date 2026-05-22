package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockEngine is a test implementation of the Engine interface.
type mockEngine struct {
	parseRequestFunc  func(r *http.Request) (interface{}, error)
	streamRequestFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request) error
	providerName      string
	version           string
}

func (m *mockEngine) ParseRequest(r *http.Request) (interface{}, error) {
	if m.parseRequestFunc != nil {
		return m.parseRequestFunc(r)
	}
	return nil, nil
}

func (m *mockEngine) StreamRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if m.streamRequestFunc != nil {
		return m.streamRequestFunc(ctx, w, r)
	}
	return nil
}

func (m *mockEngine) ProviderName() string {
	return m.providerName
}

func (m *mockEngine) Version() string {
	return m.version
}

func TestNewServer(t *testing.T) {
	engine := &mockEngine{providerName: "deepseek", version: "1.0.0"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := NewServer("8080", engine, logger)

	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	if server.engine != engine {
		t.Error("Server engine not set correctly")
	}

	if server.httpServer == nil {
		t.Fatal("Server httpServer is nil")
	}

	if server.httpServer.Addr != ":8080" {
		t.Errorf("Expected addr :8080, got %s", server.httpServer.Addr)
	}

	if server.httpServer.ReadHeaderTimeout != 10*time.Second {
		t.Error("ReadHeaderTimeout not set correctly")
	}

	if server.httpServer.IdleTimeout != 120*time.Second {
		t.Error("IdleTimeout not set correctly")
	}
}

func TestServer_RegisterRoutes(t *testing.T) {
	engine := &mockEngine{providerName: "deepseek", version: "1.0.0"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("8080", engine, logger)

	mux := http.NewServeMux()
	server.registerRoutes(mux)

	// Test that routes are registered by making requests
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "messages endpoint POST",
			method:     http.MethodPost,
			path:       "/v1/messages",
			wantStatus: http.StatusOK, // mock engine returns nil
		},
		{
			name:       "messages endpoint GET method not allowed",
			method:     http.MethodGet,
			path:       "/v1/messages",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "health endpoint GET",
			method:     http.MethodGet,
			path:       "/health",
			wantStatus: http.StatusOK,
		},
		{
			name:       "ping endpoint GET",
			method:     http.MethodGet,
			path:       "/ping",
			wantStatus: http.StatusOK,
		},
		{
			name:       "not found",
			method:     http.MethodGet,
			path:       "/unknown",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			server.httpServer.Handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

func TestHandleMessages(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		streamErr      error
		wantStatusCode int
		wantErrType    string
	}{
		{
			name:           "successful stream",
			method:         http.MethodPost,
			streamErr:      nil,
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "method not allowed",
			method:         http.MethodGet,
			streamErr:      nil,
			wantStatusCode: http.StatusMethodNotAllowed,
			wantErrType:    "invalid_request_error",
		},
		{
			name:           "stream error",
			method:         http.MethodPost,
			streamErr:      errors.New("provider connection failed"),
			wantStatusCode: http.StatusBadGateway,
			wantErrType:    "api_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &mockEngine{
				providerName: "deepseek",
				version:      "1.0.0",
				streamRequestFunc: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
					return tt.streamErr
				},
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			server := NewServer("8080", engine, logger)

			req := httptest.NewRequest(tt.method, "/v1/messages", strings.NewReader("{}"))
			rr := httptest.NewRecorder()

			server.httpServer.Handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("Expected status %d, got %d", tt.wantStatusCode, rr.Code)
			}

			if tt.wantErrType != "" {
				var resp map[string]interface{}
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to parse error response: %v", err)
				}

				errObj, ok := resp["error"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected error object in response")
				}

				if errObj["type"] != tt.wantErrType {
					t.Errorf("Expected error type %s, got %s", tt.wantErrType, errObj["type"])
				}
			}
		})
	}
}

func TestHandleMessages_SSEStream(t *testing.T) {
	engine := &mockEngine{
		providerName: "deepseek",
		version:      "1.0.0",
		streamRequestFunc: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("event: message_start\ndata: {}\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return nil
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("8080", engine, logger)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("{}"))
	rr := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "event: message_start") {
		t.Errorf("Expected SSE event in response, got: %s", body)
	}
}

func TestHandleHealth(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		providerName   string
		version        string
		wantStatusCode int
		wantResponse   HealthResponse
	}{
		{
			name:           "GET health",
			method:         http.MethodGet,
			providerName:   "deepseek",
			version:        "1.2.3",
			wantStatusCode: http.StatusOK,
			wantResponse: HealthResponse{
				Status:   "ok",
				Provider: "deepseek",
				Version:  "1.2.3",
			},
		},
		{
			name:           "POST health method not allowed",
			method:         http.MethodPost,
			providerName:   "deepseek",
			version:        "1.2.3",
			wantStatusCode: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &mockEngine{
				providerName: tt.providerName,
				version:      tt.version,
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			server := NewServer("8080", engine, logger)

			req := httptest.NewRequest(tt.method, "/health", nil)
			rr := httptest.NewRecorder()

			server.httpServer.Handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("Expected status %d, got %d", tt.wantStatusCode, rr.Code)
			}

			if tt.wantStatusCode == http.StatusOK {
				var resp HealthResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to parse health response: %v", err)
				}

				if resp.Status != tt.wantResponse.Status {
					t.Errorf("Expected status %s, got %s", tt.wantResponse.Status, resp.Status)
				}
				if resp.Provider != tt.wantResponse.Provider {
					t.Errorf("Expected provider %s, got %s", tt.wantResponse.Provider, resp.Provider)
				}
				if resp.Version != tt.wantResponse.Version {
					t.Errorf("Expected version %s, got %s", tt.wantResponse.Version, resp.Version)
				}

				// Verify Content-Type header
				contentType := rr.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", contentType)
				}
			}
		})
	}
}

func TestHandleHealth_ResponseTime(t *testing.T) {
	engine := &mockEngine{providerName: "deepseek", version: "1.0.0"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("8080", engine, logger)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	start := time.Now()
	server.httpServer.Handler.ServeHTTP(rr, req)
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("Health check took %v, expected sub-100ms", elapsed)
	}
}

func TestHandlePing(t *testing.T) {
	engine := &mockEngine{providerName: "deepseek", version: "1.0.0"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("8080", engine, logger)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	body := strings.TrimSpace(rr.Body.String())
	if body != "pong" {
		t.Errorf("Expected body 'pong', got '%s'", body)
	}
}

func TestHandleNotFound(t *testing.T) {
	engine := &mockEngine{providerName: "deepseek", version: "1.0.0"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("8080", engine, logger)

	req := httptest.NewRequest(http.MethodGet, "/unknown-endpoint", nil)
	rr := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected error object in response")
	}

	if errObj["type"] != "not_found" {
		t.Errorf("Expected error type 'not_found', got %s", errObj["type"])
	}

	if errObj["message"] != "endpoint not found" {
		t.Errorf("Expected message 'endpoint not found', got %s", errObj["message"])
	}
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		errType    string
		message    string
		statusCode int
	}{
		{
			name:       "bad request error",
			errType:    "invalid_request_error",
			message:    "missing required field",
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "not found error",
			errType:    "not_found_error",
			message:    "resource not found",
			statusCode: http.StatusNotFound,
		},
		{
			name:       "internal error",
			errType:    "api_error",
			message:    "internal server error",
			statusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			writeError(rr, tt.errType, tt.message, tt.statusCode)

			if rr.Code != tt.statusCode {
				t.Errorf("Expected status %d, got %d", tt.statusCode, rr.Code)
			}

			contentType := rr.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Expected Content-Type application/json, got %s", contentType)
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to parse error response: %v", err)
			}

			if resp["type"] != "error" {
				t.Errorf("Expected type 'error', got %v", resp["type"])
			}

			errObj, ok := resp["error"].(map[string]interface{})
			if !ok {
				t.Fatal("Expected error object in response")
			}

			if errObj["type"] != tt.errType {
				t.Errorf("Expected error type %s, got %s", tt.errType, errObj["type"])
			}

			if errObj["message"] != tt.message {
				t.Errorf("Expected message %s, got %s", tt.message, errObj["message"])
			}
		})
	}
}

func TestCORSMiddleware(t *testing.T) {
	engine := &mockEngine{providerName: "deepseek", version: "1.0.0"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("8080", engine, logger)

	tests := []struct {
		name               string
		method             string
		wantCORSOrigin     string
		wantCORSMethods    string
		wantCORSHeaders    string
		wantStatus         int
	}{
		{
			name:            "GET request includes CORS headers",
			method:          http.MethodGet,
			wantCORSOrigin:  "*",
			wantCORSMethods: "POST, GET, OPTIONS",
			wantCORSHeaders: "Content-Type, anthropic-version, x-api-key, Authorization",
			wantStatus:      http.StatusOK,
		},
		{
			name:            "OPTIONS preflight request",
			method:          http.MethodOptions,
			wantCORSOrigin:  "*",
			wantCORSMethods: "POST, GET, OPTIONS",
			wantCORSHeaders: "Content-Type, anthropic-version, x-api-key, Authorization",
			wantStatus:      http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/ping", nil)
			rr := httptest.NewRecorder()

			server.httpServer.Handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			origin := rr.Header().Get("Access-Control-Allow-Origin")
			if origin != tt.wantCORSOrigin {
				t.Errorf("Expected Access-Control-Allow-Origin %s, got %s", tt.wantCORSOrigin, origin)
			}

			methods := rr.Header().Get("Access-Control-Allow-Methods")
			if methods != tt.wantCORSMethods {
				t.Errorf("Expected Access-Control-Allow-Methods %s, got %s", tt.wantCORSMethods, methods)
			}

			headers := rr.Header().Get("Access-Control-Allow-Headers")
			if headers != tt.wantCORSHeaders {
				t.Errorf("Expected Access-Control-Allow-Headers %s, got %s", tt.wantCORSHeaders, headers)
			}
		})
	}
}

func TestTimeoutMiddleware(t *testing.T) {
	// Test that timeout middleware doesn't interfere with fast requests
	engine := &mockEngine{
		providerName: "deepseek",
		version:      "1.0.0",
		streamRequestFunc: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
			return nil
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("8080", engine, logger)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("{}"))
	rr := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestContextMiddleware(t *testing.T) {
	// Verify that context is propagated through the middleware chain
	var capturedContext context.Context
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedContext = r.Context()
		w.WriteHeader(http.StatusOK)
	})

	wrapped := contextMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	if capturedContext == nil {
		t.Error("Context was not propagated through middleware")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestServer_StartShutdown(t *testing.T) {
	engine := &mockEngine{providerName: "deepseek", version: "1.0.0"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("0", engine, logger) // port 0 = auto-assign

	// Start server in background
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("Server start error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
}

func TestApplyMiddleware_Order(t *testing.T) {
	// Verify middleware chain executes without panic
	engine := &mockEngine{providerName: "deepseek", version: "1.0.0"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("8080", engine, logger)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()

	// Should not panic
	server.httpServer.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestHandleMessages_MultipleMethods(t *testing.T) {
	engine := &mockEngine{providerName: "deepseek", version: "1.0.0"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("8080", engine, logger)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/v1/messages", nil)
			rr := httptest.NewRecorder()

			server.httpServer.Handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status %d for %s, got %d", http.StatusMethodNotAllowed, method, rr.Code)
			}
		})
	}
}

func TestHandleHealth_MultipleMethods(t *testing.T) {
	engine := &mockEngine{providerName: "deepseek", version: "1.0.0"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("8080", engine, logger)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health", nil)
			rr := httptest.NewRecorder()

			server.httpServer.Handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status %d for %s, got %d", http.StatusMethodNotAllowed, method, rr.Code)
			}
		})
	}
}

func TestServer_GracefulShutdown(t *testing.T) {
	engine := &mockEngine{providerName: "deepseek", version: "1.0.0"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("0", engine, logger)

	// Start server
	go func() {
		_ = server.Start()
	}()

	time.Sleep(50 * time.Millisecond)

	// Shutdown with context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Expected no error on graceful shutdown, got: %v", err)
	}
}

func TestWriteError_InvalidStatusCode(t *testing.T) {
	// Test writeError with various status codes
	tests := []struct {
		statusCode int
	}{
		{http.StatusBadRequest},
		{http.StatusUnauthorized},
		{http.StatusForbidden},
		{http.StatusNotFound},
		{http.StatusTooManyRequests},
		{http.StatusInternalServerError},
		{http.StatusBadGateway},
		{http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.statusCode), func(t *testing.T) {
			rr := httptest.NewRecorder()
			writeError(rr, "test_error", "test message", tt.statusCode)

			if rr.Code != tt.statusCode {
				t.Errorf("Expected status %d, got %d", tt.statusCode, rr.Code)
			}

			// Verify JSON is valid
			var resp map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Errorf("Invalid JSON response: %v", err)
			}
		})
	}
}

func TestLoggingMiddleware(t *testing.T) {
	// Verify logging middleware doesn't panic and calls next handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wrapped := loggingMiddleware(handler, logger)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	body := strings.TrimSpace(rr.Body.String())
	if body != "ok" {
		t.Errorf("Expected body 'ok', got '%s'", body)
	}
}
