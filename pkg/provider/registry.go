package provider

import (
	"fmt"
	"sync"

	"github.com/lex1ng/llm-gateway/pkg/types"
)

// Registry manages provider registration and lookup.
type Registry struct {
	mu sync.RWMutex

	// providers maps provider name to provider instance
	providers map[string]Provider

	// chatProviders provides type-safe access to ChatProvider instances
	chatProviders map[string]ChatProvider

	// embeddingProviders provides type-safe access to EmbeddingProvider instances
	embeddingProviders map[string]EmbeddingProvider

	// modelIndex maps model ID to provider name
	modelIndex map[string]string

	// tierIndex maps tier to list of providers (ordered by priority)
	tierIndex map[types.ModelTier][]TierEntry
}

// TierEntry represents a provider entry for tier-based routing.
type TierEntry struct {
	ProviderName string
	ModelID      string
	Priority     int
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers:          make(map[string]Provider),
		chatProviders:      make(map[string]ChatProvider),
		embeddingProviders: make(map[string]EmbeddingProvider),
		modelIndex:         make(map[string]string),
		tierIndex:          make(map[types.ModelTier][]TierEntry),
	}
}

// Register adds a provider to the registry.
// It auto-detects which capability interfaces the provider implements
// and validates consistency with Provider.Supports().
func (r *Registry) Register(p Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %q already registered", name)
	}

	// Auto-detect capabilities via reflection
	detectedCaps := detectCapabilities(p)

	// Validate consistency: if interface is implemented, Supports() should return true
	for _, cap := range detectedCaps {
		if !p.Supports(cap) {
			return fmt.Errorf("provider %q implements %s interface but Supports(%s) returns false",
				name, cap, cap)
		}
	}

	// Store in main registry
	r.providers[name] = p

	// Store in type-specific registries
	if cp, ok := p.(ChatProvider); ok {
		r.chatProviders[name] = cp
	}
	if ep, ok := p.(EmbeddingProvider); ok {
		r.embeddingProviders[name] = ep
	}

	// Index models
	for _, model := range p.Models() {
		r.modelIndex[model.ModelID] = name
	}

	return nil
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// GetChatProvider returns a ChatProvider by name.
// Type-safe: returns ChatProvider directly, no type assertion needed by caller.
func (r *Registry) GetChatProvider(name string) (ChatProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cp, ok := r.chatProviders[name]
	return cp, ok
}

// GetEmbeddingProvider returns an EmbeddingProvider by name.
func (r *Registry) GetEmbeddingProvider(name string) (EmbeddingProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ep, ok := r.embeddingProviders[name]
	return ep, ok
}

// GetByModel returns the provider that supports the given model ID.
func (r *Registry) GetByModel(modelID string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providerName, ok := r.modelIndex[modelID]
	if !ok {
		return nil, false
	}
	return r.providers[providerName], true
}

// GetChatProviderByModel returns the ChatProvider that supports the given model ID.
func (r *Registry) GetChatProviderByModel(modelID string) (ChatProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providerName, ok := r.modelIndex[modelID]
	if !ok {
		return nil, false
	}
	return r.chatProviders[providerName], true
}

// SetTierRouting sets the tier-based routing entries.
// Should be called after all providers are registered.
func (r *Registry) SetTierRouting(routing map[types.ModelTier][]TierEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tierIndex = routing
}

// GetForTier returns providers for the given tier, ordered by priority.
func (r *Registry) GetForTier(tier types.ModelTier) []TierEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries, ok := r.tierIndex[tier]
	if !ok {
		return nil
	}
	// Return a copy to avoid race conditions
	result := make([]TierEntry, len(entries))
	copy(result, entries)
	return result
}

// List returns all registered provider names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// ListModels returns all registered models.
func (r *Registry) ListModels() []types.ModelConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var models []types.ModelConfig
	for _, p := range r.providers {
		models = append(models, p.Models()...)
	}
	return models
}

// Close closes all registered providers.
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for name, p := range r.providers {
		if err := p.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close provider %q: %w", name, err))
		}
	}

	// Clear registries
	r.providers = make(map[string]Provider)
	r.chatProviders = make(map[string]ChatProvider)
	r.embeddingProviders = make(map[string]EmbeddingProvider)
	r.modelIndex = make(map[string]string)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing providers: %v", errs)
	}
	return nil
}
