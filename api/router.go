// Package api provides the HTTP API router for LLM Gateway.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/lex1ng/llm-gateway/api/handler"
	"github.com/lex1ng/llm-gateway/pkg/gateway"
)

// Router sets up the HTTP routes for the API.
type Router struct {
	mux    *http.ServeMux
	client *gateway.Client
}

// NewRouter creates a new API router with the given gateway client.
func NewRouter(client *gateway.Client) *Router {
	r := &Router{
		mux:    http.NewServeMux(),
		client: client,
	}
	r.setupRoutes()
	return r
}

// setupRoutes configures all API routes.
func (r *Router) setupRoutes() {
	// Chat completions (OpenAI-compatible)
	chatHandler := handler.NewChatHandler(r.client)
	r.mux.Handle("/v1/chat/completions", chatHandler)

	// Responses API (OpenAI-specific, better for reasoning models)
	responsesHandler := handler.NewResponsesHandler(r.client)
	r.mux.Handle("/v1/responses", responsesHandler)

	// Embeddings
	embeddingHandler := handler.NewEmbeddingHandler(r.client)
	r.mux.Handle("/v1/embeddings", embeddingHandler)

	// Models listing
	r.mux.HandleFunc("/v1/models", r.handleListModels)

	// Health check
	r.mux.HandleFunc("/health", r.handleHealth)
	r.mux.HandleFunc("/healthz", r.handleHealth)

	// TODO: Add more routes in later sprints:
	// - /v1/images/generations (Sprint 7)
	// - /v1/audio/speech (Sprint 7)
	// - /v1/audio/transcriptions (Sprint 7)
}

// ServeHTTP implements http.Handler with request logging.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
	r.mux.ServeHTTP(rw, req)
	slog.Info("request",
		"method", req.Method,
		"path", req.URL.Path,
		"status", rw.statusCode,
		"duration_ms", time.Since(start).Milliseconds(),
		"remote", req.RemoteAddr,
	)
}

// responseWriter wraps http.ResponseWriter to capture the status code.
// It also implements http.Flusher to support streaming responses.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher, required for SSE streaming.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// handleListModels handles GET /v1/models.
func (r *Router) handleListModels(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	models := r.client.ListModels()

	// Convert to OpenAI-compatible format
	type modelData struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	type listModelsResponse struct {
		Object string      `json:"object"`
		Data   []modelData `json:"data"`
	}

	resp := listModelsResponse{
		Object: "list",
		Data:   make([]modelData, len(models)),
	}

	for i, m := range models {
		resp.Data[i] = modelData{
			ID:      m.ModelID,
			Object:  "model",
			Created: 0,
			OwnedBy: m.Provider,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// handleHealth handles health check requests.
func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}
