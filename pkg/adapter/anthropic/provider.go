// Package anthropic implements the Anthropic (Claude) provider adapter.
// Anthropic's Messages API differs from OpenAI in several ways:
// - Auth: x-api-key + anthropic-version headers (not Bearer token)
// - System messages are extracted to a top-level "system" field
// - max_tokens is required (default 4096)
// - stop → stop_sequences
// - Usage: input_tokens/output_tokens (not prompt_tokens/completion_tokens)
// - Streaming uses named SSE events (content_block_delta, message_delta, etc.)
//
// Configurable via config.yaml extra fields:
//   - anthropic_version: API version header (default "2023-06-01")
//   - default_max_tokens: default max_tokens when not specified (default 4096)
//   - messages_path: messages endpoint path (default "/v1/messages")
package anthropic

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
	defaultBaseURL       = "https://api.anthropic.com"
	defaultMaxTokens     = 4096
	defaultMessagesPath  = "/v1/messages"
	defaultVersion       = "2023-06-01"
	providerName         = "anthropic"
)

// Anthropic implements the Anthropic Messages API adapter.
type Anthropic struct {
	client       *transport.HTTPClient
	auth         transport.AuthStrategy
	baseURL      string
	messagesPath string // configurable endpoint path
	maxTokens    int    // configurable default max_tokens
	name         string // provider name (default "anthropic")
	models       []types.ModelConfig
}

// New creates a new Anthropic provider.
//
// Configurable extra fields in config.yaml:
//
//	anthropic_version:  "2023-06-01"     # API version header
//	default_max_tokens: 4096             # default max_tokens
//	messages_path:      "/v1/messages"   # endpoint path (useful for proxies)
//	auth_type:          "anthropic"      # "anthropic" (x-api-key) or "bearer"
func New(cfg config.ProviderConfig, models []types.ModelConfig) (*Anthropic, error) {
	return NewWithName(providerName, cfg, models)
}

// NewWithName creates an Anthropic adapter with a custom provider name.
// Used when non-Anthropic providers expose an Anthropic-compatible API (e.g., Zhipu GLM).
func NewWithName(name string, cfg config.ProviderConfig, models []types.ModelConfig) (*Anthropic, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	version := cfg.GetExtra("anthropic_version", defaultVersion)
	messagesPath := cfg.GetExtra("messages_path", defaultMessagesPath)
	maxTokens := cfg.GetExtraInt("default_max_tokens", defaultMaxTokens)

	// Select auth strategy: Anthropic native (x-api-key) or Bearer token
	authType := cfg.GetExtra("auth_type", "")
	var auth transport.AuthStrategy
	if authType == "bearer" {
		auth = &transport.BearerAuth{APIKey: cfg.APIKey}
	} else if name != providerName {
		// Non-Anthropic providers default to Bearer auth
		auth = &transport.BearerAuth{APIKey: cfg.APIKey}
	} else {
		// Native Anthropic uses x-api-key + anthropic-version
		auth = &transport.AnthropicAuth{
			APIKey:  cfg.APIKey,
			Version: version,
		}
	}

	return &Anthropic{
		client:       transport.NewHTTPClientWithProxy(cfg.Proxy),
		auth:         auth,
		baseURL:      baseURL,
		messagesPath: messagesPath,
		maxTokens:    maxTokens,
		name:         name,
		models:       models,
	}, nil
}

// Name returns the provider name.
func (p *Anthropic) Name() string {
	return p.name
}

// Models returns the list of models this provider supports.
func (p *Anthropic) Models() []types.ModelConfig {
	return p.models
}

// Supports returns true if this provider supports the given capability.
func (p *Anthropic) Supports(cap provider.Capability) bool {
	switch cap {
	case provider.CapChat, provider.CapStream, provider.CapTools, provider.CapVision:
		return true
	default:
		return false
	}
}

// Close releases any resources held by the provider.
func (p *Anthropic) Close() error {
	return nil
}

// Ping verifies connectivity by sending a minimal request.
// Uses the first configured model, or falls back to a default.
func (p *Anthropic) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Use first configured model for ping
	model := "claude-sonnet-4-20250514"
	if len(p.models) > 0 {
		model = p.models[0].ModelID
	}

	pingReq := &anthropicRequest{
		Model:     model,
		MaxTokens: 1,
		Messages: []anthropicMessage{
			{Role: "user", Content: "hi"},
		},
	}

	var result anthropicResponse
	return p.client.DoJSON(ctx, http.MethodPost, p.messagesEndpoint(), p.auth, pingReq, &result)
}

// messagesEndpoint returns the messages API endpoint URL.
func (p *Anthropic) messagesEndpoint() string {
	return p.baseURL + p.messagesPath
}

// getAuth returns the appropriate auth strategy, with dynamic credentials taking priority.
func (p *Anthropic) getAuth(credentials map[string]string) transport.AuthStrategy {
	if len(credentials) > 0 {
		return transport.WithDynamicCredentials(p.auth, credentials)
	}
	return p.auth
}
