package types

// Tool represents a tool/function available to the model.
// Follows OpenAI function calling format.
type Tool struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a callable function.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"` // JSON Schema
}

// ToolCall represents a tool/function call made by the assistant.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall contains the function name and arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolResult represents the result of a tool call, sent back by the user.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"` // References ToolCall.ID
	Content    string `json:"content"`      // Tool execution result (JSON string)
	IsError    bool   `json:"is_error,omitempty"`
}
