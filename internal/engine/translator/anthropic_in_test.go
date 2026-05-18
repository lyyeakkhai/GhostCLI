package translator

import (
	"log/slog"
	"strings"
	"testing"

	"ghostcli/internal/engine/protocol"
)

func TestAnthropicInParser_Parse_ValidRequest(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *protocol.UnifiedChatRequest
	}{
		{
			name: "simple string content",
			input: `{
				"model": "claude-3-5-sonnet-20241022",
				"max_tokens": 1024,
				"messages": [
					{"role": "user", "content": "Hello, Claude!"}
				]
			}`,
			expected: &protocol.UnifiedChatRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 1024,
				Messages: []protocol.Message{
					{Role: "user", Content: "Hello, Claude!"},
				},
			},
		},
		{
			name: "with system prompt as string",
			input: `{
				"model": "claude-3-5-sonnet-20241022",
				"max_tokens": 1024,
				"system": "You are a helpful assistant.",
				"messages": [
					{"role": "user", "content": "Hello!"}
				]
			}`,
			expected: &protocol.UnifiedChatRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 1024,
				System:    "You are a helpful assistant.",
				Messages: []protocol.Message{
					{Role: "user", Content: "Hello!"},
				},
			},
		},
		{
			name: "with system prompt as content blocks",
			input: `{
				"model": "claude-3-5-sonnet-20241022",
				"max_tokens": 1024,
				"system": [
					{"type": "text", "text": "You are a helpful assistant."},
					{"type": "text", "text": "Always be polite."}
				],
				"messages": [
					{"role": "user", "content": "Hello!"}
				]
			}`,
			expected: &protocol.UnifiedChatRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 1024,
				System:    "You are a helpful assistant.\nAlways be polite.",
				Messages: []protocol.Message{
					{Role: "user", Content: "Hello!"},
				},
			},
		},
		{
			name: "with temperature and stream",
			input: `{
				"model": "claude-3-5-sonnet-20241022",
				"max_tokens": 2048,
				"temperature": 0.7,
				"stream": true,
				"messages": [
					{"role": "user", "content": "Write a poem"}
				]
			}`,
			expected: &protocol.UnifiedChatRequest{
				Model:       "claude-3-5-sonnet-20241022",
				MaxTokens:   2048,
				Temperature: 0.7,
				Stream:      true,
				Messages: []protocol.Message{
					{Role: "user", Content: "Write a poem"},
				},
			},
		},
		{
			name: "with content blocks array",
			input: `{
				"model": "claude-3-5-sonnet-20241022",
				"max_tokens": 1024,
				"messages": [
					{
						"role": "user",
						"content": [
							{"type": "text", "text": "What is the weather?"}
						]
					}
				]
			}`,
			expected: &protocol.UnifiedChatRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 1024,
				Messages: []protocol.Message{
					{Role: "user", Content: "What is the weather?"},
				},
			},
		},
		{
			name: "with multiple messages",
			input: `{
				"model": "claude-3-5-sonnet-20241022",
				"max_tokens": 1024,
				"messages": [
					{"role": "user", "content": "Hello!"},
					{"role": "assistant", "content": "Hi there!"},
					{"role": "user", "content": "How are you?"}
				]
			}`,
			expected: &protocol.UnifiedChatRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 1024,
				Messages: []protocol.Message{
					{Role: "user", Content: "Hello!"},
					{Role: "assistant", Content: "Hi there!"},
					{Role: "user", Content: "How are you?"},
				},
			},
		},
		{
			name: "with tools",
			input: `{
				"model": "claude-3-5-sonnet-20241022",
				"max_tokens": 1024,
				"messages": [
					{"role": "user", "content": "What's the weather?"}
				],
				"tools": [
					{
						"name": "get_weather",
						"description": "Get the current weather",
						"input_schema": {
							"type": "object",
							"properties": {
								"location": {"type": "string"}
							}
						}
					}
				]
			}`,
			expected: &protocol.UnifiedChatRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 1024,
				Messages: []protocol.Message{
					{Role: "user", Content: "What's the weather?"},
				},
				Tools: []protocol.Tool{
					{
						Name:        "get_weather",
						Description: "Get the current weather",
						InputSchema: map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"location": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewAnthropicInParser(slog.Default())
			got, err := parser.Parse(strings.NewReader(tt.input))

			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			// Compare fields
			if got.Model != tt.expected.Model {
				t.Errorf("Model = %v, want %v", got.Model, tt.expected.Model)
			}
			if got.MaxTokens != tt.expected.MaxTokens {
				t.Errorf("MaxTokens = %v, want %v", got.MaxTokens, tt.expected.MaxTokens)
			}
			if got.Temperature != tt.expected.Temperature {
				t.Errorf("Temperature = %v, want %v", got.Temperature, tt.expected.Temperature)
			}
			if got.Stream != tt.expected.Stream {
				t.Errorf("Stream = %v, want %v", got.Stream, tt.expected.Stream)
			}
			if got.System != tt.expected.System {
				t.Errorf("System = %v, want %v", got.System, tt.expected.System)
			}

			// Compare messages
			if len(got.Messages) != len(tt.expected.Messages) {
				t.Fatalf("Messages length = %v, want %v", len(got.Messages), len(tt.expected.Messages))
			}
			for i := range got.Messages {
				if got.Messages[i].Role != tt.expected.Messages[i].Role {
					t.Errorf("Message[%d].Role = %v, want %v", i, got.Messages[i].Role, tt.expected.Messages[i].Role)
				}
				if got.Messages[i].Content != tt.expected.Messages[i].Content {
					t.Errorf("Message[%d].Content = %v, want %v", i, got.Messages[i].Content, tt.expected.Messages[i].Content)
				}
			}

			// Compare tools
			if len(got.Tools) != len(tt.expected.Tools) {
				t.Fatalf("Tools length = %v, want %v", len(got.Tools), len(tt.expected.Tools))
			}
			for i := range got.Tools {
				if got.Tools[i].Name != tt.expected.Tools[i].Name {
					t.Errorf("Tool[%d].Name = %v, want %v", i, got.Tools[i].Name, tt.expected.Tools[i].Name)
				}
				if got.Tools[i].Description != tt.expected.Tools[i].Description {
					t.Errorf("Tool[%d].Description = %v, want %v", i, got.Tools[i].Description, tt.expected.Tools[i].Description)
				}
			}
		})
	}
}

func TestAnthropicInParser_Parse_InvalidJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "malformed JSON",
			input: `{"model": "claude-3-5-sonnet-20241022", "max_tokens": 1024`,
		},
		{
			name:  "invalid JSON syntax",
			input: `{model: claude-3-5-sonnet-20241022}`,
		},
		{
			name:  "empty string",
			input: ``,
		},
		{
			name:  "not JSON",
			input: `this is not json`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewAnthropicInParser(slog.Default())
			_, err := parser.Parse(strings.NewReader(tt.input))

			if err == nil {
				t.Error("Parse() expected error, got nil")
			}
		})
	}
}

func TestAnthropicInParser_Parse_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "missing model",
			input: `{
				"max_tokens": 1024,
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
		},
		{
			name: "missing messages",
			input: `{
				"model": "claude-3-5-sonnet-20241022",
				"max_tokens": 1024
			}`,
		},
		{
			name: "empty messages array",
			input: `{
				"model": "claude-3-5-sonnet-20241022",
				"max_tokens": 1024,
				"messages": []
			}`,
		},
		{
			name: "missing max_tokens",
			input: `{
				"model": "claude-3-5-sonnet-20241022",
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewAnthropicInParser(slog.Default())
			_, err := parser.Parse(strings.NewReader(tt.input))

			if err == nil {
				t.Error("Parse() expected error for missing required field, got nil")
			}
		})
	}
}

func TestAnthropicInParser_Parse_MessageWithToolUse(t *testing.T) {
	input := `{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": [
			{
				"role": "assistant",
				"content": [
					{"type": "text", "text": "Let me check the weather."},
					{
						"type": "tool_use",
						"id": "toolu_123",
						"name": "get_weather",
						"input": {"location": "San Francisco"}
					}
				]
			}
		]
	}`

	parser := NewAnthropicInParser(slog.Default())
	got, err := parser.Parse(strings.NewReader(input))

	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(got.Messages) != 1 {
		t.Fatalf("Messages length = %v, want 1", len(got.Messages))
	}

	// The content should include both text and tool_use representation
	expectedContent := "Let me check the weather.\n[tool_use: get_weather (id: toolu_123)]"
	if got.Messages[0].Content != expectedContent {
		t.Errorf("Message content = %v, want %v", got.Messages[0].Content, expectedContent)
	}
}

func TestAnthropicInParser_Parse_MessageWithToolResult(t *testing.T) {
	input := `{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "toolu_123",
						"content": "The weather is sunny, 72°F"
					}
				]
			}
		]
	}`

	parser := NewAnthropicInParser(slog.Default())
	got, err := parser.Parse(strings.NewReader(input))

	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(got.Messages) != 1 {
		t.Fatalf("Messages length = %v, want 1", len(got.Messages))
	}

	expectedContent := "The weather is sunny, 72°F"
	if got.Messages[0].Content != expectedContent {
		t.Errorf("Message content = %v, want %v", got.Messages[0].Content, expectedContent)
	}
}

func TestAnthropicInParser_NormalizeSystem(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string format",
			input:    "You are a helpful assistant.",
			expected: "You are a helpful assistant.",
		},
		{
			name: "content blocks array",
			input: []interface{}{
				map[string]interface{}{"type": "text", "text": "First instruction."},
				map[string]interface{}{"type": "text", "text": "Second instruction."},
			},
			expected: "First instruction.\nSecond instruction.",
		},
		{
			name:     "nil input",
			input:    nil,
			expected: "",
		},
		{
			name: "empty array",
			input: []interface{}{},
			expected: "",
		},
		{
			name: "array with non-text blocks",
			input: []interface{}{
				map[string]interface{}{"type": "image", "source": "..."},
			},
			expected: "",
		},
	}

	parser := NewAnthropicInParser(slog.Default())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.normalizeSystem(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeSystem() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAnthropicInParser_ExtractContent(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
		wantErr  bool
	}{
		{
			name:     "simple string",
			input:    "Hello, world!",
			expected: "Hello, world!",
			wantErr:  false,
		},
		{
			name: "text block array",
			input: []interface{}{
				map[string]interface{}{"type": "text", "text": "Hello"},
				map[string]interface{}{"type": "text", "text": "World"},
			},
			expected: "Hello\nWorld",
			wantErr:  false,
		},
		{
			name:     "nil content",
			input:    nil,
			expected: "",
			wantErr:  true,
		},
		{
			name: "mixed content blocks",
			input: []interface{}{
				map[string]interface{}{"type": "text", "text": "Check weather"},
				map[string]interface{}{
					"type":  "tool_use",
					"id":    "tool_123",
					"name":  "get_weather",
					"input": map[string]interface{}{"location": "NYC"},
				},
			},
			expected: "Check weather\n[tool_use: get_weather (id: tool_123)]",
			wantErr:  false,
		},
	}

	parser := NewAnthropicInParser(slog.Default())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.extractContent(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("extractContent() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("extractContent() error = %v", err)
			}

			if got != tt.expected {
				t.Errorf("extractContent() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAnthropicInParser_ConvertTools(t *testing.T) {
	input := []AnthropicTool{
		{
			Name:        "get_weather",
			Description: "Get current weather",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{"type": "string"},
				},
			},
		},
		{
			Name:        "search",
			Description: "Search the web",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
				},
			},
		},
	}

	parser := NewAnthropicInParser(slog.Default())
	got := parser.convertTools(input)

	if len(got) != 2 {
		t.Fatalf("convertTools() length = %v, want 2", len(got))
	}

	if got[0].Name != "get_weather" {
		t.Errorf("Tool[0].Name = %v, want get_weather", got[0].Name)
	}
	if got[1].Name != "search" {
		t.Errorf("Tool[1].Name = %v, want search", got[1].Name)
	}
}
