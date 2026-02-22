package transport

import (
	"net/http"
	"strings"
	"testing"
)

func TestBearerAuth_Apply(t *testing.T) {
	auth := &BearerAuth{APIKey: "sk-test-123"}
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	err := auth.Apply(req)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	expected := "Bearer sk-test-123"
	if req.Header.Get("Authorization") != expected {
		t.Errorf("expected Authorization %q, got %q", expected, req.Header.Get("Authorization"))
	}
}

func TestAnthropicAuth_Apply(t *testing.T) {
	auth := &AnthropicAuth{APIKey: "sk-ant-test", Version: "2024-01-01"}
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	err := auth.Apply(req)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if req.Header.Get("x-api-key") != "sk-ant-test" {
		t.Errorf("expected x-api-key 'sk-ant-test', got %q", req.Header.Get("x-api-key"))
	}
	if req.Header.Get("anthropic-version") != "2024-01-01" {
		t.Errorf("expected anthropic-version '2024-01-01', got %q", req.Header.Get("anthropic-version"))
	}
}

func TestAnthropicAuth_DefaultVersion(t *testing.T) {
	auth := &AnthropicAuth{APIKey: "sk-ant-test"}
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	auth.Apply(req)

	if req.Header.Get("anthropic-version") != "2023-06-01" {
		t.Errorf("expected default anthropic-version '2023-06-01', got %q", req.Header.Get("anthropic-version"))
	}
}

func TestGoogleAuth_Apply(t *testing.T) {
	auth := &GoogleAuth{APIKey: "AIza-test-key"}
	req, _ := http.NewRequest("GET", "http://example.com/api", nil)

	err := auth.Apply(req)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if req.URL.Query().Get("key") != "AIza-test-key" {
		t.Errorf("expected query param key 'AIza-test-key', got %q", req.URL.Query().Get("key"))
	}
}

func TestDynamicAuth_UseDynamic(t *testing.T) {
	staticAuth := &BearerAuth{APIKey: "static-key"}
	dynamicAuth := &DynamicAuth{
		StaticAuth:  staticAuth,
		Credentials: map[string]string{"api_key": "dynamic-key"},
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	dynamicAuth.Apply(req)

	expected := "Bearer dynamic-key"
	if req.Header.Get("Authorization") != expected {
		t.Errorf("expected Authorization %q, got %q", expected, req.Header.Get("Authorization"))
	}
}

func TestDynamicAuth_FallbackToStatic(t *testing.T) {
	staticAuth := &BearerAuth{APIKey: "static-key"}
	dynamicAuth := &DynamicAuth{
		StaticAuth:  staticAuth,
		Credentials: map[string]string{}, // Empty credentials
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	dynamicAuth.Apply(req)

	expected := "Bearer static-key"
	if req.Header.Get("Authorization") != expected {
		t.Errorf("expected Authorization %q, got %q", expected, req.Header.Get("Authorization"))
	}
}

func TestNewAuthStrategy(t *testing.T) {
	tests := []struct {
		provider string
		apiKey   string
	}{
		{"openai", "sk-test"},
		{"anthropic", "sk-ant"},
		{"google", "AIza"},
		{"alibaba", "sk-ali"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			auth := NewAuthStrategy(tt.provider, tt.apiKey)
			if auth == nil {
				t.Error("expected non-nil auth strategy")
			}
		})
	}
}

func TestSSEReader_Read(t *testing.T) {
	input := `event: message
data: {"text": "Hello"}

data: {"text": "World"}

data: [DONE]

`
	reader := NewSSEReader(strings.NewReader(input))

	// First event
	event1, err := reader.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if event1.Event != "message" {
		t.Errorf("expected event 'message', got %q", event1.Event)
	}
	if event1.Data != `{"text": "Hello"}` {
		t.Errorf("expected data '{\"text\": \"Hello\"}', got %q", event1.Data)
	}

	// Second event
	event2, err := reader.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if event2.Data != `{"text": "World"}` {
		t.Errorf("expected data '{\"text\": \"World\"}', got %q", event2.Data)
	}

	// Done event
	event3, err := reader.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if !event3.IsDone() {
		t.Error("expected done event")
	}
}

func TestSSEReader_MultiLineData(t *testing.T) {
	input := `data: line1
data: line2
data: line3

`
	reader := NewSSEReader(strings.NewReader(input))

	event, err := reader.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	expected := "line1\nline2\nline3"
	if event.Data != expected {
		t.Errorf("expected data %q, got %q", expected, event.Data)
	}
}

func TestSSEWriter_Write(t *testing.T) {
	var sb strings.Builder
	writer := NewSSEWriter(&sb)

	event := &SSEEvent{
		Event: "message",
		Data:  `{"text": "Hello"}`,
	}

	err := writer.Write(event)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	expected := "event: message\ndata: {\"text\": \"Hello\"}\n\n"
	if sb.String() != expected {
		t.Errorf("expected %q, got %q", expected, sb.String())
	}
}
