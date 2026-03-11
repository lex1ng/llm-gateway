package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/lex1ng/llm-gateway/pkg/types"
)

// Chat sends a non-streaming chat completion request to Anthropic.
func (p *Anthropic) Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) {
	startTime := time.Now()

	// Build Anthropic request
	anthropicReq := p.buildRequest(req)
	anthropicReq.Stream = false

	// Use dynamic credentials if provided
	auth := p.getAuth(req.Credentials)

	// Make request
	var anthropicResp anthropicResponse
	err := p.client.DoJSON(ctx, http.MethodPost, p.messagesEndpoint(), auth, anthropicReq, &anthropicResp)
	if err != nil {
		return nil, err
	}

	// Convert to unified response
	resp := p.buildResponse(&anthropicResp)
	resp.LatencyMs = time.Since(startTime).Milliseconds()
	resp.Provider = providerName

	return resp, nil
}

// buildRequest converts types.ChatRequest to Anthropic-specific format.
func (p *Anthropic) buildRequest(req *types.ChatRequest) *anthropicRequest {
	// Extract system messages and convert the rest
	system, messages := extractSystemAndConvert(req.Messages)

	ar := &anthropicRequest{
		Model:    req.Model,
		Messages: messages,
	}

	// System prompt (top-level field in Anthropic API)
	if system != "" {
		ar.System = system
	}

	// max_tokens is required in Anthropic API
	if req.MaxTokens != nil {
		ar.MaxTokens = *req.MaxTokens
	} else {
		ar.MaxTokens = p.maxTokens
	}

	// Temperature and TopP
	ar.Temperature = req.Temperature
	ar.TopP = req.TopP

	// stop → stop_sequences
	if len(req.Stop) > 0 {
		ar.StopSequences = req.Stop
	}

	// Convert tools
	if len(req.Tools) > 0 {
		ar.Tools = convertTools(req.Tools)
		ar.ToolChoice = convertToolChoice(req.ToolChoice)
	}

	// Pass through Anthropic-specific extra fields (e.g., "thinking")
	if v, ok := req.Extra["thinking"]; ok {
		ar.Thinking = v
	}

	return ar
}

// buildResponse converts Anthropic response to unified format.
func (p *Anthropic) buildResponse(resp *anthropicResponse) *types.ChatResponse {
	result := &types.ChatResponse{
		ID:    resp.ID,
		Model: resp.Model,
		Usage: types.TokenUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
		FinishReason: mapStopReason(resp.StopReason),
	}

	// Extract text content and tool calls from content blocks
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			result.Content += block.Text
		case "tool_use":
			argsJSON, _ := json.Marshal(block.Input)
			result.ToolCalls = append(result.ToolCalls, types.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: types.FunctionCall{
					Name:      block.Name,
					Arguments: string(argsJSON),
				},
			})
		}
	}

	return result
}

// extractSystemAndConvert separates system messages and converts the rest to Anthropic format.
func extractSystemAndConvert(messages []types.Message) (string, []anthropicMessage) {
	var system string
	var converted []anthropicMessage

	for _, msg := range messages {
		if msg.Role == types.RoleSystem {
			if system != "" {
				system += "\n"
			}
			system += msg.Content.String()
			continue
		}

		am := anthropicMessage{
			Role: mapRole(msg.Role),
		}

		// Handle tool result messages
		if msg.Role == types.RoleTool {
			am.Role = "user"
			am.Content = []anthropicContentBlock{
				{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   msg.Content.String(),
				},
			}
			converted = append(converted, am)
			continue
		}

		// Handle assistant messages with tool calls
		if msg.Role == types.RoleAssistant && len(msg.ToolCalls) > 0 {
			var blocks []anthropicContentBlock
			// Add text content if present
			text := msg.Content.String()
			if text != "" {
				blocks = append(blocks, anthropicContentBlock{
					Type: "text",
					Text: text,
				})
			}
			// Add tool use blocks
			for _, tc := range msg.ToolCalls {
				var input map[string]any
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			am.Content = blocks
			converted = append(converted, am)
			continue
		}

		// Handle multimodal content
		if msg.Content.IsMultimodal() {
			var blocks []anthropicContentBlock
			for _, block := range msg.Content.Blocks {
				blocks = append(blocks, convertContentBlock(block))
			}
			am.Content = blocks
		} else {
			// Simple text content - use string form
			am.Content = msg.Content.String()
		}

		converted = append(converted, am)
	}

	return system, converted
}

// convertContentBlock converts a unified ContentBlock to Anthropic format.
func convertContentBlock(block types.ContentBlock) anthropicContentBlock {
	switch block.Type {
	case "image_url":
		if block.ImageURL != nil {
			return anthropicContentBlock{
				Type: "image",
				Source: &anthropicImageSource{
					Type:      "url",
					URL:       block.ImageURL.URL,
				},
			}
		}
	}
	// Default to text
	return anthropicContentBlock{
		Type: "text",
		Text: block.Text,
	}
}

// mapRole converts unified role to Anthropic role.
func mapRole(role types.Role) string {
	switch role {
	case types.RoleUser:
		return "user"
	case types.RoleAssistant:
		return "assistant"
	default:
		return string(role)
	}
}

// mapStopReason converts Anthropic stop_reason to unified finish_reason.
func mapStopReason(stopReason string) string {
	switch stopReason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool_calls"
	default:
		return stopReason
	}
}

// convertTools converts unified tools to Anthropic format.
func convertTools(tools []types.Tool) []anthropicTool {
	result := make([]anthropicTool, len(tools))
	for i, tool := range tools {
		result[i] = anthropicTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.Parameters,
		}
	}
	return result
}

// convertToolChoice converts unified tool_choice to Anthropic format.
func convertToolChoice(choice any) *anthropicToolChoice {
	if choice == nil {
		return nil
	}

	switch v := choice.(type) {
	case string:
		switch v {
		case "auto":
			return &anthropicToolChoice{Type: "auto"}
		case "none":
			return nil // Anthropic doesn't have "none", just don't send tools
		case "required":
			return &anthropicToolChoice{Type: "any"}
		default:
			return &anthropicToolChoice{Type: "auto"}
		}
	case map[string]any:
		// Specific tool: {"type": "function", "function": {"name": "xxx"}}
		if fn, ok := v["function"].(map[string]any); ok {
			if name, ok := fn["name"].(string); ok {
				return &anthropicToolChoice{
					Type: "tool",
					Name: name,
				}
			}
		}
	}

	return nil
}

// --- Anthropic API Types ---

type anthropicRequest struct {
	Model         string               `json:"model"`
	Messages      []anthropicMessage   `json:"messages"`
	System        string               `json:"system,omitempty"`
	MaxTokens     int                  `json:"max_tokens"`
	Temperature   *float64             `json:"temperature,omitempty"`
	TopP          *float64             `json:"top_p,omitempty"`
	StopSequences []string             `json:"stop_sequences,omitempty"`
	Stream        bool                 `json:"stream,omitempty"`
	Tools         []anthropicTool      `json:"tools,omitempty"`
	ToolChoice    *anthropicToolChoice `json:"tool_choice,omitempty"`
	Thinking      any                  `json:"thinking,omitempty"` // Anthropic extended thinking
}

type anthropicMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content any    `json:"content"` // string or []anthropicContentBlock
}

type anthropicContentBlock struct {
	Type string `json:"type"` // "text", "image", "tool_use", "tool_result"
	Text string `json:"text,omitempty"`

	// Image source
	Source *anthropicImageSource `json:"source,omitempty"`

	// Tool use (in response)
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`

	// Tool result (in request)
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"` // For tool_result type, overloaded with Text
}

// MarshalJSON implements custom marshaling to handle the content/text overlap for tool_result.
func (b anthropicContentBlock) MarshalJSON() ([]byte, error) {
	switch b.Type {
	case "tool_result":
		return json.Marshal(struct {
			Type      string `json:"type"`
			ToolUseID string `json:"tool_use_id"`
			Content   string `json:"content"`
		}{
			Type:      b.Type,
			ToolUseID: b.ToolUseID,
			Content:   b.Content,
		})
	case "tool_use":
		return json.Marshal(struct {
			Type  string         `json:"type"`
			ID    string         `json:"id"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		}{
			Type:  b.Type,
			ID:    b.ID,
			Name:  b.Name,
			Input: b.Input,
		})
	case "image":
		return json.Marshal(struct {
			Type   string                `json:"type"`
			Source *anthropicImageSource `json:"source"`
		}{
			Type:   b.Type,
			Source: b.Source,
		})
	default: // "text"
		return json.Marshal(struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{
			Type: b.Type,
			Text: b.Text,
		})
	}
}

type anthropicImageSource struct {
	Type      string `json:"type"` // "base64" or "url"
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type string `json:"type"` // "auto", "any", "tool"
	Name string `json:"name,omitempty"` // For "tool" type
}

type anthropicResponse struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"` // "message"
	Role       string                  `json:"role"` // "assistant"
	Model      string                  `json:"model"`
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Usage      anthropicUsage          `json:"usage"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
