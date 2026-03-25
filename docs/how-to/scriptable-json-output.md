---
title: Add Scriptable JSON Output to a Command
description: How to use pkg/output to make a command produce both human-readable and machine-readable JSON output.
date: 2026-03-25
tags: [how-to, output, json, scripting, automation]
authors: [Matt Cockayne <matt@phpboyscout.com>]
---

# Add Scriptable JSON Output to a Command

GTB's `pkg/output` package makes it straightforward to give any command a `--output` flag that switches between human-readable text and machine-parseable JSON. This is essential for commands that need to integrate with CI/CD pipelines or shell scripts.

---

## How It Works

`output.Writer` wraps a single `Write(data any, textFunc func(io.Writer)) error` call. You pass the structured data *and* a text-rendering closure together — the writer decides which to use based on the configured format.

```
FormatText  →  textFunc(w) is called
FormatJSON  →  data is JSON-marshalled and written
```

---

## Step 1: Define a Result Struct

The struct is your JSON contract. Tag every exported field:

```go
type ServiceStatus struct {
    Name    string `json:"name"`
    Version string `json:"version"`
    Healthy bool   `json:"healthy"`
    Uptime  string `json:"uptime"`
}
```

---

## Step 2: Add an `--output` Flag to the Command

```go
import (
    "os"
    "fmt"

    "github.com/spf13/cobra"
    "github.com/phpboyscout/go-tool-base/pkg/output"
    "github.com/phpboyscout/go-tool-base/pkg/props"
)

func NewCmdStatus(p *props.Props) *cobra.Command {
    var outputFormat string

    cmd := &cobra.Command{
        Use:   "status",
        Short: "Show service status",
        RunE: func(cmd *cobra.Command, args []string) error {
            return runStatus(cmd, p, output.Format(outputFormat))
        },
    }

    cmd.Flags().StringVarP(&outputFormat, "output", "o", string(output.FormatText),
        `Output format: "text" or "json"`)

    return cmd
}
```

---

## Step 3: Use the Writer in the Run Function

```go
func runStatus(cmd *cobra.Command, p *props.Props, format output.Format) error {
    // Gather your data
    status := ServiceStatus{
        Name:    p.Tool.Name,
        Version: p.Version.String(),
        Healthy: true,
        Uptime:  "4h32m",
    }

    // Create a writer targeting the command's stdout
    w := output.NewWriter(cmd.OutOrStdout(), format)

    return w.Write(status, func(out io.Writer) {
        // Text rendering — only called when format == "text"
        fmt.Fprintf(out, "Service:  %s\n", status.Name)
        fmt.Fprintf(out, "Version:  %s\n", status.Version)
        fmt.Fprintf(out, "Healthy:  %v\n", status.Healthy)
        fmt.Fprintf(out, "Uptime:   %s\n", status.Uptime)
    })
}
```

---

## Step 4: Test Both Formats

```bash
# Human-readable (default)
mytool status
# Service:  mytool
# Version:  v1.2.3
# Healthy:  true
# Uptime:   4h32m

# Machine-readable
mytool status --output json
# {
#   "name": "mytool",
#   "version": "v1.2.3",
#   "healthy": true,
#   "uptime": "4h32m"
# }
```

---

## Conditional Logic Based on Format

Sometimes your text output requires extra work (e.g. table formatting) that you want to skip in JSON mode. Use `IsJSON()` to short-circuit:

```go
w := output.NewWriter(cmd.OutOrStdout(), format)

if !w.IsJSON() {
    // fetch extra data for display only
    details, err := fetchDetails()
    if err != nil {
        return err
    }
    status.ExtraInfo = details.Summary
}

return w.Write(status, func(out io.Writer) {
    renderTable(out, status)
})
```

---

## Outputting a List

`Write` accepts any JSON-serialisable value, including slices:

```go
type ServiceList struct {
    Services []ServiceStatus `json:"services"`
    Total    int             `json:"total"`
}

result := ServiceList{Services: services, Total: len(services)}

return w.Write(result, func(out io.Writer) {
    for _, svc := range result.Services {
        fmt.Fprintf(out, "%-20s %s\n", svc.Name, svc.Version)
    }
})
```

---

## Testing

In tests, use `bytes.Buffer` as the writer and pass `output.FormatJSON` to assert the structured output:

```go
func TestRunStatus(t *testing.T) {
    var buf bytes.Buffer
    cmd := &cobra.Command{}
    cmd.SetOut(&buf)

    err := runStatus(cmd, testProps, output.FormatJSON)
    require.NoError(t, err)

    var result ServiceStatus
    require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
    assert.Equal(t, "mytool", result.Name)
    assert.True(t, result.Healthy)
}
```

---

## Related Documentation

- **[Output component](../components/output.md)** — `Writer`, `Format`, `IsJSON` API reference
- **[Adding Custom Commands](custom-commands.md)** — command wiring patterns
