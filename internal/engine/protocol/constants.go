package protocol

// EventType categorizes the stream events.
type EventType string

const (
	EventStart     EventType = "start"
	EventToken     EventType = "token"
	EventThinking  EventType = "thinking"
	EventToolCall  EventType = "tool_call"
	EventStop      EventType = "stop"
	EventError     EventType = "error"
)

// Standardized role names.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// Finish reasons.
const (
	FinishReasonStop       = "stop"
	FinishReasonLength     = "length"
	FinishReasonToolCalls  = "tool_calls"
	FinishReasonError      = "error"
)
