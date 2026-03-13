package manager

import (
	"context"
	"errors"
	"testing"

	"github.com/lex1ng/llm-gateway/config"
	"github.com/lex1ng/llm-gateway/pkg/provider"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

// mockChatProvider is a minimal ChatProvider for testing.
type mockChatProvider struct {
	name   string
	models []types.ModelConfig
}

func (m *mockChatProvider) Name() string                { return m.name }
func (m *mockChatProvider) Models() []types.ModelConfig  { return m.models }
func (m *mockChatProvider) Close() error                 { return nil }
func (m *mockChatProvider) Supports(cap provider.Capability) bool {
	return cap == provider.CapChat || cap == provider.CapStream
}

func (m *mockChatProvider) Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) {
	return &types.ChatResponse{
		ID:           "mock-resp-1",
		Model:        req.Model,
		Provider:     m.name,
		Content:      "mock response from " + m.name,
		FinishReason: "stop",
	}, nil
}

func (m *mockChatProvider) ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan types.StreamEvent, error) {
	ch := make(chan types.StreamEvent, 2)
	ch <- types.NewContentDeltaEvent("streaming from " + m.name)
	ch <- types.NewDoneEvent("stop", nil)
	close(ch)
	return ch, nil
}

// setupTestManager creates a Manager with mock providers.
func setupTestManager(t *testing.T) *Manager {
	t.Helper()

	registry := provider.NewRegistry()

	openaiProvider := &mockChatProvider{
		name: "openai",
		models: []types.ModelConfig{
			{ModelID: "gpt-4o", Provider: "openai", Tier: types.TierLarge},
			{ModelID: "gpt-4o-mini", Provider: "openai", Tier: types.TierSmall},
		},
	}

	anthropicProvider := &mockChatProvider{
		name: "anthropic",
		models: []types.ModelConfig{
			{ModelID: "claude-sonnet-4-20250514", Provider: "anthropic", Tier: types.TierLarge},
		},
	}

	if err := registry.Register(openaiProvider); err != nil {
		t.Fatalf("register openai: %v", err)
	}
	if err := registry.Register(anthropicProvider); err != nil {
		t.Fatalf("register anthropic: %v", err)
	}

	// Set tier routing
	registry.SetTierRouting(map[types.ModelTier][]provider.TierEntry{
		types.TierSmall: {
			{ProviderName: "openai", ModelID: "gpt-4o-mini", Priority: 1},
		},
		types.TierMedium: {
			{ProviderName: "openai", ModelID: "gpt-4o-mini", Priority: 1},
			{ProviderName: "anthropic", ModelID: "claude-sonnet-4-20250514", Priority: 2},
		},
		types.TierLarge: {
			{ProviderName: "openai", ModelID: "gpt-4o", Priority: 1},
			{ProviderName: "anthropic", ModelID: "claude-sonnet-4-20250514", Priority: 2},
		},
	})

	cfg := &config.Config{}

	return &Manager{
		registry: registry,
		router:   NewRouter(registry, cfg),
		config:   cfg,
	}
}

func TestManager_Chat_ByModel(t *testing.T) {
	m := setupTestManager(t)

	resp, err := m.Chat(context.Background(), &types.ChatRequest{
		Model: "gpt-4o",
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("hello")},
		},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", resp.Provider)
	}
	if resp.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %q", resp.Model)
	}
}

func TestManager_Chat_ByModel_Anthropic(t *testing.T) {
	m := setupTestManager(t)

	resp, err := m.Chat(context.Background(), &types.ChatRequest{
		Model: "claude-sonnet-4-20250514",
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", resp.Provider)
	}
}

func TestManager_Chat_ModelNotFound(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.Chat(context.Background(), &types.ChatRequest{
		Model: "nonexistent-model",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent model")
	}

	pe, ok := err.(*types.ProviderError)
	if !ok {
		t.Fatalf("expected *types.ProviderError, got %T", err)
	}
	if pe.Code != types.ErrModelNotFound {
		t.Errorf("expected error code ErrModelNotFound, got %q", pe.Code)
	}
}

func TestManager_Chat_ProviderNotFound(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.Chat(context.Background(), &types.ChatRequest{
		Provider: "nonexistent-provider",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent provider")
	}

	pe, ok := err.(*types.ProviderError)
	if !ok {
		t.Fatalf("expected *types.ProviderError, got %T", err)
	}
	if pe.Code != types.ErrProviderNotFound {
		t.Errorf("expected error code ErrProviderNotFound, got %q", pe.Code)
	}
}

func TestRouter_SelectByTier(t *testing.T) {
	m := setupTestManager(t)

	resp, err := m.Chat(context.Background(), &types.ChatRequest{
		ModelTier: types.TierSmall,
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.Model != "gpt-4o-mini" {
		t.Errorf("expected model 'gpt-4o-mini' for small tier, got %q", resp.Model)
	}
}

func TestRouter_SelectByTier_Large(t *testing.T) {
	m := setupTestManager(t)

	resp, err := m.Chat(context.Background(), &types.ChatRequest{
		ModelTier: types.TierLarge,
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// Should select first priority (openai/gpt-4o)
	if resp.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o' for large tier priority 1, got %q", resp.Model)
	}
}

func TestRouter_SelectByProvider(t *testing.T) {
	m := setupTestManager(t)

	resp, err := m.Chat(context.Background(), &types.ChatRequest{
		Provider: "anthropic",
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", resp.Provider)
	}
}

func TestRouter_SelectByProviderAndTier(t *testing.T) {
	m := setupTestManager(t)

	resp, err := m.Chat(context.Background(), &types.ChatRequest{
		Provider:  "openai",
		ModelTier: types.TierSmall,
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.Model != "gpt-4o-mini" {
		t.Errorf("expected model 'gpt-4o-mini', got %q", resp.Model)
	}
}

func TestRouter_EmptyRequestRequiresExplicitSpec(t *testing.T) {
	m := setupTestManager(t)

	// No model, no provider, no tier → should return error requiring explicit specification
	_, err := m.Chat(context.Background(), &types.ChatRequest{})
	if err == nil {
		t.Fatal("expected error for empty request, got nil")
	}

	var pe *types.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Code != types.ErrInvalidRequest {
		t.Errorf("expected error code %q, got %q", types.ErrInvalidRequest, pe.Code)
	}
}

func TestManager_ChatStream(t *testing.T) {
	m := setupTestManager(t)

	events, err := m.ChatStream(context.Background(), &types.ChatRequest{
		Model: "gpt-4o",
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	var gotContent bool
	var gotDone bool
	for event := range events {
		switch event.Type {
		case types.StreamEventContentDelta:
			gotContent = true
			if event.Delta != "streaming from openai" {
				t.Errorf("unexpected delta: %q", event.Delta)
			}
		case types.StreamEventDone:
			gotDone = true
		}
	}

	if !gotContent {
		t.Error("expected content delta event")
	}
	if !gotDone {
		t.Error("expected done event")
	}
}

func TestManager_ListModels(t *testing.T) {
	m := setupTestManager(t)
	models := m.ListModels()

	if len(models) != 3 {
		t.Errorf("expected 3 models, got %d", len(models))
	}
}

func TestManager_ListProviders(t *testing.T) {
	m := setupTestManager(t)
	providers := m.ListProviders()

	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}
}
