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
func (r *Router) SelectChat(req *types.ChatRequest) (provider.ChatProvider, string, error) {
	// Priority 1: If specific model is requested, find its provider
	if req.Model != "" {
		return r.selectByModel(req.Model)
	}

	// Priority 2: If specific provider is requested with tier
	if req.Provider != "" && req.ModelTier != "" {
		return r.selectByProviderAndTier(req.Provider, req.ModelTier)
	}

	// Priority 3: If only provider is requested, use its default model
	if req.Provider != "" {
		return r.selectByProvider(req.Provider)
	}

	// Priority 4: If only tier is requested, use tier routing
	if req.ModelTier != "" {
		return r.selectByTier(req.ModelTier)
	}

	// Default: Use large tier
	return r.selectByTier(types.TierLarge)
}

// selectByModel finds the provider for a specific model.
func (r *Router) selectByModel(modelID string) (provider.ChatProvider, string, error) {
	cp, ok := r.registry.GetChatProviderByModel(modelID)
	if !ok {
		return nil, "", &types.ProviderError{
			Code:    types.ErrModelNotFound,
			Message: "model not found: " + modelID,
		}
	}
	return cp, modelID, nil
}

// selectByProvider selects the first available model from a provider.
func (r *Router) selectByProvider(providerName string) (provider.ChatProvider, string, error) {
	cp, ok := r.registry.GetChatProvider(providerName)
	if !ok {
		return nil, "", &types.ProviderError{
			Code:    types.ErrProviderError,
			Message: "provider not found: " + providerName,
		}
	}

	// Get first model from provider
	models := cp.Models()
	if len(models) == 0 {
		return nil, "", &types.ProviderError{
			Code:    types.ErrModelNotFound,
			Message: "no models available for provider: " + providerName,
		}
	}

	return cp, models[0].ModelID, nil
}

// selectByProviderAndTier selects a model from a specific provider matching the tier.
func (r *Router) selectByProviderAndTier(providerName string, tier types.ModelTier) (provider.ChatProvider, string, error) {
	cp, ok := r.registry.GetChatProvider(providerName)
	if !ok {
		return nil, "", &types.ProviderError{
			Code:    types.ErrProviderError,
			Message: "provider not found: " + providerName,
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
			Code:    types.ErrModelNotFound,
			Message: "no models available for provider: " + providerName,
		}
	}

	return cp, models[0].ModelID, nil
}

// selectByTier selects a provider and model based on tier routing configuration.
func (r *Router) selectByTier(tier types.ModelTier) (provider.ChatProvider, string, error) {
	entries := r.registry.GetForTier(tier)
	if len(entries) == 0 {
		return nil, "", &types.ProviderError{
			Code:    types.ErrModelNotFound,
			Message: "no models configured for tier: " + string(tier),
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
		Code:    types.ErrProviderError,
		Message: "no available providers for tier: " + string(tier),
	}
}
