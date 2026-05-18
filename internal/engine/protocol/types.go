package protocol

// UnifiedChatRequest represents a normalized chat request across all providers.
type UnifiedChatRequest struct {
	Model       string
	System      string
	Messages    []Message
	Temperature float32
	// TopP is the nucleus-sampling probability mass (0.0–1.0).
	// A value of 0 means the field was not set by the caller.
	TopP        float32
	MaxTokens   int
	Tools       []Tool
	Stream      bool
}

// Message represents a single chat message.
type Message struct {
	Role    string
	Content string // Simplified content for now. May be extended for multi-modal.
}

// Tool represents a tool that the model can call.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
}

// UnifiedStreamEvent represents a normalized streaming event from any provider.
type UnifiedStreamEvent struct {
	Type         EventType
	// Model is the upstream model identifier, populated on EventStart events.
	// Downstream formatters should use this value when emitting model fields.
	Model        string
	Content      string
	Thinking     string
	ToolCalls    []ToolCall
	FinishReason string
	Usage        *Usage
	Error        error
}

// ToolCall represents a specific tool invocation by the model.
type ToolCall struct {
	ID       string
	Name     string
	Function string
	Args     string
}

// Usage tracking for the stream.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}
