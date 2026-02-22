package config

import (
	"os"
	"testing"
	"time"
)

func TestExpandEnvVars(t *testing.T) {
	os.Setenv("TEST_API_KEY", "sk-test-123")
	defer os.Unsetenv("TEST_API_KEY")

	input := `api_key: "${TEST_API_KEY}"`
	expected := `api_key: "sk-test-123"`

	result := expandEnvVars(input)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestExpandEnvVars_WithDefault(t *testing.T) {
	// Ensure the env var is not set
	os.Unsetenv("UNSET_VAR")

	input := `value: "${UNSET_VAR:-default_value}"`
	expected := `value: "default_value"`

	result := expandEnvVars(input)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestExpandEnvVars_SetVarOverridesDefault(t *testing.T) {
	os.Setenv("SET_VAR", "actual_value")
	defer os.Unsetenv("SET_VAR")

	input := `value: "${SET_VAR:-default_value}"`
	expected := `value: "actual_value"`

	result := expandEnvVars(input)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestLoadFromBytes(t *testing.T) {
	yaml := `
server:
  host: "0.0.0.0"
  port: 9090

providers:
  openai:
    base_url: "https://api.openai.com/v1"
    api_key: "sk-test"
    rate_limit: 500

model_catalog:
  - id: "gpt-4o"
    provider: "openai"
    tier: "large"
    context_window: 128000
    max_output: 16384
    input_price: 0.0025
    output_price: 0.01
    capabilities:
      chat: true
      vision: true
      tools: true
      streaming: true

tier_routing:
  large:
    - provider: "openai"
      model: "gpt-4o"
      priority: 1
`

	cfg, err := LoadFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}

	// Test server config
	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}

	// Test provider config
	openai := cfg.GetProvider("openai")
	if openai == nil {
		t.Fatal("expected openai provider")
	}
	if openai.APIKey != "sk-test" {
		t.Errorf("expected api_key 'sk-test', got %q", openai.APIKey)
	}

	// Test model catalog
	gpt4o := cfg.GetModel("gpt-4o")
	if gpt4o == nil {
		t.Fatal("expected gpt-4o model")
	}
	if gpt4o.ContextWindow != 128000 {
		t.Errorf("expected context_window 128000, got %d", gpt4o.ContextWindow)
	}
	if !gpt4o.Capabilities.Chat {
		t.Error("expected chat capability")
	}

	// Test tier routing
	routes := cfg.GetTierRoutes("large")
	if len(routes) != 1 {
		t.Fatalf("expected 1 route for large tier, got %d", len(routes))
	}
	if routes[0].Provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", routes[0].Provider)
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)

	// Server defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("expected default read_timeout 30s, got %v", cfg.Server.ReadTimeout)
	}

	// Manager defaults
	if cfg.Manager.TokenSafetyMargin != 256 {
		t.Errorf("expected default token_safety_margin 256, got %d", cfg.Manager.TokenSafetyMargin)
	}

	// Circuit breaker defaults
	if cfg.Manager.CircuitBreaker.FailureThreshold != 5 {
		t.Errorf("expected default failure_threshold 5, got %d", cfg.Manager.CircuitBreaker.FailureThreshold)
	}

	// Retry defaults
	if cfg.Manager.Retry.MaxAttempts != 3 {
		t.Errorf("expected default max_attempts 3, got %d", cfg.Manager.Retry.MaxAttempts)
	}

	// Secret defaults
	if cfg.Secret.Provider != "env" {
		t.Errorf("expected default secret provider 'env', got %q", cfg.Secret.Provider)
	}
}
