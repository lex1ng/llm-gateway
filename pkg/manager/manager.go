// Package manager implements the core request orchestration logic.
package manager

import (
	"context"
	"fmt"
	"log"
	"sort"

	"github.com/lex1ng/llm-gateway/config"
	"github.com/lex1ng/llm-gateway/pkg/adapter/anthropic"
	"github.com/lex1ng/llm-gateway/pkg/adapter/openai"
	"github.com/lex1ng/llm-gateway/pkg/provider"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

// Manager orchestrates requests across providers.
// It handles routing, retries, caching, and other cross-cutting concerns.
type Manager struct {
	registry *provider.Registry
	router   *Router
	config   *config.Config
}

// New creates a new Manager with the given configuration.
func New(cfg *config.Config) (*Manager, error) {
	registry := provider.NewRegistry()

	// Register providers based on configuration
	if err := registerProviders(cfg, registry); err != nil {
		return nil, fmt.Errorf("register providers: %w", err)
	}

	// Create router with tier configuration
	router := NewRouter(registry, cfg)

	return &Manager{
		registry: registry,
		router:   router,
		config:   cfg,
	}, nil
}

// Pinger is an optional interface for providers that support connectivity checks.
type Pinger interface {
	Ping(ctx context.Context) error
}

// registerProviders creates and registers all configured providers.
func registerProviders(cfg *config.Config, registry *provider.Registry) error {
	// Get models grouped by provider
	modelsByProvider := groupModelsByProvider(cfg.ModelCatalog)

	// Register each configured provider
	for name, provCfg := range cfg.Providers {
		models := modelsByProvider[name]

		var p provider.Provider
		var err error

		switch name {
		case "openai":
			p, err = openai.New(provCfg, models)
		case "anthropic":
			// Support OpenAI-compatible proxies (e.g., OneAPI) via api_format: "openai"
			if provCfg.GetExtra("api_format", "anthropic") == "openai" {
				p, err = openai.NewWithName(name, provCfg, models)
			} else {
				p, err = anthropic.New(provCfg, models)
			}
		case "alibaba", "baidu", "volcengine", "zhipu", "minimax", "deepseek", "moonshot", "01ai", "baichuan":
			// Domestic platforms: default to OpenAI-compatible, but support api_format: "anthropic"
			if provCfg.GetExtra("api_format", "") == "anthropic" {
				p, err = anthropic.NewWithName(name, provCfg, models)
			} else {
				p, err = openai.NewWithName(name, provCfg, models)
			}
		// TODO: Add other providers (google)
		default:
			// Unknown providers: try OpenAI-compatible adapter as fallback
			log.Printf("[INFO] provider %q using OpenAI-compatible adapter", name)
			p, err = openai.NewWithName(name, provCfg, models)
		}

		if err != nil {
			return fmt.Errorf("create provider %s: %w", name, err)
		}

		// Connectivity check: verify the provider is reachable
		if pinger, ok := p.(Pinger); ok {
			if err := pinger.Ping(context.Background()); err != nil {
				log.Printf("[WARN] provider %q connectivity check failed: %v (registering anyway)", name, err)
			} else {
				log.Printf("[INFO] provider %q connectivity verified", name)
			}
		}

		if err := registry.Register(p); err != nil {
			return fmt.Errorf("register provider %s: %w", name, err)
		}
	}

	// Set up tier routing
	tierRouting := buildTierRouting(cfg.TierRouting)
	registry.SetTierRouting(tierRouting)

	return nil
}

// groupModelsByProvider groups models by their provider name.
func groupModelsByProvider(models []config.ModelCatalogEntry) map[string][]types.ModelConfig {
	result := make(map[string][]types.ModelConfig)
	for _, model := range models {
		mc := convertModelCatalogEntry(model)
		result[model.Provider] = append(result[model.Provider], mc)
	}
	return result
}

// convertModelCatalogEntry converts a config entry to types.ModelConfig.
func convertModelCatalogEntry(entry config.ModelCatalogEntry) types.ModelConfig {
	return types.ModelConfig{
		Provider:      entry.Provider,
		ModelID:       entry.ID,
		Tier:          entry.Tier,
		ContextWindow: entry.ContextWindow,
		MaxOutput:     entry.MaxOutput,
		InputPrice:    entry.InputPrice,
		OutputPrice:   entry.OutputPrice,
		Capabilities:  entry.Capabilities,
	}
}

// buildTierRouting converts config tier routing to registry format.
// Entries within each tier are sorted by Priority (ascending = higher priority).
func buildTierRouting(cfgRouting map[string][]config.RouteEntry) map[types.ModelTier][]provider.TierEntry {
	result := make(map[types.ModelTier][]provider.TierEntry)
	for tierStr, entries := range cfgRouting {
		tier := types.ModelTier(tierStr)
		for _, entry := range entries {
			result[tier] = append(result[tier], provider.TierEntry{
				ProviderName: entry.Provider,
				ModelID:      entry.Model,
				Priority:     entry.Priority,
			})
		}
		// Sort by priority (lower number = higher priority)
		sort.Slice(result[tier], func(i, j int) bool {
			return result[tier][i].Priority < result[tier][j].Priority
		})
	}
	return result
}

// Chat handles a non-streaming chat completion request.
func (m *Manager) Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) {
	// Route request to provider
	cp, model, err := m.router.SelectChat(req)
	if err != nil {
		return nil, err
	}

	// Update model in request
	req.Model = model

	// Execute request
	return cp.Chat(ctx, req)
}

// ChatStream handles a streaming chat completion request.
func (m *Manager) ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan types.StreamEvent, error) {
	// Route request to provider
	cp, model, err := m.router.SelectChat(req)
	if err != nil {
		return nil, err
	}

	// Update model in request
	req.Model = model

	// Execute request
	return cp.ChatStream(ctx, req)
}

// Responses handles a non-streaming Responses API request.
// This API is OpenAI-specific and provides better performance with reasoning models.
func (m *Manager) Responses(ctx context.Context, req *types.ResponsesRequest) (*types.ResponsesResponse, error) {
	// Route request to provider
	rp, model, err := m.router.SelectResponses(req)
	if err != nil {
		return nil, err
	}

	// Update model in request
	req.Model = model

	// Execute request
	return rp.Responses(ctx, req)
}

// ResponsesStream handles a streaming Responses API request.
func (m *Manager) ResponsesStream(ctx context.Context, req *types.ResponsesRequest) (<-chan types.ResponsesStreamEvent, error) {
	// Route request to provider
	rp, model, err := m.router.SelectResponses(req)
	if err != nil {
		return nil, err
	}

	// Update model in request
	req.Model = model

	// Execute request
	return rp.ResponsesStream(ctx, req)
}

// ListModels returns all available models.
func (m *Manager) ListModels() []types.ModelConfig {
	return m.registry.ListModels()
}

// ListProviders returns the status of all providers.
func (m *Manager) ListProviders() []types.ProviderStatus {
	names := m.registry.List()
	statuses := make([]types.ProviderStatus, len(names))

	for i, name := range names {
		p, _ := m.registry.Get(name)
		statuses[i] = types.ProviderStatus{
			Name:      name,
			Available: true, // TODO: Add health check
			Models:    len(p.Models()),
		}
	}

	return statuses
}

// Close shuts down the manager and all providers.
func (m *Manager) Close() error {
	return m.registry.Close()
}
