package types

// ChatRequest represents a chat completion request.
// Compatible with OpenAI Chat Completions API format.
type ChatRequest struct {
	// --- Basic fields ---
	Model       string    `json:"model,omitempty"`       // Model name, or empty for tier-based routing
	Messages    []Message `json:"messages"`              // Conversation messages
	MaxTokens   *int      `json:"max_tokens,omitempty"`  // Maximum tokens to generate
	Temperature *float64  `json:"temperature,omitempty"` // Sampling temperature (0-2)
	TopP        *float64  `json:"top_p,omitempty"`       // Nucleus sampling parameter
	Stream      bool      `json:"stream,omitempty"`      // Enable streaming response
	Stop        []string  `json:"stop,omitempty"`        // Stop sequences

	// --- Reasoning Models (o1, o3, gpt-5, etc.) ---
	ReasoningEffort string `json:"reasoning_effort,omitempty"` // "none", "minimal", "low", "medium", "high"

	// --- Tool Calling ---
	Tools      []Tool `json:"tools,omitempty"`       // Available tools/functions
	ToolChoice any    `json:"tool_choice,omitempty"` // "auto", "none", or specific tool

	// --- Routing Control ---
	Provider  string    `json:"provider,omitempty"`   // Force specific provider (overrides routing)
	ModelTier ModelTier `json:"model_tier,omitempty"` // Tier-based routing (small/medium/large)

	// --- Reliability ---
	IdempotencyKey string `json:"idempotency_key,omitempty"` // Idempotency key for deduplication

	// --- Dynamic Credentials (BYOK) ---
	Credentials map[string]string `json:"credentials,omitempty"` // Request-level API key override

	// --- Response Format ---
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"` // JSON mode

	// --- Extensions ---
	Extra map[string]any `json:"extra,omitempty"` // Platform-specific fields

	// --- Internal fields (not serialized) ---
	TenantID  string `json:"-"` // Set by auth middleware
	RequestID string `json:"-"` // Set by request middleware
}

// ResponseFormat specifies the format of the response.
type ResponseFormat struct {
	Type       string      `json:"type"`                  // "text", "json_object", or "json_schema"
	JSONSchema *JSONSchema `json:"json_schema,omitempty"` // Required when type is "json_schema"
}

// JSONSchema defines a JSON Schema for structured output.
type JSONSchema struct {
	Name   string `json:"name"`             // Schema name (required by OpenAI)
	Strict bool   `json:"strict"`           // Enable strict schema adherence
	Schema any    `json:"schema"`           // JSON Schema object
}

// Tool and ToolFunction are defined in tool.go

// EmbedRequest represents an embedding request.
type EmbedRequest struct {
	Model          string   `json:"model,omitempty"`
	Input          []string `json:"input"`                     // Text(s) to embed
	EncodingFormat string   `json:"encoding_format,omitempty"` // "float" or "base64"
	Dimensions     *int     `json:"dimensions,omitempty"`      // Output dimensions (if supported)
	User           string   `json:"user,omitempty"`            // End-user identifier

	// --- Routing Control ---
	Provider string `json:"provider,omitempty"`

	// --- Dynamic Credentials ---
	Credentials map[string]string `json:"credentials,omitempty"`

	// --- Internal ---
	TenantID  string `json:"-"`
	RequestID string `json:"-"`
}

// ImageGenRequest represents an image generation request.
type ImageGenRequest struct {
	Model   string `json:"model,omitempty"`
	Prompt  string `json:"prompt"`
	N       int    `json:"n,omitempty"`        // Number of images to generate
	Size    string `json:"size,omitempty"`     // e.g., "1024x1024"
	Quality string `json:"quality,omitempty"`  // "standard" or "hd"
	Style   string `json:"style,omitempty"`    // "vivid" or "natural"
	Format  string `json:"response_format,omitempty"` // "url" or "b64_json"

	// --- Routing Control ---
	Provider string `json:"provider,omitempty"`

	// --- Dynamic Credentials ---
	Credentials map[string]string `json:"credentials,omitempty"`

	// --- Extensions ---
	Extra map[string]any `json:"extra,omitempty"`

	// --- Internal ---
	TenantID  string `json:"-"`
	RequestID string `json:"-"`
}

// TTSRequest represents a text-to-speech request.
type TTSRequest struct {
	Model          string  `json:"model,omitempty"`
	Input          string  `json:"input"`                      // Text to synthesize
	Voice          string  `json:"voice"`                      // Voice ID
	ResponseFormat string  `json:"response_format,omitempty"`  // Audio format: mp3, opus, aac, flac
	Speed          float64 `json:"speed,omitempty"`            // Speed (0.25-4.0)

	// --- Routing Control ---
	Provider string `json:"provider,omitempty"`

	// --- Dynamic Credentials ---
	Credentials map[string]string `json:"credentials,omitempty"`

	// --- Internal ---
	TenantID  string `json:"-"`
	RequestID string `json:"-"`
}

// STTRequest represents a speech-to-text request.
type STTRequest struct {
	Model          string `json:"model,omitempty"`
	AudioURL       string `json:"audio_url,omitempty"`  // URL to audio file
	AudioData      []byte `json:"-"`                    // Raw audio data (not serialized)
	AudioFormat    string `json:"audio_format,omitempty"` // Format hint
	Language       string `json:"language,omitempty"`   // Language code (e.g., "en")
	Prompt         string `json:"prompt,omitempty"`     // Optional context
	ResponseFormat string `json:"response_format,omitempty"` // "json", "text", "srt", "vtt"

	// --- Routing Control ---
	Provider string `json:"provider,omitempty"`

	// --- Dynamic Credentials ---
	Credentials map[string]string `json:"credentials,omitempty"`

	// --- Internal ---
	TenantID  string `json:"-"`
	RequestID string `json:"-"`
}

// VideoGenRequest represents a video generation request.
type VideoGenRequest struct {
	Model    string `json:"model,omitempty"`
	Prompt   string `json:"prompt"`
	Duration int    `json:"duration,omitempty"` // Duration in seconds
	Size     string `json:"size,omitempty"`     // Resolution

	// --- Routing Control ---
	Provider string `json:"provider,omitempty"`

	// --- Dynamic Credentials ---
	Credentials map[string]string `json:"credentials,omitempty"`

	// --- Extensions ---
	Extra map[string]any `json:"extra,omitempty"`

	// --- Internal ---
	TenantID  string `json:"-"`
	RequestID string `json:"-"`
}

// AgentRequest represents a request to invoke an agent.
type AgentRequest struct {
	AgentID  string    `json:"agent_id"`  // Agent identifier
	Messages []Message `json:"messages"`  // Conversation messages
	Stream   bool      `json:"stream,omitempty"`

	// --- Routing Control ---
	Provider string `json:"provider,omitempty"`

	// --- Dynamic Credentials ---
	Credentials map[string]string `json:"credentials,omitempty"`

	// --- Extensions ---
	Extra map[string]any `json:"extra,omitempty"`

	// --- Internal ---
	TenantID  string `json:"-"`
	RequestID string `json:"-"`
}

// WorkflowRequest represents a request to run a workflow.
type WorkflowRequest struct {
	WorkflowID string         `json:"workflow_id"` // Workflow identifier
	Inputs     map[string]any `json:"inputs"`      // Workflow inputs
	Stream     bool           `json:"stream,omitempty"`

	// --- Routing Control ---
	Provider string `json:"provider,omitempty"`

	// --- Dynamic Credentials ---
	Credentials map[string]string `json:"credentials,omitempty"`

	// --- Extensions ---
	Extra map[string]any `json:"extra,omitempty"`

	// --- Internal ---
	TenantID  string `json:"-"`
	RequestID string `json:"-"`
}
