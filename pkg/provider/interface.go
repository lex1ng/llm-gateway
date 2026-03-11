// Package provider defines the capability interfaces for LLM providers.
package provider

import (
	"context"
	"io"

	"github.com/lex1ng/llm-gateway/pkg/types"
)

// Provider is the base interface that all providers must implement.
type Provider interface {
	// Name returns the provider identifier (e.g., "openai", "anthropic").
	Name() string
	// Models returns the list of models this provider supports.
	Models() []types.ModelConfig
	// Supports checks if the provider supports a given capability.
	Supports(cap Capability) bool
	// Close releases any resources held by the provider.
	Close() error
}

// --- P0: Chat and Streaming ---

// ChatProvider provides chat completion capabilities.
type ChatProvider interface {
	Provider
	// Chat sends a non-streaming chat completion request.
	Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error)
	// ChatStream sends a streaming chat completion request.
	// Returns a channel that will receive StreamEvent messages until completion or error.
	ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan types.StreamEvent, error)
}

// --- P0.5: Responses API (OpenAI-specific, better for reasoning models) ---

// ResponsesProvider provides the OpenAI Responses API capabilities.
// This API offers better performance with reasoning models and built-in tools.
type ResponsesProvider interface {
	Provider
	// Responses sends a non-streaming request to the Responses API.
	Responses(ctx context.Context, req *types.ResponsesRequest) (*types.ResponsesResponse, error)
	// ResponsesStream sends a streaming request to the Responses API.
	ResponsesStream(ctx context.Context, req *types.ResponsesRequest) (<-chan types.ResponsesStreamEvent, error)
}

// --- P1: Embeddings ---

// EmbeddingProvider provides text embedding capabilities.
type EmbeddingProvider interface {
	Provider
	// Embed generates embeddings for the input text.
	Embed(ctx context.Context, req *types.EmbedRequest) (*types.EmbedResponse, error)
}

// --- P2: Agent and Workflow ---

// AgentProvider provides agent invocation capabilities.
type AgentProvider interface {
	Provider
	// InvokeAgent invokes an agent with the given request.
	InvokeAgent(ctx context.Context, req *types.AgentRequest) (*types.AgentResponse, error)
	// InvokeAgentStream invokes an agent with streaming response.
	InvokeAgentStream(ctx context.Context, req *types.AgentRequest) (<-chan types.StreamEvent, error)
}

// WorkflowProvider provides workflow invocation capabilities.
type WorkflowProvider interface {
	Provider
	// RunWorkflow executes a workflow.
	RunWorkflow(ctx context.Context, req *types.WorkflowRequest) (*types.WorkflowResponse, error)
}

// --- P3: Image Generation ---

// ImageGenProvider provides image generation capabilities.
type ImageGenProvider interface {
	Provider
	// GenerateImage generates images synchronously (OpenAI-style).
	GenerateImage(ctx context.Context, req *types.ImageGenRequest) (*types.ImageGenResponse, error)
	// SubmitImageTask submits an async image generation task (for platforms like Alibaba).
	SubmitImageTask(ctx context.Context, req *types.ImageGenRequest) (*types.AsyncTask, error)
	// GetTaskStatus retrieves the status of an async task.
	GetTaskStatus(ctx context.Context, taskID string) (*types.AsyncTask, error)
}

// --- P3: Video Generation (always async) ---

// VideoGenProvider provides video generation capabilities.
type VideoGenProvider interface {
	Provider
	// SubmitVideoTask submits an async video generation task.
	SubmitVideoTask(ctx context.Context, req *types.VideoGenRequest) (*types.AsyncTask, error)
	// GetVideoTaskStatus retrieves the status of a video generation task.
	GetVideoTaskStatus(ctx context.Context, taskID string) (*types.AsyncTask, error)
	// CancelVideoTask cancels a pending video generation task.
	CancelVideoTask(ctx context.Context, taskID string) error
}

// --- P3: Audio ---

// TTSProvider provides text-to-speech capabilities.
type TTSProvider interface {
	Provider
	// Synthesize converts text to speech audio.
	// Caller is responsible for closing the returned ReadCloser.
	Synthesize(ctx context.Context, req *types.TTSRequest) (io.ReadCloser, error)
}

// STTProvider provides speech-to-text capabilities.
type STTProvider interface {
	Provider
	// Transcribe converts audio to text.
	Transcribe(ctx context.Context, req *types.STTRequest) (*types.STTResponse, error)
}
