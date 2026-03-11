package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/lex1ng/llm-gateway/pkg/transport"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

// ChatStream sends a streaming chat completion request to Anthropic.
func (p *Anthropic) ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan types.StreamEvent, error) {
	// Build Anthropic request
	anthropicReq := p.buildRequest(req)
	anthropicReq.Stream = true

	// Use dynamic credentials if provided
	auth := p.getAuth(req.Credentials)

	// Make streaming request
	body, err := p.client.DoStream(ctx, http.MethodPost, p.messagesEndpoint(), auth, anthropicReq)
	if err != nil {
		return nil, err
	}

	// Create channel and start reading goroutine
	events := make(chan types.StreamEvent, 16)
	go p.readStreamEvents(ctx, body, events)

	return events, nil
}

// sendEvent sends a stream event to the channel, respecting context cancellation.
func sendEvent(ctx context.Context, events chan<- types.StreamEvent, event types.StreamEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case events <- event:
		return true
	}
}

// readStreamEvents reads Anthropic SSE events and converts them to unified StreamEvents.
//
// Anthropic SSE event types:
//   - message_start: contains message metadata (ignored)
//   - content_block_start: new content block starting (ignored)
//   - content_block_delta: text delta → maps to content_delta
//   - content_block_stop: content block ended (ignored)
//   - message_delta: contains stop_reason and usage → maps to done
//   - message_stop: message complete (ignored, done already sent via message_delta)
//   - ping: keepalive (ignored)
func (p *Anthropic) readStreamEvents(ctx context.Context, body io.ReadCloser, events chan<- types.StreamEvent) {
	defer close(events)
	defer body.Close()

	reader := transport.NewSSEReader(body)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		sse, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				return
			}
			sendEvent(ctx, events, types.NewErrorEvent(err.Error()))
			return
		}

		// Skip empty events
		if sse.Data == "" && sse.Event == "" {
			continue
		}

		// Handle done signal
		if sse.IsDone() {
			return
		}

		// Route by Anthropic event type
		switch sse.Event {
		case "content_block_delta":
			p.handleContentBlockDelta(ctx, sse.Data, events)

		case "message_delta":
			p.handleMessageDelta(ctx, sse.Data, events)

		case "error":
			var errEvent struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal([]byte(sse.Data), &errEvent) == nil {
				sendEvent(ctx, events, types.NewErrorEvent(errEvent.Error.Message))
			}
			return

		// Ignored events: message_start, content_block_start, content_block_stop, message_stop, ping
		default:
			continue
		}
	}
}

// handleContentBlockDelta processes a content_block_delta event.
// Format: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}
func (p *Anthropic) handleContentBlockDelta(ctx context.Context, data string, events chan<- types.StreamEvent) {
	var delta struct {
		Index int `json:"index"`
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
			// Tool use delta fields
			PartialJSON string `json:"partial_json,omitempty"`
		} `json:"delta"`
	}

	if err := json.Unmarshal([]byte(data), &delta); err != nil {
		return
	}

	switch delta.Delta.Type {
	case "text_delta":
		if delta.Delta.Text != "" {
			sendEvent(ctx, events, types.NewContentDeltaEvent(delta.Delta.Text))
		}
	case "input_json_delta":
		// Tool call argument streaming - accumulate partial JSON
		if delta.Delta.PartialJSON != "" {
			sendEvent(ctx, events, types.NewToolCallDeltaEvent(&types.ToolCall{
				Function: types.FunctionCall{
					Arguments: delta.Delta.PartialJSON,
				},
			}))
		}
	}
}

// handleMessageDelta processes a message_delta event.
// Format: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":15}}
func (p *Anthropic) handleMessageDelta(ctx context.Context, data string, events chan<- types.StreamEvent) {
	var msgDelta struct {
		Delta struct {
			StopReason string `json:"stop_reason"`
		} `json:"delta"`
		Usage struct {
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal([]byte(data), &msgDelta); err != nil {
		return
	}

	finishReason := mapStopReason(msgDelta.Delta.StopReason)

	var usage *types.TokenUsage
	if msgDelta.Usage.OutputTokens > 0 {
		usage = &types.TokenUsage{
			CompletionTokens: msgDelta.Usage.OutputTokens,
			TotalTokens:      msgDelta.Usage.OutputTokens, // input_tokens not in message_delta
		}
	}

	sendEvent(ctx, events, types.NewDoneEvent(finishReason, usage))
}
