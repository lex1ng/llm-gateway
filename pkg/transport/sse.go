package transport

import (
	"bufio"
	"io"
	"strings"
)

// SSEEvent represents a Server-Sent Event.
type SSEEvent struct {
	Event string // Event type (e.g., "message", "content_block_delta")
	Data  string // Event data
	ID    string // Event ID (optional)
	Retry int    // Retry interval in milliseconds (optional)
}

// IsDone returns true if this is a done/end event.
func (e *SSEEvent) IsDone() bool {
	return e.Data == "[DONE]" || e.Event == "message_stop"
}

// SSEReader reads Server-Sent Events from an io.Reader.
type SSEReader struct {
	reader *bufio.Reader
}

// NewSSEReader creates a new SSEReader.
func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{
		reader: bufio.NewReader(r),
	}
}

// Read reads the next SSE event.
// Returns io.EOF when the stream ends.
func (r *SSEReader) Read() (*SSEEvent, error) {
	event := &SSEEvent{}
	var dataLines []string

	for {
		line, err := r.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// If we have accumulated data, return the event
				if len(dataLines) > 0 {
					event.Data = strings.Join(dataLines, "\n")
					return event, nil
				}
				return nil, io.EOF
			}
			return nil, err
		}

		// Remove trailing newline/carriage return
		line = strings.TrimRight(line, "\r\n")

		// Empty line indicates end of event
		if line == "" {
			if len(dataLines) > 0 || event.Event != "" {
				event.Data = strings.Join(dataLines, "\n")
				return event, nil
			}
			continue
		}

		// Parse the line
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimPrefix(data, " ") // Optional space after colon
			dataLines = append(dataLines, data)
		} else if strings.HasPrefix(line, "event:") {
			event.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "id:") {
			event.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		} else if strings.HasPrefix(line, "retry:") {
			// Parse retry value (ignored for now)
		} else if strings.HasPrefix(line, ":") {
			// Comment line, ignore
			continue
		}
	}
}

// ReadAll reads all SSE events from the reader.
// Stops when EOF is reached or when encountering [DONE].
func (r *SSEReader) ReadAll() ([]*SSEEvent, error) {
	var events []*SSEEvent

	for {
		event, err := r.Read()
		if err != nil {
			if err == io.EOF {
				return events, nil
			}
			return events, err
		}

		events = append(events, event)

		if event.IsDone() {
			return events, nil
		}
	}
}

// SSEWriter writes Server-Sent Events to an io.Writer.
type SSEWriter struct {
	writer io.Writer
}

// NewSSEWriter creates a new SSEWriter.
func NewSSEWriter(w io.Writer) *SSEWriter {
	return &SSEWriter{writer: w}
}

// Write writes an SSE event.
func (w *SSEWriter) Write(event *SSEEvent) error {
	var sb strings.Builder

	if event.Event != "" {
		sb.WriteString("event: ")
		sb.WriteString(event.Event)
		sb.WriteString("\n")
	}

	if event.ID != "" {
		sb.WriteString("id: ")
		sb.WriteString(event.ID)
		sb.WriteString("\n")
	}

	// Handle multi-line data
	lines := strings.Split(event.Data, "\n")
	for _, line := range lines {
		sb.WriteString("data: ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString("\n") // End of event

	_, err := w.writer.Write([]byte(sb.String()))
	return err
}

// WriteData writes a simple data-only SSE event.
func (w *SSEWriter) WriteData(data string) error {
	return w.Write(&SSEEvent{Data: data})
}

// WriteDone writes the [DONE] event.
func (w *SSEWriter) WriteDone() error {
	return w.WriteData("[DONE]")
}
