package types

// ModelTier represents the tier/level of a model.
// Used for routing requests to appropriate models based on capability needs.
type ModelTier string

const (
	TierSmall  ModelTier = "small"  // Fast, cheap, suitable for simple tasks
	TierMedium ModelTier = "medium" // Balanced performance and cost
	TierLarge  ModelTier = "large"  // Most capable, higher cost
)

// ModelConfig represents the configuration of a model.
type ModelConfig struct {
	Provider      string            `json:"provider" yaml:"provider"`
	ModelID       string            `json:"model_id" yaml:"model_id"`
	DisplayName   string            `json:"display_name,omitempty" yaml:"display_name"`
	Tier          ModelTier         `json:"tier" yaml:"tier"`
	ContextWindow int               `json:"context_window" yaml:"context_window"`
	MaxOutput     int               `json:"max_output" yaml:"max_output"`
	InputPrice    float64           `json:"input_price_per_1k" yaml:"input_price_per_1k"`   // USD per 1K tokens
	OutputPrice   float64           `json:"output_price_per_1k" yaml:"output_price_per_1k"` // USD per 1K tokens
	Capabilities  ModelCapabilities `json:"capabilities" yaml:"capabilities"`
}

// ModelCapabilities describes what a model can do.
type ModelCapabilities struct {
	Chat      bool `json:"chat" yaml:"chat"`
	Vision    bool `json:"vision" yaml:"vision"`
	Tools     bool `json:"tools" yaml:"tools"`
	JSONMode  bool `json:"json_mode" yaml:"json_mode"`
	Streaming bool `json:"streaming" yaml:"streaming"`
	Reasoning bool `json:"reasoning" yaml:"reasoning"` // Extended thinking (Claude 3.5+)
	Embedding bool `json:"embedding" yaml:"embedding"`
	ImageGen  bool `json:"image_gen" yaml:"image_gen"`
	VideoGen  bool `json:"video_gen" yaml:"video_gen"`
	TTS       bool `json:"tts" yaml:"tts"` // Text-to-Speech
	STT       bool `json:"stt" yaml:"stt"` // Speech-to-Text
}

// HasCapability checks if the model has a specific capability.
func (c ModelCapabilities) HasCapability(cap string) bool {
	switch cap {
	case "chat":
		return c.Chat
	case "vision":
		return c.Vision
	case "tools":
		return c.Tools
	case "json_mode":
		return c.JSONMode
	case "streaming":
		return c.Streaming
	case "reasoning":
		return c.Reasoning
	case "embedding":
		return c.Embedding
	case "image_gen":
		return c.ImageGen
	case "video_gen":
		return c.VideoGen
	case "tts":
		return c.TTS
	case "stt":
		return c.STT
	default:
		return false
	}
}

// SupportsChat returns true if the model supports chat completions.
func (m *ModelConfig) SupportsChat() bool {
	return m.Capabilities.Chat
}

// SupportsVision returns true if the model supports vision/image input.
func (m *ModelConfig) SupportsVision() bool {
	return m.Capabilities.Vision
}

// SupportsTools returns true if the model supports tool/function calling.
func (m *ModelConfig) SupportsTools() bool {
	return m.Capabilities.Tools
}

// SupportsStreaming returns true if the model supports streaming responses.
func (m *ModelConfig) SupportsStreaming() bool {
	return m.Capabilities.Streaming
}

// EstimateCost estimates the cost in USD for given token counts.
func (m *ModelConfig) EstimateCost(inputTokens, outputTokens int) float64 {
	inputCost := float64(inputTokens) / 1000 * m.InputPrice
	outputCost := float64(outputTokens) / 1000 * m.OutputPrice
	return inputCost + outputCost
}
