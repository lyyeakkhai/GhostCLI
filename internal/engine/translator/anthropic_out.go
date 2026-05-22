package translator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"ghostcli/internal/engine/protocol"
)

// idCounter is a package-level atomic counter used by generateID.
var idCounter uint64

// AnthropicOutFormatter converts UnifiedStreamEvent objects to Anthropic SSE format.
// It handles token usage tracking and injection, immediate flushing for zero-buffer streaming,
// and proper SSE event formatting.
type AnthropicOutFormatter struct {
	logger *slog.Logger

	// Token usage tracking across events
	inputTokens  int
	outputTokens int

	// Content block tracking
	// currentBlockIndex is the next block index to allocate.
	currentBlockIndex int
	hasStartedMessage bool
	// textBlockIndex is the SSE index of the text content block (-1 = not started).
	textBlockIndex int
	// thinkingBlockIndex is the SSE index of the thinking content block (-1 = not started).
	thinkingBlockIndex int
}

// NewAnthropicOutFormatter creates a new AnthropicOut formatter instance.
func NewAnthropicOutFormatter(logger *slog.Logger) *AnthropicOutFormatter {
	return &AnthropicOutFormatter{
		logger:             logger,
		inputTokens:        0,
		outputTokens:       0,
		currentBlockIndex:  0,
		hasStartedMessage:  false,
		textBlockIndex:     -1,
		thinkingBlockIndex: -1,
	}
}

// StreamToWriter converts a channel of UnifiedStreamEvent to Anthropic SSE format
// and writes them to the provided http.ResponseWriter with immediate flushing.
// It resets all mutable state at the start so a reused instance does not carry
// stale values from a previous call.
func (f *AnthropicOutFormatter) StreamToWriter(
	ctx context.Context,
	w http.ResponseWriter,
	eventChan <-chan protocol.UnifiedStreamEvent,
) error {
	// Reset per-stream state so reused instances start clean.
	f.inputTokens = 0
	f.outputTokens = 0
	f.currentBlockIndex = 0
	f.hasStartedMessage = false
	f.textBlockIndex = -1
	f.thinkingBlockIndex = -1

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Ensure the writer supports flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("response writer does not support flushing")
	}

	// Process events from the channel
	for {
		if err := ctx.Err(); err != nil {
			f.logger.Debug("context cancelled during streaming")
			return err
		}

		select {
		case <-ctx.Done():
			f.logger.Debug("context cancelled during streaming")
			return ctx.Err()

		case event, ok := <-eventChan:
			if !ok {
				// Channel closed, send final message_stop event
				f.logger.Debug("event channel closed, sending message_stop")
				return f.writeMessageStop(w, flusher)
			}

			// Update token usage if present in the event
			if event.Usage != nil {
				if event.Usage.PromptTokens > 0 {
					f.inputTokens = event.Usage.PromptTokens
				}
				if event.Usage.CompletionTokens > 0 {
					f.outputTokens = event.Usage.CompletionTokens
				}
			}

			// Convert and write the event
			if err := f.writeEvent(w, flusher, event); err != nil {
				f.logger.Error("failed to write event", "error", err)
				return err
			}
		}
	}
}

// writeEvent converts a UnifiedStreamEvent to Anthropic SSE format and writes it.
func (f *AnthropicOutFormatter) writeEvent(
	w io.Writer,
	flusher http.Flusher,
	event protocol.UnifiedStreamEvent,
) error {
	switch event.Type {
	case protocol.EventStart:
		return f.writeMessageStart(w, flusher, event.Model)

	case protocol.EventToken:
		return f.writeContentDelta(w, flusher, event.Content)

	case protocol.EventThinking:
		return f.writeThinkingDelta(w, flusher, event.Thinking)

	case protocol.EventToolCall:
		return f.writeToolCallEvents(w, flusher, event.ToolCalls)

	case protocol.EventStop:
		return f.writeMessageDelta(w, flusher, event.FinishReason)

	case protocol.EventError:
		return f.writeError(w, flusher, event.Error)

	default:
		f.logger.Warn("unknown event type", "type", event.Type)
		return nil
	}
}

// writeMessageStart writes the message_start SSE event.
// model is the upstream model name from UnifiedStreamEvent.Model; if empty,
// "proxy-model" is used as a generic placeholder.
func (f *AnthropicOutFormatter) writeMessageStart(w io.Writer, flusher http.Flusher, model string) error {
	if f.hasStartedMessage {
		return nil // Already sent message_start
	}

	if model == "" {
		model = "proxy-model"
	}

	data := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":    "msg_proxy_" + generateID(),
			"type":  "message",
			"role":  "assistant",
			"model": model,
			"usage": map[string]int{
				"input_tokens":  f.inputTokens,
				"output_tokens": 0,
			},
		},
	}

	f.hasStartedMessage = true
	return f.writeSSEEvent(w, flusher, "message_start", data)
}

// writeContentDelta writes content_block_start (first call) and content_block_delta events.
// It uses f.textBlockIndex as a sentinel (-1 = block not yet opened) so that the
// allocated index is stable across multiple delta calls even when other blocks
// (e.g. thinking) have already been allocated before this one.
func (f *AnthropicOutFormatter) writeContentDelta(w io.Writer, flusher http.Flusher, content string) error {
	if content == "" {
		return nil
	}

	if !f.hasStartedMessage {
		if err := f.writeMessageStart(w, flusher, ""); err != nil {
			return err
		}
	}

	// Allocate a text block on the first content call.
	if f.textBlockIndex == -1 {
		f.textBlockIndex = f.currentBlockIndex
		f.currentBlockIndex++
		startData := map[string]interface{}{
			"type":  "content_block_start",
			"index": f.textBlockIndex,
			"content_block": map[string]interface{}{
				"type": "text",
				"text": "",
			},
		}
		if err := f.writeSSEEvent(w, flusher, "content_block_start", startData); err != nil {
			return err
		}
	}

	deltaData := map[string]interface{}{
		"type":  "content_block_delta",
		"index": f.textBlockIndex,
		"delta": map[string]interface{}{
			"type": "text_delta",
			"text": content,
		},
	}

	return f.writeSSEEvent(w, flusher, "content_block_delta", deltaData)
}

// writeThinkingDelta writes thinking content as a separate content block.
// Thinking is allocated at the first available block index; subsequent text
// content is allocated at the next index (see writeContentDelta).
func (f *AnthropicOutFormatter) writeThinkingDelta(w io.Writer, flusher http.Flusher, thinking string) error {
	if thinking == "" {
		return nil
	}

	if !f.hasStartedMessage {
		if err := f.writeMessageStart(w, flusher, ""); err != nil {
			return err
		}
	}

	// Allocate the thinking block only once.
	if f.thinkingBlockIndex == -1 {
		f.thinkingBlockIndex = f.currentBlockIndex
		f.currentBlockIndex++
		startData := map[string]interface{}{
			"type":  "content_block_start",
			"index": f.thinkingBlockIndex,
			"content_block": map[string]interface{}{
				"type":     "thinking",
				"thinking": "",
			},
		}
		if err := f.writeSSEEvent(w, flusher, "content_block_start", startData); err != nil {
			return err
		}
	}

	deltaData := map[string]interface{}{
		"type":  "content_block_delta",
		"index": f.thinkingBlockIndex,
		"delta": map[string]interface{}{
			"type":     "thinking_delta",
			"thinking": thinking,
		},
	}

	return f.writeSSEEvent(w, flusher, "content_block_delta", deltaData)
}

// writeToolCallEvents writes tool_use content blocks.
func (f *AnthropicOutFormatter) writeToolCallEvents(w io.Writer, flusher http.Flusher, toolCalls []protocol.ToolCall) error {
	if len(toolCalls) == 0 {
		return nil
	}

	if !f.hasStartedMessage {
		if err := f.writeMessageStart(w, flusher, ""); err != nil {
			return err
		}
	}

	for _, toolCall := range toolCalls {
		// Send content_block_start for tool_use
		startData := map[string]interface{}{
			"type":  "content_block_start",
			"index": f.currentBlockIndex,
			"content_block": map[string]interface{}{
				"type": "tool_use",
				"id":   toolCall.ID,
				"name": toolCall.Name,
			},
		}
		if err := f.writeSSEEvent(w, flusher, "content_block_start", startData); err != nil {
			return err
		}

		// Send content_block_delta with input JSON
		if toolCall.Args != "" {
			deltaData := map[string]interface{}{
				"type":  "content_block_delta",
				"index": f.currentBlockIndex,
				"delta": map[string]interface{}{
					"type":         "input_json_delta",
					"partial_json": toolCall.Args,
				},
			}
			if err := f.writeSSEEvent(w, flusher, "content_block_delta", deltaData); err != nil {
				return err
			}
		}

		// Send content_block_stop
		stopData := map[string]interface{}{
			"type":  "content_block_stop",
			"index": f.currentBlockIndex,
		}
		if err := f.writeSSEEvent(w, flusher, "content_block_stop", stopData); err != nil {
			return err
		}

		f.currentBlockIndex++
	}

	return nil
}

// writeMessageDelta writes the message_delta event with finish reason and final usage.
// It first closes every content block that was opened during this stream
// (thinking + text blocks; tool blocks are closed by writeToolCallEvents).
func (f *AnthropicOutFormatter) writeMessageDelta(w io.Writer, flusher http.Flusher, finishReason string) error {
	// Close every non-tool content block that was opened.
	// Tool blocks emit their own content_block_stop inside writeToolCallEvents.
	for i := 0; i < f.currentBlockIndex; i++ {
		stopData := map[string]interface{}{
			"type":  "content_block_stop",
			"index": i,
		}
		if err := f.writeSSEEvent(w, flusher, "content_block_stop", stopData); err != nil {
			return err
		}
	}

	// Map finish reason to Anthropic format
	anthropicFinishReason := f.mapFinishReason(finishReason)

	// Send message_delta with final usage
	deltaData := map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason": anthropicFinishReason,
		},
		"usage": map[string]int{
			"output_tokens": f.outputTokens,
		},
	}

	return f.writeSSEEvent(w, flusher, "message_delta", deltaData)
}

// writeMessageStop writes the final message_stop SSE event.
func (f *AnthropicOutFormatter) writeMessageStop(w io.Writer, flusher http.Flusher) error {
	data := map[string]interface{}{
		"type": "message_stop",
	}

	return f.writeSSEEvent(w, flusher, "message_stop", data)
}

// writeError writes an error event in Anthropic SSE format.
func (f *AnthropicOutFormatter) writeError(w io.Writer, flusher http.Flusher, err error) error {
	if err == nil {
		return nil
	}

	data := map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    "api_error",
			"message": err.Error(),
		},
	}

	return f.writeSSEEvent(w, flusher, "error", data)
}

// writeSSEEvent writes a single SSE event with the given type and data.
func (f *AnthropicOutFormatter) writeSSEEvent(
	w io.Writer,
	flusher http.Flusher,
	eventType string,
	data interface{},
) error {
	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	// Write SSE format: event: <type>\ndata: <json>\n\n
	if _, err := fmt.Fprintf(w, "event: %s\n", eventType); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", jsonData); err != nil {
		return err
	}

	// Flush immediately for zero-buffer streaming
	flusher.Flush()

	f.logger.Debug("wrote SSE event", "type", eventType)
	return nil
}

// mapFinishReason maps UnifiedStreamEvent finish reasons to Anthropic format.
func (f *AnthropicOutFormatter) mapFinishReason(reason string) string {
	switch reason {
	case protocol.FinishReasonStop:
		return "end_turn"
	case protocol.FinishReasonLength:
		return "max_tokens"
	case protocol.FinishReasonToolCalls:
		return "tool_use"
	case protocol.FinishReasonError:
		return "error"
	default:
		return "end_turn"
	}
}

// generateID generates a practically unique ID for message tracking by combining
// the current Unix nanosecond timestamp with an atomically incremented counter.
// This avoids external dependencies while making collisions negligibly unlikely.
func generateID() string {
	seq := atomic.AddUint64(&idCounter, 1)
	return fmt.Sprintf("%x%x", time.Now().UnixNano(), seq)
}
