package pipeline

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"ghostcli/internal/engine/protocol"
)

func TestNewUsageNormalizer(t *testing.T) {
	logger := slog.Default()
	normalizer := NewUsageNormalizer(logger)

	if normalizer == nil {
		t.Fatal("NewUsageNormalizer returned nil")
	}
	if normalizer.logger == nil {
		t.Error("normalizer logger is nil")
	}
}

func TestUsageNormalizer_Normalize_InjectsLastKnownCounts(t *testing.T) {
	logger := slog.Default()
	normalizer := NewUsageNormalizer(logger)

	ctx := context.Background()
	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		// First event with usage
		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
			Usage: &protocol.Usage{
				PromptTokens:     100,
				CompletionTokens: 0,
			},
		}

		// Second event WITHOUT usage (should be injected)
		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "Hello",
		}

		// Third event with updated usage
		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventToken,
			Usage: &protocol.Usage{
				PromptTokens:     100,
				CompletionTokens: 5,
			},
		}
	}()

	normalizedChan := normalizer.Normalize(ctx, eventChan, false)

	// Collect normalized events
	var events []protocol.UnifiedStreamEvent
	for event := range normalizedChan {
		events = append(events, event)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// First event should have original usage
	if events[0].Usage.PromptTokens != 100 {
		t.Errorf("event 0: expected PromptTokens 100, got %d", events[0].Usage.PromptTokens)
	}

	// Second event should have injected usage from first event
	if events[1].Usage == nil {
		t.Fatal("event 1: usage is nil")
	}
	if events[1].Usage.PromptTokens != 100 {
		t.Errorf("event 1: expected PromptTokens 100, got %d", events[1].Usage.PromptTokens)
	}
	if events[1].Usage.CompletionTokens != 0 {
		t.Errorf("event 1: expected CompletionTokens 0, got %d", events[1].Usage.CompletionTokens)
	}

	// Third event should have updated usage
	if events[2].Usage.CompletionTokens != 5 {
		t.Errorf("event 2: expected CompletionTokens 5, got %d", events[2].Usage.CompletionTokens)
	}
}

func TestUsageNormalizer_Normalize_EstimatesTokens(t *testing.T) {
	logger := slog.Default()
	normalizer := NewUsageNormalizer(logger)

	ctx := context.Background()
	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		// Event with content but no usage
		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "This is a test message with some content",
		}
	}()

	normalizedChan := normalizer.Normalize(ctx, eventChan, true)

	// Collect normalized events
	var events []protocol.UnifiedStreamEvent
	for event := range normalizedChan {
		events = append(events, event)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Should have estimated token count
	if events[0].Usage == nil {
		t.Fatal("usage is nil")
	}
	if events[0].Usage.CompletionTokens == 0 {
		t.Error("expected estimated CompletionTokens > 0")
	}
}

func TestUsageNormalizer_Normalize_UpdatesTotalTokens(t *testing.T) {
	logger := slog.Default()
	normalizer := NewUsageNormalizer(logger)

	ctx := context.Background()
	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
			Usage: &protocol.Usage{
				PromptTokens:     50,
				CompletionTokens: 10,
			},
		}
	}()

	normalizedChan := normalizer.Normalize(ctx, eventChan, false)

	// Collect normalized events
	var events []protocol.UnifiedStreamEvent
	for event := range normalizedChan {
		events = append(events, event)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// TotalTokens should be calculated
	if events[0].Usage.TotalTokens != 60 {
		t.Errorf("expected TotalTokens 60, got %d", events[0].Usage.TotalTokens)
	}
}

func TestUsageNormalizer_Normalize_ContextCancellation(t *testing.T) {
	logger := slog.Default()
	normalizer := NewUsageNormalizer(logger)

	ctx, cancel := context.WithCancel(context.Background())
	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		eventChan <- protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
		}

		// Give time for the first event to be processed
		time.Sleep(10 * time.Millisecond)

		// Cancel context
		cancel()

		// Try to send more events
		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "Should not be processed",
		}
	}()

	normalizedChan := normalizer.Normalize(ctx, eventChan, false)

	// Collect events (should stop after cancellation)
	var events []protocol.UnifiedStreamEvent
	for event := range normalizedChan {
		events = append(events, event)
	}

	// Should have received at least the first event before cancellation
	if len(events) == 0 {
		t.Error("expected at least one event before cancellation")
	}
}

func TestUsageNormalizer_Normalize_EmptyChannel(t *testing.T) {
	logger := slog.Default()
	normalizer := NewUsageNormalizer(logger)

	ctx := context.Background()
	eventChan := make(chan protocol.UnifiedStreamEvent)
	close(eventChan)

	normalizedChan := normalizer.Normalize(ctx, eventChan, false)

	// Should close immediately
	var events []protocol.UnifiedStreamEvent
	for event := range normalizedChan {
		events = append(events, event)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestEstimateTokenCount(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		minCount int
		maxCount int
	}{
		{
			name:     "empty string",
			content:  "",
			minCount: 0,
			maxCount: 0,
		},
		{
			name:     "short text",
			content:  "Hello",
			minCount: 1,
			maxCount: 2,
		},
		{
			name:     "medium text",
			content:  "This is a test message with some content",
			minCount: 8,
			maxCount: 12,
		},
		{
			name:     "long text",
			content:  "This is a much longer test message that contains significantly more content and should result in a higher token count estimate",
			minCount: 25,
			maxCount: 35,
		},
		{
			name:     "unicode characters",
			content:  "Hello 世界 🌍",
			minCount: 1,
			maxCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := estimateTokenCount(tt.content)

			if count < tt.minCount || count > tt.maxCount {
				t.Errorf("estimateTokenCount(%q) = %d, want between %d and %d",
					tt.content, count, tt.minCount, tt.maxCount)
			}
		})
	}
}

func TestEstimateTokensForRequest(t *testing.T) {
	tests := []struct {
		name     string
		req      *protocol.UnifiedChatRequest
		minCount int
	}{
		{
			name: "simple request",
			req: &protocol.UnifiedChatRequest{
				Model:     "claude-3-5-sonnet",
				MaxTokens: 1024,
				Messages: []protocol.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			minCount: 5, // Role overhead + content
		},
		{
			name: "request with system prompt",
			req: &protocol.UnifiedChatRequest{
				Model:     "claude-3-5-sonnet",
				System:    "You are a helpful assistant.",
				MaxTokens: 1024,
				Messages: []protocol.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			minCount: 10,
		},
		{
			name: "request with multiple messages",
			req: &protocol.UnifiedChatRequest{
				Model:     "claude-3-5-sonnet",
				MaxTokens: 1024,
				Messages: []protocol.Message{
					{Role: "user", Content: "Hello"},
					{Role: "assistant", Content: "Hi there!"},
					{Role: "user", Content: "How are you?"},
				},
			},
			minCount: 15,
		},
		{
			name: "request with tools",
			req: &protocol.UnifiedChatRequest{
				Model:     "claude-3-5-sonnet",
				MaxTokens: 1024,
				Messages: []protocol.Message{
					{Role: "user", Content: "What's the weather?"},
				},
				Tools: []protocol.Tool{
					{
						Name:        "get_weather",
						Description: "Get current weather",
						InputSchema: map[string]interface{}{
							"type": "object",
						},
					},
				},
			},
			minCount: 60, // Message + tool overhead
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := EstimateTokensForRequest(tt.req)

			if count < tt.minCount {
				t.Errorf("EstimateTokensForRequest() = %d, want at least %d", count, tt.minCount)
			}
		})
	}
}

func TestNormalizeUsageInPlace(t *testing.T) {
	tests := []struct {
		name              string
		event             *protocol.UnifiedStreamEvent
		lastInputTokens   int
		lastOutputTokens  int
		expectedInput     int
		expectedOutput    int
		expectedTotal     int
	}{
		{
			name: "nil usage - inject both",
			event: &protocol.UnifiedStreamEvent{
				Type: protocol.EventToken,
			},
			lastInputTokens:  100,
			lastOutputTokens: 50,
			expectedInput:    100,
			expectedOutput:   50,
			expectedTotal:    150,
		},
		{
			name: "partial usage - fill missing input",
			event: &protocol.UnifiedStreamEvent{
				Type: protocol.EventToken,
				Usage: &protocol.Usage{
					CompletionTokens: 25,
				},
			},
			lastInputTokens:  100,
			lastOutputTokens: 50,
			expectedInput:    100,
			expectedOutput:   25,
			expectedTotal:    125,
		},
		{
			name: "partial usage - fill missing output",
			event: &protocol.UnifiedStreamEvent{
				Type: protocol.EventToken,
				Usage: &protocol.Usage{
					PromptTokens: 75,
				},
			},
			lastInputTokens:  100,
			lastOutputTokens: 50,
			expectedInput:    75,
			expectedOutput:   50,
			expectedTotal:    125,
		},
		{
			name: "complete usage - no changes",
			event: &protocol.UnifiedStreamEvent{
				Type: protocol.EventToken,
				Usage: &protocol.Usage{
					PromptTokens:     200,
					CompletionTokens: 100,
				},
			},
			lastInputTokens:  100,
			lastOutputTokens: 50,
			expectedInput:    200,
			expectedOutput:   100,
			expectedTotal:    300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NormalizeUsageInPlace(tt.event, tt.lastInputTokens, tt.lastOutputTokens)

			if tt.event.Usage == nil {
				t.Fatal("usage is nil after normalization")
			}

			if tt.event.Usage.PromptTokens != tt.expectedInput {
				t.Errorf("PromptTokens = %d, want %d",
					tt.event.Usage.PromptTokens, tt.expectedInput)
			}
			if tt.event.Usage.CompletionTokens != tt.expectedOutput {
				t.Errorf("CompletionTokens = %d, want %d",
					tt.event.Usage.CompletionTokens, tt.expectedOutput)
			}
			if tt.event.Usage.TotalTokens != tt.expectedTotal {
				t.Errorf("TotalTokens = %d, want %d",
					tt.event.Usage.TotalTokens, tt.expectedTotal)
			}
		})
	}
}

func TestUsageNormalizer_ValidateUsage(t *testing.T) {
	logger := slog.Default()
	normalizer := NewUsageNormalizer(logger)

	tests := []struct {
		name    string
		usage   *protocol.Usage
		context string
	}{
		{
			name:    "nil usage",
			usage:   nil,
			context: "test",
		},
		{
			name: "negative tokens",
			usage: &protocol.Usage{
				PromptTokens:     -10,
				CompletionTokens: 5,
			},
			context: "test",
		},
		{
			name: "inconsistent total",
			usage: &protocol.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      200, // Should be 150
			},
			context: "test",
		},
		{
			name: "unreasonably large",
			usage: &protocol.Usage{
				PromptTokens:     2_000_000,
				CompletionTokens: 1_000_000,
				TotalTokens:      3_000_000,
			},
			context: "test",
		},
		{
			name: "valid usage",
			usage: &protocol.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
			context: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			normalizer.ValidateUsage(tt.usage, tt.context)
		})
	}
}

func TestFormatUsageString(t *testing.T) {
	tests := []struct {
		name     string
		usage    *protocol.Usage
		expected string
	}{
		{
			name:     "nil usage",
			usage:    nil,
			expected: "no usage data",
		},
		{
			name: "all zeros",
			usage: &protocol.Usage{
				PromptTokens:     0,
				CompletionTokens: 0,
				TotalTokens:      0,
			},
			expected: "no tokens",
		},
		{
			name: "only input tokens",
			usage: &protocol.Usage{
				PromptTokens: 100,
			},
			expected: "input: 100",
		},
		{
			name: "only output tokens",
			usage: &protocol.Usage{
				CompletionTokens: 50,
			},
			expected: "output: 50",
		},
		{
			name: "input and output",
			usage: &protocol.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
			},
			expected: "input: 100, output: 50",
		},
		{
			name: "all fields",
			usage: &protocol.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
			expected: "input: 100, output: 50, total: 150",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatUsageString(tt.usage)
			if result != tt.expected {
				t.Errorf("FormatUsageString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestUsageNormalizer_Normalize_AccumulatesOutputTokens(t *testing.T) {
	logger := slog.Default()
	normalizer := NewUsageNormalizer(logger)

	ctx := context.Background()
	eventChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(eventChan)

		// Multiple content events with estimation enabled
		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "First chunk",
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "Second chunk",
		}

		eventChan <- protocol.UnifiedStreamEvent{
			Type:    protocol.EventToken,
			Content: "Third chunk",
		}
	}()

	normalizedChan := normalizer.Normalize(ctx, eventChan, true)

	// Collect normalized events
	var events []protocol.UnifiedStreamEvent
	for event := range normalizedChan {
		events = append(events, event)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Output tokens should accumulate
	if events[0].Usage.CompletionTokens >= events[1].Usage.CompletionTokens {
		t.Error("expected output tokens to increase from event 0 to 1")
	}
	if events[1].Usage.CompletionTokens >= events[2].Usage.CompletionTokens {
		t.Error("expected output tokens to increase from event 1 to 2")
	}
}

func TestUsageNormalizer_Normalize_HandlesSlowProducer(t *testing.T) {
	logger := slog.Default()
	normalizer := NewUsageNormalizer(logger)

	ctx := context.Background()
	eventChan := make(chan protocol.UnifiedStreamEvent, 1)

	go func() {
		defer close(eventChan)

		// Simulate slow producer
		for i := 0; i < 5; i++ {
			eventChan <- protocol.UnifiedStreamEvent{
				Type:    protocol.EventToken,
				Content: "chunk",
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	normalizedChan := normalizer.Normalize(ctx, eventChan, false)

	// Collect events
	var events []protocol.UnifiedStreamEvent
	for event := range normalizedChan {
		events = append(events, event)
	}

	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}
}
