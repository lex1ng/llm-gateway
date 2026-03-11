package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lex1ng/llm-gateway/config"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

// newTestProvider creates a Provider pointing at a local test server.
func newTestProvider(serverURL string) *OpenAI {
	cfg := config.ProviderConfig{
		BaseURL: serverURL,
		APIKey:  "sk-test",
	}
	models := []types.ModelConfig{
		{ModelID: "gpt-4o", Provider: "openai"},
	}
	p, _ := New(cfg, models)
	return p
}

func TestChat_NonStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("expected Bearer auth, got %q", r.Header.Get("Authorization"))
		}

		var req openAIChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "gpt-4o" {
			t.Errorf("expected model gpt-4o, got %q", req.Model)
		}
		if req.Stream {
			t.Error("expected stream=false for non-stream call")
		}

		// Return mock response
		resp := openAIChatResponse{
			ID:      "chatcmpl-test-123",
			Object:  "chat.completion",
			Created: 1700000000,
			Model:   "gpt-4o",
			Choices: []openAIChoice{
				{
					Index: 0,
					Message: openAIMessage{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: openAIUsage{
				PromptTokens:     10,
				CompletionTokens: 8,
				TotalTokens:      18,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	resp, err := p.Chat(context.Background(), &types.ChatRequest{
		Model: "gpt-4o",
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("Hello")},
		},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.ID != "chatcmpl-test-123" {
		t.Errorf("expected ID 'chatcmpl-test-123', got %q", resp.ID)
	}
	if resp.Content != "Hello! How can I help you?" {
		t.Errorf("unexpected content: %q", resp.Content)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("expected finish_reason 'stop', got %q", resp.FinishReason)
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("expected prompt_tokens 10, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 8 {
		t.Errorf("expected completion_tokens 8, got %d", resp.Usage.CompletionTokens)
	}
	if resp.Usage.TotalTokens != 18 {
		t.Errorf("expected total_tokens 18, got %d", resp.Usage.TotalTokens)
	}
	if resp.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", resp.Provider)
	}
	if resp.LatencyMs < 0 {
		t.Error("expected non-negative latency")
	}
}

func TestChat_Stream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Error("expected stream=true for stream call")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		// Send content deltas
		chunks := []openAIStreamChunk{
			{
				ID: "chatcmpl-stream-1", Object: "chat.completion.chunk", Model: "gpt-4o",
				Choices: []openAIStreamChoice{{Index: 0, Delta: openAIStreamDelta{Content: "Hello"}}},
			},
			{
				ID: "chatcmpl-stream-1", Object: "chat.completion.chunk", Model: "gpt-4o",
				Choices: []openAIStreamChoice{{Index: 0, Delta: openAIStreamDelta{Content: " World"}}},
			},
			{
				ID: "chatcmpl-stream-1", Object: "chat.completion.chunk", Model: "gpt-4o",
				Choices: []openAIStreamChoice{{Index: 0, FinishReason: "stop"}},
				Usage:   openAIUsage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7},
			},
		}

		for _, chunk := range chunks {
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	events, err := p.ChatStream(context.Background(), &types.ChatRequest{
		Model:  "gpt-4o",
		Stream: true,
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("Hi")},
		},
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	var contentParts []string
	var gotDone bool
	var doneUsage *types.TokenUsage

	for event := range events {
		switch event.Type {
		case types.StreamEventContentDelta:
			contentParts = append(contentParts, event.Delta)
		case types.StreamEventDone:
			gotDone = true
			doneUsage = event.Usage
		case types.StreamEventError:
			t.Fatalf("unexpected error event: %s", event.Error)
		}
	}

	fullContent := ""
	for _, p := range contentParts {
		fullContent += p
	}
	if fullContent != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", fullContent)
	}
	if !gotDone {
		t.Error("expected done event")
	}
	if doneUsage != nil && doneUsage.TotalTokens != 7 {
		t.Errorf("expected total_tokens 7, got %d", doneUsage.TotalTokens)
	}
}

func TestChat_401Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Incorrect API key provided",
				"type":    "invalid_request_error",
				"code":    "invalid_api_key",
			},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	_, err := p.Chat(context.Background(), &types.ChatRequest{
		Model: "gpt-4o",
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("Hello")},
		},
	})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}

	pe, ok := err.(*types.ProviderError)
	if !ok {
		t.Fatalf("expected *types.ProviderError, got %T: %v", err, err)
	}
	if pe.StatusCode != 401 {
		t.Errorf("expected status 401, got %d", pe.StatusCode)
	}
}

func TestChat_404ModelNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "The model 'nonexistent' does not exist",
				"type":    "invalid_request_error",
				"code":    "model_not_found",
			},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	_, err := p.Chat(context.Background(), &types.ChatRequest{
		Model: "nonexistent",
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("Hello")},
		},
	})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	pe, ok := err.(*types.ProviderError)
	if !ok {
		t.Fatalf("expected *types.ProviderError, got %T: %v", err, err)
	}
	if pe.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", pe.StatusCode)
	}
}

func TestChat_WithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIChatRequest
		json.NewDecoder(r.Body).Decode(&req)

		if len(req.Tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(req.Tools))
		}

		resp := openAIChatResponse{
			ID: "chatcmpl-tool-1", Object: "chat.completion", Model: "gpt-4o",
			Choices: []openAIChoice{
				{
					Index: 0,
					Message: openAIMessage{
						Role: "assistant",
						ToolCalls: []openAIToolCall{
							{
								ID:   "call_abc123",
								Type: "function",
								Function: openAIFunctionCall{
									Name:      "get_weather",
									Arguments: `{"location":"Beijing"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: openAIUsage{PromptTokens: 20, CompletionTokens: 15, TotalTokens: 35},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	resp, err := p.Chat(context.Background(), &types.ChatRequest{
		Model: "gpt-4o",
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("What's the weather in Beijing?")},
		},
		Tools: []types.Tool{
			{
				Type: "function",
				Function: types.ToolFunction{
					Name:        "get_weather",
					Description: "Get current weather",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.FinishReason != "tool_calls" {
		t.Errorf("expected finish_reason 'tool_calls', got %q", resp.FinishReason)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "call_abc123" {
		t.Errorf("expected tool call ID 'call_abc123', got %q", tc.ID)
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("expected function 'get_weather', got %q", tc.Function.Name)
	}
}

func TestChat_DynamicCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The dynamic auth should override the static key
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer sk-user-key" {
			t.Errorf("expected dynamic key 'sk-user-key', got %q", authHeader)
		}

		resp := openAIChatResponse{
			ID: "chatcmpl-dyn-1", Object: "chat.completion", Model: "gpt-4o",
			Choices: []openAIChoice{{Index: 0, Message: openAIMessage{Role: "assistant", Content: "OK"}, FinishReason: "stop"}},
			Usage:   openAIUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	resp, err := p.Chat(context.Background(), &types.ChatRequest{
		Model: "gpt-4o",
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("test")},
		},
		Credentials: map[string]string{"api_key": "sk-user-key"},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp.Content != "OK" {
		t.Errorf("unexpected content: %q", resp.Content)
	}
}
