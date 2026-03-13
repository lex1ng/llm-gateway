// Package openai implements the OpenAI provider adapter.
// Domestic platforms (Alibaba, Volcengine, Zhipu, etc.) reuse this adapter
// with a different name and baseURL.
//
// Configurable via config.yaml extra fields:
//   - chat_path: chat completions endpoint path (default "/chat/completions")
//   - responses_path: responses API endpoint path (default "/responses")
//   - models_path: models list endpoint path (default "/models")
package openai

import (
	"context"
	"net/http"
	"time"

	"github.com/lex1ng/llm-gateway/config"
	"github.com/lex1ng/llm-gateway/pkg/provider"
	"github.com/lex1ng/llm-gateway/pkg/transport"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

const (
	defaultBaseURL        = "https://api.openai.com/v1"
	defaultChatPath       = "/chat/completions"
	defaultResponsesPath  = "/responses"
	defaultModelsPath     = "/models"
	defaultEmbeddingsPath = "/embeddings"
	providerName          = "openai"
)

// OpenAI implements the OpenAI API adapter.
// Since our internal format is OpenAI-compatible, most requests are nearly pass-through.
type OpenAI struct {
	client         *transport.HTTPClient
	auth           transport.AuthStrategy
	baseURL        string
	chatPath       string // configurable chat completions endpoint path
	responsesPath  string // configurable responses endpoint path
	modelsPath     string // configurable models list endpoint path
	embeddingsPath string // configurable embeddings endpoint path
	name           string
	models         []types.ModelConfig
}

// New creates a new OpenAI provider.
func New(cfg config.ProviderConfig, models []types.ModelConfig) (*OpenAI, error) {
	return NewWithName(providerName, cfg, models)
}

// NewWithName creates a new OpenAI-compatible provider with a custom name.
// Used by domestic platforms that implement the OpenAI-compatible API.
//
// Configurable extra fields in config.yaml:
//
//	chat_path:      "/chat/completions"   # chat completions endpoint path
//	responses_path: "/responses"          # responses API endpoint path
//	models_path:    "/models"             # models list endpoint path
func NewWithName(name string, cfg config.ProviderConfig, models []types.ModelConfig) (*OpenAI, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	chatPath := cfg.GetExtra("chat_path", defaultChatPath)
	responsesPath := cfg.GetExtra("responses_path", defaultResponsesPath)
	modelsPath := cfg.GetExtra("models_path", defaultModelsPath)
	embeddingsPath := cfg.GetExtra("embeddings_path", defaultEmbeddingsPath)

	// OpenAI adapter always uses Bearer auth, regardless of provider name.
	// (NewAuthStrategy would pick AnthropicAuth/GoogleAuth based on name, which is wrong here)
	auth := &transport.BearerAuth{APIKey: cfg.APIKey}

	return &OpenAI{
		client:         transport.NewHTTPClientWithTimeout(cfg.Timeout),
		auth:           auth,
		baseURL:        baseURL,
		chatPath:       chatPath,
		responsesPath:  responsesPath,
		modelsPath:     modelsPath,
		embeddingsPath: embeddingsPath,
		name:           name,
		models:         models,
	}, nil
}

// Name returns the provider name.
func (p *OpenAI) Name() string {
	return p.name
}

// Models returns the list of models this provider supports.
func (p *OpenAI) Models() []types.ModelConfig {
	return p.models
}

// Supports returns true if this provider supports the given capability.
func (p *OpenAI) Supports(cap provider.Capability) bool {
	switch cap {
	case provider.CapChat, provider.CapResponses, provider.CapStream, provider.CapTools, provider.CapVision, provider.CapJSONMode, provider.CapEmbed:
		return true
	default:
		return false
	}
}

// Close releases any resources held by the provider.
func (p *OpenAI) Close() error {
	return nil
}

// Ping verifies connectivity by calling GET /models with a short timeout.
func (p *OpenAI) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	return p.client.DoJSON(ctx, http.MethodGet, p.baseURL+p.modelsPath, p.auth, nil, &result)
}

// chatEndpoint returns the chat completions endpoint URL.
func (p *OpenAI) chatEndpoint() string {
	return p.baseURL + p.chatPath
}

// responsesEndpoint returns the Responses API endpoint URL.
func (p *OpenAI) responsesEndpoint() string {
	return p.baseURL + p.responsesPath
}

// embeddingsEndpoint returns the embeddings API endpoint URL.
func (p *OpenAI) embeddingsEndpoint() string {
	return p.baseURL + p.embeddingsPath
}
