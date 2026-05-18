package translator

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"ghostcli/internal/engine/protocol"
)

// AnthropicInParser converts incoming Anthropic Messages API requests
// into the unified internal format (UnifiedChatRequest).
type AnthropicInParser struct {
	logger *slog.Logger
}

// NewAnthropicInParser creates a new AnthropicIn parser instance.
func NewAnthropicInParser(logger *slog.Logger) *AnthropicInParser {
	return &AnthropicInParser{logger: logger}
}

// AnthropicRequest represents the incoming Anthropic Messages API request format.
type AnthropicRequest struct {
	Model       string      `json:"model"`
	Messages    []AnthropicMessage `json:"messages"`
	System      interface{} `json:"system,omitempty"` // Can be string or array of content blocks
	MaxTokens   int         `json:"max_tokens"`
	Temperature float32     `json:"temperature,omitempty"`
	TopP        float32     `json:"top_p,omitempty"`
	Stream      bool        `json:"stream,omitempty"`
	Tools       []AnthropicTool `json:"tools,omitempty"`
}

// AnthropicMessage represents a message in the Anthropic format.
type AnthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or array of content blocks
}

// AnthropicContentBlock represents a content block in Anthropic format.
type AnthropicContentBlock struct {
	Type string `json:"type"` // "text", "tool_use", "tool_result"
	Text string `json:"text,omitempty"`
	
	// For tool_use blocks
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
	
	// For tool_result blocks
	ToolUseID string `json:"tool_use_id,omitempty"`
}

// AnthropicTool represents a tool definition in Anthropic format.
type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// Parse decodes an Anthropic Messages API request from the provided reader
// and converts it to a UnifiedChatRequest. Uses streaming JSON decoding
// to avoid buffering the entire request body.
func (p *AnthropicInParser) Parse(r io.Reader) (*protocol.UnifiedChatRequest, error) {
	var anthropicReq AnthropicRequest
	
	// Use streaming decoder to avoid buffering entire request
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&anthropicReq); err != nil {
		p.logger.Error("failed to decode anthropic request", "error", err)
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	
	// Validate required fields
	if anthropicReq.Model == "" {
		return nil, fmt.Errorf("missing required field: model")
	}
	if len(anthropicReq.Messages) == 0 {
		return nil, fmt.Errorf("missing required field: messages")
	}
	if anthropicReq.MaxTokens == 0 {
		return nil, fmt.Errorf("missing required field: max_tokens")
	}
	
	// Convert to unified format
	unified := &protocol.UnifiedChatRequest{
		Model:       anthropicReq.Model,
		MaxTokens:   anthropicReq.MaxTokens,
		Temperature: anthropicReq.Temperature,
		TopP:        anthropicReq.TopP,
		Stream:      anthropicReq.Stream,
	}
	
	// Normalize system prompt (handles both string and content block array formats)
	unified.System = p.normalizeSystem(anthropicReq.System)
	
	// Convert messages
	messages, err := p.convertMessages(anthropicReq.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}
	unified.Messages = messages
	
	// Convert tools if present
	if len(anthropicReq.Tools) > 0 {
		unified.Tools = p.convertTools(anthropicReq.Tools)
	}
	
	p.logger.Debug("parsed anthropic request",
		"model", unified.Model,
		"message_count", len(unified.Messages),
		"tool_count", len(unified.Tools),
		"stream", unified.Stream)
	
	return unified, nil
}

// normalizeSystem converts the Anthropic system field to a simple string.
// The system field can be either:
// - A string (simple format)
// - An array of content blocks (complex format)
func (p *AnthropicInParser) normalizeSystem(system interface{}) string {
	if system == nil {
		return ""
	}
	
	switch v := system.(type) {
	case string:
		// Simple string format
		return v
		
	case []interface{}:
		// Array of content blocks - extract text from each block
		var parts []string
		for _, block := range v {
			if m, ok := block.(map[string]interface{}); ok {
				if m["type"] == "text" {
					if text, ok := m["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
		}
		return strings.Join(parts, "\n")
		
	default:
		p.logger.Warn("unexpected system prompt type", "type", fmt.Sprintf("%T", v))
		return ""
	}
}

// convertMessages converts Anthropic messages to unified format.
func (p *AnthropicInParser) convertMessages(messages []AnthropicMessage) ([]protocol.Message, error) {
	unified := make([]protocol.Message, 0, len(messages))
	
	for i, msg := range messages {
		if msg.Role == "" {
			return nil, fmt.Errorf("message %d: missing role", i)
		}
		
		content, err := p.extractContent(msg.Content)
		if err != nil {
			return nil, fmt.Errorf("message %d: %w", i, err)
		}
		
		unified = append(unified, protocol.Message{
			Role:    msg.Role,
			Content: content,
		})
	}
	
	return unified, nil
}

// extractContent extracts text content from various Anthropic content formats.
// Content can be:
// - A string (simple format)
// - An array of content blocks (complex format with text, tool_use, tool_result)
func (p *AnthropicInParser) extractContent(content interface{}) (string, error) {
	if content == nil {
		return "", fmt.Errorf("content is nil")
	}
	
	switch v := content.(type) {
	case string:
		// Simple string format
		return v, nil
		
	case []interface{}:
		// Array of content blocks
		var parts []string
		for _, block := range v {
			m, ok := block.(map[string]interface{})
			if !ok {
				continue
			}
			
			blockType, _ := m["type"].(string)
			switch blockType {
			case "text":
				if text, ok := m["text"].(string); ok {
					parts = append(parts, text)
				}
			case "tool_use":
				// For tool_use blocks, create a representation
				name, _ := m["name"].(string)
				id, _ := m["id"].(string)
				parts = append(parts, fmt.Sprintf("[tool_use: %s (id: %s)]", name, id))
			case "tool_result":
				// For tool_result blocks, extract the content
				if toolContent, ok := m["content"].(string); ok {
					parts = append(parts, toolContent)
				}
			}
		}
		return strings.Join(parts, "\n"), nil
		
	default:
		return "", fmt.Errorf("unexpected content type: %T", v)
	}
}

// convertTools converts Anthropic tool definitions to unified format.
func (p *AnthropicInParser) convertTools(tools []AnthropicTool) []protocol.Tool {
	unified := make([]protocol.Tool, 0, len(tools))
	
	for _, tool := range tools {
		unified = append(unified, protocol.Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	
	return unified
}
