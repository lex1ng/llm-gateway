package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lex1ng/llm-gateway/pkg/gateway"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

// ============================================================
// Mock server — returns canned OpenAI-compatible responses
// ============================================================

func intPtr(n int) *int { return &n }

// newMockLLMServer creates a mock server that handles chat, stream, and embedding endpoints.
func newMockLLMServer() *httptest.Server {
	mux := http.NewServeMux()

	// GET /models — connectivity check (Ping)
	mux.HandleFunc("/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data":   []any{},
		})
	})

	// POST /chat/completions — non-streaming and streaming
	mux.HandleFunc("/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)

		model, _ := req["model"].(string)
		stream, _ := req["stream"].(bool)

		if stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			flusher, _ := w.(http.Flusher)

			// Send content delta
			chunk := map[string]any{
				"id":      "chatcmpl-mock",
				"object":  "chat.completion.chunk",
				"model":   model,
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "Hello from mock"}}},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			// Send done
			doneChunk := map[string]any{
				"id":      "chatcmpl-mock",
				"object":  "chat.completion.chunk",
				"model":   model,
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}},
			}
			data, _ = json.Marshal(doneChunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		// Non-streaming response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-mock",
			"object":  "chat.completion",
			"model":   model,
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "Hello from mock"}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		})
	})

	// POST /embeddings
	mux.HandleFunc("/embeddings", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)

		model, _ := req["model"].(string)
		input, _ := req["input"].([]any)

		data := make([]any, len(input))
		for i := range input {
			// Return a small 4-dim vector for each input
			data[i] = map[string]any{
				"object":    "embedding",
				"index":     i,
				"embedding": []float64{0.1, 0.2, 0.3, 0.4},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data":   data,
			"model":  model,
			"usage":  map[string]any{"prompt_tokens": 5, "total_tokens": 5},
		})
	})

	return httptest.NewServer(mux)
}

// buildMockClient creates a Client pointing at the mock server.
func buildMockClient(t *testing.T, serverURL string) *gateway.Client {
	t.Helper()
	client, err := gateway.NewBuilder().
		AddProvider("mock", gateway.ProviderOpts{
			BaseURL: serverURL,
			APIKey:  "sk-mock-key",
		}).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return client
}

// ============================================================
// Builder Validation Tests
// ============================================================

func TestBuilder_NoProviders(t *testing.T) {
	_, err := gateway.NewBuilder().Build()
	if err == nil {
		t.Fatal("expected error for empty builder, got nil")
	}
}

func TestBuilder_EmptyName(t *testing.T) {
	_, err := gateway.NewBuilder().
		AddProvider("", gateway.ProviderOpts{BaseURL: "http://x", APIKey: "k"}).
		Build()
	if err == nil {
		t.Fatal("expected error for empty provider name")
	}
}

func TestBuilder_MissingBaseURL(t *testing.T) {
	_, err := gateway.NewBuilder().
		AddProvider("test", gateway.ProviderOpts{APIKey: "k"}).
		Build()
	if err == nil {
		t.Fatal("expected error for missing base_url")
	}
}

func TestBuilder_MissingAPIKey(t *testing.T) {
	_, err := gateway.NewBuilder().
		AddProvider("test", gateway.ProviderOpts{BaseURL: "http://x"}).
		Build()
	if err == nil {
		t.Fatal("expected error for missing api_key")
	}
}

// ============================================================
// Builder Construction Tests (with mock server)
// ============================================================

func TestBuilder_SingleProvider(t *testing.T) {
	srv := newMockLLMServer()
	defer srv.Close()

	client := buildMockClient(t, srv.URL)
	defer client.Close()

	providers := client.ListProviders()
	if len(providers) != 1 || providers[0].Name != "mock" {
		t.Errorf("expected 1 provider named 'mock', got %v", providers)
	}
}

func TestBuilder_MultipleProviders(t *testing.T) {
	srv := newMockLLMServer()
	defer srv.Close()

	client, err := gateway.NewBuilder().
		AddProvider("provider-a", gateway.ProviderOpts{
			BaseURL: srv.URL, APIKey: "key-a",
		}).
		AddProvider("provider-b", gateway.ProviderOpts{
			BaseURL: srv.URL, APIKey: "key-b",
		}).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer client.Close()

	providers := client.ListProviders()
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}

	names := map[string]bool{}
	for _, p := range providers {
		names[p.Name] = true
	}
	if !names["provider-a"] || !names["provider-b"] {
		t.Errorf("expected provider-a and provider-b, got %v", names)
	}
}

func TestBuilder_SetTimeout(t *testing.T) {
	srv := newMockLLMServer()
	defer srv.Close()

	client, err := gateway.NewBuilder().
		AddProvider("mock", gateway.ProviderOpts{
			BaseURL: srv.URL, APIKey: "key",
		}).
		SetTimeout(60 * time.Second).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer client.Close()
}

func TestBuilder_WithDebugOption(t *testing.T) {
	srv := newMockLLMServer()
	defer srv.Close()

	client, err := gateway.NewBuilder().
		AddProvider("mock", gateway.ProviderOpts{
			BaseURL: srv.URL, APIKey: "key",
		}).
		WithOption(gateway.WithDebug()).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer client.Close()
}

func TestBuilder_NoModelCatalog(t *testing.T) {
	srv := newMockLLMServer()
	defer srv.Close()

	client := buildMockClient(t, srv.URL)
	defer client.Close()

	models := client.ListModels()
	if len(models) != 0 {
		t.Errorf("expected 0 models in catalog, got %d", len(models))
	}
}

// ============================================================
// Chat Tests (mock server, no real API calls)
// ============================================================

func TestChat_NonStreaming(t *testing.T) {
	srv := newMockLLMServer()
	defer srv.Close()

	client := buildMockClient(t, srv.URL)
	defer client.Close()

	resp, err := client.Chat(context.Background(), &types.ChatRequest{
		Provider: "mock",
		Model:    "test-model",
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("Hello")},
		},
		MaxTokens: intPtr(10),
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if resp.Content != "Hello from mock" {
		t.Errorf("expected 'Hello from mock', got %q", resp.Content)
	}
	if resp.Model != "test-model" {
		t.Errorf("expected model 'test-model', got %q", resp.Model)
	}
	if resp.Usage.PromptTokens != 10 || resp.Usage.CompletionTokens != 5 {
		t.Errorf("unexpected usage: %+v", resp.Usage)
	}
}

func TestChat_Streaming(t *testing.T) {
	srv := newMockLLMServer()
	defer srv.Close()

	client := buildMockClient(t, srv.URL)
	defer client.Close()

	stream, err := client.ChatStream(context.Background(), &types.ChatRequest{
		Provider: "mock",
		Model:    "test-model",
		Stream:   true,
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("Hello")},
		},
		MaxTokens: intPtr(50),
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	var content string
	var gotDone bool
	for event := range stream {
		switch event.Type {
		case types.StreamEventContentDelta:
			content += event.Delta
		case types.StreamEventDone:
			gotDone = true
		case types.StreamEventError:
			t.Fatalf("stream error: %s", event.Error)
		}
	}

	if content != "Hello from mock" {
		t.Errorf("expected 'Hello from mock', got %q", content)
	}
	if !gotDone {
		t.Error("expected done event")
	}
}

// ============================================================
// Embedding Tests (mock server, no real API calls)
// ============================================================

func TestEmbed(t *testing.T) {
	srv := newMockLLMServer()
	defer srv.Close()

	client := buildMockClient(t, srv.URL)
	defer client.Close()

	resp, err := client.Embed(context.Background(), &types.EmbedRequest{
		Provider: "mock",
		Model:    "text-embedding-mock",
		Input:    []string{"hello", "world"},
	})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(resp.Data))
	}
	if resp.Model != "text-embedding-mock" {
		t.Errorf("expected model 'text-embedding-mock', got %q", resp.Model)
	}

	for i, item := range resp.Data {
		if len(item.Embedding) != 4 {
			t.Errorf("embedding[%d]: expected 4 dims, got %d", i, len(item.Embedding))
		}
	}
	if resp.Usage.PromptTokens != 5 {
		t.Errorf("expected 5 prompt tokens, got %d", resp.Usage.PromptTokens)
	}
}

// ============================================================
// Error Handling Tests (mock server, no real API calls)
// ============================================================

func TestError_NonexistentProvider(t *testing.T) {
	srv := newMockLLMServer()
	defer srv.Close()

	client := buildMockClient(t, srv.URL)
	defer client.Close()

	_, err := client.Chat(context.Background(), &types.ChatRequest{
		Provider: "nonexistent",
		Model:    "some-model",
		Messages: []types.Message{{Role: types.RoleUser, Content: types.NewTextContent("test")}},
	})
	if err == nil {
		t.Fatal("expected error for nonexistent provider")
	}

	var pe *types.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Code != types.ErrProviderNotFound {
		t.Errorf("expected code %q, got %q", types.ErrProviderNotFound, pe.Code)
	}
}

func TestError_EmptyRequest(t *testing.T) {
	srv := newMockLLMServer()
	defer srv.Close()

	client := buildMockClient(t, srv.URL)
	defer client.Close()

	_, err := client.Chat(context.Background(), &types.ChatRequest{
		Messages: []types.Message{{Role: types.RoleUser, Content: types.NewTextContent("test")}},
	})
	if err == nil {
		t.Fatal("expected error for empty request (no model, no provider)")
	}

	var pe *types.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Code != types.ErrInvalidRequest {
		t.Errorf("expected code %q, got %q", types.ErrInvalidRequest, pe.Code)
	}
}

func TestError_ContextTimeout(t *testing.T) {
	srv := newMockLLMServer()
	defer srv.Close()

	client := buildMockClient(t, srv.URL)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond) // ensure deadline exceeded

	_, err := client.Chat(ctx, &types.ChatRequest{
		Provider: "mock",
		Model:    "test-model",
		Messages: []types.Message{{Role: types.RoleUser, Content: types.NewTextContent("test")}},
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestError_EmbeddingNoModel(t *testing.T) {
	srv := newMockLLMServer()
	defer srv.Close()

	client := buildMockClient(t, srv.URL)
	defer client.Close()

	_, err := client.Embed(context.Background(), &types.EmbedRequest{
		Input: []string{"test"},
	})
	if err == nil {
		t.Fatal("expected error for embedding without model")
	}

	var pe *types.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
}

// ============================================================
// Multi-Provider Routing Tests
// ============================================================

func TestMultiProvider_RouteByProvider(t *testing.T) {
	srv := newMockLLMServer()
	defer srv.Close()

	client, err := gateway.NewBuilder().
		AddProvider("provider-a", gateway.ProviderOpts{
			BaseURL: srv.URL, APIKey: "key-a",
		}).
		AddProvider("provider-b", gateway.ProviderOpts{
			BaseURL: srv.URL, APIKey: "key-b",
		}).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer client.Close()

	// Route to provider-a
	resp1, err := client.Chat(context.Background(), &types.ChatRequest{
		Provider:  "provider-a",
		Model:     "model-a",
		Messages:  []types.Message{{Role: types.RoleUser, Content: types.NewTextContent("hi")}},
		MaxTokens: intPtr(10),
	})
	if err != nil {
		t.Fatalf("provider-a Chat: %v", err)
	}
	if resp1.Model != "model-a" {
		t.Errorf("expected model-a, got %q", resp1.Model)
	}

	// Route to provider-b
	resp2, err := client.Chat(context.Background(), &types.ChatRequest{
		Provider:  "provider-b",
		Model:     "model-b",
		Messages:  []types.Message{{Role: types.RoleUser, Content: types.NewTextContent("hi")}},
		MaxTokens: intPtr(10),
	})
	if err != nil {
		t.Fatalf("provider-b Chat: %v", err)
	}
	if resp2.Model != "model-b" {
		t.Errorf("expected model-b, got %q", resp2.Model)
	}
}
