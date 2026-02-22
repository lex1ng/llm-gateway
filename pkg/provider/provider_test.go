package provider

import (
	"context"
	"testing"

	"github.com/lex1ng/llm-gateway/pkg/types"
)

// mockChatProvider is a minimal ChatProvider for testing.
type mockChatProvider struct {
	name   string
	models []types.ModelConfig
	caps   map[Capability]bool
}

func (m *mockChatProvider) Name() string {
	return m.name
}

func (m *mockChatProvider) Models() []types.ModelConfig {
	return m.models
}

func (m *mockChatProvider) Supports(cap Capability) bool {
	return m.caps[cap]
}

func (m *mockChatProvider) Close() error {
	return nil
}

func (m *mockChatProvider) Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) {
	return &types.ChatResponse{
		ID:      "test-id",
		Content: "Hello, World!",
	}, nil
}

func (m *mockChatProvider) ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan types.StreamEvent, error) {
	ch := make(chan types.StreamEvent, 1)
	ch <- types.NewDoneEvent("stop", nil)
	close(ch)
	return ch, nil
}

// mockBaseProvider only implements Provider (not ChatProvider).
type mockBaseProvider struct {
	name   string
	models []types.ModelConfig
	caps   map[Capability]bool
}

func (m *mockBaseProvider) Name() string {
	return m.name
}

func (m *mockBaseProvider) Models() []types.ModelConfig {
	return m.models
}

func (m *mockBaseProvider) Supports(cap Capability) bool {
	return m.caps[cap]
}

func (m *mockBaseProvider) Close() error {
	return nil
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	provider := &mockChatProvider{
		name: "test-provider",
		models: []types.ModelConfig{
			{ModelID: "test-model-1", Provider: "test-provider"},
			{ModelID: "test-model-2", Provider: "test-provider"},
		},
		caps: map[Capability]bool{
			CapChat:   true,
			CapStream: true,
		},
	}

	err := r.Register(provider)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Verify provider is registered
	p, ok := r.Get("test-provider")
	if !ok {
		t.Fatal("expected provider to be registered")
	}
	if p.Name() != "test-provider" {
		t.Errorf("expected name 'test-provider', got %q", p.Name())
	}
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	r := NewRegistry()

	provider := &mockChatProvider{
		name: "test-provider",
		caps: map[Capability]bool{CapChat: true},
	}

	err := r.Register(provider)
	if err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	// Second registration should fail
	err = r.Register(provider)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestRegistry_Register_InconsistentCapabilities(t *testing.T) {
	r := NewRegistry()

	// Provider implements ChatProvider interface but Supports(CapChat) returns false
	provider := &mockChatProvider{
		name: "inconsistent-provider",
		caps: map[Capability]bool{
			CapChat: false, // This is inconsistent with implementing ChatProvider
		},
	}

	err := r.Register(provider)
	if err == nil {
		t.Error("expected error for inconsistent capabilities")
	}
}

func TestRegistry_GetChatProvider(t *testing.T) {
	r := NewRegistry()

	provider := &mockChatProvider{
		name: "chat-provider",
		caps: map[Capability]bool{CapChat: true},
	}

	err := r.Register(provider)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Get as ChatProvider (type-safe)
	cp, ok := r.GetChatProvider("chat-provider")
	if !ok {
		t.Fatal("expected ChatProvider to be found")
	}

	// Verify it's usable
	resp, err := cp.Chat(context.Background(), &types.ChatRequest{})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp.Content != "Hello, World!" {
		t.Errorf("unexpected response: %q", resp.Content)
	}
}

func TestRegistry_GetChatProvider_NotFound(t *testing.T) {
	r := NewRegistry()

	_, ok := r.GetChatProvider("nonexistent")
	if ok {
		t.Error("expected ChatProvider not to be found")
	}
}

func TestRegistry_GetByModel(t *testing.T) {
	r := NewRegistry()

	provider := &mockChatProvider{
		name: "model-provider",
		models: []types.ModelConfig{
			{ModelID: "gpt-4o", Provider: "model-provider"},
			{ModelID: "gpt-4o-mini", Provider: "model-provider"},
		},
		caps: map[Capability]bool{CapChat: true},
	}

	err := r.Register(provider)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Find by model ID
	p, ok := r.GetByModel("gpt-4o")
	if !ok {
		t.Fatal("expected provider to be found for model")
	}
	if p.Name() != "model-provider" {
		t.Errorf("expected 'model-provider', got %q", p.Name())
	}

	// Non-existent model
	_, ok = r.GetByModel("nonexistent-model")
	if ok {
		t.Error("expected model not to be found")
	}
}

func TestRegistry_GetChatProviderByModel(t *testing.T) {
	r := NewRegistry()

	provider := &mockChatProvider{
		name: "chat-model-provider",
		models: []types.ModelConfig{
			{ModelID: "claude-3-5-sonnet", Provider: "chat-model-provider"},
		},
		caps: map[Capability]bool{CapChat: true},
	}

	err := r.Register(provider)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	cp, ok := r.GetChatProviderByModel("claude-3-5-sonnet")
	if !ok {
		t.Fatal("expected ChatProvider to be found for model")
	}

	resp, err := cp.Chat(context.Background(), &types.ChatRequest{})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp.ID != "test-id" {
		t.Errorf("unexpected response ID: %q", resp.ID)
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()

	providers := []*mockChatProvider{
		{name: "provider-a", caps: map[Capability]bool{CapChat: true}},
		{name: "provider-b", caps: map[Capability]bool{CapChat: true}},
		{name: "provider-c", caps: map[Capability]bool{CapChat: true}},
	}

	for _, p := range providers {
		if err := r.Register(p); err != nil {
			t.Fatalf("Register failed: %v", err)
		}
	}

	names := r.List()
	if len(names) != 3 {
		t.Errorf("expected 3 providers, got %d", len(names))
	}
}

func TestRegistry_ListModels(t *testing.T) {
	r := NewRegistry()

	provider := &mockChatProvider{
		name: "multi-model-provider",
		models: []types.ModelConfig{
			{ModelID: "model-1", Provider: "multi-model-provider"},
			{ModelID: "model-2", Provider: "multi-model-provider"},
			{ModelID: "model-3", Provider: "multi-model-provider"},
		},
		caps: map[Capability]bool{CapChat: true},
	}

	err := r.Register(provider)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := r.ListModels()
	if len(models) != 3 {
		t.Errorf("expected 3 models, got %d", len(models))
	}
}

func TestRegistry_TierRouting(t *testing.T) {
	r := NewRegistry()

	provider := &mockChatProvider{
		name: "tier-provider",
		caps: map[Capability]bool{CapChat: true},
	}

	err := r.Register(provider)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Set tier routing
	r.SetTierRouting(map[types.ModelTier][]TierEntry{
		types.TierSmall: {
			{ProviderName: "tier-provider", ModelID: "small-model", Priority: 1},
		},
		types.TierLarge: {
			{ProviderName: "tier-provider", ModelID: "large-model-1", Priority: 1},
			{ProviderName: "tier-provider", ModelID: "large-model-2", Priority: 2},
		},
	})

	// Get small tier entries
	smallEntries := r.GetForTier(types.TierSmall)
	if len(smallEntries) != 1 {
		t.Errorf("expected 1 small tier entry, got %d", len(smallEntries))
	}

	// Get large tier entries
	largeEntries := r.GetForTier(types.TierLarge)
	if len(largeEntries) != 2 {
		t.Errorf("expected 2 large tier entries, got %d", len(largeEntries))
	}

	// Get non-existent tier
	mediumEntries := r.GetForTier(types.TierMedium)
	if len(mediumEntries) != 0 {
		t.Errorf("expected 0 medium tier entries, got %d", len(mediumEntries))
	}
}

func TestRegistry_Close(t *testing.T) {
	r := NewRegistry()

	provider := &mockChatProvider{
		name:   "closable-provider",
		models: []types.ModelConfig{{ModelID: "test-model", Provider: "closable-provider"}},
		caps:   map[Capability]bool{CapChat: true},
	}

	err := r.Register(provider)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err = r.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Registry should be empty after close
	names := r.List()
	if len(names) != 0 {
		t.Errorf("expected 0 providers after close, got %d", len(names))
	}
}

func TestCapability_DetectCapabilities(t *testing.T) {
	chatProvider := &mockChatProvider{
		name: "chat-only",
		caps: map[Capability]bool{CapChat: true},
	}

	caps := detectCapabilities(chatProvider)

	found := false
	for _, cap := range caps {
		if cap == CapChat {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected CapChat to be detected")
	}
}

func TestCapability_IsInterfaceCapability(t *testing.T) {
	tests := []struct {
		cap      Capability
		expected bool
	}{
		{CapChat, true},
		{CapEmbed, true},
		{CapStream, false},  // Feature-level, not interface-level
		{CapTools, false},   // Feature-level
		{CapVision, false},  // Feature-level
	}

	for _, tt := range tests {
		t.Run(string(tt.cap), func(t *testing.T) {
			result := IsInterfaceCapability(tt.cap)
			if result != tt.expected {
				t.Errorf("IsInterfaceCapability(%s) = %v, want %v", tt.cap, result, tt.expected)
			}
		})
	}
}
