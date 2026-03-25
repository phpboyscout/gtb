---
title: Output
description: Structured output formatting for CLI commands supporting human-readable text and machine-readable JSON.
date: 2026-03-25
tags: [components, output, json, formatting, cli]
authors: [Matt Cockayne <matt@phpboyscout.com>]
---

# Output

`pkg/output` provides dual-format output for CLI commands. Commands write once
using `Writer.Write` and the format — text or JSON — is determined at runtime
by the caller's `--output` flag. This makes GTB commands scriptable without
any branching in command logic.

---

## Quick Start

```go
import (
    "os"
    "fmt"
    "github.com/phpboyscout/go-tool-base/pkg/output"
)

type Result struct {
    Name    string `json:"name"`
    Version string `json:"version"`
}

func runMyCommand(cmd *cobra.Command, args []string) error {
    format, _ := cmd.Flags().GetString("output")
    w := output.NewWriter(os.Stdout, output.Format(format))

    result := &Result{Name: "myapp", Version: "v1.2.3"}

    return w.Write(result, func(out io.Writer) {
        fmt.Fprintf(out, "Name:    %s\n", result.Name)
        fmt.Fprintf(out, "Version: %s\n", result.Version)
    })
}
```

Running `mytool info` produces:

```
Name:    myapp
Version: v1.2.3
```

Running `mytool info --output json` produces:

```json
{
  "name": "myapp",
  "version": "v1.2.3"
}
```

---

## API Reference

### Format

```go
type Format string

const (
    FormatText Format = "text"  // Human-readable (default)
    FormatJSON Format = "json"  // Machine-readable JSON
)
```

### Writer

```go
// NewWriter creates an output writer for the given io.Writer and format.
func NewWriter(w io.Writer, format Format) *Writer

// Write outputs data in the configured format.
// For JSON, data is marshalled to indented JSON via encoding/json.
// For text, textFunc is called with the output writer.
func (o *Writer) Write(data any, textFunc func(io.Writer)) error

// IsJSON returns true if the writer is configured for JSON output.
func (o *Writer) IsJSON() bool
```

---

## Adding an Output Flag

Add a standard `--output` flag to any command:

```go
func NewMyCommand(p *props.Props) *cobra.Command {
    var outputFormat string

    cmd := &cobra.Command{
        Use:   "list",
        Short: "List resources",
        RunE: func(cmd *cobra.Command, args []string) error {
            w := output.NewWriter(os.Stdout, output.Format(outputFormat))
            items := fetchItems()
            return w.Write(items, func(out io.Writer) {
                for _, item := range items {
                    fmt.Fprintln(out, item.Name)
                }
            })
        },
    }

    cmd.Flags().StringVarP(&outputFormat, "output", "o", string(output.FormatText),
        `Output format: "text" or "json"`)

    return cmd
}
```

---

## JSON Output Design

When `FormatJSON` is active, `Write` marshals `data` using `encoding/json`
with two-space indentation. The `textFunc` is not called.

```go
type ServiceStatus struct {
    Name    string `json:"name"`
    Running bool   `json:"running"`
    Uptime  string `json:"uptime,omitempty"`
}

statuses := []ServiceStatus{
    {Name: "http", Running: true, Uptime: "2h34m"},
    {Name: "grpc", Running: false},
}

w := output.NewWriter(os.Stdout, output.FormatJSON)
w.Write(statuses, nil) // textFunc unused for JSON
```

Output:

```json
[
  {
    "name": "http",
    "running": true,
    "uptime": "2h34m"
  },
  {
    "name": "grpc",
    "running": false
  }
]
```

---

## Conditional Formatting

Use `IsJSON()` when you need to alter behaviour beyond output formatting
(e.g., suppress progress indicators in JSON mode):

```go
w := output.NewWriter(os.Stdout, format)

if !w.IsJSON() {
    spinner := startSpinner("Fetching...")
    defer spinner.Stop()
}

result := fetchData()
return w.Write(result, func(out io.Writer) {
    fmt.Fprintf(out, "Fetched %d records\n", len(result))
})
```

---

## Writing to Stderr or Custom Writers

`NewWriter` accepts any `io.Writer`. Use `os.Stderr` for error output or
`bytes.Buffer` in tests:

```go
// Write errors as JSON to stderr
errWriter := output.NewWriter(os.Stderr, format)

// Capture output in tests
var buf bytes.Buffer
w := output.NewWriter(&buf, output.FormatJSON)
w.Write(data, textFunc)
assert.Contains(t, buf.String(), `"name"`)
```

---

## Related Documentation

- **[Props](props.md)** — dependency injection container
- **[Chat](chat.md)** — structured output from AI responses
