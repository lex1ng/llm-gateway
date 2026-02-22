// Package gateway provides the SDK entry point for using LLM Gateway as a library.
package gateway

import (
	"context"
	"log/slog"

	"github.com/lex1ng/llm-gateway/config"
	"github.com/lex1ng/llm-gateway/pkg/manager"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

// Client is the main SDK entry point for LLM Gateway.
// It can be used directly in Go applications without running an HTTP server.
type Client struct {
	manager *manager.Manager
	logger  *slog.Logger
	opts    *clientOptions
}

// New creates a new Gateway Client from a configuration file.
func New(cfgPath string, opts ...Option) (*Client, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	return NewWithConfig(cfg, opts...)
}

// NewWithConfig creates a new Gateway Client from a Config struct.
func NewWithConfig(cfg *config.Config, opts ...Option) (*Client, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	mgr, err := manager.New(cfg)
	if err != nil {
		return nil, err
	}

	return &Client{
		manager: mgr,
		logger:  options.logger,
		opts:    options,
	}, nil
}

// Chat sends a non-streaming chat completion request.
func (c *Client) Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) {
	return c.manager.Chat(ctx, req)
}

// ChatStream sends a streaming chat completion request.
// Returns a channel that will receive StreamEvent messages until completion.
func (c *Client) ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan types.StreamEvent, error) {
	return c.manager.ChatStream(ctx, req)
}

// ListModels returns all available models.
func (c *Client) ListModels() []types.ModelConfig {
	return c.manager.ListModels()
}

// ListProviders returns the status of all providers.
func (c *Client) ListProviders() []types.ProviderStatus {
	return c.manager.ListProviders()
}

// Close releases all resources held by the client.
func (c *Client) Close() error {
	return c.manager.Close()
}
