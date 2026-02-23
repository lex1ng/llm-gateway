package manager

import (
	"github.com/lex1ng/llm-gateway/config"
	"github.com/lex1ng/llm-gateway/pkg/provider"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

// Router handles request routing to appropriate providers and models.
type Router struct {
	registry *provider.Registry
	config   *config.Config
}

// NewRouter creates a new Router.
func NewRouter(registry *provider.Registry, cfg *config.Config) *Router {
	return &Router{
		registry: registry,
		config:   cfg,
	}
}

// SelectChat selects a ChatProvider and model for the given request.
// Returns the provider, the model ID to use, and any error.
//
// Priority order (per design §7.2):
//  1. Provider explicitly specified → use that provider
//  2. Model explicitly specified → reverse-lookup provider
//  3. Tier routing (default TierMedium if not specified)
func (r *Router) SelectChat(req *types.ChatRequest) (provider.ChatProvider, string, error) {
	// Priority 1: Explicit provider
	if req.Provider != "" {
		if req.Model != "" {
			// Provider + Model: verify model belongs to provider, then use it
			return r.selectByModel(req.Model)
		}
		if req.ModelTier != "" {
			// Provider + Tier: find matching tier model from provider
			return r.selectByProviderAndTier(req.Provider, req.ModelTier)
		}
		// Provider only: use its default (first) model
		return r.selectByProvider(req.Provider)
	}

	// Priority 2: Explicit model → reverse-lookup provider
	if req.Model != "" {
		return r.selectByModel(req.Model)
	}

	// Priority 3: Tier routing (default to TierMedium)
	tier := req.ModelTier
	if tier == "" {
		tier = types.TierMedium
	}
	return r.selectByTier(tier)
}

// selectByModel finds the provider for a specific model.
func (r *Router) selectByModel(modelID string) (provider.ChatProvider, string, error) {
	cp, ok := r.registry.GetChatProviderByModel(modelID)
	if !ok {
		return nil, "", &types.ProviderError{
			Code:       types.ErrModelNotFound,
			Message:    "model not found: " + modelID,
			StatusCode: 404,
		}
	}
	return cp, modelID, nil
}

// selectByProvider selects the first available model from a provider.
func (r *Router) selectByProvider(providerName string) (provider.ChatProvider, string, error) {
	cp, ok := r.registry.GetChatProvider(providerName)
	if !ok {
		return nil, "", &types.ProviderError{
			Code:       types.ErrProviderNotFound,
			Message:    "provider not found: " + providerName,
			StatusCode: 404,
		}
	}

	// Get first model from provider
	models := cp.Models()
	if len(models) == 0 {
		return nil, "", &types.ProviderError{
			Code:       types.ErrModelNotFound,
			Message:    "no models available for provider: " + providerName,
			StatusCode: 404,
		}
	}

	return cp, models[0].ModelID, nil
}

// selectByProviderAndTier selects a model from a specific provider matching the tier.
func (r *Router) selectByProviderAndTier(providerName string, tier types.ModelTier) (provider.ChatProvider, string, error) {
	cp, ok := r.registry.GetChatProvider(providerName)
	if !ok {
		return nil, "", &types.ProviderError{
			Code:       types.ErrProviderNotFound,
			Message:    "provider not found: " + providerName,
			StatusCode: 404,
		}
	}

	// Find first model matching tier
	for _, model := range cp.Models() {
		if model.Tier == tier {
			return cp, model.ModelID, nil
		}
	}

	// Fallback to any model
	models := cp.Models()
	if len(models) == 0 {
		return nil, "", &types.ProviderError{
			Code:       types.ErrModelNotFound,
			Message:    "no models available for provider: " + providerName,
			StatusCode: 404,
		}
	}

	return cp, models[0].ModelID, nil
}

// selectByTier selects a provider and model based on tier routing configuration.
func (r *Router) selectByTier(tier types.ModelTier) (provider.ChatProvider, string, error) {
	entries := r.registry.GetForTier(tier)
	if len(entries) == 0 {
		return nil, "", &types.ProviderError{
			Code:       types.ErrModelNotFound,
			Message:    "no models configured for tier: " + string(tier),
			StatusCode: 404,
		}
	}

	// Try each entry in priority order
	for _, entry := range entries {
		cp, ok := r.registry.GetChatProvider(entry.ProviderName)
		if ok {
			return cp, entry.ModelID, nil
		}
	}

	return nil, "", &types.ProviderError{
		Code:       types.ErrProviderNotFound,
		Message:    "no available providers for tier: " + string(tier),
		StatusCode: 404,
	}
}
