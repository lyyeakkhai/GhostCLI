// Package base provides reusable provider adapter implementations that can be
// embedded or composed by concrete provider packages (DeepSeek, Kimi, OpenAI, etc.).
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

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

// OpenAIConfig holds the configuration for an OpenAI-compatible provider adapter.
// It is used by both the base adapter and any concrete provider that embeds it.
type OpenAIConfig struct {
	// Name is the unique provider identifier (e.g. "deepseek", "kimi", "openai").
	Name string

	// BaseURL is the root URL of the provider's API, including the path prefix
	// up to but not including "/chat/completions".
	// Example: "https://api.deepseek.com/v1"
	BaseURL string

	// APIKey is the secret token used to authenticate with the provider.
	APIKey string

	// ModelMap maps Anthropic model names (as received in UnifiedChatRequest.Model)
	// to the provider-specific model identifier.
	// Requirements: 19
	ModelMap map[string]string

	// AuthHeader is the HTTP header used to send the API key.
	// Defaults to "Authorization" when empty.
	AuthHeader string

	// AuthPrefix is prepended to the APIKey value in the auth header.
	// Defaults to "Bearer " when empty.
	AuthPrefix string

	// HTTPClient is the HTTP client used for all outbound requests.
	// A sensible default (30 s timeout, pooled transport) is used when nil.
	HTTPClient *http.Client
}

// ---------------------------------------------------------------------------
// OpenAI wire format types
// ---------------------------------------------------------------------------

// openAIRequest is the JSON body sent to OpenAI-compatible /v1/chat/completions.
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float32         `json:"temperature,omitempty"`
	TopP        float32         `json:"top_p,omitempty"`
	Stream      bool            `json:"stream"`
	StreamOptions *openAIStreamOptions `json:"stream_options,omitempty"`
	Tools       []openAITool    `json:"tools,omitempty"`
}

// openAIStreamOptions requests additional fields in stream chunks.
type openAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// openAIMessage is a single message in the OpenAI chat format.
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAITool is the OpenAI function-calling tool definition.
type openAITool struct {
	Type     string          `json:"type"` // always "function"
	Function openAIFunction  `json:"function"`
}

// openAIFunction contains the schema for a callable function.
type openAIFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// openAIStreamChunk represents a single SSE chunk in the streaming response.
type openAIStreamChunk struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []openAIChoice   `json:"choices"`
	Usage   *openAIUsage     `json:"usage,omitempty"`
}

// openAIChoice represents a single choice in the streaming response.
type openAIChoice struct {
	Index        int            `json:"index"`
	Delta        openAIDelta    `json:"delta"`
	FinishReason *string        `json:"finish_reason"`
}

// openAIDelta contains the incremental content in a streaming chunk.
type openAIDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
}

// openAIToolCall represents a streamed tool call delta.
type openAIToolCall struct {
	Index    int                  `json:"index"`
	ID       string               `json:"id,omitempty"`
	Type     string               `json:"type,omitempty"`
	Function openAIFunctionCall   `json:"function"`
}

// openAIFunctionCall represents function name and partial arguments.
type openAIFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// openAIUsage represents token usage reported in the final chunk.
type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// openAIError is the JSON body of an error response from an OpenAI-compatible API.
type openAIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// ---------------------------------------------------------------------------
// OpenAIAdapter
// ---------------------------------------------------------------------------

// OpenAIAdapter is the reusable base adapter for all providers that implement
// the OpenAI Chat Completions API format (DeepSeek, Kimi, OpenAI, Groq, etc.).
//
// It satisfies the providers.Provider interface and can be used directly or
// embedded by concrete provider packages.
//
// Requirements: 4, 6, 7, 8, 19
type OpenAIAdapter struct {
	cfg    OpenAIConfig
	client *http.Client
	logger *slog.Logger
}

// NewOpenAIAdapter constructs an OpenAIAdapter with the given configuration.
// It fills in default values for any optional fields that were left empty.
func NewOpenAIAdapter(cfg OpenAIConfig, logger *slog.Logger) *OpenAIAdapter {
	// Apply defaults
	if cfg.AuthHeader == "" {
		cfg.AuthHeader = "Authorization"
	}
	if cfg.AuthPrefix == "" {
		cfg.AuthPrefix = "Bearer "
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}

	return &OpenAIAdapter{
		cfg:    cfg,
		client: client,
		logger: logger,
	}
}

// ---------------------------------------------------------------------------
// Provider interface methods
// ---------------------------------------------------------------------------

// Name returns the unique identifier for this provider.
// Requirements: 4
func (a *OpenAIAdapter) Name() string {
	return a.cfg.Name
}

// SupportsTools returns true; all OpenAI-compatible providers support tool calling.
// Requirements: 4, 20
func (a *OpenAIAdapter) SupportsTools() bool {
	return true
}

// SupportsThinking returns false; standard OpenAI-compatible APIs do not emit
// reasoning/thinking blocks in the streaming response.
// Requirements: 4
func (a *OpenAIAdapter) SupportsThinking() bool {
	return false
}

// MapModel translates an Anthropic model name to the provider-specific model
// identifier using the ModelMap in the configuration. If no mapping is found,
// the original name is returned unchanged.
// Requirements: 19
func (a *OpenAIAdapter) MapModel(anthropicModel string) string {
	if mapped, ok := a.cfg.ModelMap[anthropicModel]; ok {
		a.logger.Debug("model mapped",
			"provider", a.cfg.Name,
			"from", anthropicModel,
			"to", mapped,
		)
		return mapped
	}
	a.logger.Debug("no model mapping found, using original",
		"provider", a.cfg.Name,
		"model", anthropicModel,
	)
	return anthropicModel
}

// StreamChat initiates a streaming chat request to the OpenAI-compatible API.
// It converts the UnifiedChatRequest to the OpenAI wire format, sends the HTTP
// request, and returns a channel that will receive UnifiedStreamEvent objects
// as the provider streams its response.
//
// The caller owns the returned channel; it is closed when the stream ends or
// when the context is cancelled.
//
// Requirements: 4, 6, 7, 8
func (a *OpenAIAdapter) StreamChat(
	ctx context.Context,
	req *protocol.UnifiedChatRequest,
) (<-chan protocol.UnifiedStreamEvent, error) {
	// Build the OpenAI request payload.
	openaiReq := a.convertToOpenAIFormat(req)

	// Encode to JSON.
	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("openai_base: failed to encode request: %w", err)
	}

	// Build the HTTP request.
	url := strings.TrimRight(a.cfg.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai_base: failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set(a.cfg.AuthHeader, a.cfg.AuthPrefix+a.cfg.APIKey)

	a.logger.Debug("sending OpenAI request",
		"provider", a.cfg.Name,
		"model", openaiReq.Model,
		"messages", len(openaiReq.Messages),
		"url", url,
	)

	// Execute the HTTP request.
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai_base: HTTP request failed: %w", err)
	}

	// Non-200 responses indicate a provider-level error.
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, a.parseErrorResponse(resp)
	}

	// Spin up a goroutine to stream SSE chunks into the event channel.
	eventChan := make(chan protocol.UnifiedStreamEvent, 16)
	go a.streamResponse(ctx, resp.Body, eventChan)

	return eventChan, nil
}

// ---------------------------------------------------------------------------
// Request conversion
// ---------------------------------------------------------------------------

// convertToOpenAIFormat translates a UnifiedChatRequest into the OpenAI wire
// format, applying model mapping and tool conversion.
// Requirements: 4.3, 6.2, 7.2, 8.2
func (a *OpenAIAdapter) convertToOpenAIFormat(req *protocol.UnifiedChatRequest) openAIRequest {
	// Map the model name.
	modelName := a.MapModel(req.Model)

	// Build the message list.
	// If a system prompt is set, prepend a system message.
	messages := make([]openAIMessage, 0, len(req.Messages)+1)
	if req.System != "" {
		messages = append(messages, openAIMessage{
			Role:    protocol.RoleSystem,
			Content: req.System,
		})
	}
	for _, m := range req.Messages {
		messages = append(messages, openAIMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	openaiReq := openAIRequest{
		Model:       modelName,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
	}

	// Request usage in stream chunks so token counts are available.
	if req.Stream {
		openaiReq.StreamOptions = &openAIStreamOptions{IncludeUsage: true}
	}

	// Convert tools if present.
	if len(req.Tools) > 0 {
		openaiReq.Tools = convertTools(req.Tools)
	}

	return openaiReq
}

// convertTools converts the provider-agnostic Tool slice to OpenAI format.
func convertTools(tools []protocol.Tool) []openAITool {
	result := make([]openAITool, 0, len(tools))
	for _, t := range tools {
		result = append(result, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}
	return result
}

// ---------------------------------------------------------------------------
// Streaming response parser
// ---------------------------------------------------------------------------

// streamResponse reads the SSE body, parses each data line as an
// openAIStreamChunk, converts it to UnifiedStreamEvent objects, and sends them
// on eventChan. It closes eventChan when done (either EOF, [DONE], or context
// cancellation).
//
// Requirements: 6.5, 7.5, 8.5
func (a *OpenAIAdapter) streamResponse(
	ctx context.Context,
	body io.ReadCloser,
	eventChan chan<- protocol.UnifiedStreamEvent,
) {
	defer close(eventChan)
	defer body.Close()

	// We track in-flight tool call state across chunks because OpenAI streams
	// tool call name and arguments incrementally.
	toolCallState := make(map[int]*protocol.ToolCall)
	sentStart := false

	scanner := bufio.NewScanner(body)
	// Increase scanner buffer for large SSE payloads.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		// Respect context cancellation between lines.
		select {
		case <-ctx.Done():
			a.sendError(ctx, eventChan, ctx.Err())
			return
		default:
		}

		line := scanner.Text()

		// SSE spec: blank lines are event delimiters; comment lines start with ":"
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// All meaningful content lines start with "data: "
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// End-of-stream sentinel.
		if data == "[DONE]" {
			// Flush any pending tool calls.
			a.flushToolCalls(ctx, toolCallState, eventChan)
			return
		}

		// Parse the JSON chunk.
		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			a.logger.Error("openai_base: failed to parse SSE chunk",
				"provider", a.cfg.Name,
				"error", err,
				"data", data,
			)
			continue
		}

		// Emit a start event on the first chunk that carries a model name.
		if !sentStart {
			modelName := chunk.Model
			if modelName == "" {
				modelName = a.cfg.Name
			}
			if err := a.send(ctx, eventChan, protocol.UnifiedStreamEvent{
				Type:  protocol.EventStart,
				Model: modelName,
			}); err != nil {
				return
			}
			sentStart = true
		}

		// Handle token usage if present (typically on the final chunk).
		if chunk.Usage != nil {
			// Requirements: 6.6, 7.6, 8.6
			if err := a.send(ctx, eventChan, protocol.UnifiedStreamEvent{
				Type: protocol.EventToken,
				Usage: &protocol.Usage{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
					TotalTokens:      chunk.Usage.TotalTokens,
				},
			}); err != nil {
				return
			}
		}

		// Process choices.
		for _, choice := range chunk.Choices {
			// Accumulate tool call deltas.
			if len(choice.Delta.ToolCalls) > 0 {
				a.accumulateToolCalls(choice.Delta.ToolCalls, toolCallState)
			}

			// Emit content token events.
			if choice.Delta.Content != "" {
				if err := a.send(ctx, eventChan, protocol.UnifiedStreamEvent{
					Type:    protocol.EventToken,
					Content: choice.Delta.Content,
				}); err != nil {
					return
				}
			}

			// Emit stop event when finish_reason is set.
			if choice.FinishReason != nil {
				finishReason := mapFinishReason(*choice.FinishReason)

				// If we finished with tool calls, flush them first.
				if finishReason == protocol.FinishReasonToolCalls {
					a.flushToolCalls(ctx, toolCallState, eventChan)
				}

				if err := a.send(ctx, eventChan, protocol.UnifiedStreamEvent{
					Type:         protocol.EventStop,
					FinishReason: finishReason,
				}); err != nil {
					return
				}
			}
		}
	}

	// If scanner stopped due to an error (not EOF), emit an error event.
	if err := scanner.Err(); err != nil {
		a.sendError(ctx, eventChan, fmt.Errorf("openai_base: scanner error: %w", err))
	}
}

// accumulateToolCalls merges incremental tool call deltas into toolCallState.
// OpenAI streams tool calls as: first chunk has id+name, subsequent chunks
// have more arguments JSON.
func (a *OpenAIAdapter) accumulateToolCalls(
	deltas []openAIToolCall,
	state map[int]*protocol.ToolCall,
) {
	for _, delta := range deltas {
		idx := delta.Index
		if _, ok := state[idx]; !ok {
			state[idx] = &protocol.ToolCall{}
		}
		tc := state[idx]
		if delta.ID != "" {
			tc.ID = delta.ID
		}
		if delta.Function.Name != "" {
			tc.Name = delta.Function.Name
		}
		tc.Args += delta.Function.Arguments
	}
}

// flushToolCalls converts accumulated tool call state into a single EventToolCall
// and sends it on the channel.
func (a *OpenAIAdapter) flushToolCalls(
	ctx context.Context,
	state map[int]*protocol.ToolCall,
	eventChan chan<- protocol.UnifiedStreamEvent,
) {
	if len(state) == 0 {
		return
	}

	// Build an ordered slice from the index map.
	calls := make([]protocol.ToolCall, len(state))
	for idx, tc := range state {
		if idx < len(calls) {
			calls[idx] = *tc
		}
	}

	_ = a.send(ctx, eventChan, protocol.UnifiedStreamEvent{
		Type:      protocol.EventToolCall,
		ToolCalls: calls,
	})
}

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

// parseErrorResponse reads an error HTTP response body and returns an error
// with the provider name, HTTP status code, and the API error message.
// Requirements: 4.6, 16
func (a *OpenAIAdapter) parseErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var apiErr openAIError
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Message != "" {
		return fmt.Errorf("openai_base [%s]: HTTP %d – %s (type: %s)",
			a.cfg.Name, resp.StatusCode, apiErr.Error.Message, apiErr.Error.Type)
	}

	return fmt.Errorf("openai_base [%s]: HTTP %d – %s",
		a.cfg.Name, resp.StatusCode, strings.TrimSpace(string(body)))
}

// send writes an event to eventChan, respecting context cancellation.
// Returns a non-nil error only when the context has been cancelled.
func (a *OpenAIAdapter) send(
	ctx context.Context,
	eventChan chan<- protocol.UnifiedStreamEvent,
	event protocol.UnifiedStreamEvent,
) error {
	select {
	case eventChan <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// sendError attempts to send an error event. It is a best-effort operation;
// failures (e.g. channel full or context done) are silently discarded.
func (a *OpenAIAdapter) sendError(
	ctx context.Context,
	eventChan chan<- protocol.UnifiedStreamEvent,
	err error,
) {
	select {
	case eventChan <- protocol.UnifiedStreamEvent{
		Type:  protocol.EventError,
		Error: err,
	}:
	case <-ctx.Done():
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mapFinishReason converts an OpenAI finish_reason string to the unified
// protocol constant.
func mapFinishReason(reason string) string {
	switch reason {
	case "stop":
		return protocol.FinishReasonStop
	case "length":
		return protocol.FinishReasonLength
	case "tool_calls":
		return protocol.FinishReasonToolCalls
	default:
		return protocol.FinishReasonStop
	}
}
