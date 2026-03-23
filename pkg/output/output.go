// Package output provides structured output formatting for CLI commands.
// It supports text (default) and JSON output modes, allowing commands to
// produce machine-readable output for CI/CD pipelines and scripting.
package output

import (
	"encoding/json"
	"io"
)

// Format represents the output format for commands.
type Format string

const (
	// FormatText is the default human-readable output format.
	FormatText Format = "text"
	// FormatJSON produces machine-readable JSON output.
	FormatJSON Format = "json"
)

// Writer handles formatted output based on the configured format.
type Writer struct {
	format Format
	w      io.Writer
}

// NewWriter creates an output writer for the given format.
func NewWriter(w io.Writer, format Format) *Writer {
	return &Writer{format: format, w: w}
}

// Write outputs data in the configured format.
// For JSON format, data is marshalled to indented JSON.
// For text format, the textFunc is called to produce human-readable output.
func (o *Writer) Write(data any, textFunc func(io.Writer)) error {
	if o.format == FormatJSON {
		enc := json.NewEncoder(o.w)
		enc.SetIndent("", "  ")

		return enc.Encode(data)
	}

	textFunc(o.w)

	return nil
}

// IsJSON returns true if the writer is configured for JSON output.
func (o *Writer) IsJSON() bool {
	return o.format == FormatJSON
}
