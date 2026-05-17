package protocol

// UnifiedChatRequest represents a normalized chat request across all providers.
type UnifiedChatRequest struct {
	Model       string
	System      string
	Messages    []Message
	Temperature float32
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
