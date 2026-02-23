// Package openai implements the OpenAI provider adapter.
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
	defaultBaseURL = "https://api.openai.com/v1"
	providerName   = "openai"
)

// OpenAI implements the OpenAI API adapter.
// Since our internal format is OpenAI-compatible, most requests are nearly pass-through.
type OpenAI struct {
	client  *transport.HTTPClient
	auth    transport.AuthStrategy
	baseURL string
	models  []types.ModelConfig
}

// New creates a new OpenAI provider.
func New(cfg config.ProviderConfig, models []types.ModelConfig) (*OpenAI, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &OpenAI{
		client:  transport.DefaultHTTPClient(),
		auth:    transport.NewAuthStrategy(providerName, cfg.APIKey),
		baseURL: baseURL,
		models:  models,
	}, nil
}

// Name returns the provider name.
func (p *OpenAI) Name() string {
	return providerName
}

// Models returns the list of models this provider supports.
func (p *OpenAI) Models() []types.ModelConfig {
	return p.models
}

// Supports returns true if this provider supports the given capability.
func (p *OpenAI) Supports(cap provider.Capability) bool {
	switch cap {
	case provider.CapChat, provider.CapStream, provider.CapTools, provider.CapVision, provider.CapJSONMode:
		return true
	case provider.CapEmbed:
		// Check if any model supports embedding
		for _, m := range p.models {
			if m.Capabilities.Embedding {
				return true
			}
		}
		return false
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
	return p.client.DoJSON(ctx, http.MethodGet, p.baseURL+"/models", p.auth, nil, &result)
}

// chatEndpoint returns the chat completions endpoint URL.
func (p *OpenAI) chatEndpoint() string {
	return p.baseURL + "/chat/completions"
}
