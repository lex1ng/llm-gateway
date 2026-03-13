package gateway

import (
	"fmt"
	"time"

	"github.com/lex1ng/llm-gateway/config"
)

// ProviderOpts configures a single provider.
type ProviderOpts struct {
	// BaseURL is the provider's API endpoint (required).
	// Examples:
	//   "https://dashscope.aliyuncs.com/compatible-mode/v1"  (alibaba)
	//   "https://api.openai.com/v1"                          (openai)
	//   "https://api.deepseek.com/v1"                        (deepseek)
	BaseURL string

	// APIKey is the authentication key (required).
	APIKey string

	// Timeout overrides the default per-request timeout for this provider.
	Timeout time.Duration

	// Extra holds provider-specific options.
	// Examples:
	//   "api_format":          "openai"     — use OpenAI protocol for Anthropic provider
	//   "api_format":          "anthropic"  — use Anthropic protocol for domestic providers
	//   "anthropic_version":   "2023-06-01" — Anthropic API version
	//   "default_max_tokens":  4096         — Anthropic required max_tokens
	//   "chat_path":           "/chat/completions" — custom endpoint path
	//   "embeddings_path":     "/embeddings"       — custom embeddings path
	Extra map[string]any
}

// Builder constructs a Gateway Client programmatically, without config files.
//
// HTTP proxy is controlled by system environment variables (HTTP_PROXY/HTTPS_PROXY).
// No per-provider proxy configuration needed.
//
// Usage:
//
//	client, err := gateway.NewBuilder().
//	    AddProvider("alibaba", gateway.ProviderOpts{
//	        BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
//	        APIKey:  os.Getenv("DASHSCOPE_API_KEY"),
//	    }).
//	    Build()
//
//	resp, _ := client.Chat(ctx, &types.ChatRequest{
//	    Provider: "alibaba",
//	    Model:    "qwen-plus",
//	    Messages: messages,
//	})
type Builder struct {
	providers map[string]ProviderOpts
	timeout   *time.Duration
	opts      []Option
	err       error
}

// NewBuilder creates a new Builder for programmatic configuration.
func NewBuilder() *Builder {
	return &Builder{
		providers: make(map[string]ProviderOpts),
	}
}

// AddProvider registers a provider. Can be called multiple times.
//
// The name should match the provider type for proper adapter selection:
//
//	"openai", "anthropic", "alibaba", "volcengine", "zhipu", "deepseek", "baidu", etc.
//
// Unknown names default to the OpenAI-compatible adapter.
func (b *Builder) AddProvider(name string, opts ProviderOpts) *Builder {
	if name == "" {
		b.err = fmt.Errorf("provider name cannot be empty")
		return b
	}
	if opts.BaseURL == "" {
		b.err = fmt.Errorf("provider %q: base_url is required", name)
		return b
	}
	if opts.APIKey == "" {
		b.err = fmt.Errorf("provider %q: api_key is required", name)
		return b
	}
	b.providers[name] = opts
	return b
}

// SetTimeout overrides the default request timeout (default: 120s non-stream, 300s stream).
func (b *Builder) SetTimeout(d time.Duration) *Builder {
	b.timeout = &d
	return b
}

// WithOption adds a Client-level option (e.g., WithLogger, WithDebug).
func (b *Builder) WithOption(opt Option) *Builder {
	b.opts = append(b.opts, opt)
	return b
}

// Build creates the Client. Returns an error if no providers are configured.
func (b *Builder) Build() (*Client, error) {
	if b.err != nil {
		return nil, b.err
	}
	if len(b.providers) == 0 {
		return nil, fmt.Errorf("at least one provider is required")
	}

	// Build config.Config from builder state
	cfg := &config.Config{
		Providers: make(map[string]config.ProviderConfig, len(b.providers)),
	}

	for name, opts := range b.providers {
		pc := config.ProviderConfig{
			BaseURL: opts.BaseURL,
			APIKey:  opts.APIKey,
			Timeout: opts.Timeout,
			Extra:   opts.Extra,
		}
		cfg.Providers[name] = pc
	}

	// Apply timeout override
	if b.timeout != nil {
		cfg.Manager.Timeout.TotalNonStream = *b.timeout
		cfg.Manager.Timeout.TotalStream = *b.timeout
	}

	// applyDefaults fills in all zero-valued Manager/Server/Security fields
	return NewWithConfig(cfg, b.opts...)
}
