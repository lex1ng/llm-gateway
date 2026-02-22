// Package types defines the core types for LLM Gateway.
// All types follow OpenAI format as the internal standard.
package types

import (
	"encoding/json"
	"strings"
)

// Role represents the role of a message sender.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message represents a unified message structure.
// Compatible with OpenAI Chat Completions API format.
type Message struct {
	Role       Role       `json:"role"`
	Content    Content    `json:"content"`                // string or []ContentBlock
	Name       string     `json:"name,omitempty"`         // optional sender name
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // assistant-initiated tool calls
	ToolCallID string     `json:"tool_call_id,omitempty"` // tool role: reference to tool_call
}

// Content supports dual-mode serialization:
// - String mode: simple text content
// - Block mode: multimodal content blocks (text, image, audio, etc.)
type Content struct {
	Text   string         // plain text mode
	Blocks []ContentBlock // multimodal mode
}

// IsMultimodal returns true if content contains multiple blocks.
func (c Content) IsMultimodal() bool {
	return len(c.Blocks) > 0
}

// String returns the text content.
// For multimodal content, concatenates all text blocks.
func (c Content) String() string {
	if !c.IsMultimodal() {
		return c.Text
	}
	var sb strings.Builder
	for _, block := range c.Blocks {
		if block.Type == ContentTypeText {
			sb.WriteString(block.Text)
		}
	}
	return sb.String()
}

// MarshalJSON implements json.Marshaler for dual-mode serialization.
// - If Blocks is non-empty, serializes as []ContentBlock
// - Otherwise, serializes as string
func (c Content) MarshalJSON() ([]byte, error) {
	if c.IsMultimodal() {
		return json.Marshal(c.Blocks)
	}
	return json.Marshal(c.Text)
}

// UnmarshalJSON implements json.Unmarshaler for dual-mode deserialization.
// - If JSON is string, sets Text field
// - If JSON is array, sets Blocks field
func (c *Content) UnmarshalJSON(data []byte) error {
	// Try string first
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		c.Text = text
		c.Blocks = nil
		return nil
	}

	// Try array of content blocks
	var blocks []ContentBlock
	if err := json.Unmarshal(data, &blocks); err == nil {
		c.Blocks = blocks
		c.Text = ""
		return nil
	}

	// If both fail, treat as empty string
	c.Text = ""
	c.Blocks = nil
	return nil
}

// NewTextContent creates a Content with plain text.
func NewTextContent(text string) Content {
	return Content{Text: text}
}

// NewMultimodalContent creates a Content with multiple blocks.
func NewMultimodalContent(blocks ...ContentBlock) Content {
	return Content{Blocks: blocks}
}

// ContentType defines the type of content block.
type ContentType string

const (
	ContentTypeText     ContentType = "text"
	ContentTypeImageURL ContentType = "image_url"
	ContentTypeAudio    ContentType = "audio"
	ContentTypeDocument ContentType = "document"
)

// ContentBlock represents a multimodal content block.
type ContentBlock struct {
	Type     ContentType `json:"type"`
	Text     string      `json:"text,omitempty"`
	ImageURL *ImageURL   `json:"image_url,omitempty"`
	Audio    *Audio      `json:"audio,omitempty"`
	Document *Document   `json:"document,omitempty"`
}

// NewTextBlock creates a text content block.
func NewTextBlock(text string) ContentBlock {
	return ContentBlock{
		Type: ContentTypeText,
		Text: text,
	}
}

// NewImageURLBlock creates an image URL content block.
func NewImageURLBlock(url string, detail ImageDetail) ContentBlock {
	return ContentBlock{
		Type: ContentTypeImageURL,
		ImageURL: &ImageURL{
			URL:    url,
			Detail: detail,
		},
	}
}

// ImageURL represents an image reference in content.
type ImageURL struct {
	URL    string      `json:"url"`              // URL or data:base64
	Detail ImageDetail `json:"detail,omitempty"` // low / high / auto
}

// ImageDetail specifies the detail level for image processing.
type ImageDetail string

const (
	ImageDetailLow  ImageDetail = "low"
	ImageDetailHigh ImageDetail = "high"
	ImageDetailAuto ImageDetail = "auto"
)

// Audio represents audio content in a message.
type Audio struct {
	Data   string `json:"data,omitempty"`   // base64 encoded audio
	Format string `json:"format,omitempty"` // wav, mp3, etc.
	URL    string `json:"url,omitempty"`    // alternative: audio URL
}

// Document represents a document attachment.
type Document struct {
	Type    string `json:"type,omitempty"`    // pdf, docx, etc.
	Data    string `json:"data,omitempty"`    // base64 encoded
	URL     string `json:"url,omitempty"`     // alternative: document URL
	Name    string `json:"name,omitempty"`    // filename
	MediaID string `json:"media_id,omitempty"` // platform-specific media ID
}

// ToolCall represents a tool/function call made by the assistant.
// Defined here to avoid circular imports; full Tool types in tool.go.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall contains the function name and arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}
