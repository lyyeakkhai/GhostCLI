// Package base provides reusable provider adapter implementations for common
// API protocol families. Each base adapter handles the low-level HTTP request
// construction, SSE/EventStream parsing, and UnifiedStreamEvent emission so
// that concrete provider packages only need to supply provider-specific config.
package base

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"ghostcli/internal/engine/protocol"
)

// AnthropicConfig holds all configuration required to construct an AnthropicAdapter.
type AnthropicConfig struct {
	// Name is the unique provider identifier (e.g. "anthropic", "openrouter").
	Name string

	// BaseURL is the root URL for the provider's Messages API, including the
	// path prefix if any (e.g. "https://api.anthropic.com").
	// The adapter appends "/v1/messages" when constructing each request.
	BaseURL string

	// APIKey is the secret key sent in the x-api-key header.
	APIKey string

	// AnthropicVersion is the value for the anthropic-version header.
	// Defaults to "2023-06-01" when empty.
	AnthropicVersion string

	// ModelMap maps Anthropic model names to provider-specific model identifiers.
	// When a model name is absent from the map the original name is passed through.
	ModelMap map[string]string

	// Logger is the structured logger used for debug / error output.
	Logger *slog.Logger

	// HTTPClient is an optional pre-configured *http.Client.
	// When nil a default client with a 5-minute timeout is created.
	HTTPClient *http.Client

	// ExtraHeaders holds additional HTTP headers to include in every request
	// (e.g. "HTTP-Referer", "X-Title" used by OpenRouter).
	ExtraHeaders map[string]string
}

// AnthropicAdapter is the base implementation for providers that speak the
// Anthropic Messages API natively (Anthropic direct, OpenRouter, Kiro Gateway,
// …). It acts as a near-passthrough: the UnifiedChatRequest is re-serialised
// into Anthropic JSON format, sent upstream, and the incoming SSE stream is
// parsed back into UnifiedStreamEvent objects with minimal transformation.
//
// Satisfies the providers.Provider interface.
type AnthropicAdapter struct {
	cfg    AnthropicConfig
	client *http.Client
	logger *slog.Logger
}

// NewAnthropicAdapter creates a ready-to-use AnthropicAdapter from the
// supplied configuration.
func NewAnthropicAdapter(cfg AnthropicConfig) *AnthropicAdapter {
	if cfg.AnthropicVersion == "" {
		cfg.AnthropicVersion = "2023-06-01"
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &AnthropicAdapter{
		cfg:    cfg,
		client: client,
		logger: logger,
	}
}

// ────────────────────────────────────────────────────────────────────────────
// providers.Provider interface
// ────────────────────────────────────────────────────────────────────────────

// Name returns the unique provider identifier supplied at construction time.
func (a *AnthropicAdapter) Name() string { return a.cfg.Name }

// SupportsTools returns true because the Anthropic Messages API supports
// tool / function calling natively.
func (a *AnthropicAdapter) SupportsTools() bool { return true }

// SupportsThinking returns true because Claude 3.5+ models can emit extended
// thinking blocks via the Anthropic API.
func (a *AnthropicAdapter) SupportsThinking() bool { return true }

// MapModel translates an Anthropic model name to the provider-specific
// identifier using the ModelMap from config. Unknown names are passed through
// unchanged.
func (a *AnthropicAdapter) MapModel(anthropicModel string) string {
	if mapped, ok := a.cfg.ModelMap[anthropicModel]; ok {
		a.logger.Debug("model mapped", "provider", a.cfg.Name, "from", anthropicModel, "to", mapped)
		return mapped
	}
	return anthropicModel
}

// StreamChat translates req into an Anthropic Messages API request, sends it
// to the upstream provider, and returns a channel that emits UnifiedStreamEvent
// objects as SSE chunks arrive. The channel is closed when the stream ends or
// the context is cancelled.
func (a *AnthropicAdapter) StreamChat(
	ctx context.Context,
	req *protocol.UnifiedChatRequest,
) (<-chan protocol.UnifiedStreamEvent, error) {
	// Build the HTTP request.
	httpReq, err := a.buildRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("anthropic adapter (%s): build request: %w", a.cfg.Name, err)
	}

	// Execute the HTTP request.
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic adapter (%s): http request: %w", a.cfg.Name, err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, a.readErrorResponse(resp)
	}

	// Kick off the streaming goroutine and return the channel immediately.
	eventChan := make(chan protocol.UnifiedStreamEvent, 16)
	go a.streamResponse(ctx, resp.Body, eventChan)

	return eventChan, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ────────────────────────────────────────────────────────────────────────────

// anthropicRequest is the JSON body sent to the Anthropic Messages API.
type anthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream"`

	// Optional parameters – omit when zero to keep the payload minimal.
	Temperature float32          `json:"temperature,omitempty"`
	TopP        float32          `json:"top_p,omitempty"`
	Tools       []anthropicTool  `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// buildRequest constructs the *http.Request for the upstream provider.
func (a *AnthropicAdapter) buildRequest(ctx context.Context, req *protocol.UnifiedChatRequest) (*http.Request, error) {
	// Translate model name.
	model := a.MapModel(req.Model)

	// Build Anthropic-format messages.
	msgs := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, anthropicMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Build tool definitions.
	var tools []anthropicTool
	if len(req.Tools) > 0 {
		tools = make([]anthropicTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, anthropicTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
	}

	body := anthropicRequest{
		Model:       model,
		Messages:    msgs,
		System:      req.System,
		MaxTokens:   req.MaxTokens,
		Stream:      true, // always stream at the HTTP level
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Tools:       tools,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	endpoint := strings.TrimRight(a.cfg.BaseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}

	// Anthropic-specific mandatory headers.
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", a.cfg.AnthropicVersion)
	httpReq.Header.Set("Accept", "text/event-stream")

	// Provider-specific extra headers (e.g. OpenRouter's HTTP-Referer).
	for k, v := range a.cfg.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	a.logger.Debug("built anthropic request",
		"provider", a.cfg.Name,
		"model", model,
		"endpoint", endpoint,
		"message_count", len(msgs),
		"tools", len(tools),
	)

	return httpReq, nil
}

// readErrorResponse drains an error response body and returns a descriptive error.
func (a *AnthropicAdapter) readErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	// Try to extract the Anthropic error JSON { "error": { "message": "..." } }.
	var apiErr struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Error.Message != "" {
		return fmt.Errorf("provider %s: HTTP %d – %s (%s)",
			a.cfg.Name, resp.StatusCode, apiErr.Error.Message, apiErr.Error.Type)
	}

	return fmt.Errorf("provider %s: HTTP %d – %s", a.cfg.Name, resp.StatusCode, strings.TrimSpace(string(body)))
}

// ────────────────────────────────────────────────────────────────────────────
// SSE streaming
// ────────────────────────────────────────────────────────────────────────────

// streamResponse reads the SSE body, converts each Anthropic SSE event to a
// UnifiedStreamEvent, and sends it on eventChan. It closes eventChan when done.
func (a *AnthropicAdapter) streamResponse(
	ctx context.Context,
	body io.ReadCloser,
	eventChan chan<- protocol.UnifiedStreamEvent,
) {
	defer close(eventChan)
	defer body.Close()

	scanner := bufio.NewScanner(body)

	// Track accumulated state across SSE events.
	var (
		currentEventType string // value of the last "event: ..." line
		model            string // captured from message_start
		sentStart        bool
	)

	for scanner.Scan() {
		// Honour context cancellation on each iteration.
		select {
		case <-ctx.Done():
			a.logger.Debug("context cancelled, stopping anthropic stream", "provider", a.cfg.Name)
			return
		default:
		}

		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "event:"):
			currentEventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))

		case strings.HasPrefix(line, "data:"):
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" || data == "[DONE]" {
				continue
			}

			event, ok := a.parseSSEEvent(currentEventType, data, &model, &sentStart)
			if !ok {
				continue
			}

			select {
			case eventChan <- event:
			case <-ctx.Done():
				return
			}

		case line == "":
			// blank line resets the current event type (SSE boundary)
			currentEventType = ""
		}
	}

	if err := scanner.Err(); err != nil {
		a.logger.Error("scanner error while reading anthropic stream",
			"provider", a.cfg.Name, "error", err)
		select {
		case eventChan <- protocol.UnifiedStreamEvent{
			Type:  protocol.EventError,
			Error: fmt.Errorf("stream read error: %w", err),
		}:
		case <-ctx.Done():
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Anthropic SSE event types and parsing
// ────────────────────────────────────────────────────────────────────────────

// Raw Anthropic SSE event payloads (only the fields we consume).

type anthropicMessageStartEvent struct {
	Type    string `json:"type"`
	Message struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type anthropicContentBlockStartEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	ContentBlock struct {
		Type string `json:"type"` // "text", "thinking", "tool_use"
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"content_block"`
}

type anthropicContentBlockDeltaEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type        string `json:"type"`  // "text_delta", "thinking_delta", "input_json_delta"
		Text        string `json:"text,omitempty"`
		Thinking    string `json:"thinking,omitempty"`
		PartialJSON string `json:"partial_json,omitempty"`
	} `json:"delta"`
}

type anthropicContentBlockStopEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

type anthropicMessageDeltaEvent struct {
	Type  string `json:"type"`
	Delta struct {
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicMessageStopEvent struct {
	Type string `json:"type"`
}

type anthropicErrorEvent struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// parseSSEEvent converts a single (eventType, data) pair from the upstream
// Anthropic SSE stream into a UnifiedStreamEvent.
// Returns (event, true) when a meaningful event is produced; (zero, false) otherwise.
//
// The model and sentStart pointers carry cross-event state managed by streamResponse.
func (a *AnthropicAdapter) parseSSEEvent(
	eventType, data string,
	model *string,
	sentStart *bool,
) (protocol.UnifiedStreamEvent, bool) {

	switch eventType {
	case "message_start":
		var ev anthropicMessageStartEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			a.logger.Warn("failed to parse message_start", "error", err)
			return protocol.UnifiedStreamEvent{}, false
		}
		*model = ev.Message.Model
		if !*sentStart {
			*sentStart = true
			return protocol.UnifiedStreamEvent{
				Type:  protocol.EventStart,
				Model: ev.Message.Model,
				Usage: &protocol.Usage{
					PromptTokens: ev.Message.Usage.InputTokens,
				},
			}, true
		}
		return protocol.UnifiedStreamEvent{}, false

	case "content_block_delta":
		var ev anthropicContentBlockDeltaEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			a.logger.Warn("failed to parse content_block_delta", "error", err)
			return protocol.UnifiedStreamEvent{}, false
		}
		switch ev.Delta.Type {
		case "text_delta":
			if ev.Delta.Text == "" {
				return protocol.UnifiedStreamEvent{}, false
			}
			return protocol.UnifiedStreamEvent{
				Type:    protocol.EventToken,
				Content: ev.Delta.Text,
			}, true
		case "thinking_delta":
			if ev.Delta.Thinking == "" {
				return protocol.UnifiedStreamEvent{}, false
			}
			return protocol.UnifiedStreamEvent{
				Type:     protocol.EventThinking,
				Thinking: ev.Delta.Thinking,
			}, true
		case "input_json_delta":
			// Tool input accumulation – emit as a partial tool call.
			if ev.Delta.PartialJSON == "" {
				return protocol.UnifiedStreamEvent{}, false
			}
			return protocol.UnifiedStreamEvent{
				Type: protocol.EventToolCall,
				ToolCalls: []protocol.ToolCall{
					{Args: ev.Delta.PartialJSON},
				},
			}, true
		}
		return protocol.UnifiedStreamEvent{}, false

	case "content_block_start":
		// tool_use blocks announce their ID and name here; we emit a ToolCall
		// event so downstream formatters can track the tool being invoked.
		var ev anthropicContentBlockStartEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			a.logger.Warn("failed to parse content_block_start", "error", err)
			return protocol.UnifiedStreamEvent{}, false
		}
		if ev.ContentBlock.Type == "tool_use" {
			return protocol.UnifiedStreamEvent{
				Type: protocol.EventToolCall,
				ToolCalls: []protocol.ToolCall{
					{
						ID:   ev.ContentBlock.ID,
						Name: ev.ContentBlock.Name,
					},
				},
			}, true
		}
		return protocol.UnifiedStreamEvent{}, false

	case "content_block_stop":
		// No direct mapping; the stop signal is conveyed through the channel close.
		return protocol.UnifiedStreamEvent{}, false

	case "message_delta":
		var ev anthropicMessageDeltaEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			a.logger.Warn("failed to parse message_delta", "error", err)
			return protocol.UnifiedStreamEvent{}, false
		}
		finishReason := a.mapStopReason(ev.Delta.StopReason)
		return protocol.UnifiedStreamEvent{
			Type:         protocol.EventStop,
			FinishReason: finishReason,
			Usage: &protocol.Usage{
				CompletionTokens: ev.Usage.OutputTokens,
			},
		}, true

	case "message_stop":
		// The channel close (deferred in streamResponse) signals end-of-stream;
		// no additional event needed.
		return protocol.UnifiedStreamEvent{}, false

	case "error":
		var ev anthropicErrorEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			a.logger.Warn("failed to parse error event", "error", err)
			return protocol.UnifiedStreamEvent{
				Type:  protocol.EventError,
				Error: fmt.Errorf("upstream error (unparseable)"),
			}, true
		}
		return protocol.UnifiedStreamEvent{
			Type:  protocol.EventError,
			Error: fmt.Errorf("upstream error %s: %s", ev.Error.Type, ev.Error.Message),
		}, true

	default:
		// Unknown / ping / comment events are silently ignored.
		return protocol.UnifiedStreamEvent{}, false
	}
}

// mapStopReason converts an Anthropic stop_reason to the unified protocol constant.
func (a *AnthropicAdapter) mapStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return protocol.FinishReasonStop
	case "max_tokens":
		return protocol.FinishReasonLength
	case "tool_use":
		return protocol.FinishReasonToolCalls
	case "stop_sequence":
		return protocol.FinishReasonStop
	default:
		return protocol.FinishReasonStop
	}
}
