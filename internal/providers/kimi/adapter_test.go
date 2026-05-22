package kimi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ghostcli/internal/engine/protocol"
	"ghostcli/internal/providers/kimi"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func buildSSEResponse(chunks []string) string {
	var sb strings.Builder
	for _, chunk := range chunks {
		sb.WriteString("data: ")
		sb.WriteString(chunk)
		sb.WriteString("\n\n")
	}
	sb.WriteString("data: [DONE]\n\n")
	return sb.String()
}

func collectEvents(ch <-chan protocol.UnifiedStreamEvent) []protocol.UnifiedStreamEvent {
	var events []protocol.UnifiedStreamEvent
	for e := range ch {
		events = append(events, e)
	}
	return events
}

func makeChunk(model, content string, finishReason *string) string {
	fr := "null"
	if finishReason != nil {
		fr = fmt.Sprintf("%q", *finishReason)
	}
	return fmt.Sprintf(
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":%q,"choices":[{"index":0,"delta":{"content":%q},"finish_reason":%s}]}`,
		model, content, fr,
	)
}

// newAdapterWithServer creates a Kimi adapter pointed at the given test server
// so that tests avoid real network calls.
func newAdapterWithServer(t *testing.T, serverURL string) *kimi.Adapter {
	t.Helper()
	return kimi.NewAdapterWithBaseURL(serverURL+"/v1", "test-api-key", nil, discardLogger())
}

// ---------------------------------------------------------------------------
// Provider identity tests
// ---------------------------------------------------------------------------

func TestAdapter_Name(t *testing.T) {
	a := kimi.NewAdapter("sk-test", nil, discardLogger())
	if got := a.Name(); got != "kimi" {
		t.Errorf("Name() = %q, want %q", got, "kimi")
	}
}

func TestAdapter_SupportsTools(t *testing.T) {
	a := kimi.NewAdapter("sk-test", nil, discardLogger())
	if !a.SupportsTools() {
		t.Error("SupportsTools() = false, want true")
	}
}

func TestAdapter_SupportsThinking(t *testing.T) {
	a := kimi.NewAdapter("sk-test", nil, discardLogger())
	if a.SupportsThinking() {
		t.Error("SupportsThinking() = true, want false")
	}
}

// ---------------------------------------------------------------------------
// Model mapping tests (Requirements: 7.3, 19)
// ---------------------------------------------------------------------------

func TestAdapter_MapModel_DefaultMapping(t *testing.T) {
	a := kimi.NewAdapter("sk-test", nil, discardLogger())

	tests := []struct {
		input string
		want  string
	}{
		{"claude-3-5-sonnet", "moonshot-v1-128k"},
		{"claude-3-5-sonnet-20241022", "moonshot-v1-128k"},
		{"claude-3-5-sonnet-20240620", "moonshot-v1-128k"},
		{"claude-3-opus-20240229", "moonshot-v1-128k"},
		{"claude-3-opus", "moonshot-v1-128k"},
		{"claude-3-haiku-20240307", "moonshot-v1-8k"},
		{"claude-3-haiku", "moonshot-v1-8k"},
		{"claude-3-5-haiku", "moonshot-v1-8k"},
		{"claude-3-5-haiku-20241022", "moonshot-v1-8k"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := a.MapModel(tt.input); got != tt.want {
				t.Errorf("MapModel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAdapter_MapModel_UnknownFallsThrough(t *testing.T) {
	a := kimi.NewAdapter("sk-test", nil, discardLogger())
	const unknown = "some-future-model"
	if got := a.MapModel(unknown); got != unknown {
		t.Errorf("MapModel(%q) = %q, want original name", unknown, got)
	}
}

func TestAdapter_MapModel_CustomOverride(t *testing.T) {
	custom := map[string]string{
		"claude-3-5-sonnet": "moonshot-v1-32k",
	}
	a := kimi.NewAdapter("sk-test", custom, discardLogger())
	if got := a.MapModel("claude-3-5-sonnet"); got != "moonshot-v1-32k" {
		t.Errorf("MapModel() = %q, want %q", got, "moonshot-v1-32k")
	}
}

func TestAdapter_MapModel_NilMapUsesDefault(t *testing.T) {
	// Nil model map should fall back to DefaultModelMap, not panic.
	a := kimi.NewAdapter("sk-test", nil, discardLogger())
	if got := a.MapModel("claude-3-5-sonnet"); got != "moonshot-v1-128k" {
		t.Errorf("MapModel() = %q, want %q", got, "moonshot-v1-128k")
	}
}

// ---------------------------------------------------------------------------
// HTTP integration tests
// ---------------------------------------------------------------------------

func TestStreamChat_SendsBearerToken(t *testing.T) {
	// Requirements: 7.7 — API key goes in Authorization header as Bearer token
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	a := newAdapterWithServer(t, srv.URL)
	ch, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model:    "claude-3-5-sonnet",
		Stream:   true,
		Messages: []protocol.Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}
	for range ch {
	}

	if capturedAuth != "Bearer test-api-key" {
		t.Errorf("Authorization = %q, want %q", capturedAuth, "Bearer test-api-key")
	}
}

func TestStreamChat_SendsToCorrectPath(t *testing.T) {
	// Requirements: 7.4 — requests go to /v1/chat/completions
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	a := newAdapterWithServer(t, srv.URL)
	ch, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model:    "claude-3-5-sonnet",
		Stream:   true,
		Messages: []protocol.Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}
	for range ch {
	}

	if capturedPath != "/v1/chat/completions" {
		t.Errorf("path = %q, want %q", capturedPath, "/v1/chat/completions")
	}
}

func TestStreamChat_MapsModelInRequestBody(t *testing.T) {
	// Requirements: 7.2, 7.3 — request body uses moonshot model name
	var capturedModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			capturedModel = body.Model
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	a := newAdapterWithServer(t, srv.URL)
	ch, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model:    "claude-3-5-sonnet",
		Stream:   true,
		Messages: []protocol.Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}
	for range ch {
	}

	if capturedModel != "moonshot-v1-128k" {
		t.Errorf("model in request = %q, want %q", capturedModel, "moonshot-v1-128k")
	}
}

func TestStreamChat_BasicTokenStream(t *testing.T) {
	// Requirements: 7.5 — SSE chunks are parsed into UnifiedStreamEvent objects
	stop := "stop"
	chunks := []string{
		makeChunk("moonshot-v1-128k", "Hello", nil),
		makeChunk("moonshot-v1-128k", " world", &stop),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buildSSEResponse(chunks))
	}))
	defer srv.Close()

	a := newAdapterWithServer(t, srv.URL)
	ch, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model:    "claude-3-5-sonnet",
		Stream:   true,
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	events := collectEvents(ch)

	var tokenContent string
	for _, e := range events {
		if e.Type == protocol.EventToken {
			tokenContent += e.Content
		}
	}
	if tokenContent != "Hello world" {
		t.Errorf("combined content = %q, want %q", tokenContent, "Hello world")
	}
}

func TestStreamChat_ExtractsTokenUsage(t *testing.T) {
	// Requirements: 7.6 — token usage extracted from final SSE chunk
	stop := "stop"
	usageChunk := `{"id":"c1","object":"chat.completion.chunk","model":"moonshot-v1-128k","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":15,"completion_tokens":6,"total_tokens":21}}`
	chunks := []string{
		makeChunk("moonshot-v1-128k", "Hi", &stop),
		usageChunk,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buildSSEResponse(chunks))
	}))
	defer srv.Close()

	a := newAdapterWithServer(t, srv.URL)
	ch, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model:    "claude-3-5-sonnet",
		Stream:   true,
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	events := collectEvents(ch)

	var usageEvent *protocol.UnifiedStreamEvent
	for i := range events {
		if events[i].Usage != nil {
			usageEvent = &events[i]
		}
	}
	if usageEvent == nil {
		t.Fatal("no event with token usage received")
	}
	if usageEvent.Usage.PromptTokens != 15 {
		t.Errorf("prompt_tokens = %d, want 15", usageEvent.Usage.PromptTokens)
	}
	if usageEvent.Usage.CompletionTokens != 6 {
		t.Errorf("completion_tokens = %d, want 6", usageEvent.Usage.CompletionTokens)
	}
}

func TestStreamChat_HTTPErrorPropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"invalid api key","type":"auth_error","code":"invalid_api_key"}}`)
	}))
	defer srv.Close()

	a := newAdapterWithServer(t, srv.URL)
	_, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model:    "claude-3-5-sonnet",
		Stream:   true,
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for HTTP 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should contain status 401, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Package-level constant tests
// ---------------------------------------------------------------------------

func TestDefaultBaseURL(t *testing.T) {
	if kimi.DefaultBaseURL != "https://api.moonshot.cn/v1" {
		t.Errorf("DefaultBaseURL = %q, want %q",
			kimi.DefaultBaseURL, "https://api.moonshot.cn/v1")
	}
}

func TestProviderName(t *testing.T) {
	if kimi.ProviderName != "kimi" {
		t.Errorf("ProviderName = %q, want %q", kimi.ProviderName, "kimi")
	}
}
