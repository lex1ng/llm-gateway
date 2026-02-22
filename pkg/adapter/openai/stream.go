package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/lex1ng/llm-gateway/pkg/transport"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

// ChatStream sends a streaming chat completion request to OpenAI.
func (p *Provider) ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan types.StreamEvent, error) {
	// Build OpenAI request body
	openAIReq := p.buildChatRequest(req)
	openAIReq.Stream = true

	// Use dynamic credentials if provided
	auth := p.getAuth(req.Credentials)

	// Make streaming request
	body, err := p.client.DoStream(ctx, http.MethodPost, p.chatEndpoint(), auth, openAIReq)
	if err != nil {
		return nil, err
	}

	// Create channel and start reading goroutine
	events := make(chan types.StreamEvent, 16)
	go p.readStreamEvents(body, events)

	return events, nil
}

// readStreamEvents reads SSE events from the response body and sends them to the channel.
func (p *Provider) readStreamEvents(body io.ReadCloser, events chan<- types.StreamEvent) {
	defer close(events)
	defer body.Close()

	reader := transport.NewSSEReader(body)

	for {
		sse, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				return
			}
			events <- types.NewErrorEvent(err.Error())
			return
		}

		// Skip empty data
		if sse.Data == "" {
			continue
		}

		// Check for done signal
		if sse.IsDone() {
			return
		}

		// Parse the streaming chunk
		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(sse.Data), &chunk); err != nil {
			events <- types.NewErrorEvent("parse error: " + err.Error())
			continue
		}

		// Convert to unified stream events
		for _, choice := range chunk.Choices {
			// Content delta
			if choice.Delta.Content != "" {
				events <- types.NewContentDeltaEvent(choice.Delta.Content)
			}

			// Tool call delta
			if len(choice.Delta.ToolCalls) > 0 {
				for _, tc := range choice.Delta.ToolCalls {
					events <- types.NewToolCallDeltaEvent(&types.ToolCall{
						ID:   tc.ID,
						Type: tc.Type,
						Function: types.FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					})
				}
			}

			// Finish reason (done)
			if choice.FinishReason != "" {
				var usage *types.TokenUsage
				if chunk.Usage.TotalTokens > 0 {
					usage = &types.TokenUsage{
						PromptTokens:     chunk.Usage.PromptTokens,
						CompletionTokens: chunk.Usage.CompletionTokens,
						TotalTokens:      chunk.Usage.TotalTokens,
					}
				}
				events <- types.NewDoneEvent(choice.FinishReason, usage)
			}
		}

		// Usage event (sent separately in newer API versions)
		if chunk.Usage.TotalTokens > 0 && len(chunk.Choices) == 0 {
			events <- types.NewUsageEvent(types.TokenUsage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			})
		}
	}
}

// --- OpenAI Streaming Types ---

type openAIStreamChunk struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []openAIStreamChoice `json:"choices"`
	Usage   openAIUsage          `json:"usage,omitempty"`
}

type openAIStreamChoice struct {
	Index        int              `json:"index"`
	Delta        openAIStreamDelta `json:"delta"`
	FinishReason string           `json:"finish_reason,omitempty"`
}

type openAIStreamDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
}
