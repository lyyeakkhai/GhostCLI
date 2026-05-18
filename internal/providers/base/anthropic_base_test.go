package base

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ghostcli/internal/engine/protocol"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func buildAnthropicSSE(events []string) string {
	var sb strings.Builder
	for _, evt := range events {
		sb.WriteString(evt)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func newTestAnthropicAdapter(serverURL string) *AnthropicAdapter {
	return NewAnthropicAdapter(AnthropicConfig{
		Name:    "test-anthropic",
		BaseURL: serverURL,
		APIKey:  "test-key",
	})
}

// ---------------------------------------------------------------------------
// Provider interface methods
// ---------------------------------------------------------------------------

func TestAnthropicAdapter_Name(t *testing.T) {
	a := newTestAnthropicAdapter("http://localhost")
	if got := a.Name(); got != "test-anthropic" {
		t.Errorf("Name() = %q, want %q", got, "test-anthropic")
	}
}

func TestAnthropicAdapter_SupportsTools(t *testing.T) {
	a := newTestAnthropicAdapter("http://localhost")
	if !a.SupportsTools() {
		t.Error("SupportsTools() = false, want true")
	}
}

func TestAnthropicAdapter_SupportsThinking(t *testing.T) {
	a := newTestAnthropicAdapter("http://localhost")
	if !a.SupportsThinking() {
		t.Error("SupportsThinking() = false, want true")
	}
}

func TestAnthropicAdapter_MapModel_WithMapping(t *testing.T) {
	a := NewAnthropicAdapter(AnthropicConfig{
		Name:   "test",
		APIKey: "k",
		ModelMap: map[string]string{
			"claude-3-5-sonnet": "claude-3-5-sonnet-20241022",
		},
	})
	if got := a.MapModel("claude-3-5-sonnet"); got != "claude-3-5-sonnet-20241022" {
		t.Errorf("MapModel() = %q, want %q", got, "claude-3-5-sonnet-20241022")
	}
}

func TestAnthropicAdapter_MapModel_Passthrough(t *testing.T) {
	a := newTestAnthropicAdapter("http://localhost")
	if got := a.MapModel("claude-3-opus"); got != "claude-3-opus" {
		t.Errorf("MapModel() = %q, want passthrough", got)
	}
}

// ---------------------------------------------------------------------------
// StreamChat via test HTTP server
// ---------------------------------------------------------------------------

func TestAnthropicAdapter_StreamChat_BasicFlow(t *testing.T) {
	sseBody := buildAnthropicSSE([]string{
		`event: message_start` + "\n" + `data: {"type":"message_start","message":{"model":"claude-3-5-sonnet","usage":{"input_tokens":10}}}`,
		`event: content_block_start` + "\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
		`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		`event: message_delta` + "\n" + `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
		`event: message_stop` + "\n" + `data: {"type":"message_stop"}`,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	a := newTestAnthropicAdapter(srv.URL)
	req := &protocol.UnifiedChatRequest{
		Model:     "claude-3-5-sonnet",
		MaxTokens: 100,
		Stream:    true,
		Messages:  []protocol.Message{{Role: "user", Content: "Hi"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := a.StreamChat(ctx, req)
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	var events []protocol.UnifiedStreamEvent
	for e := range ch {
		events = append(events, e)
	}

	if len(events) == 0 {
		t.Fatal("received no events")
	}
	if events[0].Type != protocol.EventStart {
		t.Errorf("first event type = %q, want %q", events[0].Type, protocol.EventStart)
	}

	var tokenFound bool
	for _, e := range events {
		if e.Type == protocol.EventToken && e.Content == "Hello" {
			tokenFound = true
		}
	}
	if !tokenFound {
		t.Error("expected EventToken with content 'Hello'")
	}
}

func TestAnthropicAdapter_StreamChat_HTTP401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"invalid api key","type":"authentication_error"}}`)
	}))
	defer srv.Close()

	a := newTestAnthropicAdapter(srv.URL)
	_, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model:    "claude-3-5-sonnet",
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})

	if err == nil {
		t.Fatal("expected error for HTTP 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention 401, got: %v", err)
	}
}

func TestAnthropicAdapter_StreamChat_ErrorEvent(t *testing.T) {
	sseBody := buildAnthropicSSE([]string{
		`event: error` + "\n" + `data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	a := newTestAnthropicAdapter(srv.URL)
	ch, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model:    "claude-3-5-sonnet",
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected StreamChat error: %v", err)
	}

	var events []protocol.UnifiedStreamEvent
	for e := range ch {
		events = append(events, e)
	}

	var errFound bool
	for _, e := range events {
		if e.Type == protocol.EventError {
			errFound = true
		}
	}
	if !errFound {
		t.Error("expected EventError event from error SSE")
	}
}

func TestAnthropicAdapter_StreamChat_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for i := 0; i < 100; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			fmt.Fprintf(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"tok\"}}\n\n")
			flusher.Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer srv.Close()

	a := newTestAnthropicAdapter(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())

	ch, err := a.StreamChat(ctx, &protocol.UnifiedChatRequest{
		Model:    "claude-3-5-sonnet",
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	count := 0
	for range ch {
		count++
		if count == 2 {
			cancel()
		}
		if count > 50 {
			t.Error("context cancellation did not stop stream")
			cancel()
			break
		}
	}
}

func TestAnthropicAdapter_StreamChat_RequestBodyFormat(t *testing.T) {
	var capturedBody anthropicRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewAnthropicAdapter(AnthropicConfig{
		Name:   "test",
		BaseURL: srv.URL,
		APIKey: "key",
		ModelMap: map[string]string{
			"claude-3-5-sonnet": "claude-3-5-sonnet-20241022",
		},
	})

	a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model:     "claude-3-5-sonnet",
		System:    "Be helpful",
		MaxTokens: 200,
		Messages:  []protocol.Message{{Role: "user", Content: "Hello"}},
	})

	if capturedBody.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("model in body = %q, want %q", capturedBody.Model, "claude-3-5-sonnet-20241022")
	}
	if capturedBody.System != "Be helpful" {
		t.Errorf("system in body = %q, want %q", capturedBody.System, "Be helpful")
	}
	if capturedBody.MaxTokens != 200 {
		t.Errorf("max_tokens = %d, want 200", capturedBody.MaxTokens)
	}
	if !capturedBody.Stream {
		t.Error("stream should be true")
	}
}
