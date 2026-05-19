package deepseek_test

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
	"ghostcli/internal/providers/deepseek"
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

// newAdapterWithServer creates a DeepSeek adapter pointed at the given test
// server so that integration tests avoid real network calls.
func newAdapterWithServer(t *testing.T, serverURL string) *deepseek.Adapter {
	t.Helper()
	return deepseek.NewAdapterWithBaseURL(serverURL+"/v1", "test-api-key", nil, discardLogger())
}

// ---------------------------------------------------------------------------
// Provider identity tests
// ---------------------------------------------------------------------------

func TestAdapter_Name(t *testing.T) {
	a := deepseek.NewAdapter("sk-test", nil, discardLogger())
	if got := a.Name(); got != "deepseek" {
		t.Errorf("Name() = %q, want %q", got, "deepseek")
	}
}

func TestAdapter_SupportsTools(t *testing.T) {
	a := deepseek.NewAdapter("sk-test", nil, discardLogger())
	if !a.SupportsTools() {
		t.Error("SupportsTools() = false, want true")
	}
}

func TestAdapter_SupportsThinking(t *testing.T) {
	a := deepseek.NewAdapter("sk-test", nil, discardLogger())
	if a.SupportsThinking() {
		t.Error("SupportsThinking() = true, want false")
	}
}

// ---------------------------------------------------------------------------
// Model mapping tests (Requirements: 6.3)
// ---------------------------------------------------------------------------

func TestAdapter_MapModel_DefaultMapping(t *testing.T) {
	a := deepseek.NewAdapter("sk-test", nil, discardLogger())

	tests := []struct {
		input string
		want  string
	}{
		{"claude-3-5-sonnet", "deepseek-chat"},
		{"claude-3-5-sonnet-20241022", "deepseek-chat"},
		{"claude-3-5-sonnet-20240620", "deepseek-chat"},
		{"claude-3-5-haiku", "deepseek-chat"},
		{"claude-3-opus-20240229", "deepseek-chat"},
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
	a := deepseek.NewAdapter("sk-test", nil, discardLogger())
	const unknown = "some-future-model"
	if got := a.MapModel(unknown); got != unknown {
		t.Errorf("MapModel(%q) = %q, want original name", unknown, got)
	}
}

func TestAdapter_MapModel_CustomOverride(t *testing.T) {
	custom := map[string]string{
		"claude-3-5-sonnet": "deepseek-reasoner",
	}
	a := deepseek.NewAdapter("sk-test", custom, discardLogger())
	if got := a.MapModel("claude-3-5-sonnet"); got != "deepseek-reasoner" {
		t.Errorf("MapModel() = %q, want %q", got, "deepseek-reasoner")
	}
}

func TestAdapter_MapModel_NilMapUsesDefault(t *testing.T) {
	// Nil model map should fall back to DefaultModelMap, not panic.
	a := deepseek.NewAdapter("sk-test", nil, discardLogger())
	if got := a.MapModel("claude-3-5-sonnet"); got != "deepseek-chat" {
		t.Errorf("MapModel() = %q, want %q", got, "deepseek-chat")
	}
}

// ---------------------------------------------------------------------------
// HTTP integration tests
// ---------------------------------------------------------------------------

func TestStreamChat_SendsBearerToken(t *testing.T) {
	// Requirements: 6.7 — API key goes in Authorization header as Bearer token
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
	// Requirements: 6.4 — requests go to /v1/chat/completions
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
	// Requirements: 6.2, 6.3 — request body uses deepseek-chat model name
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

	if capturedModel != "deepseek-chat" {
		t.Errorf("model in request = %q, want %q", capturedModel, "deepseek-chat")
	}
}

func TestStreamChat_BasicTokenStream(t *testing.T) {
	// Requirements: 6.5 — SSE chunks are parsed into UnifiedStreamEvent objects
	stop := "stop"
	chunks := []string{
		makeChunk("deepseek-chat", "Hello", nil),
		makeChunk("deepseek-chat", " world", &stop),
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
	// Requirements: 6.6 — token usage extracted from final SSE chunk
	stop := "stop"
	usageChunk := `{"id":"c1","object":"chat.completion.chunk","model":"deepseek-chat","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":8,"total_tokens":28}}`
	chunks := []string{
		makeChunk("deepseek-chat", "Hi", &stop),
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
	if usageEvent.Usage.PromptTokens != 20 {
		t.Errorf("prompt_tokens = %d, want 20", usageEvent.Usage.PromptTokens)
	}
	if usageEvent.Usage.CompletionTokens != 8 {
		t.Errorf("completion_tokens = %d, want 8", usageEvent.Usage.CompletionTokens)
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
	if deepseek.DefaultBaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("DefaultBaseURL = %q, want %q",
			deepseek.DefaultBaseURL, "https://api.deepseek.com/v1")
	}
}

func TestProviderName(t *testing.T) {
	if deepseek.ProviderName != "deepseek" {
		t.Errorf("ProviderName = %q, want %q", deepseek.ProviderName, "deepseek")
	}
}
