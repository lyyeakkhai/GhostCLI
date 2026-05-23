package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ghostcli/internal/config"
	"ghostcli/internal/engine/protocol"
	"ghostcli/internal/engine/translator"
	"ghostcli/internal/telemetry"
)

// flushRecorder wraps httptest.ResponseRecorder to implement http.Flusher.
type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushRecorder) Flush() {
	// No-op: httptest.ResponseRecorder buffers everything anyway
}

// mockProvider is a test implementation of providers.Provider.
type mockProvider struct {
	name            string
	supportsTools   bool
	supportsThinking bool
	modelMap        map[string]string
	streamFunc      func(ctx context.Context, req *protocol.UnifiedChatRequest) (<-chan protocol.UnifiedStreamEvent, error)
}

func (m *mockProvider) StreamChat(ctx context.Context, req *protocol.UnifiedChatRequest) (<-chan protocol.UnifiedStreamEvent, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req)
	}
	ch := make(chan protocol.UnifiedStreamEvent)
	close(ch)
	return ch, nil
}

func (m *mockProvider) Name() string             { return m.name }
func (m *mockProvider) SupportsTools() bool      { return m.supportsTools }
func (m *mockProvider) SupportsThinking() bool   { return m.supportsThinking }
func (m *mockProvider) MapModel(anthropicModel string) string {
	if mapped, ok := m.modelMap[anthropicModel]; ok {
		return mapped
	}
	return anthropicModel
}

// TestNewApp verifies the DI container wires all components correctly.
func TestNewApp(t *testing.T) {
	cfg := &config.Config{
		Port:           3200,
		Host:           "127.0.0.1",
		Timeout:        300,
		ActiveProvider: "mock",
		Providers: map[string]config.ProviderConfig{
			"mock": {
				Name:    "mock",
				Pattern: "openai",
				BaseURL: "http://localhost:9999",
				APIKey:  "test-key",
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := NewApp(cfg, logger)

	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	if app == nil {
		t.Fatal("NewApp returned nil")
	}

	if app.Config != cfg {
		t.Error("App.Config not set correctly")
	}

	if app.Logger == nil {
		t.Error("App.Logger is nil")
	}

	if app.Server == nil {
		t.Error("App.Server is nil")
	}

	if app.Metrics == nil {
		t.Error("App.Metrics is nil")
	}

	if app.ProviderName() != "mock" {
		t.Errorf("Expected provider name 'mock', got '%s'", app.ProviderName())
	}
}

// TestNewApp_NilConfig verifies NewApp rejects nil configuration.
func TestNewApp_NilConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := NewApp(nil, logger)
	if err == nil {
		t.Error("Expected error for nil config, got nil")
	}
}

// TestProxyEngine_StreamRequest verifies the end-to-end streaming flow.
func TestProxyEngine_StreamRequest(t *testing.T) {
	provider := &mockProvider{
		name: "mock",
		streamFunc: func(ctx context.Context, req *protocol.UnifiedChatRequest) (<-chan protocol.UnifiedStreamEvent, error) {
			ch := make(chan protocol.UnifiedStreamEvent, 4)

			go func() {
				defer close(ch)
				ch <- protocol.UnifiedStreamEvent{
					Type:  protocol.EventStart,
					Model: "mock-model",
					Usage: &protocol.Usage{PromptTokens: 10},
				}
				ch <- protocol.UnifiedStreamEvent{
					Type:    protocol.EventToken,
					Content: "Hello",
				}
				ch <- protocol.UnifiedStreamEvent{
					Type:    protocol.EventToken,
					Content: " world",
				}
				ch <- protocol.UnifiedStreamEvent{
					Type:         protocol.EventStop,
					FinishReason: protocol.FinishReasonStop,
					Usage:        &protocol.Usage{CompletionTokens: 2},
				}
			}()

			return ch, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	parser := translator.NewAnthropicInParser(logger)
	metrics := telemetry.NewMetricsCollector()
	engine := NewProxyEngine(parser, provider, logger, metrics, "mock", "1.0.0")

	// Build a minimal Anthropic request
	body := `{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": "Hello"}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rr := &flushRecorder{httptest.NewRecorder()}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := engine.StreamRequest(ctx, rr, req)
	if err != nil {
		t.Fatalf("StreamRequest failed: %v", err)
	}

	// Verify SSE response
	resp := rr.Result()
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	respBody := string(bodyBytes)

	// Should contain message_start, content_block_delta, message_stop
	if !strings.Contains(respBody, "message_start") {
		t.Error("Response missing message_start event")
	}
	if !strings.Contains(respBody, "Hello") {
		t.Error("Response missing content 'Hello'")
	}
	if !strings.Contains(respBody, "message_stop") {
		t.Error("Response missing message_stop event")
	}

	// Verify metrics were recorded
	snap := metrics.Snapshot()
	if snap.TotalRequests != 1 {
		t.Errorf("TotalRequests = %d, want 1", snap.TotalRequests)
	}
	if snap.SuccessRequests != 1 {
		t.Errorf("SuccessRequests = %d, want 1", snap.SuccessRequests)
	}
}

// TestProxyEngine_StreamRequest_ParseError verifies error handling for invalid JSON.
func TestProxyEngine_StreamRequest_ParseError(t *testing.T) {
	provider := &mockProvider{name: "mock"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	parser := translator.NewAnthropicInParser(logger)
	metrics := telemetry.NewMetricsCollector()
	engine := NewProxyEngine(parser, provider, logger, metrics, "mock", "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("{invalid"))
	rr := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := engine.StreamRequest(ctx, rr, req)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}

	// Verify error was recorded in metrics
	snap := metrics.Snapshot()
	if snap.ErrorRequests != 1 {
		t.Errorf("Expected 1 error request, got %d", snap.ErrorRequests)
	}
}

// TestProxyEngine_StreamRequest_ProviderError verifies error handling when provider fails.
func TestProxyEngine_StreamRequest_ProviderError(t *testing.T) {
	provider := &mockProvider{
		name: "mock",
		streamFunc: func(ctx context.Context, req *protocol.UnifiedChatRequest) (<-chan protocol.UnifiedStreamEvent, error) {
			return nil, fmt.Errorf("provider connection refused")
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	parser := translator.NewAnthropicInParser(logger)
	metrics := telemetry.NewMetricsCollector()
	engine := NewProxyEngine(parser, provider, logger, metrics, "mock", "1.0.0")

	body := `{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": "Hello"}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rr := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := engine.StreamRequest(ctx, rr, req)
	if err == nil {
		t.Error("Expected error for provider failure, got nil")
	}

	// Verify error was recorded
	snap := metrics.Snapshot()
	if snap.ErrorRequests != 1 {
		t.Errorf("Expected 1 error request, got %d", snap.ErrorRequests)
	}
}

// TestProxyEngine_ProviderName_Version verifies metadata methods.
func TestProxyEngine_ProviderName_Version(t *testing.T) {
	provider := &mockProvider{name: "test-provider"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	parser := translator.NewAnthropicInParser(logger)
	metrics := telemetry.NewMetricsCollector()
	engine := NewProxyEngine(parser, provider, logger, metrics, "test-provider", "2.0.0")

	if engine.ProviderName() != "test-provider" {
		t.Errorf("ProviderName() = %s, want test-provider", engine.ProviderName())
	}

	if engine.Version() != "2.0.0" {
		t.Errorf("Version() = %s, want 2.0.0", engine.Version())
	}
}

// TestApp_GetMetricsSnapshot verifies metrics collection works end-to-end.
func TestApp_GetMetricsSnapshot(t *testing.T) {
	cfg := &config.Config{
		Port:           3200,
		Host:           "127.0.0.1",
		Timeout:        300,
		ActiveProvider: "mock",
		Providers: map[string]config.ProviderConfig{
			"mock": {
				Name:    "mock",
				Pattern: "openai",
				BaseURL: "http://localhost:9999",
				APIKey:  "test-key",
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := NewApp(cfg, logger)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	// Record some metrics
	app.Metrics.RecordRequest()
	app.Metrics.RecordSuccess()
	app.Metrics.RecordTTFT(50 * time.Millisecond)
	app.Metrics.RecordDuration(200 * time.Millisecond)

	snap := app.GetMetricsSnapshot()

	if snap.TotalRequests != 1 {
		t.Errorf("TotalRequests = %d, want 1", snap.TotalRequests)
	}
	if snap.SuccessRequests != 1 {
		t.Errorf("SuccessRequests = %d, want 1", snap.SuccessRequests)
	}
	if snap.TTFTCount != 1 {
		t.Errorf("TTFTCount = %d, want 1", snap.TTFTCount)
	}
	if snap.DurationCount != 1 {
		t.Errorf("DurationCount = %d, want 1", snap.DurationCount)
	}
}

// TestProxyEngine_ModelMapping verifies model names are mapped through the provider.
func TestProxyEngine_ModelMapping(t *testing.T) {
	provider := &mockProvider{
		name: "mock",
		modelMap: map[string]string{
			"claude-3-5-sonnet-20241022": "mapped-model",
		},
		streamFunc: func(ctx context.Context, req *protocol.UnifiedChatRequest) (<-chan protocol.UnifiedStreamEvent, error) {
			// Verify the model was mapped
			if req.Model != "mapped-model" {
				return nil, fmt.Errorf("expected model 'mapped-model', got '%s'", req.Model)
			}
			ch := make(chan protocol.UnifiedStreamEvent)
			close(ch)
			return ch, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	parser := translator.NewAnthropicInParser(logger)
	metrics := telemetry.NewMetricsCollector()
	engine := NewProxyEngine(parser, provider, logger, metrics, "mock", "1.0.0")

	body := `{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": "Hello"}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rr := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := engine.StreamRequest(ctx, rr, req)
	if err != nil {
		t.Fatalf("StreamRequest failed: %v", err)
	}
}

// TestApp_Run_Shutdown verifies the lifecycle management works correctly.
func TestApp_Run_Shutdown(t *testing.T) {
	cfg := &config.Config{
		Port:           0, // auto-assign
		Host:           "127.0.0.1",
		Timeout:        5,
		ActiveProvider: "mock",
		Providers: map[string]config.ProviderConfig{
			"mock": {
				Name:    "mock",
				Pattern: "openai",
				BaseURL: "http://localhost:9999",
				APIKey:  "test-key",
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := NewApp(cfg, logger)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	// Start server in background
	go func() {
		if err := app.Run(); err != nil && err != http.ErrServerClosed {
			t.Errorf("Run error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := app.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
}
