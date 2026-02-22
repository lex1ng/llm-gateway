package types

import (
	"encoding/json"
	"testing"
)

func TestContent_MarshalJSON_Text(t *testing.T) {
	c := NewTextContent("Hello, world!")

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	expected := `"Hello, world!"`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, string(data))
	}
}

func TestContent_MarshalJSON_Multimodal(t *testing.T) {
	c := NewMultimodalContent(
		NewTextBlock("Check this image:"),
		NewImageURLBlock("https://example.com/image.png", ImageDetailAuto),
	)

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	// Should be an array
	var blocks []ContentBlock
	if err := json.Unmarshal(data, &blocks); err != nil {
		t.Fatalf("Result should be unmarshalable as []ContentBlock: %v", err)
	}

	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(blocks))
	}
}

func TestContent_UnmarshalJSON_String(t *testing.T) {
	data := []byte(`"Hello, world!"`)

	var c Content
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	if c.Text != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got '%s'", c.Text)
	}
	if c.IsMultimodal() {
		t.Error("expected non-multimodal content")
	}
}

func TestContent_UnmarshalJSON_Array(t *testing.T) {
	data := []byte(`[
		{"type": "text", "text": "Hello"},
		{"type": "image_url", "image_url": {"url": "https://example.com/img.png", "detail": "high"}}
	]`)

	var c Content
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	if !c.IsMultimodal() {
		t.Error("expected multimodal content")
	}
	if len(c.Blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(c.Blocks))
	}
	if c.Blocks[0].Type != ContentTypeText {
		t.Errorf("expected text type, got %s", c.Blocks[0].Type)
	}
	if c.Blocks[1].Type != ContentTypeImageURL {
		t.Errorf("expected image_url type, got %s", c.Blocks[1].Type)
	}
	if c.Blocks[1].ImageURL.Detail != ImageDetailHigh {
		t.Errorf("expected high detail, got %s", c.Blocks[1].ImageURL.Detail)
	}
}

func TestContent_String(t *testing.T) {
	// Text mode
	c1 := NewTextContent("Hello")
	if c1.String() != "Hello" {
		t.Errorf("expected 'Hello', got '%s'", c1.String())
	}

	// Multimodal mode - should concatenate text blocks
	c2 := NewMultimodalContent(
		NewTextBlock("Hello "),
		NewImageURLBlock("https://example.com/img.png", ImageDetailAuto),
		NewTextBlock("World"),
	)
	if c2.String() != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", c2.String())
	}
}

func TestMessage_JSON_RoundTrip(t *testing.T) {
	msg := Message{
		Role:    RoleUser,
		Content: NewTextContent("What's in this image?"),
		Name:    "user123",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Role != msg.Role {
		t.Errorf("role mismatch: expected %s, got %s", msg.Role, decoded.Role)
	}
	if decoded.Content.Text != msg.Content.Text {
		t.Errorf("content mismatch: expected %s, got %s", msg.Content.Text, decoded.Content.Text)
	}
	if decoded.Name != msg.Name {
		t.Errorf("name mismatch: expected %s, got %s", msg.Name, decoded.Name)
	}
}

func TestMessage_WithToolCalls(t *testing.T) {
	msg := Message{
		Role:    RoleAssistant,
		Content: NewTextContent(""),
		ToolCalls: []ToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: FunctionCall{
					Name:      "get_weather",
					Arguments: `{"location": "Beijing"}`,
				},
			},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(decoded.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(decoded.ToolCalls))
	}
	if decoded.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("expected 'get_weather', got '%s'", decoded.ToolCalls[0].Function.Name)
	}
}
