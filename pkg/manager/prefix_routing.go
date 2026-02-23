// Package manager implements the core request orchestration logic.
package manager

import "strings"

// PrefixRule defines a model name prefix to provider mapping.
type PrefixRule struct {
	Prefix   string // Model name prefix to match
	Provider string // Provider to route to
}

// ProviderPrefixRules defines the prefix-to-provider mapping for passthrough routing.
// When a model is not found in the catalog, these rules are used to determine
// which provider should handle the request based on model name prefix.
//
// To add support for a new provider or model prefix:
// 1. Add a new PrefixRule entry to the appropriate provider section
// 2. Ensure the provider is registered and has a valid API key
//
// Note: Rules are evaluated in order; first match wins.
// More specific prefixes should come before general ones.
var ProviderPrefixRules = []PrefixRule{
	// ========== OpenAI ==========
	// GPT models (chat, code, etc.)
	{Prefix: "gpt-", Provider: "openai"},
	// Reasoning models
	{Prefix: "o1-", Provider: "openai"},
	{Prefix: "o1", Provider: "openai"},  // o1 without dash
	{Prefix: "o3-", Provider: "openai"},
	{Prefix: "o3", Provider: "openai"},
	{Prefix: "o4-", Provider: "openai"},
	{Prefix: "o4", Provider: "openai"},
	// Image generation
	{Prefix: "dall-e-", Provider: "openai"},
	// Audio
	{Prefix: "whisper-", Provider: "openai"},
	{Prefix: "tts-", Provider: "openai"},
	// Embeddings
	{Prefix: "text-embedding-", Provider: "openai"},
	// Moderation
	{Prefix: "omni-moderation-", Provider: "openai"},
	// Legacy
	{Prefix: "davinci-", Provider: "openai"},
	{Prefix: "babbage-", Provider: "openai"},
	// Video (Sora)
	{Prefix: "sora-", Provider: "openai"},

	// ========== Anthropic ==========
	{Prefix: "claude-", Provider: "anthropic"},

	// ========== Alibaba (DashScope) ==========
	{Prefix: "qwen-", Provider: "alibaba"},
	{Prefix: "qwen2", Provider: "alibaba"}, // qwen2.5-coder etc.

	// ========== Google ==========
	{Prefix: "gemini-", Provider: "google"},
	{Prefix: "palm-", Provider: "google"},

	// ========== Volcengine (Doubao) ==========
	{Prefix: "doubao-", Provider: "volcengine"},
	{Prefix: "ep-", Provider: "volcengine"}, // endpoint ID format

	// ========== DeepSeek ==========
	{Prefix: "deepseek-", Provider: "deepseek"},

	// ========== Moonshot ==========
	{Prefix: "moonshot-", Provider: "moonshot"},

	// ========== Zhipu (GLM) ==========
	{Prefix: "glm-", Provider: "zhipu"},
	{Prefix: "chatglm", Provider: "zhipu"},

	// ========== Baichuan ==========
	{Prefix: "baichuan", Provider: "baichuan"},

	// ========== Minimax ==========
	{Prefix: "abab", Provider: "minimax"},

	// ========== 01.AI (Yi) ==========
	{Prefix: "yi-", Provider: "01ai"},

	// ========== Mistral ==========
	{Prefix: "mistral-", Provider: "mistral"},
	{Prefix: "mixtral-", Provider: "mistral"},
	{Prefix: "codestral-", Provider: "mistral"},
	{Prefix: "open-mistral-", Provider: "mistral"},
	{Prefix: "open-mixtral-", Provider: "mistral"},
}

// matchProviderByPrefix finds the provider for a model based on prefix rules.
// Returns empty string if no matching prefix is found.
func matchProviderByPrefix(modelID string) string {
	modelLower := strings.ToLower(modelID)
	for _, rule := range ProviderPrefixRules {
		if strings.HasPrefix(modelLower, strings.ToLower(rule.Prefix)) {
			return rule.Provider
		}
	}
	return ""
}