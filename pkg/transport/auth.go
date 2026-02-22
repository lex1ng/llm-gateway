package transport

import "net/http"

// AuthStrategy defines the interface for authentication strategies.
type AuthStrategy interface {
	// Apply adds authentication to the request.
	Apply(req *http.Request) error
}

// BearerAuth implements AuthStrategy using Bearer token authentication.
// Used by: OpenAI, Alibaba (DashScope), and most compatible providers.
type BearerAuth struct {
	APIKey string
}

// Apply adds the Bearer token to the Authorization header.
func (a *BearerAuth) Apply(req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+a.APIKey)
	return nil
}

// AnthropicAuth implements AuthStrategy for Anthropic API.
// Uses x-api-key header and anthropic-version header.
type AnthropicAuth struct {
	APIKey  string
	Version string // Default: "2023-06-01"
}

// Apply adds Anthropic-specific headers.
func (a *AnthropicAuth) Apply(req *http.Request) error {
	req.Header.Set("x-api-key", a.APIKey)
	version := a.Version
	if version == "" {
		version = "2023-06-01"
	}
	req.Header.Set("anthropic-version", version)
	return nil
}

// GoogleAuth implements AuthStrategy for Google Gemini API.
// Uses API key as query parameter.
type GoogleAuth struct {
	APIKey string
}

// Apply adds the API key as a query parameter.
func (a *GoogleAuth) Apply(req *http.Request) error {
	q := req.URL.Query()
	q.Set("key", a.APIKey)
	req.URL.RawQuery = q.Encode()
	return nil
}

// DynamicAuth implements AuthStrategy with dynamic credential support (BYOK).
// If credentials contain api_key, it takes priority over static auth.
type DynamicAuth struct {
	StaticAuth  AuthStrategy      // Fallback static authentication
	Credentials map[string]string // Dynamic credentials from request
}

// Apply uses dynamic credentials if available, otherwise falls back to static auth.
func (a *DynamicAuth) Apply(req *http.Request) error {
	// Check for dynamic API key
	if apiKey, ok := a.Credentials["api_key"]; ok && apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		return nil
	}

	// Check for Anthropic-specific key
	if apiKey, ok := a.Credentials["x_api_key"]; ok && apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
		if version, ok := a.Credentials["anthropic_version"]; ok {
			req.Header.Set("anthropic-version", version)
		} else {
			req.Header.Set("anthropic-version", "2023-06-01")
		}
		return nil
	}

	// Fallback to static auth
	if a.StaticAuth != nil {
		return a.StaticAuth.Apply(req)
	}

	return nil
}

// NoAuth is a no-op AuthStrategy for unauthenticated requests.
type NoAuth struct{}

// Apply does nothing.
func (a *NoAuth) Apply(req *http.Request) error {
	return nil
}

// NewAuthStrategy creates an AuthStrategy based on the provider type.
func NewAuthStrategy(provider, apiKey string) AuthStrategy {
	switch provider {
	case "anthropic":
		return &AnthropicAuth{APIKey: apiKey}
	case "google":
		return &GoogleAuth{APIKey: apiKey}
	default:
		// OpenAI and compatible providers use Bearer auth
		return &BearerAuth{APIKey: apiKey}
	}
}

// WithDynamicCredentials wraps an AuthStrategy with dynamic credential support.
func WithDynamicCredentials(staticAuth AuthStrategy, credentials map[string]string) AuthStrategy {
	if len(credentials) == 0 {
		return staticAuth
	}
	return &DynamicAuth{
		StaticAuth:  staticAuth,
		Credentials: credentials,
	}
}
