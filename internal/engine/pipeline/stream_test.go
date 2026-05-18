package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ghostcli/internal/engine/protocol"
)

func TestNewStreamOrchestrator(t *testing.T) {
	logger := slog.Default()
	orchestrator := NewStreamOrchestrator(logger)

	if orchestrator == nil {
		t.Fatal("NewStreamOrchestrator returned nil")
	}
	if orchestrator.logger == nil {
		t.Error("orchestrator logger is nil")
	}
	if orchestrator.formatter == nil {
		t.Error("orchestrator formatter is nil")
	}
	if orchestrator.normalizer == nil {
		t.Error("orchestrator normalizer is nil")
	}
}

func TestDefaultStreamOptions(t *testing.T) {
	opts := DefaultStreamOptions()

	if !opts.EnableUsageNormalization {
		t.Error("expected EnableUsageNormalization to be true by default")
	}
	if !opts.EstimateTokens {
		t.Error("expected EstimateTokens to be true by default")
	}
}

func TestStreamOrchestrator_StreamWithDefaults(t *testing.T) {
	logger := slog.Default()
	orchestrator := NewStreamOrchestrator(logger)

	w := httptest.NewRecorder()
	ctx := context.Background()

	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
			Usage: &protocol.Usage{
				PromptTokens:     100,
				CompletionTokens: 0,
			},
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "Hello world",
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:         protocol.EventStop,
			FinishReason: protocol.FinishReasonStop,
			Usage: &protocol.Usage{
				PromptTokens:     100,
				CompletionTokens: 10,
			},
		}
	}()

	err := orchestrator.StreamWithDefaults(ctx, w, eventChan)
	if err != nil {
		t.Fatalf("StreamWithDefaults failed: %v", err)
	}

	// Verify SSE headers were set
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", contentType)
	}

	// Verify response contains events
	body := w.Body.String()
	if !strings.Contains(body, "message_start") {
		t.Error("expected response to contain message_start event")
	}
	if !strings.Contains(body, "Hello world") {
		t.Error("expected response to contain 'Hello world' content")
	}
}

func TestStreamOrchestrator_Stream_WithUsageNormalization(t *testing.T) {
	logger := slog.Default()
	orchestrator := NewStreamOrchestrator(logger)

	w := httptest.NewRecorder()
	ctx := context.Background()

	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		// Start event with usage
		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
			Usage: &protocol.Usage{
				PromptTokens:     50,
				CompletionTokens: 0,
			},
		}

		// Content event WITHOUT usage (should be normalized)
		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "Test content",
		}

		// Stop event with final usage
		eventChan <- protocol.UnifiedStreamEvent{
			Type:         protocol.EventStop,
			FinishReason: protocol.FinishReasonStop,
			Usage: &protocol.Usage{
				PromptTokens:     50,
				CompletionTokens: 5,
			},
		}
	}()

	opts := StreamOptions{
		EnableUsageNormalization: true,
		EstimateTokens:           true,
	}

	err := orchestrator.Stream(ctx, w, eventChan, opts)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	body := w.Body.String()
	
	// Verify usage information is present
	if !strings.Contains(body, "input_tokens") {
		t.Error("expected response to contain input_tokens")
	}
	if !strings.Contains(body, "output_tokens") {
		t.Error("expected response to contain output_tokens")
	}
}

func TestStreamOrchestrator_Stream_WithoutUsageNormalization(t *testing.T) {
	logger := slog.Default()
	orchestrator := NewStreamOrchestrator(logger)

	w := httptest.NewRecorder()
	ctx := context.Background()

	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
			Usage: &protocol.Usage{
				PromptTokens:     50,
				CompletionTokens: 0,
			},
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "Test",
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:         protocol.EventStop,
			FinishReason: protocol.FinishReasonStop,
		}
	}()

	opts := StreamOptions{
		EnableUsageNormalization: false,
		EstimateTokens:           false,
	}

	err := orchestrator.Stream(ctx, w, eventChan, opts)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Should still complete successfully even without normalization
	body := w.Body.String()
	if !strings.Contains(body, "message_start") {
		t.Error("expected response to contain message_start event")
	}
}

func TestStreamOrchestrator_Stream_ContextCancellation(t *testing.T) {
	logger := slog.Default()
	orchestrator := NewStreamOrchestrator(logger)

	w := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())

	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
		}

		// Cancel context
		cancel()

		// Try to send more events (should be ignored)
		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "Should not be processed",
		}
	}()

	err := orchestrator.StreamWithDefaults(ctx, w, eventChan)
	
	// Should return context.Canceled error
	if err == nil {
		t.Error("expected error due to context cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestStreamOrchestrator_Stream_ContextDeadline(t *testing.T) {
	logger := slog.Default()
	orchestrator := NewStreamOrchestrator(logger)

	w := httptest.NewRecorder()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
		}

		// Wait for context to timeout
		time.Sleep(50 * time.Millisecond)

		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "Too late",
		}
	}()

	err := orchestrator.StreamWithDefaults(ctx, w, eventChan)
	
	// Should return context.DeadlineExceeded error
	if err == nil {
		t.Error("expected error due to context deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded error, got %v", err)
	}
}

func TestStreamOrchestrator_Stream_EmptyChannel(t *testing.T) {
	logger := slog.Default()
	orchestrator := NewStreamOrchestrator(logger)

	w := httptest.NewRecorder()
	ctx := context.Background()

	// Create and immediately close the channel
	eventChan := make(chan protocol.UnifiedStreamEvent)
	close(eventChan)

	err := orchestrator.StreamWithDefaults(ctx, w, eventChan)
	if err != nil {
		t.Fatalf("Stream failed with empty channel: %v", err)
	}

	// Should send message_stop event
	body := w.Body.String()
	if !strings.Contains(body, "message_stop") {
		t.Error("expected response to contain message_stop event")
	}
}

func TestStreamOrchestrator_Stream_MultipleContentBlocks(t *testing.T) {
	logger := slog.Default()
	orchestrator := NewStreamOrchestrator(logger)

	w := httptest.NewRecorder()
	ctx := context.Background()

	eventChan := make(chan protocol.UnifiedStreamEvent, 20)

	go func() {
		defer close(eventChan)

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
		}

		// Send multiple content tokens
		for i := 0; i < 10; i++ {
			eventChan <- protocol.UnifiedStreamEvent{
				Type:    protocol.EventToken,
				Content: "chunk ",
			}
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:         protocol.EventStop,
			FinishReason: protocol.FinishReasonStop,
		}
	}()

	err := orchestrator.StreamWithDefaults(ctx, w, eventChan)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	body := w.Body.String()
	
	// Should contain multiple content_block_delta events
	deltaCount := strings.Count(body, "content_block_delta")
	if deltaCount < 10 {
		t.Errorf("expected at least 10 content_block_delta events, got %d", deltaCount)
	}
}

func TestStreamOrchestrator_Stream_WithThinkingAndContent(t *testing.T) {
	logger := slog.Default()
	orchestrator := NewStreamOrchestrator(logger)

	w := httptest.NewRecorder()
	ctx := context.Background()

	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:     protocol.EventThinking,
			Thinking: "Let me think...",
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

	err := orchestrator.StreamWithDefaults(ctx, w, eventChan)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	body := w.Body.String()
	
	// Should contain both thinking and content
	if !strings.Contains(body, "thinking") {
		t.Error("expected response to contain thinking content")
	}
	if !strings.Contains(body, "The answer is 42") {
		t.Error("expected response to contain regular content")
	}
}

func TestStreamOrchestrator_Stream_WithToolCalls(t *testing.T) {
	logger := slog.Default()
	orchestrator := NewStreamOrchestrator(logger)

	w := httptest.NewRecorder()
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
					ID:   "tool_123",
					Name: "get_weather",
					Args: `{"location":"NYC"}`,
				},
			},
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:         protocol.EventStop,
			FinishReason: protocol.FinishReasonToolCalls,
		}
	}()

	err := orchestrator.StreamWithDefaults(ctx, w, eventChan)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	body := w.Body.String()
	
	// Should contain tool_use content
	if !strings.Contains(body, "tool_use") {
		t.Error("expected response to contain tool_use")
	}
	if !strings.Contains(body, "get_weather") {
		t.Error("expected response to contain tool name")
	}
}

func TestStreamOrchestrator_Stream_WithError(t *testing.T) {
	logger := slog.Default()
	orchestrator := NewStreamOrchestrator(logger)

	w := httptest.NewRecorder()
	ctx := context.Background()

	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:  protocol.EventError,
			Error: errors.New("provider API error"),
		}
	}()

	err := orchestrator.StreamWithDefaults(ctx, w, eventChan)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	body := w.Body.String()
	
	// Should contain error event
	if !strings.Contains(body, "error") {
		t.Error("expected response to contain error event")
	}
	if !strings.Contains(body, "provider API error") {
		t.Error("expected response to contain error message")
	}
}

func TestStreamOrchestrator_HandleCancellation(t *testing.T) {
	logger := slog.Default()
	orchestrator := NewStreamOrchestrator(logger)

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan bool)
		go func() {
			orchestrator.HandleCancellation(ctx, "test")
			done <- true
		}()

		// Cancel context
		cancel()

		// Wait for HandleCancellation to complete
		select {
		case <-done:
			// Success
		case <-time.After(100 * time.Millisecond):
			t.Error("HandleCancellation did not complete in time")
		}
	})

	t.Run("context deadline exceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		done := make(chan bool)
		go func() {
			orchestrator.HandleCancellation(ctx, "test")
			done <- true
		}()

		// Wait for timeout
		time.Sleep(10 * time.Millisecond)

		// Wait for HandleCancellation to complete
		select {
		case <-done:
			// Success
		case <-time.After(100 * time.Millisecond):
			t.Error("HandleCancellation did not complete in time")
		}
	})
}
