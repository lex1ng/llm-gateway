package handler

import (
	"encoding/json"
	"net/http"

	"github.com/lex1ng/llm-gateway/pkg/gateway"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

// ResponsesHandler handles OpenAI Responses API requests.
type ResponsesHandler struct {
	client *gateway.Client
}

// NewResponsesHandler creates a new ResponsesHandler.
func NewResponsesHandler(client *gateway.Client) *ResponsesHandler {
	return &ResponsesHandler{client: client}
}

// ServeHTTP handles POST /v1/responses requests.
func (h *ResponsesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.ResponsesRequest
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

// handleNonStream handles non-streaming Responses API request.
func (h *ResponsesHandler) handleNonStream(w http.ResponseWriter, r *http.Request, req *types.ResponsesRequest) {
	resp, err := h.client.Responses(r.Context(), req)
	if err != nil {
		handleProviderError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleStream handles streaming Responses API request.
func (h *ResponsesHandler) handleStream(w http.ResponseWriter, r *http.Request, req *types.ResponsesRequest) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	events, err := h.client.ResponsesStream(r.Context(), req)
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
		if event.Type == types.ResponsesEventError {
			writeSSE(w, string(event.Type), `{"error":{"message":"`+event.Error+`"}}`)
			flusher.Flush()
			return
		}

		// Marshal the event
		data, err := json.Marshal(event)
		if err != nil {
			writeSSE(w, "error", `{"error":{"message":"marshal error"}}`)
			flusher.Flush()
			return
		}

		writeSSE(w, string(event.Type), string(data))
		flusher.Flush()

		// Check if this is the final event
		if event.Type == types.ResponsesEventDone || event.Type == types.ResponsesEventFailed {
			return
		}
	}
}
