package openai

import (
	"context"
	"net/http"
	"time"

	"github.com/lex1ng/llm-gateway/pkg/types"
)

// openAIEmbedRequest is the OpenAI-native embedding request format.
type openAIEmbedRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
	Dimensions     *int     `json:"dimensions,omitempty"`
	User           string   `json:"user,omitempty"`
}

// openAIEmbedResponse is the OpenAI-native embedding response format.
type openAIEmbedResponse struct {
	Object string              `json:"object"` // "list"
	Data   []openAIEmbedData   `json:"data"`
	Model  string              `json:"model"`
	Usage  openAIEmbeddingUsage `json:"usage"`
}

type openAIEmbedData struct {
	Object    string    `json:"object"` // "embedding"
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type openAIEmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// Embed generates embeddings for the input text.
func (p *OpenAI) Embed(ctx context.Context, req *types.EmbedRequest) (*types.EmbedResponse, error) {
	startTime := time.Now()

	// Build request
	oaiReq := &openAIEmbedRequest{
		Model:          req.Model,
		Input:          req.Input,
		EncodingFormat: req.EncodingFormat,
		Dimensions:     req.Dimensions,
		User:           req.User,
	}

	// Select auth (support BYOK)
	auth := p.auth
	if len(req.Credentials) > 0 {
		auth = p.getAuth(req.Credentials)
	}

	// Call API
	var oaiResp openAIEmbedResponse
	if err := p.client.DoJSON(ctx, http.MethodPost, p.embeddingsEndpoint(), auth, oaiReq, &oaiResp); err != nil {
		return nil, err
	}

	// Convert response
	resp := &types.EmbedResponse{
		Model:    oaiResp.Model,
		Provider: p.name,
		Data:     make([]types.Embedding, len(oaiResp.Data)),
		Usage: types.TokenUsage{
			PromptTokens: oaiResp.Usage.PromptTokens,
			TotalTokens:  oaiResp.Usage.TotalTokens,
		},
	}

	for i, d := range oaiResp.Data {
		resp.Data[i] = types.Embedding{
			Index:     d.Index,
			Embedding: d.Embedding,
			Object:    "embedding",
		}
	}

	_ = startTime // latency tracking reserved for Sprint 10

	return resp, nil
}
