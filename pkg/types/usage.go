package types

// TokenUsage represents token usage statistics for a request.
// Follows OpenAI usage format as the internal standard.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`

	// Extended fields for detailed tracking
	CachedTokens   int `json:"cached_tokens,omitempty"`   // Tokens served from cache
	ReasoningTokens int `json:"reasoning_tokens,omitempty"` // Tokens used for extended thinking
}

// Add adds another TokenUsage to this one (for aggregation).
func (u *TokenUsage) Add(other TokenUsage) {
	u.PromptTokens += other.PromptTokens
	u.CompletionTokens += other.CompletionTokens
	u.TotalTokens += other.TotalTokens
	u.CachedTokens += other.CachedTokens
	u.ReasoningTokens += other.ReasoningTokens
}

// IsEmpty returns true if no tokens were used.
func (u *TokenUsage) IsEmpty() bool {
	return u.TotalTokens == 0 && u.PromptTokens == 0 && u.CompletionTokens == 0
}

// ComputeTotal calculates TotalTokens from Prompt and Completion tokens.
func (u *TokenUsage) ComputeTotal() {
	u.TotalTokens = u.PromptTokens + u.CompletionTokens
}

// Cost represents the cost of a request in USD.
type Cost struct {
	InputCost  float64 `json:"input_cost"`  // Cost for input/prompt tokens
	OutputCost float64 `json:"output_cost"` // Cost for output/completion tokens
	TotalCost  float64 `json:"total_cost"`  // Total cost
	Currency   string  `json:"currency"`    // Usually "USD"
}

// NewCost creates a new Cost with computed total.
func NewCost(inputCost, outputCost float64) Cost {
	return Cost{
		InputCost:  inputCost,
		OutputCost: outputCost,
		TotalCost:  inputCost + outputCost,
		Currency:   "USD",
	}
}

// CalculateCost calculates cost based on token usage and pricing.
func CalculateCost(usage TokenUsage, inputPricePer1K, outputPricePer1K float64) Cost {
	inputCost := float64(usage.PromptTokens) / 1000.0 * inputPricePer1K
	outputCost := float64(usage.CompletionTokens) / 1000.0 * outputPricePer1K
	return NewCost(inputCost, outputCost)
}

// Add adds another Cost to this one.
func (c *Cost) Add(other Cost) {
	c.InputCost += other.InputCost
	c.OutputCost += other.OutputCost
	c.TotalCost += other.TotalCost
}

// SpendRecord represents a spend record for billing/analytics.
type SpendRecord struct {
	TenantID     string     `json:"tenant_id"`
	RequestID    string     `json:"request_id"`
	Provider     string     `json:"provider"`
	Model        string     `json:"model"`
	Usage        TokenUsage `json:"usage"`
	Cost         Cost       `json:"cost"`
	Timestamp    int64      `json:"timestamp"`
	Cached       bool       `json:"cached"`
	LatencyMs    int64      `json:"latency_ms"`
}
