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

func TestResponses_NonStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/responses" {
			t.Errorf("expected /responses, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("expected Bearer auth, got %q", r.Header.Get("Authorization"))
		}

		var req openAIResponsesRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "gpt-4o" {
			t.Errorf("expected model gpt-4o, got %q", req.Model)
		}
		if req.Stream {
			t.Error("expected stream=false for non-stream call")
		}

		// Return mock response
		resp := openAIResponsesResponse{
			ID:        "resp_test_123",
			Object:    "response",
			CreatedAt: 1700000000,
			Status:    "completed",
			Model:     "gpt-4o",
			Output: []openAIResponseOutputItem{
				{
					Type: "message",
					ID:   "msg_123",
					Role: "assistant",
					Content: []openAIResponseContentPart{
						{Type: "output_text", Text: "Hello! How can I help you?"},
					},
				},
			},
			Usage: &openAIResponsesUsage{
				InputTokens:  10,
				OutputTokens: 8,
				TotalTokens:  18,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	resp, err := p.Responses(context.Background(), &types.ResponsesRequest{
		Model: "gpt-4o",
		Input: "Hello",
	})
	if err != nil {
		t.Fatalf("Responses failed: %v", err)
	}

	if resp.ID != "resp_test_123" {
		t.Errorf("expected ID 'resp_test_123', got %q", resp.ID)
	}
	if resp.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", resp.Status)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(resp.Output))
	}
	if resp.Output[0].Type != "message" {
		t.Errorf("expected output type 'message', got %q", resp.Output[0].Type)
	}
	if len(resp.Output[0].Content) != 1 {
		t.Fatalf("expected 1 content part, got %d", len(resp.Output[0].Content))
	}
	if resp.Output[0].Content[0].Text != "Hello! How can I help you?" {
		t.Errorf("unexpected content: %q", resp.Output[0].Content[0].Text)
	}
	if resp.Usage == nil {
		t.Fatal("expected usage, got nil")
	}
	if resp.Usage.TotalTokens != 18 {
		t.Errorf("expected total_tokens 18, got %d", resp.Usage.TotalTokens)
	}
	if resp.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", resp.Provider)
	}
	// LatencyMs may be 0 for very fast local tests, just ensure it's non-negative
	if resp.LatencyMs < 0 {
		t.Error("expected non-negative latency")
	}
}

func TestResponses_Stream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIResponsesRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Error("expected stream=true for stream call")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		// Send streaming events
		events := []openAIResponsesStreamEvent{
			{
				Type: "response.created",
				Response: &openAIResponsesResponse{
					ID: "resp_stream_1", Object: "response", Status: "in_progress", Model: "gpt-4o",
				},
			},
			{
				Type: "response.output_item.added",
				Item: &openAIResponseOutputItem{
					Type: "message", ID: "msg_1", Role: "assistant",
				},
			},
			{
				Type:         "response.content_part.delta",
				ContentIndex: 0,
				Delta:        &openAIContentDelta{Type: "text_delta", Text: "Hello"},
			},
			{
				Type:         "response.content_part.delta",
				ContentIndex: 0,
				Delta:        &openAIContentDelta{Type: "text_delta", Text: " World"},
			},
			{
				Type: "response.done",
				Response: &openAIResponsesResponse{
					ID: "resp_stream_1", Object: "response", Status: "completed", Model: "gpt-4o",
					Usage: &openAIResponsesUsage{InputTokens: 5, OutputTokens: 2, TotalTokens: 7},
				},
			},
		}

		for _, event := range events {
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	events, err := p.ResponsesStream(context.Background(), &types.ResponsesRequest{
		Model:  "gpt-4o",
		Stream: true,
		Input:  "Hi",
	})
	if err != nil {
		t.Fatalf("ResponsesStream failed: %v", err)
	}

	var contentParts []string
	var gotDone bool
	var doneUsage *types.ResponsesUsage

	for event := range events {
		switch event.Type {
		case types.ResponsesEventContentDelta:
			if event.Delta != nil {
				contentParts = append(contentParts, event.Delta.Text)
			}
		case types.ResponsesEventDone:
			gotDone = true
			if event.Response != nil {
				doneUsage = event.Response.Usage
			}
		case types.ResponsesEventError:
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

func TestResponses_WithInstructions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIResponsesRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Instructions != "You are a helpful assistant" {
			t.Errorf("expected instructions 'You are a helpful assistant', got %q", req.Instructions)
		}

		resp := openAIResponsesResponse{
			ID: "resp_instr_1", Object: "response", Status: "completed", Model: "gpt-4o",
			Output: []openAIResponseOutputItem{
				{Type: "message", ID: "msg_1", Role: "assistant", Content: []openAIResponseContentPart{{Type: "output_text", Text: "OK"}}},
			},
			Usage: &openAIResponsesUsage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	resp, err := p.Responses(context.Background(), &types.ResponsesRequest{
		Model:        "gpt-4o",
		Input:        "test",
		Instructions: "You are a helpful assistant",
	})
	if err != nil {
		t.Fatalf("Responses failed: %v", err)
	}
	if resp.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", resp.Status)
	}
}

func TestResponses_WithFunctionTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIResponsesRequest
		json.NewDecoder(r.Body).Decode(&req)

		if len(req.Tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(req.Tools))
		}
		if req.Tools[0].Type != "function" {
			t.Errorf("expected tool type 'function', got %q", req.Tools[0].Type)
		}
		if req.Tools[0].Function == nil || req.Tools[0].Function.Name != "get_weather" {
			t.Error("expected function name 'get_weather'")
		}

		resp := openAIResponsesResponse{
			ID: "resp_tool_1", Object: "response", Status: "completed", Model: "gpt-4o",
			Output: []openAIResponseOutputItem{
				{
					Type:      "function_call",
					ID:        "call_123",
					CallID:    "call_123",
					Name:      "get_weather",
					Arguments: `{"location":"Beijing"}`,
				},
			},
			Usage: &openAIResponsesUsage{InputTokens: 20, OutputTokens: 15, TotalTokens: 35},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	resp, err := p.Responses(context.Background(), &types.ResponsesRequest{
		Model: "gpt-4o",
		Input: "What's the weather in Beijing?",
		Tools: []types.ResponseTool{
			{
				Type: "function",
				Function: &types.ToolFunction{
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
		t.Fatalf("Responses failed: %v", err)
	}

	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(resp.Output))
	}
	if resp.Output[0].Type != "function_call" {
		t.Errorf("expected output type 'function_call', got %q", resp.Output[0].Type)
	}
	if resp.Output[0].Name != "get_weather" {
		t.Errorf("expected function name 'get_weather', got %q", resp.Output[0].Name)
	}
}

func TestResponses_DynamicCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The dynamic auth should override the static key
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer sk-user-key" {
			t.Errorf("expected dynamic key 'sk-user-key', got %q", authHeader)
		}

		resp := openAIResponsesResponse{
			ID: "resp_dyn_1", Object: "response", Status: "completed", Model: "gpt-4o",
			Output: []openAIResponseOutputItem{
				{Type: "message", ID: "msg_1", Role: "assistant", Content: []openAIResponseContentPart{{Type: "output_text", Text: "OK"}}},
			},
			Usage: &openAIResponsesUsage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	resp, err := p.Responses(context.Background(), &types.ResponsesRequest{
		Model:       "gpt-4o",
		Input:       "test",
		Credentials: map[string]string{"api_key": "sk-user-key"},
	})
	if err != nil {
		t.Fatalf("Responses failed: %v", err)
	}
	if resp.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", resp.Status)
	}
}

func TestResponses_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Invalid model specified",
				"type":    "invalid_request_error",
				"code":    "invalid_model",
			},
		})
	}))
	defer server.Close()

	cfg := config.ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "sk-test",
	}
	models := []types.ModelConfig{
		{ModelID: "gpt-4o", Provider: "openai"},
	}
	p, _ := New(cfg, models)

	_, err := p.Responses(context.Background(), &types.ResponsesRequest{
		Model: "invalid-model",
		Input: "Hello",
	})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}

	pe, ok := err.(*types.ProviderError)
	if !ok {
		t.Fatalf("expected *types.ProviderError, got %T: %v", err, err)
	}
	if pe.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", pe.StatusCode)
	}
}
