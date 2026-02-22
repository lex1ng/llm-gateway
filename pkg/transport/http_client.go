// Package transport provides HTTP client and authentication utilities.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/lex1ng/llm-gateway/pkg/types"
)

// HTTPClient is a unified HTTP client for making requests to LLM providers.
type HTTPClient struct {
	client  *http.Client
	timeout time.Duration
}

// HTTPClientConfig contains configuration for HTTPClient.
type HTTPClientConfig struct {
	Timeout       time.Duration
	MaxIdleConns  int
	IdleConnTimeout time.Duration
}

// NewHTTPClient creates a new HTTPClient with the given configuration.
func NewHTTPClient(cfg HTTPClientConfig) *HTTPClient {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 100
	}
	if cfg.IdleConnTimeout == 0 {
		cfg.IdleConnTimeout = 90 * time.Second
	}

	transport := &http.Transport{
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConns,
		IdleConnTimeout:     cfg.IdleConnTimeout,
	}

	return &HTTPClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   cfg.Timeout,
		},
		timeout: cfg.Timeout,
	}
}

// DefaultHTTPClient returns an HTTPClient with default settings.
func DefaultHTTPClient() *HTTPClient {
	return NewHTTPClient(HTTPClientConfig{})
}

// Do sends an HTTP request and returns the response.
func (c *HTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	return c.client.Do(req)
}

// DoJSON sends a request with JSON body and decodes the JSON response.
func (c *HTTPClient) DoJSON(ctx context.Context, method, url string, auth AuthStrategy, body any, result any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if auth != nil {
		if err := auth.Apply(req); err != nil {
			return fmt.Errorf("apply auth: %w", err)
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return parseProviderError(resp.StatusCode, respBody)
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

// DoStream sends a request and returns the response body for streaming.
// The caller is responsible for closing the response body.
func (c *HTTPClient) DoStream(ctx context.Context, method, url string, auth AuthStrategy, body any) (io.ReadCloser, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	if auth != nil {
		if err := auth.Apply(req); err != nil {
			return nil, fmt.Errorf("apply auth: %w", err)
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, parseProviderError(resp.StatusCode, respBody)
	}

	return resp.Body, nil
}

// parseProviderError parses an error response from a provider.
func parseProviderError(statusCode int, body []byte) *types.ProviderError {
	pe := &types.ProviderError{
		StatusCode: statusCode,
		Raw:        body,
	}

	// Try to parse error message from common formats
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
		Message string `json:"message"` // Some providers use this
	}

	if err := json.Unmarshal(body, &errResp); err == nil {
		if errResp.Error.Message != "" {
			pe.Message = errResp.Error.Message
		} else if errResp.Message != "" {
			pe.Message = errResp.Message
		}
	}

	if pe.Message == "" {
		pe.Message = fmt.Sprintf("HTTP %d", statusCode)
	}

	// Classify error
	switch statusCode {
	case 401, 403:
		pe.Code = types.ErrAuthentication
		pe.Retryable = false
	case 429:
		pe.Code = types.ErrRateLimit
		pe.Retryable = true
	case 400, 422:
		pe.Code = types.ErrInvalidRequest
		pe.Retryable = false
	case 404:
		pe.Code = types.ErrModelNotFound
		pe.Retryable = false
	case 500, 502, 503, 504:
		pe.Code = types.ErrProviderError
		pe.Retryable = true
	default:
		pe.Code = types.ErrProviderError
		pe.Retryable = statusCode >= 500
	}

	return pe
}

// SetTimeout updates the client timeout.
func (c *HTTPClient) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
	c.client.Timeout = timeout
}
