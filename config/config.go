// Package config handles configuration loading and management.
package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/lex1ng/llm-gateway/pkg/types"
	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure.
type Config struct {
	Server      ServerConfig              `yaml:"server"`
	Providers   map[string]ProviderConfig `yaml:"providers"`
	ModelCatalog []ModelCatalogEntry       `yaml:"model_catalog"`
	TierRouting map[string][]RouteEntry   `yaml:"tier_routing"`
	Manager     ManagerConfig             `yaml:"manager"`
	Security    SecurityConfig            `yaml:"security"`
	Secret      SecretConfig              `yaml:"secret"`
	Observability ObservabilityConfig     `yaml:"observability"`
}

// ServerConfig defines HTTP server settings.
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// ProviderConfig defines a provider's connection settings.
type ProviderConfig struct {
	BaseURL   string `yaml:"base_url"`
	APIKey    string `yaml:"api_key"`
	Platform  string `yaml:"platform,omitempty"` // For compatible providers: alibaba, volcengine, etc.
	RateLimit int    `yaml:"rate_limit"`         // Requests per minute
	Timeout   time.Duration `yaml:"timeout,omitempty"`

	// Proxy controls HTTP proxy behavior for this provider:
	//   - "http://host:port" or "socks5://host:port": use this proxy
	//   - "none" or "direct": bypass proxy, always direct connect
	//   - "" (empty): use system environment (HTTP_PROXY/HTTPS_PROXY)
	Proxy string `yaml:"proxy,omitempty"`

	// Extra holds provider-specific configuration options.
	// Examples:
	//   anthropic_version: "2023-06-01"   (Anthropic API version header)
	//   default_max_tokens: 4096          (Anthropic required max_tokens)
	Extra map[string]any `yaml:"extra,omitempty"`
}

// GetExtra returns a string value from the provider's extra config, or the default if not set.
func (c ProviderConfig) GetExtra(key, defaultVal string) string {
	if c.Extra == nil {
		return defaultVal
	}
	if v, ok := c.Extra[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

// GetExtraInt returns an int value from the provider's extra config, or the default if not set.
func (c ProviderConfig) GetExtraInt(key string, defaultVal int) int {
	if c.Extra == nil {
		return defaultVal
	}
	if v, ok := c.Extra[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		}
	}
	return defaultVal
}

// ModelCatalogEntry defines a model in the catalog.
type ModelCatalogEntry struct {
	ID            string                   `yaml:"id"`
	Provider      string                   `yaml:"provider"`
	Tier          types.ModelTier          `yaml:"tier"`
	ContextWindow int                      `yaml:"context_window"`
	MaxOutput     int                      `yaml:"max_output"`
	InputPrice    float64                  `yaml:"input_price"`
	OutputPrice   float64                  `yaml:"output_price"`
	Capabilities  types.ModelCapabilities  `yaml:"capabilities"`
}

// RouteEntry defines a routing rule for tier-based routing.
type RouteEntry struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	Priority int    `yaml:"priority"`
}

// ManagerConfig defines the manager/orchestration settings.
type ManagerConfig struct {
	Cache          CacheConfig          `yaml:"cache"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	HedgedRequest  HedgedRequestConfig  `yaml:"hedged_request"`
	Retry          RetryConfig          `yaml:"retry"`
	Cooldown       CooldownConfig       `yaml:"cooldown"`
	Quota          QuotaConfig          `yaml:"quota"`
	SpendWriter    SpendWriterConfig    `yaml:"spend_writer"`
	CostCalculator CostCalculatorConfig `yaml:"cost_calculator"`
	Timeout        TimeoutConfig        `yaml:"timeout"`
	TokenSafetyMargin int              `yaml:"token_safety_margin"`
}

// CacheConfig defines cache settings.
type CacheConfig struct {
	Enabled bool             `yaml:"enabled"`
	Memory  MemoryCacheConfig `yaml:"memory"`
	Redis   RedisCacheConfig  `yaml:"redis"`
}

// MemoryCacheConfig defines in-memory cache settings.
type MemoryCacheConfig struct {
	MaxSize int           `yaml:"max_size"`
	TTL     time.Duration `yaml:"ttl"`
}

// RedisCacheConfig defines Redis cache settings.
type RedisCacheConfig struct {
	Enabled bool          `yaml:"enabled"`
	URL     string        `yaml:"url"`
	TTL     time.Duration `yaml:"ttl"`
}

// CircuitBreakerConfig defines circuit breaker settings.
type CircuitBreakerConfig struct {
	FailureThreshold int           `yaml:"failure_threshold"`
	RecoveryTimeout  time.Duration `yaml:"recovery_timeout"`
}

// HedgedRequestConfig defines hedged request settings.
type HedgedRequestConfig struct {
	Enabled bool          `yaml:"enabled"`
	Delay   time.Duration `yaml:"delay"`
}

// RetryConfig defines retry policy settings.
type RetryConfig struct {
	MaxAttempts   int           `yaml:"max_attempts"`
	InitialDelay  time.Duration `yaml:"initial_delay"`
	MaxDelay      time.Duration `yaml:"max_delay"`
	BackoffFactor float64       `yaml:"backoff_factor"`
	RetryBudget   float64       `yaml:"retry_budget"`
	Deadline      time.Duration `yaml:"deadline"`
	BudgetWindow  time.Duration `yaml:"budget_window"`
}

// CooldownConfig defines per-model cooldown settings.
type CooldownConfig struct {
	Enabled         bool            `yaml:"enabled"`
	BackoffSequence []time.Duration `yaml:"backoff_sequence"`
	MaxFailures     int             `yaml:"max_failures"`
}

// QuotaConfig defines quota management settings.
type QuotaConfig struct {
	Enabled        bool          `yaml:"enabled"`
	Store          string        `yaml:"store"` // database, redis, memory
	PreconsumedTTL time.Duration `yaml:"preconsumed_ttl"`
}

// SpendWriterConfig defines async spend writer settings.
type SpendWriterConfig struct {
	Enabled       bool          `yaml:"enabled"`
	BatchSize     int           `yaml:"batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval"`
	QueueSize     int           `yaml:"queue_size"`
	WALPath       string        `yaml:"wal_path"`
}

// CostCalculatorConfig defines cost calculation settings.
type CostCalculatorConfig struct {
	PricingFile   string        `yaml:"pricing_file"`
	FallbackPrice FallbackPrice `yaml:"fallback_price"`
}

// FallbackPrice defines fallback pricing for unknown models.
type FallbackPrice struct {
	InputPer1K  float64 `yaml:"input_per_1k"`
	OutputPer1K float64 `yaml:"output_per_1k"`
}

// TimeoutConfig defines timeout settings.
type TimeoutConfig struct {
	Connect          time.Duration `yaml:"connect"`
	FirstToken       time.Duration `yaml:"first_token"`
	TotalNonStream   time.Duration `yaml:"total_non_stream"`
	TotalStream      time.Duration `yaml:"total_stream"`
	IdleBetweenChunks time.Duration `yaml:"idle_between_chunks"`
}

// SecurityConfig defines security settings.
type SecurityConfig struct {
	TenantsFile string         `yaml:"tenants_file"`
	Sanitize    SanitizeConfig `yaml:"sanitize"`
	Audit       AuditConfig    `yaml:"audit"`
	ToolSandbox ToolSandboxConfig `yaml:"tool_sandbox"`
}

// SanitizeConfig defines log sanitization settings.
type SanitizeConfig struct {
	MessagePolicy  string `yaml:"message_policy"` // none, hash, truncate, mask
	TruncateLength int    `yaml:"truncate_length"`
}

// AuditConfig defines audit logging settings.
type AuditConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Output        string `yaml:"output"` // file, stdout, remote
	FilePath      string `yaml:"file_path"`
	RetentionDays int    `yaml:"retention_days"`
}

// ToolSandboxConfig defines tool calling sandbox settings.
type ToolSandboxConfig struct {
	Enabled         bool     `yaml:"enabled"`
	AllowedDomains  []string `yaml:"allowed_domains"`
	MaxResponseSize string   `yaml:"max_response_size"`
	Timeout         time.Duration `yaml:"timeout"`
}

// SecretConfig defines secret management settings.
type SecretConfig struct {
	Provider string      `yaml:"provider"` // env, kms, vault
	KMS      KMSConfig   `yaml:"kms,omitempty"`
	Vault    VaultConfig `yaml:"vault,omitempty"`
}

// KMSConfig defines KMS settings.
type KMSConfig struct {
	Region string `yaml:"region"`
	KeyID  string `yaml:"key_id"`
}

// VaultConfig defines Vault settings.
type VaultConfig struct {
	Addr string `yaml:"addr"`
	Path string `yaml:"path"`
}

// ObservabilityConfig defines observability settings.
type ObservabilityConfig struct {
	Tracing TracingConfig `yaml:"tracing"`
	Metrics MetricsConfig `yaml:"metrics"`
}

// TracingConfig defines tracing settings.
type TracingConfig struct {
	Enabled  bool    `yaml:"enabled"`
	Exporter string  `yaml:"exporter"` // otlp, jaeger, zipkin
	Endpoint string  `yaml:"endpoint"`
	Sample   float64 `yaml:"sample"` // 0.0 - 1.0
}

// MetricsConfig defines metrics settings.
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"` // /metrics
	Port    int    `yaml:"port"`
}

// Load loads configuration from a YAML file.
// It also loads the models catalog file (models.yaml) from the same directory.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Expand environment variables
	expanded := expandEnvVars(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Load models catalog from the same directory as the config file
	if err := loadModelsCatalog(path, &cfg); err != nil {
		return nil, err
	}

	// Remove providers with empty API keys and prune related models/routes
	pruneUnavailableProviders(&cfg)

	// Apply defaults
	applyDefaults(&cfg)

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// modelsFile is a helper struct for unmarshaling models.yaml.
type modelsFile struct {
	ModelCatalog []ModelCatalogEntry       `yaml:"model_catalog"`
	TierRouting  map[string][]RouteEntry   `yaml:"tier_routing"`
}

// loadModelsCatalog loads model_catalog and tier_routing from models.yaml
// located in the same directory as the main config file.
// If models.yaml doesn't exist, it's silently skipped (models may be inline in config.yaml).
func loadModelsCatalog(configPath string, cfg *Config) error {
	// If config already has models defined inline, skip
	if len(cfg.ModelCatalog) > 0 {
		return nil
	}

	modelsPath := filepath.Join(filepath.Dir(configPath), "models.yaml")
	data, err := os.ReadFile(modelsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // models.yaml is optional
		}
		return fmt.Errorf("read models file: %w", err)
	}

	var mf modelsFile
	if err := yaml.Unmarshal(data, &mf); err != nil {
		return fmt.Errorf("parse models file: %w", err)
	}

	cfg.ModelCatalog = mf.ModelCatalog
	if len(cfg.TierRouting) == 0 {
		cfg.TierRouting = mf.TierRouting
	}

	return nil
}

// pruneUnavailableProviders removes providers with empty API keys,
// and filters out related model_catalog entries and tier_routing entries.
// This allows keeping all providers in config.yaml while only activating
// those with real API keys set.
func pruneUnavailableProviders(cfg *Config) {
	// Collect providers with empty API keys and remove them
	removed := make(map[string]bool)
	for name, prov := range cfg.Providers {
		if prov.APIKey == "" {
			removed[name] = true
			delete(cfg.Providers, name)
			log.Printf("[INFO] provider %q skipped: no api_key configured", name)
		}
	}

	if len(removed) == 0 {
		return
	}

	// Filter model catalog
	filtered := cfg.ModelCatalog[:0]
	for _, m := range cfg.ModelCatalog {
		if !removed[m.Provider] {
			filtered = append(filtered, m)
		}
	}
	cfg.ModelCatalog = filtered

	// Filter tier routing
	for tier, entries := range cfg.TierRouting {
		var kept []RouteEntry
		for _, e := range entries {
			if !removed[e.Provider] {
				kept = append(kept, e)
			}
		}
		if len(kept) == 0 {
			delete(cfg.TierRouting, tier)
		} else {
			cfg.TierRouting[tier] = kept
		}
	}
}

// LoadFromBytes loads configuration from YAML bytes.
func LoadFromBytes(data []byte) (*Config, error) {
	expanded := expandEnvVars(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyDefaults(&cfg)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks the configuration for required fields and consistency.
func (c *Config) Validate() error {
	// At least one provider must be configured
	if len(c.Providers) == 0 {
		return fmt.Errorf("validate: no providers configured")
	}

	// Each provider must have an API key (or be explicitly skipped)
	for name, prov := range c.Providers {
		if prov.APIKey == "" {
			return fmt.Errorf("validate: provider %q has no api_key", name)
		}
	}

	// Model catalog entries must reference existing providers
	providerSet := make(map[string]bool, len(c.Providers))
	for name := range c.Providers {
		providerSet[name] = true
	}
	for _, model := range c.ModelCatalog {
		if !providerSet[model.Provider] {
			return fmt.Errorf("validate: model %q references unconfigured provider %q", model.ID, model.Provider)
		}
	}

	// Tier routing entries must reference existing providers and models
	modelSet := make(map[string]bool, len(c.ModelCatalog))
	for _, model := range c.ModelCatalog {
		modelSet[model.ID] = true
	}
	for tier, entries := range c.TierRouting {
		for _, entry := range entries {
			if !providerSet[entry.Provider] {
				return fmt.Errorf("validate: tier_routing[%s] references unconfigured provider %q", tier, entry.Provider)
			}
			if !modelSet[entry.Model] {
				return fmt.Errorf("validate: tier_routing[%s] references unconfigured model %q", tier, entry.Model)
			}
		}
	}

	return nil
}

// envVarRegex matches ${VAR_NAME} patterns.
var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// expandEnvVars replaces ${VAR_NAME} with environment variable values.
func expandEnvVars(input string) string {
	return envVarRegex.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable name from ${VAR_NAME}
		varName := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")

		// Support default values: ${VAR_NAME:-default}
		parts := strings.SplitN(varName, ":-", 2)
		envVal := os.Getenv(parts[0])

		if envVal == "" && len(parts) > 1 {
			return parts[1] // Return default value
		}
		return envVal
	})
}

// applyDefaults sets default values for unset fields.
func applyDefaults(cfg *Config) {
	// Server defaults
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 30 * time.Second
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = 120 * time.Second
	}

	// Manager defaults
	if cfg.Manager.TokenSafetyMargin == 0 {
		cfg.Manager.TokenSafetyMargin = 256
	}

	// Cache defaults
	if cfg.Manager.Cache.Memory.MaxSize == 0 {
		cfg.Manager.Cache.Memory.MaxSize = 1000
	}
	if cfg.Manager.Cache.Memory.TTL == 0 {
		cfg.Manager.Cache.Memory.TTL = 5 * time.Minute
	}

	// Circuit breaker defaults
	if cfg.Manager.CircuitBreaker.FailureThreshold == 0 {
		cfg.Manager.CircuitBreaker.FailureThreshold = 5
	}
	if cfg.Manager.CircuitBreaker.RecoveryTimeout == 0 {
		cfg.Manager.CircuitBreaker.RecoveryTimeout = 30 * time.Second
	}

	// Retry defaults
	if cfg.Manager.Retry.MaxAttempts == 0 {
		cfg.Manager.Retry.MaxAttempts = 3
	}
	if cfg.Manager.Retry.InitialDelay == 0 {
		cfg.Manager.Retry.InitialDelay = 100 * time.Millisecond
	}
	if cfg.Manager.Retry.MaxDelay == 0 {
		cfg.Manager.Retry.MaxDelay = 5 * time.Second
	}
	if cfg.Manager.Retry.BackoffFactor == 0 {
		cfg.Manager.Retry.BackoffFactor = 2.0
	}
	if cfg.Manager.Retry.RetryBudget == 0 {
		cfg.Manager.Retry.RetryBudget = 0.1
	}
	if cfg.Manager.Retry.Deadline == 0 {
		cfg.Manager.Retry.Deadline = 30 * time.Second
	}

	// Timeout defaults
	if cfg.Manager.Timeout.Connect == 0 {
		cfg.Manager.Timeout.Connect = 5 * time.Second
	}
	if cfg.Manager.Timeout.FirstToken == 0 {
		cfg.Manager.Timeout.FirstToken = 30 * time.Second
	}
	if cfg.Manager.Timeout.TotalNonStream == 0 {
		cfg.Manager.Timeout.TotalNonStream = 120 * time.Second
	}
	if cfg.Manager.Timeout.TotalStream == 0 {
		cfg.Manager.Timeout.TotalStream = 300 * time.Second
	}
	if cfg.Manager.Timeout.IdleBetweenChunks == 0 {
		cfg.Manager.Timeout.IdleBetweenChunks = 30 * time.Second
	}

	// SpendWriter defaults
	if cfg.Manager.SpendWriter.BatchSize == 0 {
		cfg.Manager.SpendWriter.BatchSize = 100
	}
	if cfg.Manager.SpendWriter.FlushInterval == 0 {
		cfg.Manager.SpendWriter.FlushInterval = 5 * time.Second
	}
	if cfg.Manager.SpendWriter.QueueSize == 0 {
		cfg.Manager.SpendWriter.QueueSize = 10000
	}

	// Security defaults
	if cfg.Security.Sanitize.MessagePolicy == "" {
		cfg.Security.Sanitize.MessagePolicy = "truncate"
	}
	if cfg.Security.Sanitize.TruncateLength == 0 {
		cfg.Security.Sanitize.TruncateLength = 100
	}

	// Secret defaults
	if cfg.Secret.Provider == "" {
		cfg.Secret.Provider = "env"
	}

	// Metrics defaults
	if cfg.Observability.Metrics.Path == "" {
		cfg.Observability.Metrics.Path = "/metrics"
	}
}

// GetModel returns the ModelCatalogEntry for a given model ID.
func (c *Config) GetModel(modelID string) *ModelCatalogEntry {
	for i := range c.ModelCatalog {
		if c.ModelCatalog[i].ID == modelID {
			return &c.ModelCatalog[i]
		}
	}
	return nil
}

// GetProvider returns the ProviderConfig for a given provider name.
func (c *Config) GetProvider(name string) *ProviderConfig {
	if p, ok := c.Providers[name]; ok {
		return &p
	}
	return nil
}

// GetTierRoutes returns the routing rules for a given tier.
func (c *Config) GetTierRoutes(tier types.ModelTier) []RouteEntry {
	return c.TierRouting[string(tier)]
}
