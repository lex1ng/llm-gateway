// Package handler provides HTTP handlers for the LLM Gateway API.
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/lex1ng/llm-gateway/pkg/gateway"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

// ChatHandler handles chat completion requests.
type ChatHandler struct {
	client *gateway.Client
}

// NewChatHandler creates a new ChatHandler.
func NewChatHandler(client *gateway.Client) *ChatHandler {
	return &ChatHandler{client: client}
}

// ServeHTTP handles POST /v1/chat/completions requests.
func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body: "+err.Error())
		return
	}

	// Check if streaming is requested
	if req.Stream {
		h.handleStream(w, r, &req)
	} else {
		h.handleNonStream(w, r, &req)
	}
}

// handleNonStream handles non-streaming chat completion.
func (h *ChatHandler) handleNonStream(w http.ResponseWriter, r *http.Request, req *types.ChatRequest) {
	resp, err := h.client.Chat(r.Context(), req)
	if err != nil {
		handleProviderError(w, err)
		return
	}

	// Convert to OpenAI-compatible response format
	openAIResp := convertToOpenAIResponse(resp)
	writeJSON(w, http.StatusOK, openAIResp)
}

// handleStream handles streaming chat completion.
func (h *ChatHandler) handleStream(w http.ResponseWriter, r *http.Request, req *types.ChatRequest) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	events, err := h.client.ChatStream(r.Context(), req)
	if err != nil {
		handleProviderError(w, err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	for event := range events {
		if event.Type == types.StreamEventError {
			writeSSE(w, "error", fmt.Sprintf(`{"error":{"message":%q}}`, event.Error))
			flusher.Flush()
			return
		}

		chunk := convertStreamEventToChunk(&event)
		data, _ := json.Marshal(chunk)
		writeSSE(w, "", string(data))
		flusher.Flush()

		if event.Type == types.StreamEventDone {
			writeSSE(w, "", "[DONE]")
			flusher.Flush()
			return
		}
	}
}

// OpenAI-compatible response structures

type openAIChatCompletionResponse struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"`
	Created int64                   `json:"created"`
	Model   string                  `json:"model"`
	Choices []openAIChoice          `json:"choices"`
	Usage   openAIUsage             `json:"usage"`
}

type openAIChoice struct {
	Index        int             `json:"index"`
	Message      openAIMessage   `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

type openAIMessage struct {
	Role             string              `json:"role"`
	Content          string              `json:"content"`
	ReasoningContent string              `json:"reasoning_content,omitempty"`
	ToolCalls        []openAIToolCall    `json:"tool_calls,omitempty"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIStreamChunk struct {
	ID      string                `json:"id"`
	Object  string                `json:"object"`
	Created int64                 `json:"created"`
	Model   string                `json:"model"`
	Choices []openAIStreamChoice  `json:"choices"`
	Usage   *openAIUsage          `json:"usage,omitempty"`
}

type openAIStreamChoice struct {
	Index        int              `json:"index"`
	Delta        openAIStreamDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason,omitempty"`
}

type openAIStreamDelta struct {
	Role             string           `json:"role,omitempty"`
	Content          string           `json:"content,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	ToolCalls        []openAIToolCall `json:"tool_calls,omitempty"`
}

func convertToOpenAIResponse(resp *types.ChatResponse) *openAIChatCompletionResponse {
	openAIResp := &openAIChatCompletionResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: resp.CreatedAt,
		Model:   resp.Model,
		Choices: []openAIChoice{
			{
				Index: 0,
				Message: openAIMessage{
					Role:             "assistant",
					Content:          resp.Content,
					ReasoningContent: resp.ReasoningContent,
				},
				FinishReason: resp.FinishReason,
			},
		},
		Usage: openAIUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	// Convert tool calls
	if len(resp.ToolCalls) > 0 {
		openAIResp.Choices[0].Message.ToolCalls = make([]openAIToolCall, len(resp.ToolCalls))
		for i, tc := range resp.ToolCalls {
			openAIResp.Choices[0].Message.ToolCalls[i] = openAIToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: openAIFunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	return openAIResp
}

func convertStreamEventToChunk(event *types.StreamEvent) *openAIStreamChunk {
	chunk := &openAIStreamChunk{
		ID:      "chatcmpl-stream",
		Object:  "chat.completion.chunk",
		Created: 0,
		Choices: []openAIStreamChoice{
			{
				Index: 0,
				Delta: openAIStreamDelta{},
			},
		},
	}

	switch event.Type {
	case types.StreamEventContentDelta:
		chunk.Choices[0].Delta.Content = event.Delta
	case types.StreamEventReasoningDelta:
		chunk.Choices[0].Delta.ReasoningContent = event.Delta
	case types.StreamEventToolCallDelta:
		if event.ToolCall != nil {
			chunk.Choices[0].Delta.ToolCalls = []openAIToolCall{
				{
					ID:   event.ToolCall.ID,
					Type: event.ToolCall.Type,
					Function: openAIFunctionCall{
						Name:      event.ToolCall.Function.Name,
						Arguments: event.ToolCall.Function.Arguments,
					},
				},
			}
		}
	case types.StreamEventDone:
		if event.FinishReason != "" {
			chunk.Choices[0].FinishReason = &event.FinishReason
		}
		if event.Usage != nil {
			chunk.Usage = &openAIUsage{
				PromptTokens:     event.Usage.PromptTokens,
				CompletionTokens: event.Usage.CompletionTokens,
				TotalTokens:      event.Usage.TotalTokens,
			}
		}
	case types.StreamEventUsage:
		if event.Usage != nil {
			chunk.Usage = &openAIUsage{
				PromptTokens:     event.Usage.PromptTokens,
				CompletionTokens: event.Usage.CompletionTokens,
				TotalTokens:      event.Usage.TotalTokens,
			}
		}
	}

	return chunk
}

// --- Error handling ---

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

func writeError(w http.ResponseWriter, status int, errType, message string) {
	writeJSON(w, status, errorResponse{
		Error: errorDetail{
			Message: message,
			Type:    errType,
		},
	})
}

func handleProviderError(w http.ResponseWriter, err error) {
	if pe, ok := err.(*types.ProviderError); ok {
		status := pe.StatusCode
		if status == 0 {
			status = http.StatusInternalServerError
		}
		writeError(w, status, string(pe.Code), pe.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, "server_error", err.Error())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}

func writeSSE(w http.ResponseWriter, event, data string) {
	var sb strings.Builder
	if event != "" {
		sb.WriteString("event: ")
		sb.WriteString(event)
		sb.WriteString("\n")
	}
	sb.WriteString("data: ")
	sb.WriteString(data)
	sb.WriteString("\n\n")
	w.Write([]byte(sb.String()))
}
