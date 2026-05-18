// Package base provides reusable provider adapter implementations
// for common API protocol patterns (OpenAI-compatible, Anthropic-native, AWS EventStream).
package base

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"ghostcli/internal/engine/protocol"
)

// AWSAdapterConfig holds all configuration required by the AWS EventStream adapter.
type AWSAdapterConfig struct {
	// Name is the provider identifier (e.g., "kiro", "bedrock").
	Name string

	// BaseURL is the root URL of the provider API endpoint (e.g., "https://kiro.aws.amazon.com").
	BaseURL string

	// ChatPath is the path appended to BaseURL for chat completions (e.g., "/v1/messages").
	ChatPath string

	// APIKey is the credential used to authenticate requests.
	// Placed in the Authorization header as "Bearer <APIKey>".
	APIKey string

	// ModelMap translates Anthropic model identifiers to provider-specific ones.
	ModelMap map[string]string

	// Logger is the structured logger; defaults to slog.Default() if nil.
	Logger *slog.Logger
}

// AWSAdapter is a provider adapter for services that use the AWS EventStream
// binary framing protocol (e.g., Kiro, Amazon Bedrock).
//
// AWS EventStream encodes each message as a binary frame with:
//   - 4-byte total byte length
//   - 4-byte header byte length
//   - 4-byte prelude CRC (crc32 of the first 8 bytes)
//   - N bytes of headers (key-value pairs)
//   - M bytes of payload
//   - 4-byte message CRC (crc32 of everything before this field)
//
// The adapter translates UnifiedChatRequests into Anthropic-format JSON bodies,
// sends them to the configured endpoint, and decodes the EventStream frames
// back into UnifiedStreamEvent values emitted on a channel.
type AWSAdapter struct {
	config AWSAdapterConfig
	client *http.Client
	logger *slog.Logger
}

// NewAWSAdapter creates an AWSAdapter with the supplied configuration.
// A default HTTP client with a 5-minute timeout and connection pooling is used.
func NewAWSAdapter(cfg AWSAdapterConfig) *AWSAdapter {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &AWSAdapter{
		config: cfg,
		client: &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger: logger,
	}
}

// Name returns the provider identifier.
func (a *AWSAdapter) Name() string {
	return a.config.Name
}

// SupportsTools returns false; AWS EventStream providers do not support
// function/tool calling through this base adapter.
func (a *AWSAdapter) SupportsTools() bool {
	return false
}

// SupportsThinking returns false; AWS EventStream providers do not expose
// extended thinking blocks through this base adapter.
func (a *AWSAdapter) SupportsThinking() bool {
	return false
}

// MapModel translates an Anthropic model name to the provider-specific identifier.
// If no mapping is found the original name is returned unchanged.
func (a *AWSAdapter) MapModel(anthropicModel string) string {
	if a.config.ModelMap != nil {
		if mapped, ok := a.config.ModelMap[anthropicModel]; ok {
			a.logger.Debug("model mapped",
				"provider", a.config.Name,
				"from", anthropicModel,
				"to", mapped,
			)
			return mapped
		}
	}
	return anthropicModel
}

// StreamChat initiates a streaming chat request to the provider.
//
// The method:
//  1. Converts the UnifiedChatRequest to an Anthropic-format JSON body.
//  2. Sends an HTTP POST request with appropriate AWS/Anthropic headers.
//  3. Launches a goroutine that decodes EventStream frames and emits
//     UnifiedStreamEvent values on the returned channel.
//
// The channel is closed when streaming ends, an error occurs, or the context
// is cancelled.
func (a *AWSAdapter) StreamChat(
	ctx context.Context,
	req *protocol.UnifiedChatRequest,
) (<-chan protocol.UnifiedStreamEvent, error) {
	// Build the provider request body.
	body, err := a.buildRequestBody(req)
	if err != nil {
		return nil, fmt.Errorf("aws adapter: build request: %w", err)
	}

	// Build the HTTP request.
	endpoint := strings.TrimRight(a.config.BaseURL, "/") + a.chatPath()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("aws adapter: create http request: %w", err)
	}

	a.setHeaders(httpReq)

	a.logger.Debug("sending request to aws provider",
		"provider", a.config.Name,
		"endpoint", endpoint,
		"model", req.Model,
	)

	// Send the request.
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("aws adapter: http request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, a.handleErrorResponse(resp)
	}

	// Start streaming goroutine.
	eventChan := make(chan protocol.UnifiedStreamEvent, 16)
	go a.streamEventStream(ctx, resp.Body, eventChan)

	return eventChan, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// chatPath returns the configured chat path or the default Anthropic messages path.
func (a *AWSAdapter) chatPath() string {
	if a.config.ChatPath != "" {
		return a.config.ChatPath
	}
	return "/v1/messages"
}

// setHeaders adds the required HTTP headers for the AWS/Anthropic endpoint.
func (a *AWSAdapter) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.amazon.eventstream")
	req.Header.Set("Authorization", "Bearer "+a.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("X-Stainless-Helper-Version", "ghostcli")
}

// awsRequest is the Anthropic-format request body sent to the provider.
type awsRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	Stream    bool          `json:"stream"`
	System    string        `json:"system,omitempty"`
	Messages  []awsMessage  `json:"messages"`
	Tools     []awsTool     `json:"tools,omitempty"`

	Temperature float32 `json:"temperature,omitempty"`
	TopP        float32 `json:"top_p,omitempty"`
}

type awsMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type awsTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// buildRequestBody converts a UnifiedChatRequest into the JSON payload expected
// by the Anthropic-format endpoint used by AWS EventStream providers.
func (a *AWSAdapter) buildRequestBody(req *protocol.UnifiedChatRequest) ([]byte, error) {
	model := a.MapModel(req.Model)

	messages := make([]awsMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, awsMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	var tools []awsTool
	for _, t := range req.Tools {
		tools = append(tools, awsTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	payload := awsRequest{
		Model:       model,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
		System:      req.System,
		Messages:    messages,
		Tools:       tools,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	return data, nil
}

// handleErrorResponse reads a non-200 HTTP response and returns a descriptive error.
func (a *AWSAdapter) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("aws provider %s returned HTTP %d: %s",
		a.config.Name, resp.StatusCode, strings.TrimSpace(string(body)))
}

// ---------------------------------------------------------------------------
// AWS EventStream binary framing
// ---------------------------------------------------------------------------
//
// Each EventStream message has the following wire format (all multi-byte
// integers are big-endian):
//
//   ┌────────────────────────────────────────────────────────────┐
//   │ total_byte_length  (4 bytes)                               │
//   │ headers_byte_length (4 bytes)                              │
//   │ prelude_crc         (4 bytes, CRC32 of previous 8 bytes)   │
//   │ headers             (headers_byte_length bytes)            │
//   │ payload             (total − 16 − headers_byte_length bytes)│
//   │ message_crc         (4 bytes, CRC32 of all preceding bytes)│
//   └────────────────────────────────────────────────────────────┘
//
// Header fields are encoded as:
//   name_length (1 byte) | name (N bytes) | type (1 byte) | value_length (2 bytes) | value

const (
	eventStreamPreludeLen = 8  // total_byte_length + headers_byte_length
	eventStreamCRCLen     = 4
	eventStreamMinLen     = eventStreamPreludeLen + eventStreamCRCLen*2 // prelude + 2 CRCs
)

// eventStreamFrame holds the decoded contents of a single EventStream message.
type eventStreamFrame struct {
	headers map[string]string
	payload []byte
}

// decodeEventStreamFrame reads exactly one EventStream frame from r.
// It validates both CRC fields and returns an error if they do not match.
func decodeEventStreamFrame(r io.Reader) (*eventStreamFrame, error) {
	// Read the 12-byte prelude (8 bytes of lengths + 4-byte CRC).
	prelude := make([]byte, eventStreamPreludeLen+eventStreamCRCLen)
	if _, err := io.ReadFull(r, prelude); err != nil {
		return nil, fmt.Errorf("read prelude: %w", err)
	}

	totalLen := binary.BigEndian.Uint32(prelude[0:4])
	headersLen := binary.BigEndian.Uint32(prelude[4:8])
	preludeCRC := binary.BigEndian.Uint32(prelude[8:12])

	// Validate prelude CRC.
	computed := crc32.ChecksumIEEE(prelude[:8])
	if computed != preludeCRC {
		return nil, fmt.Errorf("eventstream prelude crc mismatch: got %08x, want %08x", preludeCRC, computed)
	}

	if totalLen < uint32(eventStreamMinLen) {
		return nil, fmt.Errorf("eventstream frame too small: %d bytes", totalLen)
	}

	// Read the remainder of the frame (headers + payload + message CRC).
	remaining := int(totalLen) - len(prelude)
	rest := make([]byte, remaining)
	if _, err := io.ReadFull(r, rest); err != nil {
		return nil, fmt.Errorf("read frame body: %w", err)
	}

	// Validate message CRC (covers everything except the last 4 bytes).
	msgCRC := binary.BigEndian.Uint32(rest[len(rest)-4:])
	fullMsg := append(prelude, rest[:len(rest)-4]...)
	computedMsg := crc32.ChecksumIEEE(fullMsg)
	if computedMsg != msgCRC {
		return nil, fmt.Errorf("eventstream message crc mismatch: got %08x, want %08x", msgCRC, computedMsg)
	}

	// Decode headers.
	headerBytes := rest[:headersLen]
	headers, err := decodeEventStreamHeaders(headerBytes)
	if err != nil {
		return nil, fmt.Errorf("decode headers: %w", err)
	}

	// Extract payload (between headers and message CRC).
	payloadEnd := len(rest) - 4
	payload := rest[headersLen:payloadEnd]

	return &eventStreamFrame{
		headers: headers,
		payload: payload,
	}, nil
}

// decodeEventStreamHeaders parses the binary header section of an EventStream frame.
// Each header is: name_len(1) | name(N) | type(1) | value_len(2) | value(M).
// Only string-type headers (type == 7) are decoded; others are skipped.
func decodeEventStreamHeaders(data []byte) (map[string]string, error) {
	headers := make(map[string]string)
	pos := 0

	for pos < len(data) {
		// Name length.
		if pos >= len(data) {
			break
		}
		nameLen := int(data[pos])
		pos++

		if pos+nameLen > len(data) {
			return nil, fmt.Errorf("header name truncated at offset %d", pos)
		}
		name := string(data[pos : pos+nameLen])
		pos += nameLen

		// Value type.
		if pos >= len(data) {
			return nil, fmt.Errorf("header type missing for %q", name)
		}
		headerType := data[pos]
		pos++

		// Value length (2 bytes).
		if pos+2 > len(data) {
			return nil, fmt.Errorf("header value length truncated for %q", name)
		}
		valueLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2

		if pos+valueLen > len(data) {
			return nil, fmt.Errorf("header value truncated for %q", name)
		}
		value := data[pos : pos+valueLen]
		pos += valueLen

		// Type 7 == string.
		if headerType == 7 {
			headers[name] = string(value)
		}
	}

	return headers, nil
}

// ---------------------------------------------------------------------------
// Streaming goroutine
// ---------------------------------------------------------------------------

// streamEventStream reads frames from body and converts them to UnifiedStreamEvent
// values, emitting each on eventChan. It closes the channel when done.
func (a *AWSAdapter) streamEventStream(
	ctx context.Context,
	body io.ReadCloser,
	eventChan chan<- protocol.UnifiedStreamEvent,
) {
	defer close(eventChan)
	defer body.Close()

	for {
		// Check for cancellation before each frame.
		select {
		case <-ctx.Done():
			a.sendError(ctx, eventChan, fmt.Errorf("context cancelled: %w", ctx.Err()))
			return
		default:
		}

		frame, err := decodeEventStreamFrame(body)
		if err != nil {
			if err == io.EOF || isEOF(err) {
				return
			}
			a.logger.Error("eventstream decode error",
				"provider", a.config.Name,
				"error", err,
			)
			a.sendError(ctx, eventChan, fmt.Errorf("eventstream decode: %w", err))
			return
		}

		event, done := a.convertFrame(frame)
		if event == nil {
			// Unrecognised frame type – skip.
			continue
		}

		select {
		case eventChan <- *event:
		case <-ctx.Done():
			return
		}

		if done {
			return
		}
	}
}

// sendError emits a single error event on eventChan, respecting context cancellation.
func (a *AWSAdapter) sendError(ctx context.Context, eventChan chan<- protocol.UnifiedStreamEvent, err error) {
	select {
	case eventChan <- protocol.UnifiedStreamEvent{
		Type:  protocol.EventError,
		Error: err,
	}:
	case <-ctx.Done():
	}
}

// isEOF returns true if the error wraps or is io.EOF / io.ErrUnexpectedEOF.
func isEOF(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "EOF") || err == io.ErrUnexpectedEOF
}

// ---------------------------------------------------------------------------
// Frame → UnifiedStreamEvent conversion
// ---------------------------------------------------------------------------

// awsSSEEvent is the minimal Anthropic SSE payload structure used when the
// AWS endpoint wraps Anthropic events inside EventStream frames.
type awsSSEEvent struct {
	Type    string         `json:"type"`
	Index   int            `json:"index"`
	Delta   *awsDelta      `json:"delta,omitempty"`
	Usage   *awsUsage      `json:"usage,omitempty"`
	// message_start embeds the model name here.
	Message *awsMsgWrapper `json:"message,omitempty"`
}

type awsDelta struct {
	Type         string `json:"type"`
	Text         string `json:"text,omitempty"`
	ThinkingText string `json:"thinking,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
}

type awsUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type awsMsgWrapper struct {
	Model string     `json:"model"`
	Usage *awsUsage  `json:"usage,omitempty"`
}

// convertFrame converts a decoded EventStream frame into a UnifiedStreamEvent.
// It returns (nil, false) for frames that should be silently skipped.
// It returns (event, true) for terminal frames that signal end-of-stream.
func (a *AWSAdapter) convertFrame(frame *eventStreamFrame) (*protocol.UnifiedStreamEvent, bool) {
	eventType := frame.headers[":event-type"]
	msgType := frame.headers[":message-type"]

	// Handle explicit error messages from the server.
	if msgType == "exception" || eventType == "modelStreamErrorException" {
		errMsg := extractErrorMessage(frame.payload)
		return &protocol.UnifiedStreamEvent{
			Type:  protocol.EventError,
			Error: fmt.Errorf("provider error: %s", errMsg),
		}, true
	}

	// Frames without a recognised event-type are skipped.
	if eventType == "" {
		return nil, false
	}

	// The payload contains an Anthropic-format SSE event JSON.
	var sseEvt awsSSEEvent
	if err := json.Unmarshal(frame.payload, &sseEvt); err != nil {
		a.logger.Warn("failed to unmarshal eventstream payload",
			"event_type", eventType,
			"error", err,
		)
		return nil, false
	}

	switch sseEvt.Type {
	case "message_start":
		evt := &protocol.UnifiedStreamEvent{
			Type: protocol.EventStart,
		}
		if sseEvt.Message != nil {
			evt.Model = sseEvt.Message.Model
			if sseEvt.Message.Usage != nil {
				evt.Usage = &protocol.Usage{
					PromptTokens: sseEvt.Message.Usage.InputTokens,
				}
			}
		}
		return evt, false

	case "content_block_start":
		// No content yet – just signals the beginning of a block.
		return nil, false

	case "content_block_delta":
		if sseEvt.Delta == nil {
			return nil, false
		}
		switch sseEvt.Delta.Type {
		case "text_delta":
			return &protocol.UnifiedStreamEvent{
				Type:    protocol.EventToken,
				Content: sseEvt.Delta.Text,
			}, false
		case "thinking_delta":
			return &protocol.UnifiedStreamEvent{
				Type:     protocol.EventThinking,
				Thinking: sseEvt.Delta.ThinkingText,
			}, false
		}
		return nil, false

	case "content_block_stop":
		return nil, false

	case "message_delta":
		var usage *protocol.Usage
		if sseEvt.Usage != nil {
			usage = &protocol.Usage{
				CompletionTokens: sseEvt.Usage.OutputTokens,
			}
		}
		finishReason := ""
		if sseEvt.Delta != nil {
			finishReason = awsMapFinishReason(sseEvt.Delta.StopReason)
		}
		return &protocol.UnifiedStreamEvent{
			Type:         protocol.EventStop,
			FinishReason: finishReason,
			Usage:        usage,
		}, false

	case "message_stop":
		return &protocol.UnifiedStreamEvent{
			Type:         protocol.EventStop,
			FinishReason: protocol.FinishReasonStop,
		}, true

	case "error":
		errMsg := extractErrorMessage(frame.payload)
		return &protocol.UnifiedStreamEvent{
			Type:  protocol.EventError,
			Error: fmt.Errorf("stream error: %s", errMsg),
		}, true

	default:
		return nil, false
	}
}

// awsMapFinishReason converts Anthropic stop reasons to the canonical protocol constants.
// It differs from the OpenAI mapFinishReason by preserving unknown stop reasons as-is
// instead of defaulting to "stop", which is more appropriate for AWS providers.
func awsMapFinishReason(reason string) string {
	switch reason {
	case "end_turn":
		return protocol.FinishReasonStop
	case "max_tokens":
		return protocol.FinishReasonLength
	case "tool_use":
		return protocol.FinishReasonToolCalls
	default:
		return reason // preserve unknown reasons (empty string included)
	}
}

// extractErrorMessage attempts to pull a human-readable message from a JSON payload.
func extractErrorMessage(payload []byte) string {
	var obj map[string]interface{}
	if err := json.Unmarshal(payload, &obj); err == nil {
		for _, key := range []string{"message", "error", "errorMessage"} {
			if v, ok := obj[key].(string); ok && v != "" {
				return v
			}
		}
	}
	if len(payload) > 0 {
		return string(payload)
	}
	return "unknown error"
}
