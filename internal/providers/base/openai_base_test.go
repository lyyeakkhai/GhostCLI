package base

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
	"time"

	"ghostcli/internal/engine/protocol"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestLogger returns a discarding slog.Logger for use in tests.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestAdapter creates an OpenAIAdapter pointed at the given test server URL.
func newTestAdapter(serverURL string, modelMap map[string]string) *OpenAIAdapter {
	cfg := OpenAIConfig{
		Name:     "test-provider",
		BaseURL:  serverURL,
		APIKey:   "test-api-key",
		ModelMap: modelMap,
	}
	return NewOpenAIAdapter(cfg, newTestLogger())
}

// buildSSEResponse builds a minimal SSE response body from a slice of JSON
// strings, terminating with "data: [DONE]\n\n".
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

// collectEvents drains an event channel and returns all received events.
func collectEvents(ch <-chan protocol.UnifiedStreamEvent) []protocol.UnifiedStreamEvent {
	var events []protocol.UnifiedStreamEvent
	for e := range ch {
		events = append(events, e)
	}
	return events
}

// ---------------------------------------------------------------------------
// Unit tests — Name / SupportsTools / SupportsThinking / MapModel
// ---------------------------------------------------------------------------

func TestOpenAIAdapter_Name(t *testing.T) {
	a := newTestAdapter("http://localhost", nil)
	if got := a.Name(); got != "test-provider" {
		t.Errorf("Name() = %q, want %q", got, "test-provider")
	}
}

func TestOpenAIAdapter_SupportsTools(t *testing.T) {
	a := newTestAdapter("http://localhost", nil)
	if !a.SupportsTools() {
		t.Error("SupportsTools() = false, want true")
	}
}

func TestOpenAIAdapter_SupportsThinking(t *testing.T) {
	a := newTestAdapter("http://localhost", nil)
	if a.SupportsThinking() {
		t.Error("SupportsThinking() = true, want false")
	}
}

func TestOpenAIAdapter_MapModel_WithMapping(t *testing.T) {
	a := newTestAdapter("http://localhost", map[string]string{
		"claude-3-5-sonnet": "gpt-4o",
	})
	if got := a.MapModel("claude-3-5-sonnet"); got != "gpt-4o" {
		t.Errorf("MapModel() = %q, want %q", got, "gpt-4o")
	}
}

func TestOpenAIAdapter_MapModel_FallsBackToOriginal(t *testing.T) {
	a := newTestAdapter("http://localhost", map[string]string{
		"claude-3-5-sonnet": "gpt-4o",
	})
	// A model with no mapping should be returned unchanged.
	if got := a.MapModel("unknown-model"); got != "unknown-model" {
		t.Errorf("MapModel() = %q, want %q", got, "unknown-model")
	}
}

func TestOpenAIAdapter_MapModel_NilMap(t *testing.T) {
	a := newTestAdapter("http://localhost", nil)
	if got := a.MapModel("claude-3-5-sonnet"); got != "claude-3-5-sonnet" {
		t.Errorf("MapModel() = %q, want %q", got, "claude-3-5-sonnet")
	}
}

// ---------------------------------------------------------------------------
// Unit tests — convertToOpenAIFormat
// ---------------------------------------------------------------------------

func TestConvertToOpenAIFormat_BasicRequest(t *testing.T) {
	a := newTestAdapter("http://localhost", map[string]string{
		"claude-3-5-sonnet": "gpt-4o",
	})

	req := &protocol.UnifiedChatRequest{
		Model:     "claude-3-5-sonnet",
		MaxTokens: 512,
		Stream:    true,
		Messages: []protocol.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	got := a.convertToOpenAIFormat(req)

	if got.Model != "gpt-4o" {
		t.Errorf("model = %q, want %q", got.Model, "gpt-4o")
	}
	if !got.Stream {
		t.Error("stream = false, want true")
	}
	if got.MaxTokens != 512 {
		t.Errorf("max_tokens = %d, want 512", got.MaxTokens)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("messages count = %d, want 1", len(got.Messages))
	}
	if got.Messages[0].Role != "user" || got.Messages[0].Content != "Hello" {
		t.Errorf("message = %+v, want {user Hello}", got.Messages[0])
	}
	// StreamOptions should be set when streaming.
	if got.StreamOptions == nil || !got.StreamOptions.IncludeUsage {
		t.Error("stream_options.include_usage should be true for streaming requests")
	}
}

func TestConvertToOpenAIFormat_WithSystemPrompt(t *testing.T) {
	a := newTestAdapter("http://localhost", nil)

	req := &protocol.UnifiedChatRequest{
		Model:  "gpt-4",
		System: "You are a helpful assistant.",
		Messages: []protocol.Message{
			{Role: "user", Content: "Hi"},
		},
		Stream: true,
	}

	got := a.convertToOpenAIFormat(req)

	// System prompt should be prepended as a system-role message.
	if len(got.Messages) != 2 {
		t.Fatalf("messages count = %d, want 2", len(got.Messages))
	}
	if got.Messages[0].Role != "system" {
		t.Errorf("first message role = %q, want %q", got.Messages[0].Role, "system")
	}
	if got.Messages[0].Content != "You are a helpful assistant." {
		t.Errorf("system content = %q", got.Messages[0].Content)
	}
}

func TestConvertToOpenAIFormat_WithTools(t *testing.T) {
	a := newTestAdapter("http://localhost", nil)

	req := &protocol.UnifiedChatRequest{
		Model: "gpt-4",
		Messages: []protocol.Message{
			{Role: "user", Content: "Call a tool"},
		},
		Tools: []protocol.Tool{
			{
				Name:        "get_weather",
				Description: "Get the weather",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
		Stream: false,
	}

	got := a.convertToOpenAIFormat(req)

	if len(got.Tools) != 1 {
		t.Fatalf("tools count = %d, want 1", len(got.Tools))
	}
	if got.Tools[0].Type != "function" {
		t.Errorf("tool type = %q, want %q", got.Tools[0].Type, "function")
	}
	if got.Tools[0].Function.Name != "get_weather" {
		t.Errorf("tool name = %q", got.Tools[0].Function.Name)
	}
	// StreamOptions should NOT be set for non-streaming requests.
	if got.StreamOptions != nil {
		t.Error("stream_options should be nil for non-streaming requests")
	}
}

func TestConvertToOpenAIFormat_NoSystemPrompt(t *testing.T) {
	a := newTestAdapter("http://localhost", nil)

	req := &protocol.UnifiedChatRequest{
		Model: "gpt-4",
		Messages: []protocol.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	got := a.convertToOpenAIFormat(req)

	// No extra system message should be prepended.
	if len(got.Messages) != 1 {
		t.Errorf("messages count = %d, want 1 (no system message)", len(got.Messages))
	}
}

// ---------------------------------------------------------------------------
// Unit tests — mapFinishReason
// ---------------------------------------------------------------------------

func TestMapFinishReason(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"stop", protocol.FinishReasonStop},
		{"length", protocol.FinishReasonLength},
		{"tool_calls", protocol.FinishReasonToolCalls},
		{"content_filter", protocol.FinishReasonStop}, // unknown → stop
		{"", protocol.FinishReasonStop},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := mapFinishReason(tt.input); got != tt.want {
				t.Errorf("mapFinishReason(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Integration-style tests — StreamChat against a fake HTTP server
// ---------------------------------------------------------------------------

// makeChunk builds a minimal openAIStreamChunk JSON string with one content delta.
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

func TestStreamChat_BasicTokenStream(t *testing.T) {
	stop := "stop"
	chunks := []string{
		makeChunk("gpt-4o", "Hello", nil),
		makeChunk("gpt-4o", ", world", nil),
		makeChunk("gpt-4o", "!", &stop),
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path and auth header.
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("missing/incorrect auth header: %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buildSSEResponse(chunks))
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL+"/v1", nil)

	ctx := context.Background()
	req := &protocol.UnifiedChatRequest{
		Model:     "gpt-4",
		MaxTokens: 100,
		Stream:    true,
		Messages:  []protocol.Message{{Role: "user", Content: "Hi"}},
	}

	ch, err := a.StreamChat(ctx, req)
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	events := collectEvents(ch)

	// First event should be EventStart.
	if len(events) == 0 {
		t.Fatal("received no events")
	}
	if events[0].Type != protocol.EventStart {
		t.Errorf("first event type = %q, want %q", events[0].Type, protocol.EventStart)
	}
	if events[0].Model != "gpt-4o" {
		t.Errorf("start event model = %q, want %q", events[0].Model, "gpt-4o")
	}

	// Collect token events.
	var tokens []string
	for _, e := range events {
		if e.Type == protocol.EventToken && e.Content != "" {
			tokens = append(tokens, e.Content)
		}
	}
	combined := strings.Join(tokens, "")
	if combined != "Hello, world!" {
		t.Errorf("combined content = %q, want %q", combined, "Hello, world!")
	}

	// Last meaningful event before channel close should be EventStop.
	var lastStop *protocol.UnifiedStreamEvent
	for i := range events {
		if events[i].Type == protocol.EventStop {
			lastStop = &events[i]
		}
	}
	if lastStop == nil {
		t.Error("no EventStop received")
	} else if lastStop.FinishReason != protocol.FinishReasonStop {
		t.Errorf("finish reason = %q, want %q", lastStop.FinishReason, protocol.FinishReasonStop)
	}
}

func TestStreamChat_WithTokenUsage(t *testing.T) {
	stop := "stop"
	// Final chunk carries usage.
	usageChunk := `{"id":"c1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`
	chunks := []string{
		makeChunk("gpt-4o", "Hi", &stop),
		usageChunk,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buildSSEResponse(chunks))
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL+"/v1", nil)

	ch, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model: "gpt-4", Stream: true,
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	events := collectEvents(ch)

	// Find a usage event.
	var usageEvent *protocol.UnifiedStreamEvent
	for i := range events {
		if events[i].Usage != nil {
			usageEvent = &events[i]
		}
	}
	if usageEvent == nil {
		t.Fatal("no event with usage received")
	}
	if usageEvent.Usage.PromptTokens != 10 {
		t.Errorf("prompt_tokens = %d, want 10", usageEvent.Usage.PromptTokens)
	}
	if usageEvent.Usage.CompletionTokens != 5 {
		t.Errorf("completion_tokens = %d, want 5", usageEvent.Usage.CompletionTokens)
	}
}

func TestStreamChat_HTTPErrorReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"invalid api key","type":"auth_error","code":"invalid_api_key"}}`)
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL+"/v1", nil)

	_, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model: "gpt-4", Stream: true,
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})

	if err == nil {
		t.Fatal("expected error for HTTP 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention status code 401, got: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("error should contain provider message, got: %v", err)
	}
}

func TestStreamChat_ContextCancellation(t *testing.T) {
	// Server that streams slowly so we can cancel mid-stream.
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
			fmt.Fprintf(w, "data: %s\n\n", makeChunk("gpt-4o", "tok", nil))
			flusher.Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL+"/v1", nil)

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := a.StreamChat(ctx, &protocol.UnifiedChatRequest{
		Model: "gpt-4", Stream: true,
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Read a couple of events then cancel.
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

func TestStreamChat_SendsAuthHeader(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL+"/v1", nil)
	ch, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model: "gpt-4", Stream: true,
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}
	// Drain channel.
	for range ch {
	}

	if capturedAuth != "Bearer test-api-key" {
		t.Errorf("Authorization header = %q, want %q", capturedAuth, "Bearer test-api-key")
	}
}

func TestStreamChat_CustomAuthHeader(t *testing.T) {
	var capturedKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	cfg := OpenAIConfig{
		Name:       "custom-provider",
		BaseURL:    srv.URL + "/v1",
		APIKey:     "my-secret-key",
		AuthHeader: "X-API-Key",
		AuthPrefix: "", // No prefix
	}
	a := NewOpenAIAdapter(cfg, newTestLogger())
	ch, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model: "some-model", Stream: true,
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}
	for range ch {
	}

	if capturedKey != "my-secret-key" {
		t.Errorf("X-API-Key = %q, want %q", capturedKey, "my-secret-key")
	}
}

func TestStreamChat_RequestBodyFormat(t *testing.T) {
	var capturedBody openAIRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL+"/v1", map[string]string{
		"claude-3-5-sonnet": "gpt-4o",
	})

	req := &protocol.UnifiedChatRequest{
		Model:       "claude-3-5-sonnet",
		System:      "Be helpful",
		MaxTokens:   256,
		Temperature: 0.7,
		Stream:      true,
		Messages: []protocol.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ch, err := a.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}
	for range ch {
	}

	if capturedBody.Model != "gpt-4o" {
		t.Errorf("model in body = %q, want %q", capturedBody.Model, "gpt-4o")
	}
	if capturedBody.MaxTokens != 256 {
		t.Errorf("max_tokens in body = %d, want 256", capturedBody.MaxTokens)
	}
	// System message should be the first message.
	if len(capturedBody.Messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(capturedBody.Messages))
	}
	if capturedBody.Messages[0].Role != "system" || capturedBody.Messages[0].Content != "Be helpful" {
		t.Errorf("system message = %+v", capturedBody.Messages[0])
	}
	if capturedBody.Messages[1].Role != "user" {
		t.Errorf("user message role = %q", capturedBody.Messages[1].Role)
	}
}

func TestStreamChat_EmptyStream(t *testing.T) {
	// Server responds with only [DONE] — zero content.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL+"/v1", nil)
	ch, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model: "gpt-4", Stream: true,
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	events := collectEvents(ch)
	// Should receive no events (no start event either, because no chunk was received).
	if len(events) != 0 {
		t.Errorf("expected 0 events for empty stream, got %d", len(events))
	}
}

func TestStreamChat_MalformedChunkIgnored(t *testing.T) {
	stop := "stop"
	chunks := []string{
		`this is not json`,
		makeChunk("gpt-4o", "valid token", &stop),
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buildSSEResponse(chunks))
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL+"/v1", nil)
	ch, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model: "gpt-4", Stream: true,
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	events := collectEvents(ch)

	// The malformed chunk should be skipped; the valid token should arrive.
	var tokenFound bool
	for _, e := range events {
		if e.Type == protocol.EventToken && e.Content == "valid token" {
			tokenFound = true
		}
	}
	if !tokenFound {
		t.Error("valid token event was not received after malformed chunk")
	}
}

func TestStreamChat_ToolCallStream(t *testing.T) {
	// Two chunks: first starts the tool call, second adds arguments.
	chunk1 := `{"id":"c1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}`
	chunk2 := `{"id":"c2","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\":\"NYC\"}"}}]},"finish_reason":"tool_calls"}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buildSSEResponse([]string{chunk1, chunk2}))
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL+"/v1", nil)
	ch, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model: "gpt-4", Stream: true,
		Messages: []protocol.Message{{Role: "user", Content: "What is the weather?"}},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	events := collectEvents(ch)

	// Should have an EventToolCall event with the assembled call.
	var toolEvent *protocol.UnifiedStreamEvent
	for i := range events {
		if events[i].Type == protocol.EventToolCall {
			toolEvent = &events[i]
		}
	}
	if toolEvent == nil {
		t.Fatal("no EventToolCall received")
	}
	if len(toolEvent.ToolCalls) == 0 {
		t.Fatal("EventToolCall has empty ToolCalls slice")
	}
	tc := toolEvent.ToolCalls[0]
	if tc.Name != "get_weather" {
		t.Errorf("tool name = %q, want %q", tc.Name, "get_weather")
	}
	if tc.ID != "call_abc" {
		t.Errorf("tool id = %q, want %q", tc.ID, "call_abc")
	}
	if !strings.Contains(tc.Args, "NYC") {
		t.Errorf("tool args = %q, want to contain %q", tc.Args, "NYC")
	}

	// Should also have an EventStop with tool_calls finish reason.
	var stopEvent *protocol.UnifiedStreamEvent
	for i := range events {
		if events[i].Type == protocol.EventStop {
			stopEvent = &events[i]
		}
	}
	if stopEvent == nil {
		t.Fatal("no EventStop received")
	}
	if stopEvent.FinishReason != protocol.FinishReasonToolCalls {
		t.Errorf("finish reason = %q, want %q", stopEvent.FinishReason, protocol.FinishReasonToolCalls)
	}
}

// ---------------------------------------------------------------------------
// Edge-case tests
// ---------------------------------------------------------------------------

func TestStreamChat_CommentLinesIgnored(t *testing.T) {
	stop := "stop"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Insert SSE comment lines that must be ignored.
		fmt.Fprintf(w, ": keep-alive\n\n")
		fmt.Fprintf(w, "data: %s\n\n", makeChunk("gpt-4o", "hi", &stop))
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL+"/v1", nil)
	ch, err := a.StreamChat(context.Background(), &protocol.UnifiedChatRequest{
		Model: "gpt-4", Stream: true,
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	events := collectEvents(ch)

	var tokenFound bool
	for _, e := range events {
		if e.Type == protocol.EventToken && e.Content == "hi" {
			tokenFound = true
		}
	}
	if !tokenFound {
		t.Error("token event not received when SSE comments are present")
	}
}
