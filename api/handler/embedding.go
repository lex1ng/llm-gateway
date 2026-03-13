package handler

import (
	"encoding/json"
	"net/http"

	"github.com/lex1ng/llm-gateway/pkg/gateway"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

// EmbeddingHandler handles embedding requests.
type EmbeddingHandler struct {
	client *gateway.Client
}

// NewEmbeddingHandler creates a new EmbeddingHandler.
func NewEmbeddingHandler(client *gateway.Client) *EmbeddingHandler {
	return &EmbeddingHandler{client: client}
}

// ServeHTTP handles POST /v1/embeddings requests.
func (h *EmbeddingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.EmbedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body: "+err.Error())
		return
	}

	if len(req.Input) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "input is required and must be non-empty")
		return
	}

	resp, err := h.client.Embed(r.Context(), &req)
	if err != nil {
		handleProviderError(w, err)
		return
	}

	// Return OpenAI-compatible embedding response
	openAIResp := map[string]any{
		"object": "list",
		"data":   resp.Data,
		"model":  resp.Model,
		"usage": map[string]int{
			"prompt_tokens": resp.Usage.PromptTokens,
			"total_tokens":  resp.Usage.TotalTokens,
		},
	}
	writeJSON(w, http.StatusOK, openAIResp)
}
