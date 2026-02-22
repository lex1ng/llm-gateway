package types

// ChatResponse represents a chat completion response.
type ChatResponse struct {
	ID           string     `json:"id"`
	Model        string     `json:"model"`
	Provider     string     `json:"provider"`
	Content      string     `json:"content"`
	FinishReason string     `json:"finish_reason"` // stop, length, tool_calls, content_filter
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	Usage        TokenUsage `json:"usage"`
	LatencyMs    int64      `json:"latency_ms"`
	Cached       bool       `json:"cached,omitempty"`
	CreatedAt    int64      `json:"created_at"`
}

// StreamEventType defines the type of streaming event.
type StreamEventType string

const (
	StreamEventContentDelta  StreamEventType = "content_delta"
	StreamEventToolCallDelta StreamEventType = "tool_call_delta"
	StreamEventUsage         StreamEventType = "usage"
	StreamEventDone          StreamEventType = "done"
	StreamEventError         StreamEventType = "error"
)

// StreamEvent represents a unified streaming event.
// Solves the dual-stream-interface problem in Shannon.
type StreamEvent struct {
	Type         StreamEventType `json:"type"`
	Delta        string          `json:"delta,omitempty"`         // Content delta
	ToolCall     *ToolCall       `json:"tool_call,omitempty"`     // Tool call delta
	Usage        *TokenUsage     `json:"usage,omitempty"`         // Final usage stats
	FinishReason string          `json:"finish_reason,omitempty"` // Set on done event
	Error        string          `json:"error,omitempty"`         // Error message
}

// NewContentDeltaEvent creates a content delta event.
func NewContentDeltaEvent(delta string) StreamEvent {
	return StreamEvent{
		Type:  StreamEventContentDelta,
		Delta: delta,
	}
}

// NewToolCallDeltaEvent creates a tool call delta event.
func NewToolCallDeltaEvent(toolCall *ToolCall) StreamEvent {
	return StreamEvent{
		Type:     StreamEventToolCallDelta,
		ToolCall: toolCall,
	}
}

// NewUsageEvent creates a usage event.
func NewUsageEvent(usage TokenUsage) StreamEvent {
	return StreamEvent{
		Type:  StreamEventUsage,
		Usage: &usage,
	}
}

// NewDoneEvent creates a done event.
func NewDoneEvent(finishReason string, usage *TokenUsage) StreamEvent {
	return StreamEvent{
		Type:         StreamEventDone,
		FinishReason: finishReason,
		Usage:        usage,
	}
}

// NewErrorEvent creates an error event.
func NewErrorEvent(err string) StreamEvent {
	return StreamEvent{
		Type:  StreamEventError,
		Error: err,
	}
}

// EmbedResponse represents an embedding response.
type EmbedResponse struct {
	Model    string       `json:"model"`
	Provider string       `json:"provider"`
	Data     []Embedding  `json:"data"`
	Usage    TokenUsage   `json:"usage"`
}

// Embedding represents a single embedding vector.
type Embedding struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
	Object    string    `json:"object"` // "embedding"
}

// ImageGenResponse represents an image generation response.
type ImageGenResponse struct {
	Model    string       `json:"model"`
	Provider string       `json:"provider"`
	Data     []ImageData  `json:"data"`
	TaskID   string       `json:"task_id,omitempty"` // For async generation
}

// ImageData represents a generated image.
type ImageData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// TTSResponse represents a text-to-speech response.
type TTSResponse struct {
	Model       string `json:"model"`
	Provider    string `json:"provider"`
	AudioURL    string `json:"audio_url,omitempty"`
	AudioData   []byte `json:"-"` // Raw audio data
	AudioFormat string `json:"audio_format"`
	DurationMs  int64  `json:"duration_ms,omitempty"`
}

// STTResponse represents a speech-to-text response.
type STTResponse struct {
	Model    string     `json:"model"`
	Provider string     `json:"provider"`
	Text     string     `json:"text"`
	Duration float64     `json:"duration,omitempty"` // Audio duration in seconds
	Language string      `json:"language,omitempty"` // Detected language
	Segments []Segment   `json:"segments,omitempty"` // For detailed transcription
	Usage    *TokenUsage `json:"usage,omitempty"`
}

// Segment represents a transcription segment with timing.
type Segment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"` // Start time in seconds
	End   float64 `json:"end"`   // End time in seconds
	Text  string  `json:"text"`
}

// VideoGenResponse represents a video generation response.
type VideoGenResponse struct {
	Model    string `json:"model"`
	Provider string `json:"provider"`
	TaskID   string `json:"task_id"`           // Always async
	VideoURL string `json:"video_url,omitempty"` // Set when complete
}

// ListModelsResponse represents a response listing available models.
type ListModelsResponse struct {
	Models []ModelConfig `json:"models"`
}

// ListProvidersResponse represents a response listing providers and their status.
type ListProvidersResponse struct {
	Providers []ProviderStatus `json:"providers"`
}

// ProviderStatus represents the status of a provider.
type ProviderStatus struct {
	Name        string `json:"name"`
	Available   bool   `json:"available"`
	CircuitOpen bool   `json:"circuit_open"`
	Models      int    `json:"models"` // Number of available models
}
