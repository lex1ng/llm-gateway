package types

// ResponsesRequest represents a request to the OpenAI Responses API.
// This API provides better performance with reasoning models and built-in tools.
type ResponsesRequest struct {
	// --- Basic fields ---
	Model             string   `json:"model"`                         // Model name
	Input             any      `json:"input"`                         // String or []ResponseInputItem
	Instructions      string   `json:"instructions,omitempty"`        // System prompt
	MaxOutputTokens   *int     `json:"max_output_tokens,omitempty"`   // Maximum tokens to generate
	Temperature       *float64 `json:"temperature,omitempty"`         // Sampling temperature (0-2)
	TopP              *float64 `json:"top_p,omitempty"`               // Nucleus sampling parameter
	Stream            bool     `json:"stream,omitempty"`              // Enable streaming response
	Modalities        []string `json:"modalities,omitempty"`          // Output modalities: ["text"], ["text", "audio"]

	// --- Reasoning Models ---
	ReasoningEffort string `json:"reasoning_effort,omitempty"` // "none", "minimal", "low", "medium", "high"

	// --- Tool Calling ---
	Tools      []ResponseTool `json:"tools,omitempty"`       // Available tools/functions
	ToolChoice any            `json:"tool_choice,omitempty"` // "auto", "none", "required", or specific tool

	// --- Conversation Continuity ---
	PreviousResponseID string `json:"previous_response_id,omitempty"` // Continue from previous response

	// --- Routing Control ---
	Provider string `json:"provider,omitempty"` // Force specific provider (overrides routing)

	// --- Dynamic Credentials (BYOK) ---
	Credentials map[string]string `json:"credentials,omitempty"` // Request-level API key override

	// --- Extensions ---
	Extra map[string]any `json:"extra,omitempty"` // Platform-specific fields

	// --- Internal fields (not serialized) ---
	TenantID  string `json:"-"` // Set by auth middleware
	RequestID string `json:"-"` // Set by request middleware
}

// ResponseInputItem represents an input item in the Responses API.
type ResponseInputItem struct {
	Type    string `json:"type"` // "message", "item_reference"
	ID      string `json:"id,omitempty"`
	Role    string `json:"role,omitempty"`    // "user", "assistant", "system"
	Content any    `json:"content,omitempty"` // String or []ContentPart
}

// ResponseTool represents a tool in the Responses API.
// Supports both function tools and built-in tools (web_search, code_interpreter, etc.)
type ResponseTool struct {
	Type        string        `json:"type"` // "function", "web_search", "code_interpreter", "file_search"
	Function    *ToolFunction `json:"function,omitempty"`
	WebSearch   *WebSearchTool `json:"web_search,omitempty"`
	FileSearch  *FileSearchTool `json:"file_search,omitempty"`
}

// WebSearchTool configures web search built-in tool.
type WebSearchTool struct {
	Type          string `json:"type,omitempty"` // "web_search_preview"
	SearchContext string `json:"search_context_size,omitempty"` // "low", "medium", "high"
}

// FileSearchTool configures file search built-in tool.
type FileSearchTool struct {
	VectorStoreIDs []string `json:"vector_store_ids,omitempty"`
	MaxResults     int      `json:"max_num_results,omitempty"`
}

// ResponsesResponse represents a response from the Responses API.
type ResponsesResponse struct {
	ID        string              `json:"id"`
	Object    string              `json:"object"` // "response"
	CreatedAt int64               `json:"created_at"`
	Status    string              `json:"status"` // "in_progress", "completed", "incomplete", "failed"
	Model     string              `json:"model"`
	Output    []ResponseOutputItem `json:"output"`
	Usage     *ResponsesUsage     `json:"usage,omitempty"`
	Error     *ResponseError      `json:"error,omitempty"`

	// Gateway extensions
	Provider  string `json:"provider,omitempty"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
}

// ResponseOutputItem represents an output item from the Responses API.
type ResponseOutputItem struct {
	Type    string               `json:"type"` // "message", "function_call", "function_call_output", "web_search_call", "reasoning"
	ID      string               `json:"id,omitempty"`
	Role    string               `json:"role,omitempty"` // "assistant"
	Status  string               `json:"status,omitempty"`
	Content []ResponseContentPart `json:"content,omitempty"`

	// For function calls
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Output    string `json:"output,omitempty"`

	// For reasoning
	Summary []ResponseContentPart `json:"summary,omitempty"`
}

// ResponseContentPart represents a content part in the response.
type ResponseContentPart struct {
	Type        string `json:"type"` // "output_text", "refusal", "input_text", "input_audio"
	Text        string `json:"text,omitempty"`
	Refusal     string `json:"refusal,omitempty"`
	Annotations []any  `json:"annotations,omitempty"` // For web search citations
}

// ResponsesUsage represents token usage for Responses API.
type ResponsesUsage struct {
	InputTokens         int                   `json:"input_tokens"`
	OutputTokens        int                   `json:"output_tokens"`
	TotalTokens         int                   `json:"total_tokens,omitempty"`
	InputTokensDetails  *InputTokensDetails   `json:"input_tokens_details,omitempty"`
	OutputTokensDetails *OutputTokensDetails  `json:"output_tokens_details,omitempty"`
}

// InputTokensDetails provides detailed input token breakdown.
type InputTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

// OutputTokensDetails provides detailed output token breakdown.
type OutputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

// ResponseError represents an error from the Responses API.
type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// --- Streaming Types for Responses API ---

// ResponsesStreamEventType defines the type of Responses API streaming event.
type ResponsesStreamEventType string

const (
	ResponsesEventCreated         ResponsesStreamEventType = "response.created"
	ResponsesEventInProgress      ResponsesStreamEventType = "response.in_progress"
	ResponsesEventCompleted       ResponsesStreamEventType = "response.completed"
	ResponsesEventFailed          ResponsesStreamEventType = "response.failed"
	ResponsesEventOutputItemAdded ResponsesStreamEventType = "response.output_item.added"
	ResponsesEventOutputItemDone  ResponsesStreamEventType = "response.output_item.done"
	ResponsesEventContentDelta    ResponsesStreamEventType = "response.content_part.delta"
	ResponsesEventContentDone     ResponsesStreamEventType = "response.content_part.done"
	ResponsesEventDone            ResponsesStreamEventType = "response.done"
	ResponsesEventError           ResponsesStreamEventType = "error"
)

// ResponsesStreamEvent represents a streaming event from the Responses API.
type ResponsesStreamEvent struct {
	Type         ResponsesStreamEventType `json:"type"`
	Response     *ResponsesResponse       `json:"response,omitempty"`
	OutputItem   *ResponseOutputItem      `json:"item,omitempty"`
	ContentIndex int                      `json:"content_index,omitempty"`
	Delta        *ResponseContentDelta    `json:"delta,omitempty"`
	Error        string                   `json:"error,omitempty"`
}

// ResponseContentDelta represents a content delta in streaming.
type ResponseContentDelta struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// NewResponsesContentDeltaEvent creates a content delta event for Responses API.
func NewResponsesContentDeltaEvent(text string) ResponsesStreamEvent {
	return ResponsesStreamEvent{
		Type: ResponsesEventContentDelta,
		Delta: &ResponseContentDelta{
			Type: "text_delta",
			Text: text,
		},
	}
}

// NewResponsesDoneEvent creates a done event for Responses API.
func NewResponsesDoneEvent(response *ResponsesResponse) ResponsesStreamEvent {
	return ResponsesStreamEvent{
		Type:     ResponsesEventDone,
		Response: response,
	}
}

// NewResponsesErrorEvent creates an error event for Responses API.
func NewResponsesErrorEvent(err string) ResponsesStreamEvent {
	return ResponsesStreamEvent{
		Type:  ResponsesEventError,
		Error: err,
	}
}
