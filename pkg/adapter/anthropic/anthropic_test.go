package anthropic

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

// newTestProvider creates an Anthropic provider pointing at a local test server.
func newTestProvider(serverURL string) *Anthropic {
	cfg := config.ProviderConfig{
		BaseURL: serverURL,
		APIKey:  "sk-ant-test",
	}
	models := []types.ModelConfig{
		{ModelID: "claude-sonnet-4-20250514", Provider: "anthropic"},
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
		if r.URL.Path != "/v1/messages" {
			t.Errorf("expected /v1/messages, got %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "sk-ant-test" {
			t.Errorf("expected x-api-key 'sk-ant-test', got %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("expected anthropic-version '2023-06-01', got %q", r.Header.Get("anthropic-version"))
		}

		var req anthropicRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "claude-sonnet-4-20250514" {
			t.Errorf("expected model claude-sonnet-4-20250514, got %q", req.Model)
		}
		if req.Stream {
			t.Error("expected stream=false for non-stream call")
		}
		if req.MaxTokens != defaultMaxTokens {
			t.Errorf("expected max_tokens %d, got %d", defaultMaxTokens, req.MaxTokens)
		}

		// Return mock Anthropic response
		resp := anthropicResponse{
			ID:    "msg_test_123",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-sonnet-4-20250514",
			Content: []anthropicContentBlock{
				{Type: "text", Text: "Hello! How can I help you?"},
			},
			StopReason: "end_turn",
			Usage: anthropicUsage{
				InputTokens:  10,
				OutputTokens: 8,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	resp, err := p.Chat(context.Background(), &types.ChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("Hello")},
		},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.ID != "msg_test_123" {
		t.Errorf("expected ID 'msg_test_123', got %q", resp.ID)
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
	if resp.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", resp.Provider)
	}
}

func TestChat_SystemMessageExtraction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req anthropicRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify system is extracted to top-level
		if req.System != "You are a helpful assistant." {
			t.Errorf("expected system 'You are a helpful assistant.', got %q", req.System)
		}

		// Verify system message is NOT in messages array
		for _, msg := range req.Messages {
			if msg.Role == "system" {
				t.Error("system message should not be in messages array")
			}
		}

		// Verify only user message remains
		if len(req.Messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(req.Messages))
		}

		resp := anthropicResponse{
			ID: "msg_sys_test", Type: "message", Role: "assistant", Model: "claude-sonnet-4-20250514",
			Content:    []anthropicContentBlock{{Type: "text", Text: "OK"}},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 5, OutputTokens: 1},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	_, err := p.Chat(context.Background(), &types.ChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []types.Message{
			{Role: types.RoleSystem, Content: types.NewTextContent("You are a helpful assistant.")},
			{Role: types.RoleUser, Content: types.NewTextContent("Hello")},
		},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
}

func TestChat_StopSequences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req anthropicRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify stop → stop_sequences mapping
		if len(req.StopSequences) != 2 {
			t.Errorf("expected 2 stop_sequences, got %d", len(req.StopSequences))
		}
		if req.StopSequences[0] != "END" || req.StopSequences[1] != "STOP" {
			t.Errorf("unexpected stop_sequences: %v", req.StopSequences)
		}

		resp := anthropicResponse{
			ID: "msg_stop_test", Type: "message", Role: "assistant", Model: "claude-sonnet-4-20250514",
			Content:    []anthropicContentBlock{{Type: "text", Text: "OK"}},
			StopReason: "stop_sequence",
			Usage:      anthropicUsage{InputTokens: 5, OutputTokens: 1},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	resp, err := p.Chat(context.Background(), &types.ChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("Hello")},
		},
		Stop: []string{"END", "STOP"},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("expected finish_reason 'stop', got %q", resp.FinishReason)
	}
}

func TestChat_WithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req anthropicRequest
		json.NewDecoder(r.Body).Decode(&req)

		if len(req.Tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(req.Tools))
		}
		if req.Tools[0].Name != "get_weather" {
			t.Errorf("expected tool name 'get_weather', got %q", req.Tools[0].Name)
		}
		if req.Tools[0].InputSchema == nil {
			t.Error("expected input_schema to be set")
		}

		// Return tool use response
		resp := anthropicResponse{
			ID: "msg_tool_1", Type: "message", Role: "assistant", Model: "claude-sonnet-4-20250514",
			Content: []anthropicContentBlock{
				{Type: "text", Text: "Let me check the weather."},
				{
					Type:  "tool_use",
					ID:    "toolu_abc123",
					Name:  "get_weather",
					Input: map[string]any{"location": "Beijing"},
				},
			},
			StopReason: "tool_use",
			Usage:      anthropicUsage{InputTokens: 20, OutputTokens: 15},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	resp, err := p.Chat(context.Background(), &types.ChatRequest{
		Model: "claude-sonnet-4-20250514",
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
	if resp.Content != "Let me check the weather." {
		t.Errorf("unexpected content: %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "toolu_abc123" {
		t.Errorf("expected tool call ID 'toolu_abc123', got %q", tc.ID)
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("expected function 'get_weather', got %q", tc.Function.Name)
	}
}

func TestChat_Stream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req anthropicRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Error("expected stream=true for stream call")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		// Anthropic SSE format with named events
		events := []string{
			"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_stream_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-20250514\",\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n",
			"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\" World\"}}\n\n",
			"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n",
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n",
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		}

		for _, event := range events {
			fmt.Fprint(w, event)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	events, err := p.ChatStream(context.Background(), &types.ChatRequest{
		Model:  "claude-sonnet-4-20250514",
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
	if doneUsage != nil && doneUsage.CompletionTokens != 2 {
		t.Errorf("expected completion_tokens 2, got %d", doneUsage.CompletionTokens)
	}
}

func TestChat_401Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Invalid API key",
				"type":    "authentication_error",
			},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	_, err := p.Chat(context.Background(), &types.ChatRequest{
		Model: "claude-sonnet-4-20250514",
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

func TestChat_DynamicCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Dynamic auth with x_api_key should override static key
		apiKey := r.Header.Get("x-api-key")
		if apiKey != "sk-ant-user-key" {
			t.Errorf("expected dynamic key 'sk-ant-user-key', got %q", apiKey)
		}

		resp := anthropicResponse{
			ID: "msg_dyn_1", Type: "message", Role: "assistant", Model: "claude-sonnet-4-20250514",
			Content:    []anthropicContentBlock{{Type: "text", Text: "OK"}},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 1, OutputTokens: 1},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	resp, err := p.Chat(context.Background(), &types.ChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("test")},
		},
		Credentials: map[string]string{"x_api_key": "sk-ant-user-key"},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp.Content != "OK" {
		t.Errorf("unexpected content: %q", resp.Content)
	}
}

func TestMapStopReason(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"end_turn", "stop"},
		{"max_tokens", "length"},
		{"stop_sequence", "stop"},
		{"tool_use", "tool_calls"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		result := mapStopReason(tt.input)
		if result != tt.expected {
			t.Errorf("mapStopReason(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExtractSystemAndConvert(t *testing.T) {
	messages := []types.Message{
		{Role: types.RoleSystem, Content: types.NewTextContent("Be helpful.")},
		{Role: types.RoleSystem, Content: types.NewTextContent("Be concise.")},
		{Role: types.RoleUser, Content: types.NewTextContent("Hello")},
		{Role: types.RoleAssistant, Content: types.NewTextContent("Hi!")},
		{Role: types.RoleUser, Content: types.NewTextContent("How are you?")},
	}

	system, converted := extractSystemAndConvert(messages)

	if system != "Be helpful.\nBe concise." {
		t.Errorf("expected system 'Be helpful.\\nBe concise.', got %q", system)
	}
	if len(converted) != 3 {
		t.Fatalf("expected 3 converted messages, got %d", len(converted))
	}
	if converted[0].Role != "user" {
		t.Errorf("expected first message role 'user', got %q", converted[0].Role)
	}
	if converted[1].Role != "assistant" {
		t.Errorf("expected second message role 'assistant', got %q", converted[1].Role)
	}
}

func TestConvertToolChoice(t *testing.T) {
	tests := []struct {
		input    any
		expected *anthropicToolChoice
	}{
		{"auto", &anthropicToolChoice{Type: "auto"}},
		{"none", nil},
		{"required", &anthropicToolChoice{Type: "any"}},
		{nil, nil},
	}

	for _, tt := range tests {
		result := convertToolChoice(tt.input)
		if tt.expected == nil {
			if result != nil {
				t.Errorf("convertToolChoice(%v) = %v, want nil", tt.input, result)
			}
		} else if result == nil {
			t.Errorf("convertToolChoice(%v) = nil, want %v", tt.input, tt.expected)
		} else if result.Type != tt.expected.Type {
			t.Errorf("convertToolChoice(%v).Type = %q, want %q", tt.input, result.Type, tt.expected.Type)
		}
	}
}

func TestChat_MaxTokensOverride(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req anthropicRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify custom max_tokens is passed through
		if req.MaxTokens != 100 {
			t.Errorf("expected max_tokens 100, got %d", req.MaxTokens)
		}

		resp := anthropicResponse{
			ID: "msg_max_1", Type: "message", Role: "assistant", Model: "claude-sonnet-4-20250514",
			Content:    []anthropicContentBlock{{Type: "text", Text: "OK"}},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 5, OutputTokens: 1},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	maxTokens := 100
	_, err := p.Chat(context.Background(), &types.ChatRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: &maxTokens,
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("Hello")},
		},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
}
