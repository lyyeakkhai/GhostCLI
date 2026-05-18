package translator

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"ghostcli/internal/engine/protocol"
)

// mockResponseWriter wraps httptest.ResponseRecorder to capture SSE events.
type mockResponseWriter struct {
	*httptest.ResponseRecorder
}

func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
	}
}

// parseSSEEvents parses SSE events from the response body.
func parseSSEEvents(body string) []map[string]string {
	var events []map[string]string
	scanner := bufio.NewScanner(strings.NewReader(body))

	var currentEvent map[string]string
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Empty line marks end of event
			if currentEvent != nil {
				events = append(events, currentEvent)
				currentEvent = nil
			}
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			if currentEvent == nil {
				currentEvent = make(map[string]string)
			}
			currentEvent["event"] = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			if currentEvent == nil {
				currentEvent = make(map[string]string)
			}
			currentEvent["data"] = strings.TrimPrefix(line, "data: ")
		}
	}

	// Add last event if exists
	if currentEvent != nil {
		events = append(events, currentEvent)
	}

	return events
}

func TestAnthropicOutFormatter_StreamToWriter_BasicTextStream(t *testing.T) {
	logger := slog.Default()
	formatter := NewAnthropicOutFormatter(logger)

	w := newMockResponseWriter()
	ctx := context.Background()

	// Create event channel
	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	// Send events in a goroutine
	go func() {
		defer close(eventChan)

		// Start event
		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
			Usage: &protocol.Usage{
				PromptTokens:     100,
				CompletionTokens: 0,
			},
		}

		// Content tokens
		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "Hello",
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: " world",
		}

		// Stop event
		eventChan <- protocol.UnifiedStreamEvent{
			Type:         protocol.EventStop,
			FinishReason: protocol.FinishReasonStop,
			Usage: &protocol.Usage{
				PromptTokens:     100,
				CompletionTokens: 10,
			},
		}
	}()

	// Stream to writer
	err := formatter.StreamToWriter(ctx, w, eventChan)
	if err != nil {
		t.Fatalf("StreamToWriter failed: %v", err)
	}

	// Parse SSE events
	body := w.Body.String()
	events := parseSSEEvents(body)

	// Verify event sequence
	if len(events) < 4 {
		t.Fatalf("expected at least 4 events, got %d", len(events))
	}

	// Check message_start
	if events[0]["event"] != "message_start" {
		t.Errorf("expected first event to be message_start, got %s", events[0]["event"])
	}

	// Check content_block_start
	if events[1]["event"] != "content_block_start" {
		t.Errorf("expected second event to be content_block_start, got %s", events[1]["event"])
	}

	// Check content_block_delta events
	foundHello := false
	foundWorld := false
	for _, event := range events {
		if event["event"] == "content_block_delta" {
			if strings.Contains(event["data"], "Hello") {
				foundHello = true
			}
			if strings.Contains(event["data"], "world") {
				foundWorld = true
			}
		}
	}

	if !foundHello {
		t.Error("expected to find 'Hello' in content_block_delta events")
	}
	if !foundWorld {
		t.Error("expected to find 'world' in content_block_delta events")
	}

	// Check message_stop
	lastEvent := events[len(events)-1]
	if lastEvent["event"] != "message_stop" {
		t.Errorf("expected last event to be message_stop, got %s", lastEvent["event"])
	}
}

func TestAnthropicOutFormatter_StreamToWriter_WithThinking(t *testing.T) {
	logger := slog.Default()
	formatter := NewAnthropicOutFormatter(logger)

	w := newMockResponseWriter()
	ctx := context.Background()

	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:     protocol.EventThinking,
			Thinking: "Let me think about this...",
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "The answer is 42",
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:         protocol.EventStop,
			FinishReason: protocol.FinishReasonStop,
		}
	}()

	err := formatter.StreamToWriter(ctx, w, eventChan)
	if err != nil {
		t.Fatalf("StreamToWriter failed: %v", err)
	}

	body := w.Body.String()
	events := parseSSEEvents(body)

	// Helper to decode an SSE event's data payload into a generic map.
	decodePayload := func(e map[string]string) map[string]interface{} {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(e["data"]), &m); err != nil {
			t.Fatalf("failed to decode event payload: %v (data=%q)", err, e["data"])
		}
		return m
	}

	// Helper to read an index field from a decoded payload.
	getIndex := func(m map[string]interface{}) int {
		v, ok := m["index"]
		if !ok {
			t.Fatal("missing 'index' field in event")
		}
		f, ok := v.(float64)
		if !ok {
			t.Fatalf("'index' is not a number: %T", v)
		}
		return int(f)
	}

	// Expected ordered sequence of SSE event types.
	// Sequence: message_start, content_block_start (thinking@0),
	//           content_block_delta (thinking@0),
	//           content_block_start (text@1), content_block_delta (text@1),
	//           content_block_stop@0, content_block_stop@1,
	//           message_delta, message_stop
	expectedTypes := []string{
		"message_start",
		"content_block_start",  // thinking block at index 0
		"content_block_delta",  // thinking delta at index 0
		"content_block_start",  // text block at index 1
		"content_block_delta",  // text delta at index 1
		"content_block_stop",   // close index 0
		"content_block_stop",   // close index 1
		"message_delta",
		"message_stop",
	}

	if len(events) != len(expectedTypes) {
		t.Fatalf("expected %d events, got %d: %v", len(expectedTypes), len(events), events)
	}

	for i, expType := range expectedTypes {
		if events[i]["event"] != expType {
			t.Errorf("event[%d]: expected type %q, got %q", i, expType, events[i]["event"])
		}
	}

	// content_block_start thinking → index 0 with type "thinking"
	thinkingStart := decodePayload(events[1])
	if idx := getIndex(thinkingStart); idx != 0 {
		t.Errorf("thinking content_block_start: expected index 0, got %d", idx)
	}
	if cb, ok := thinkingStart["content_block"].(map[string]interface{}); ok {
		if cb["type"] != "thinking" {
			t.Errorf("thinking content_block type: expected 'thinking', got %q", cb["type"])
		}
	} else {
		t.Error("thinking content_block_start missing 'content_block' field")
	}

	// content_block_delta thinking → index 0
	thinkingDelta := decodePayload(events[2])
	if idx := getIndex(thinkingDelta); idx != 0 {
		t.Errorf("thinking content_block_delta: expected index 0, got %d", idx)
	}
	if d, ok := thinkingDelta["delta"].(map[string]interface{}); ok {
		if d["type"] != "thinking_delta" {
			t.Errorf("thinking delta type: expected 'thinking_delta', got %q", d["type"])
		}
	}

	// content_block_start text → index 1 with type "text"
	textStart := decodePayload(events[3])
	if idx := getIndex(textStart); idx != 1 {
		t.Errorf("text content_block_start: expected index 1, got %d", idx)
	}
	if cb, ok := textStart["content_block"].(map[string]interface{}); ok {
		if cb["type"] != "text" {
			t.Errorf("text content_block type: expected 'text', got %q", cb["type"])
		}
	} else {
		t.Error("text content_block_start missing 'content_block' field")
	}

	// content_block_delta text → index 1
	textDelta := decodePayload(events[4])
	if idx := getIndex(textDelta); idx != 1 {
		t.Errorf("text content_block_delta: expected index 1, got %d", idx)
	}
	if !strings.Contains(events[4]["data"], "42") {
		t.Error("text delta should contain '42'")
	}

	// content_block_stop events → indices 0 then 1
	for i, expectedIdx := range []int{0, 1} {
		stopPayload := decodePayload(events[5+i])
		if idx := getIndex(stopPayload); idx != expectedIdx {
			t.Errorf("content_block_stop[%d]: expected index %d, got %d", i, expectedIdx, idx)
		}
	}
}

func TestAnthropicOutFormatter_StreamToWriter_WithToolCalls(t *testing.T) {
	logger := slog.Default()
	formatter := NewAnthropicOutFormatter(logger)

	w := newMockResponseWriter()
	ctx := context.Background()

	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventToolCall,
			ToolCalls: []protocol.ToolCall{
				{
					ID:   "toolu_123",
					Name: "get_weather",
					Args: `{"location":"San Francisco"}`,
				},
			},
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:         protocol.EventStop,
			FinishReason: protocol.FinishReasonToolCalls,
		}
	}()

	err := formatter.StreamToWriter(ctx, w, eventChan)
	if err != nil {
		t.Fatalf("StreamToWriter failed: %v", err)
	}

	body := w.Body.String()
	events := parseSSEEvents(body)

	// Verify tool_use content is present
	foundToolUse := false
	foundToolName := false
	for _, event := range events {
		if strings.Contains(event["data"], "tool_use") {
			foundToolUse = true
		}
		if strings.Contains(event["data"], "get_weather") {
			foundToolName = true
		}
	}

	if !foundToolUse {
		t.Error("expected to find tool_use in events")
	}
	if !foundToolName {
		t.Error("expected to find tool name 'get_weather' in events")
	}
}

func TestAnthropicOutFormatter_StreamToWriter_WithError(t *testing.T) {
	logger := slog.Default()
	formatter := NewAnthropicOutFormatter(logger)

	w := newMockResponseWriter()
	ctx := context.Background()

	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:  protocol.EventError,
			Error: errors.New("API rate limit exceeded"),
		}
	}()

	err := formatter.StreamToWriter(ctx, w, eventChan)
	if err != nil {
		t.Fatalf("StreamToWriter failed: %v", err)
	}

	body := w.Body.String()
	events := parseSSEEvents(body)

	// Verify error event is present
	foundError := false
	for _, event := range events {
		if event["event"] == "error" {
			foundError = true
			if !strings.Contains(event["data"], "rate limit") {
				t.Error("expected error message to contain 'rate limit'")
			}
		}
	}

	if !foundError {
		t.Error("expected to find error event")
	}
}

func TestAnthropicOutFormatter_StreamToWriter_ContextCancellation(t *testing.T) {
	logger := slog.Default()
	formatter := NewAnthropicOutFormatter(logger)

	w := newMockResponseWriter()
	ctx, cancel := context.WithCancel(context.Background())

	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
		}

		// Cancel context before sending more events
		cancel()

		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "This should not be processed",
		}
	}()

	err := formatter.StreamToWriter(ctx, w, eventChan)
	if err == nil {
		t.Error("expected error due to context cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestAnthropicOutFormatter_TokenUsageTracking(t *testing.T) {
	logger := slog.Default()
	formatter := NewAnthropicOutFormatter(logger)

	w := newMockResponseWriter()
	ctx := context.Background()

	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		// Start with initial usage
		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
			Usage: &protocol.Usage{
				PromptTokens:     50,
				CompletionTokens: 0,
			},
		}

		// Content with updated usage
		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "Hello",
			Usage: &protocol.Usage{
				PromptTokens:     50,
				CompletionTokens: 5,
			},
		}

		// Stop with final usage
		eventChan <- protocol.UnifiedStreamEvent{
			Type:         protocol.EventStop,
			FinishReason: protocol.FinishReasonStop,
			Usage: &protocol.Usage{
				PromptTokens:     50,
				CompletionTokens: 10,
			},
		}
	}()

	err := formatter.StreamToWriter(ctx, w, eventChan)
	if err != nil {
		t.Fatalf("StreamToWriter failed: %v", err)
	}

	// Verify token counts are tracked
	if formatter.inputTokens != 50 {
		t.Errorf("expected inputTokens to be 50, got %d", formatter.inputTokens)
	}
	if formatter.outputTokens != 10 {
		t.Errorf("expected outputTokens to be 10, got %d", formatter.outputTokens)
	}

	// Verify usage is in the response
	body := w.Body.String()
	if !strings.Contains(body, "input_tokens") {
		t.Error("expected response to contain input_tokens")
	}
	if !strings.Contains(body, "output_tokens") {
		t.Error("expected response to contain output_tokens")
	}
}

func TestAnthropicOutFormatter_EmptyContentIgnored(t *testing.T) {
	logger := slog.Default()
	formatter := NewAnthropicOutFormatter(logger)

	w := newMockResponseWriter()
	ctx := context.Background()

	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
		}

		// Empty content should be ignored
		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "",
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "Real content",
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:         protocol.EventStop,
			FinishReason: protocol.FinishReasonStop,
		}
	}()

	err := formatter.StreamToWriter(ctx, w, eventChan)
	if err != nil {
		t.Fatalf("StreamToWriter failed: %v", err)
	}

	body := w.Body.String()
	events := parseSSEEvents(body)

	// Count content_block_delta events (should only be 1 for "Real content")
	deltaCount := 0
	for _, event := range events {
		if event["event"] == "content_block_delta" {
			deltaCount++
		}
	}

	if deltaCount != 1 {
		t.Errorf("expected 1 content_block_delta event, got %d", deltaCount)
	}
}

func TestAnthropicOutFormatter_FinishReasonMapping(t *testing.T) {
	tests := []struct {
		name           string
		finishReason   string
		expectedReason string
	}{
		{
			name:           "stop maps to end_turn",
			finishReason:   protocol.FinishReasonStop,
			expectedReason: "end_turn",
		},
		{
			name:           "length maps to max_tokens",
			finishReason:   protocol.FinishReasonLength,
			expectedReason: "max_tokens",
		},
		{
			name:           "tool_calls maps to tool_use",
			finishReason:   protocol.FinishReasonToolCalls,
			expectedReason: "tool_use",
		},
		{
			name:           "error maps to error",
			finishReason:   protocol.FinishReasonError,
			expectedReason: "error",
		},
		{
			name:           "unknown maps to end_turn",
			finishReason:   "unknown",
			expectedReason: "end_turn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			formatter := NewAnthropicOutFormatter(logger)

			mapped := formatter.mapFinishReason(tt.finishReason)
			if mapped != tt.expectedReason {
				t.Errorf("expected %s, got %s", tt.expectedReason, mapped)
			}
		})
	}
}
