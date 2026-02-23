package openai

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/lex1ng/llm-gateway/pkg/transport"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

// Chat sends a non-streaming chat completion request to OpenAI.
func (p *OpenAI) Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) {
	startTime := time.Now()

	// Build OpenAI request body
	openAIReq := p.buildChatRequest(req)
	openAIReq.Stream = false

	// Use dynamic credentials if provided
	auth := p.getAuth(req.Credentials)

	// Make request
	var openAIResp openAIChatResponse
	err := p.client.DoJSON(ctx, http.MethodPost, p.chatEndpoint(), auth, openAIReq, &openAIResp)
	if err != nil {
		return nil, err
	}

	// Convert to unified response
	resp := p.buildChatResponse(&openAIResp)
	resp.LatencyMs = time.Since(startTime).Milliseconds()
	resp.Provider = providerName

	return resp, nil
}

// getAuth returns the appropriate auth strategy, with dynamic credentials taking priority.
func (p *OpenAI) getAuth(credentials map[string]string) transport.AuthStrategy {
	if len(credentials) > 0 {
		return transport.WithDynamicCredentials(p.auth, credentials)
	}
	return p.auth
}

// buildChatRequest converts types.ChatRequest to OpenAI-specific format.
// Since our internal format is OpenAI-compatible, this is mostly a pass-through.
func (p *OpenAI) buildChatRequest(req *types.ChatRequest) *openAIChatRequest {
	openAIReq := &openAIChatRequest{
		Model:           req.Model,
		Messages:        convertMessages(req.Messages),
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		Stream:          req.Stream,
		Stop:            req.Stop,
		ReasoningEffort: req.ReasoningEffort,
	}

	// Use max_completion_tokens for newer models, max_tokens for older ones
	if req.MaxTokens != nil {
		if usesMaxCompletionTokens(req.Model) {
			openAIReq.MaxCompletionTokens = req.MaxTokens
		} else {
			openAIReq.MaxTokens = req.MaxTokens
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		openAIReq.Tools = convertTools(req.Tools)
		openAIReq.ToolChoice = req.ToolChoice
	}

	// Response format
	if req.ResponseFormat != nil {
		openAIReq.ResponseFormat = &openAIResponseFormat{
			Type: req.ResponseFormat.Type,
		}
	}

	return openAIReq
}

// usesMaxCompletionTokens returns true for models that require max_completion_tokens
// instead of max_tokens (newer OpenAI models: o1, o3, o4, gpt-5, etc.)
func usesMaxCompletionTokens(model string) bool {
	model = strings.ToLower(model)
	// Reasoning models (o1, o3, o4 series)
	if strings.HasPrefix(model, "o1") ||
		strings.HasPrefix(model, "o3") ||
		strings.HasPrefix(model, "o4") {
		return true
	}
	// GPT-5 series
	if strings.HasPrefix(model, "gpt-5") {
		return true
	}
	// GPT-4.1+ series (gpt-4.1, gpt-4.2, etc.)
	if strings.HasPrefix(model, "gpt-4.") {
		return true
	}
	return false
}

// buildChatResponse converts OpenAI response to unified format.
func (p *OpenAI) buildChatResponse(resp *openAIChatResponse) *types.ChatResponse {
	result := &types.ChatResponse{
		ID:        resp.ID,
		Model:     resp.Model,
		CreatedAt: resp.Created,
		Usage: types.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	// Extract content from first choice
	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		// Content can be string or nil
		if content, ok := choice.Message.Content.(string); ok {
			result.Content = content
		}
		result.FinishReason = choice.FinishReason

		// Convert tool calls
		if len(choice.Message.ToolCalls) > 0 {
			result.ToolCalls = make([]types.ToolCall, len(choice.Message.ToolCalls))
			for i, tc := range choice.Message.ToolCalls {
				result.ToolCalls[i] = types.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: types.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
	}

	return result
}

// --- OpenAI API Types ---

type openAIChatRequest struct {
	Model               string                 `json:"model"`
	Messages            []openAIMessage        `json:"messages"`
	MaxTokens           *int                   `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int                   `json:"max_completion_tokens,omitempty"` // For newer models (o1, o3, gpt-5, etc.)
	Temperature         *float64               `json:"temperature,omitempty"`
	TopP                *float64               `json:"top_p,omitempty"`
	Stream              bool                   `json:"stream,omitempty"`
	Stop                []string               `json:"stop,omitempty"`
	Tools               []openAITool           `json:"tools,omitempty"`
	ToolChoice          any                    `json:"tool_choice,omitempty"`
	ResponseFormat      *openAIResponseFormat  `json:"response_format,omitempty"`
	ReasoningEffort     string                 `json:"reasoning_effort,omitempty"` // "none", "minimal", "low", "medium", "high"
}

type openAIMessage struct {
	Role       string          `json:"role"`
	Content    any             `json:"content"` // string or []contentPart
	Name       string          `json:"name,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type openAIContentPart struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	ImageURL *openAIImageURL   `json:"image_url,omitempty"`
}

type openAIImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type openAITool struct {
	Type     string           `json:"type"`
	Function openAIFunction   `json:"function"`
}

type openAIFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAIToolCall struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Function openAIFunctionCall  `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIResponseFormat struct {
	Type string `json:"type"`
}

type openAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

type openAIChoice struct {
	Index        int              `json:"index"`
	Message      openAIMessage    `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- Conversion helpers ---

func convertMessages(messages []types.Message) []openAIMessage {
	result := make([]openAIMessage, len(messages))
	for i, msg := range messages {
		result[i] = convertMessage(msg)
	}
	return result
}

func convertMessage(msg types.Message) openAIMessage {
	openAIMsg := openAIMessage{
		Role:       string(msg.Role),
		Name:       msg.Name,
		ToolCallID: msg.ToolCallID,
	}

	// Handle multimodal content
	if msg.Content.IsMultimodal() {
		parts := make([]openAIContentPart, len(msg.Content.Blocks))
		for i, block := range msg.Content.Blocks {
			parts[i] = convertContentBlock(block)
		}
		openAIMsg.Content = parts
	} else {
		openAIMsg.Content = msg.Content.String()
	}

	// Convert tool calls
	if len(msg.ToolCalls) > 0 {
		openAIMsg.ToolCalls = make([]openAIToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			openAIMsg.ToolCalls[i] = openAIToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: openAIFunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	return openAIMsg
}

func convertContentBlock(block types.ContentBlock) openAIContentPart {
	switch block.Type {
	case "text":
		return openAIContentPart{Type: "text", Text: block.Text}
	case "image_url":
		return openAIContentPart{
			Type: "image_url",
			ImageURL: &openAIImageURL{
				URL:    block.ImageURL.URL,
				Detail: string(block.ImageURL.Detail),
			},
		}
	default:
		return openAIContentPart{Type: "text", Text: block.Text}
	}
}

func convertTools(tools []types.Tool) []openAITool {
	result := make([]openAITool, len(tools))
	for i, tool := range tools {
		result[i] = openAITool{
			Type: tool.Type,
			Function: openAIFunction{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			},
		}
	}
	return result
}
