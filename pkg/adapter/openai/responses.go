package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/lex1ng/llm-gateway/pkg/transport"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

// Responses sends a non-streaming request to the OpenAI Responses API.
func (p *OpenAI) Responses(ctx context.Context, req *types.ResponsesRequest) (*types.ResponsesResponse, error) {
	startTime := time.Now()

	// Build OpenAI request body
	openAIReq := p.buildResponsesRequest(req)

	// Use dynamic credentials if provided
	auth := p.getAuth(req.Credentials)

	// Make request
	var openAIResp openAIResponsesResponse
	err := p.client.DoJSON(ctx, http.MethodPost, p.responsesEndpoint(), auth, openAIReq, &openAIResp)
	if err != nil {
		return nil, err
	}

	// Convert to unified response
	resp := p.buildResponsesResponse(&openAIResp)
	resp.LatencyMs = time.Since(startTime).Milliseconds()
	resp.Provider = p.name

	return resp, nil
}

// ResponsesStream sends a streaming request to the OpenAI Responses API.
func (p *OpenAI) ResponsesStream(ctx context.Context, req *types.ResponsesRequest) (<-chan types.ResponsesStreamEvent, error) {
	// Build OpenAI request body
	openAIReq := p.buildResponsesRequest(req)
	openAIReq.Stream = true

	// Use dynamic credentials if provided
	auth := p.getAuth(req.Credentials)

	// Make streaming request
	body, err := p.client.DoStream(ctx, http.MethodPost, p.responsesEndpoint(), auth, openAIReq)
	if err != nil {
		return nil, err
	}

	// Create channel and start reading goroutine
	events := make(chan types.ResponsesStreamEvent, 16)
	go p.readResponsesStreamEvents(ctx, body, events)

	return events, nil
}

// responsesEndpoint returns the Responses API endpoint URL.
func (p *OpenAI) responsesEndpoint() string {
	return p.baseURL + "/responses"
}

// buildResponsesRequest converts types.ResponsesRequest to OpenAI-specific format.
func (p *OpenAI) buildResponsesRequest(req *types.ResponsesRequest) *openAIResponsesRequest {
	openAIReq := &openAIResponsesRequest{
		Model:              req.Model,
		Input:              req.Input,
		Instructions:       req.Instructions,
		MaxOutputTokens:    req.MaxOutputTokens,
		Temperature:        req.Temperature,
		TopP:               req.TopP,
		Stream:             req.Stream,
		Modalities:         req.Modalities,
		ReasoningEffort:    req.ReasoningEffort,
		ToolChoice:         req.ToolChoice,
		PreviousResponseID: req.PreviousResponseID,
	}

	// Convert tools
	if len(req.Tools) > 0 {
		openAIReq.Tools = convertResponsesTools(req.Tools)
	}

	return openAIReq
}

// buildResponsesResponse converts OpenAI response to unified format.
func (p *OpenAI) buildResponsesResponse(resp *openAIResponsesResponse) *types.ResponsesResponse {
	result := &types.ResponsesResponse{
		ID:        resp.ID,
		Object:    resp.Object,
		CreatedAt: resp.CreatedAt,
		Status:    resp.Status,
		Model:     resp.Model,
	}

	// Convert output items
	if len(resp.Output) > 0 {
		result.Output = make([]types.ResponseOutputItem, len(resp.Output))
		for i, item := range resp.Output {
			result.Output[i] = convertOutputItem(item)
		}
	}

	// Convert usage
	if resp.Usage != nil {
		result.Usage = &types.ResponsesUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
		if resp.Usage.InputTokensDetails != nil {
			result.Usage.InputTokensDetails = &types.InputTokensDetails{
				CachedTokens: resp.Usage.InputTokensDetails.CachedTokens,
			}
		}
		if resp.Usage.OutputTokensDetails != nil {
			result.Usage.OutputTokensDetails = &types.OutputTokensDetails{
				ReasoningTokens: resp.Usage.OutputTokensDetails.ReasoningTokens,
			}
		}
	}

	// Convert error
	if resp.Error != nil {
		result.Error = &types.ResponseError{
			Code:    resp.Error.Code,
			Message: resp.Error.Message,
		}
	}

	return result
}

// readResponsesStreamEvents reads SSE events from the Responses API stream.
func (p *OpenAI) readResponsesStreamEvents(ctx context.Context, body io.ReadCloser, events chan<- types.ResponsesStreamEvent) {
	defer close(events)
	defer body.Close()

	reader := transport.NewSSEReader(body)

	for {
		// Check context before reading
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
			sendResponsesEvent(ctx, events, types.NewResponsesErrorEvent(err.Error()))
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

		// Parse the streaming event
		var streamEvent openAIResponsesStreamEvent
		if err := json.Unmarshal([]byte(sse.Data), &streamEvent); err != nil {
			sendResponsesEvent(ctx, events, types.NewResponsesErrorEvent("parse error: "+err.Error()))
			continue
		}

		// Convert to unified stream event
		unifiedEvent := p.convertResponsesStreamEvent(&streamEvent)
		if !sendResponsesEvent(ctx, events, unifiedEvent) {
			return
		}

		// Check if this is the final event
		if streamEvent.Type == "response.done" || streamEvent.Type == "response.failed" {
			return
		}
	}
}

// sendResponsesEvent sends a stream event to the channel, respecting context cancellation.
func sendResponsesEvent(ctx context.Context, events chan<- types.ResponsesStreamEvent, event types.ResponsesStreamEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case events <- event:
		return true
	}
}

// convertResponsesStreamEvent converts an OpenAI stream event to unified format.
func (p *OpenAI) convertResponsesStreamEvent(event *openAIResponsesStreamEvent) types.ResponsesStreamEvent {
	result := types.ResponsesStreamEvent{
		Type: types.ResponsesStreamEventType(event.Type),
	}

	// Convert based on event type
	switch event.Type {
	case "response.created", "response.in_progress", "response.completed", "response.failed", "response.done":
		if event.Response != nil {
			result.Response = p.buildResponsesResponse(event.Response)
		}
	case "response.output_item.added", "response.output_item.done":
		if event.Item != nil {
			item := convertOutputItem(*event.Item)
			result.OutputItem = &item
		}
	case "response.content_part.delta":
		result.ContentIndex = event.ContentIndex
		if event.Delta != nil {
			result.Delta = &types.ResponseContentDelta{
				Type: event.Delta.Type,
				Text: event.Delta.Text,
			}
		}
	case "response.content_part.done":
		result.ContentIndex = event.ContentIndex
	case "error":
		if event.Error != nil {
			result.Error = event.Error.Message
		}
	}

	return result
}

// convertOutputItem converts an OpenAI output item to unified format.
func convertOutputItem(item openAIResponseOutputItem) types.ResponseOutputItem {
	result := types.ResponseOutputItem{
		Type:      item.Type,
		ID:        item.ID,
		Role:      item.Role,
		Status:    item.Status,
		CallID:    item.CallID,
		Name:      item.Name,
		Arguments: item.Arguments,
		Output:    item.Output,
	}

	// Convert content parts
	if len(item.Content) > 0 {
		result.Content = make([]types.ResponseContentPart, len(item.Content))
		for i, part := range item.Content {
			result.Content[i] = types.ResponseContentPart{
				Type:        part.Type,
				Text:        part.Text,
				Refusal:     part.Refusal,
				Annotations: part.Annotations,
			}
		}
	}

	// Convert summary (for reasoning)
	if len(item.Summary) > 0 {
		result.Summary = make([]types.ResponseContentPart, len(item.Summary))
		for i, part := range item.Summary {
			result.Summary[i] = types.ResponseContentPart{
				Type: part.Type,
				Text: part.Text,
			}
		}
	}

	return result
}

// convertResponsesTools converts unified tools to OpenAI format.
func convertResponsesTools(tools []types.ResponseTool) []openAIResponseTool {
	result := make([]openAIResponseTool, len(tools))
	for i, tool := range tools {
		result[i] = openAIResponseTool{
			Type: tool.Type,
		}
		if tool.Function != nil {
			result[i].Function = &openAIFunction{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			}
		}
		if tool.WebSearch != nil {
			result[i].WebSearch = &openAIWebSearchTool{
				Type:          tool.WebSearch.Type,
				SearchContext: tool.WebSearch.SearchContext,
			}
		}
		if tool.FileSearch != nil {
			result[i].FileSearch = &openAIFileSearchTool{
				VectorStoreIDs: tool.FileSearch.VectorStoreIDs,
				MaxResults:     tool.FileSearch.MaxResults,
			}
		}
	}
	return result
}

// --- OpenAI Responses API Types ---

type openAIResponsesRequest struct {
	Model              string               `json:"model"`
	Input              any                  `json:"input"` // String or []inputItem
	Instructions       string               `json:"instructions,omitempty"`
	MaxOutputTokens    *int                 `json:"max_output_tokens,omitempty"`
	Temperature        *float64             `json:"temperature,omitempty"`
	TopP               *float64             `json:"top_p,omitempty"`
	Stream             bool                 `json:"stream,omitempty"`
	Modalities         []string             `json:"modalities,omitempty"`
	ReasoningEffort    string               `json:"reasoning_effort,omitempty"`
	Tools              []openAIResponseTool `json:"tools,omitempty"`
	ToolChoice         any                  `json:"tool_choice,omitempty"`
	PreviousResponseID string               `json:"previous_response_id,omitempty"`
}

type openAIResponseTool struct {
	Type       string               `json:"type"`
	Function   *openAIFunction      `json:"function,omitempty"`
	WebSearch  *openAIWebSearchTool `json:"web_search,omitempty"`
	FileSearch *openAIFileSearchTool `json:"file_search,omitempty"`
}

type openAIWebSearchTool struct {
	Type          string `json:"type,omitempty"`
	SearchContext string `json:"search_context_size,omitempty"`
}

type openAIFileSearchTool struct {
	VectorStoreIDs []string `json:"vector_store_ids,omitempty"`
	MaxResults     int      `json:"max_num_results,omitempty"`
}

type openAIResponsesResponse struct {
	ID        string                    `json:"id"`
	Object    string                    `json:"object"`
	CreatedAt int64                     `json:"created_at"`
	Status    string                    `json:"status"`
	Model     string                    `json:"model"`
	Output    []openAIResponseOutputItem `json:"output"`
	Usage     *openAIResponsesUsage     `json:"usage,omitempty"`
	Error     *openAIResponseError      `json:"error,omitempty"`
}

type openAIResponseOutputItem struct {
	Type      string                    `json:"type"`
	ID        string                    `json:"id,omitempty"`
	Role      string                    `json:"role,omitempty"`
	Status    string                    `json:"status,omitempty"`
	Content   []openAIResponseContentPart `json:"content,omitempty"`
	CallID    string                    `json:"call_id,omitempty"`
	Name      string                    `json:"name,omitempty"`
	Arguments string                    `json:"arguments,omitempty"`
	Output    string                    `json:"output,omitempty"`
	Summary   []openAIResponseContentPart `json:"summary,omitempty"`
}

type openAIResponseContentPart struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Refusal     string `json:"refusal,omitempty"`
	Annotations []any  `json:"annotations,omitempty"`
}

type openAIResponsesUsage struct {
	InputTokens         int                          `json:"input_tokens"`
	OutputTokens        int                          `json:"output_tokens"`
	TotalTokens         int                          `json:"total_tokens,omitempty"`
	InputTokensDetails  *openAIInputTokensDetails    `json:"input_tokens_details,omitempty"`
	OutputTokensDetails *openAIOutputTokensDetails   `json:"output_tokens_details,omitempty"`
}

type openAIInputTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

type openAIOutputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

type openAIResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// --- Streaming Types ---

type openAIResponsesStreamEvent struct {
	Type         string                    `json:"type"`
	Response     *openAIResponsesResponse  `json:"response,omitempty"`
	Item         *openAIResponseOutputItem `json:"item,omitempty"`
	ContentIndex int                       `json:"content_index,omitempty"`
	Delta        *openAIContentDelta       `json:"delta,omitempty"`
	Error        *openAIResponseError      `json:"error,omitempty"`
}

type openAIContentDelta struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}
