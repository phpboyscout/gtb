---
title: "Operational Tooling Specification"
description: "Add structured JSON output mode, shell completion generation, and a doctor/diagnose command for configuration validation and environment checks."
date: 2026-03-21
status: DRAFT
tags:
  - specification
  - cli
  - ux
  - feature
  - operations
author:
  - name: Matt Cockayne
    email: matt@phpboyscout.com
  - name: Claude (claude-opus-4-6)
    role: AI drafting assistant
---

# Operational Tooling Specification

Authors
:   Matt Cockayne, Claude (claude-opus-4-6) *(AI drafting assistant)*

Date
:   21 March 2026

Status
:   DRAFT

---

## Overview

Three related CLI features improve the operational experience for both human users and CI/CD pipelines:

1. **Structured JSON output** (`--output json`): Machine-readable output for CI integration, scripting, and programmatic consumption. Currently all output is human-formatted text with no structured alternative.

2. **Shell completions** (`completion` command): Tab-completion for bash, zsh, fish, and PowerShell via Cobra's built-in completion generation. This is a low-effort, high-value UX improvement.

3. **Doctor/diagnose command** (`doctor`): A single command that validates configuration, tests VCS connectivity, verifies API keys, and reports runtime environment details. Reduces support burden by giving users a self-service diagnostic tool.

---

## Design Decisions

**Global `--output` flag**: The output format flag is added to the root command as a persistent flag, making it available to all subcommands. Commands that support structured output check this flag and format accordingly. Commands that don't support it ignore it gracefully.

**Cobra native completions**: Cobra provides `GenBashCompletion`, `GenZshCompletion`, `GenFishCompletion`, and `GenPowerShellCompletion` out of the box. We wrap these in a `completion` subcommand with a shell argument.

**Doctor as checklist**: The `doctor` command runs a series of checks and reports pass/fail/warn for each. This is modelled after `brew doctor` and `flutter doctor`. Each check is independent — one failure doesn't prevent others from running.

---

## Public API Changes

### New: Output Format Types

```go
// OutputFormat represents the output format for commands.
type OutputFormat string

const (
    OutputFormatText OutputFormat = "text"
    OutputFormatJSON OutputFormat = "json"
)
```

### New: `doctor` Check Types

```go
// CheckResult represents the outcome of a single doctor check.
type CheckResult struct {
    Name    string       `json:"name"`
    Status  CheckStatus  `json:"status"`
    Message string       `json:"message"`
    Details string       `json:"details,omitempty"`
}

// CheckStatus is the outcome of a check.
type CheckStatus string

const (
    CheckPass CheckStatus = "pass"
    CheckWarn CheckStatus = "warn"
    CheckFail CheckStatus = "fail"
    CheckSkip CheckStatus = "skip"
)

// DoctorReport contains all check results.
type DoctorReport struct {
    Tool    string        `json:"tool"`
    Version string        `json:"version"`
    Checks  []CheckResult `json:"checks"`
}
```

---

## Internal Implementation

### Structured JSON Output

#### Global Flag

```go
func setupRootFlags(rootCmd *cobra.Command, props *p.Props, state *rootState) {
    rootCmd.PersistentFlags().StringVar(&state.outputFormat, "output", "text", "output format (text, json)")
}
```

#### Output Helper

```go
// pkg/output/output.go

// Writer handles formatted output based on the configured format.
type Writer struct {
    format OutputFormat
    w      io.Writer
}

// NewWriter creates an output writer for the given format.
func NewWriter(w io.Writer, format OutputFormat) *Writer {
    return &Writer{format: format, w: w}
}

// Write outputs data in the configured format.
// For JSON format, data is marshalled to JSON.
// For text format, the textFunc is called to produce human-readable output.
func (o *Writer) Write(data any, textFunc func(io.Writer)) error {
    switch o.format {
    case OutputFormatJSON:
        enc := json.NewEncoder(o.w)
        enc.SetIndent("", "  ")
        return enc.Encode(data)
    default:
        textFunc(o.w)
        return nil
    }
}
```

#### Example Command Usage

```go
func runVersionCmd(cmd *cobra.Command, props *p.Props) error {
    format, _ := cmd.Flags().GetString("output")
    out := output.NewWriter(os.Stdout, output.OutputFormat(format))

    data := struct {
        Tool    string `json:"tool"`
        Version string `json:"version"`
    }{
        Tool:    props.Tool.Name,
        Version: props.Version.String(),
    }

    return out.Write(data, func(w io.Writer) {
        fmt.Fprintf(w, "%s version %s\n", data.Tool, data.Version)
    })
}
```

### Shell Completions

```go
// pkg/cmd/completion/completion.go

func NewCmdCompletion() *cobra.Command {
    cmd := &cobra.Command{
        Use:       "completion [bash|zsh|fish|powershell]",
        Short:     "Generate shell completion scripts",
        Long:      completionLong,
        Args:      cobra.ExactArgs(1),
        ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
        RunE: func(cmd *cobra.Command, args []string) error {
            switch args[0] {
            case "bash":
                return cmd.Root().GenBashCompletion(os.Stdout)
            case "zsh":
                return cmd.Root().GenZshCompletion(os.Stdout)
            case "fish":
                return cmd.Root().GenFishCompletion(os.Stdout, true)
            case "powershell":
                return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
            default:
                return errors.Newf("unsupported shell: %s", args[0])
            }
        },
    }
    return cmd
}

var completionLong = `Generate shell completion scripts for the specified shell.

To load completions:

Bash:
  $ source <(toolname completion bash)
  # Or add to ~/.bashrc:
  $ toolname completion bash > /etc/bash_completion.d/toolname

Zsh:
  $ toolname completion zsh > "${fpath[1]}/_toolname"

Fish:
  $ toolname completion fish | source
  # Or persist:
  $ toolname completion fish > ~/.config/fish/completions/toolname.fish

PowerShell:
  PS> toolname completion powershell | Out-String | Invoke-Expression
`
```

### Doctor Command

```go
// pkg/cmd/doctor/doctor.go

func NewCmdDoctor(props *p.Props) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "doctor",
        Short: "Check environment and configuration health",
        RunE: func(cmd *cobra.Command, args []string) error {
            format, _ := cmd.Flags().GetString("output")
            out := output.NewWriter(os.Stdout, output.OutputFormat(format))

            report := runChecks(cmd.Context(), props)
            return out.Write(report, func(w io.Writer) {
                printReport(w, report)
            })
        },
    }
    return cmd
}

func runChecks(ctx context.Context, props *p.Props) *DoctorReport {
    report := &DoctorReport{
        Tool:    props.Tool.Name,
        Version: props.Version.String(),
    }

    checks := []func(context.Context, *p.Props) CheckResult{
        checkGoVersion,
        checkConfig,
        checkGitConnectivity,
        checkAPIKeys,
        checkDiskSpace,
        checkPermissions,
    }

    for _, check := range checks {
        report.Checks = append(report.Checks, check(ctx, props))
    }

    return report
}
```

#### Individual Checks

```go
func checkGoVersion(ctx context.Context, props *p.Props) CheckResult {
    version := runtime.Version()
    if strings.HasPrefix(version, "go1.21") || strings.HasPrefix(version, "go1.22") {
        return CheckResult{Name: "Go version", Status: CheckPass, Message: version}
    }
    return CheckResult{Name: "Go version", Status: CheckWarn, Message: version, Details: "Go 1.21+ recommended"}
}

func checkConfig(ctx context.Context, props *p.Props) CheckResult {
    if props.Config == nil {
        return CheckResult{Name: "Configuration", Status: CheckFail, Message: "no configuration loaded"}
    }
    return CheckResult{Name: "Configuration", Status: CheckPass, Message: "loaded successfully"}
}

func checkGitConnectivity(ctx context.Context, props *p.Props) CheckResult {
    // Check if git is available and the repo is accessible
    cmd := exec.CommandContext(ctx, "git", "status")
    if err := cmd.Run(); err != nil {
        return CheckResult{Name: "Git", Status: CheckWarn, Message: "git not available or not in a repository"}
    }
    return CheckResult{Name: "Git", Status: CheckPass, Message: "repository accessible"}
}

func checkAPIKeys(ctx context.Context, props *p.Props) CheckResult {
    keys := map[string]string{
        "anthropic": props.Config.GetString("anthropic.api_key"),
        "openai":    props.Config.GetString("openai.api_key"),
        "gemini":    props.Config.GetString("gemini.api_key"),
    }

    configured := 0
    for _, v := range keys {
        if v != "" {
            configured++
        }
    }

    if configured == 0 {
        return CheckResult{Name: "API keys", Status: CheckWarn, Message: "no AI provider API keys configured"}
    }
    return CheckResult{Name: "API keys", Status: CheckPass, Message: fmt.Sprintf("%d provider(s) configured", configured)}
}
```

#### Report Output

```go
func printReport(w io.Writer, report *DoctorReport) {
    fmt.Fprintf(w, "%s %s\n\n", report.Tool, report.Version)

    for _, check := range report.Checks {
        var icon string
        switch check.Status {
        case CheckPass:
            icon = "[OK]"
        case CheckWarn:
            icon = "[!!]"
        case CheckFail:
            icon = "[FAIL]"
        case CheckSkip:
            icon = "[SKIP]"
        }
        fmt.Fprintf(w, "  %s %s: %s\n", icon, check.Name, check.Message)
        if check.Details != "" {
            fmt.Fprintf(w, "       %s\n", check.Details)
        }
    }
}
```

---

## Project Structure

```
pkg/output/
├── output.go          ← NEW: Writer, OutputFormat
├── output_test.go     ← NEW: output formatting tests
pkg/cmd/completion/
├── completion.go      ← NEW: shell completion command
├── completion_test.go ← NEW: completion generation tests
pkg/cmd/doctor/
├── doctor.go          ← NEW: doctor command, checks
├── checks.go          ← NEW: individual check implementations
├── doctor_test.go     ← NEW: check tests
pkg/cmd/root/
├── root.go            ← MODIFIED: register completion + doctor commands, --output flag
```

---

## Testing Strategy

### JSON Output

| Test | Scenario |
|------|----------|
| `TestWriter_JSON` | Struct data → valid JSON output |
| `TestWriter_Text` | Text function called, data ignored |
| `TestWriter_JSONIndent` | Output is indented for readability |

### Completions

| Test | Scenario |
|------|----------|
| `TestCompletion_Bash` | Generates valid bash completion script |
| `TestCompletion_Zsh` | Generates valid zsh completion script |
| `TestCompletion_Fish` | Generates valid fish completion script |
| `TestCompletion_PowerShell` | Generates valid PowerShell completion script |
| `TestCompletion_InvalidShell` | Unknown shell → error |

### Doctor

| Test | Scenario |
|------|----------|
| `TestCheckGoVersion_Current` | Current Go version → pass |
| `TestCheckConfig_Loaded` | Config present → pass |
| `TestCheckConfig_Missing` | Nil config → fail |
| `TestCheckAPIKeys_None` | No keys configured → warn |
| `TestCheckAPIKeys_Some` | One key configured → pass with count |
| `TestDoctorReport_JSONOutput` | Report as JSON → valid structure |
| `TestDoctorReport_TextOutput` | Report as text → formatted with icons |

### Coverage
- Target: 90%+ for `pkg/output/`, `pkg/cmd/completion/`, `pkg/cmd/doctor/`.

---

## Linting

- `golangci-lint run --fix` must pass.
- No new `nolint` directives.

---

## Documentation

- Godoc for `pkg/output/` types.
- Godoc for `pkg/cmd/doctor/` check types and report format.
- User-facing documentation:
  - `docs/commands/completion.md` — shell completion setup instructions
  - `docs/commands/doctor.md` — doctor command usage and check descriptions
  - Update `docs/commands/` index with new commands
- Update root command help to list new subcommands.

---

## Backwards Compatibility

- **No breaking changes**. All features are additive.
- `--output text` is the default, preserving existing behaviour.
- New commands (`completion`, `doctor`) don't conflict with existing commands.

---

## Future Considerations

- **Additional output formats**: YAML, table, CSV for different CI/CD systems.
- **Custom doctor checks**: Allow plugins to register their own health checks.
- **Completion enhancements**: Dynamic completions based on config values (e.g., available VCS providers).
- **Doctor auto-fix**: Some checks could offer automatic remediation (e.g., creating missing config files).

---

## Implementation Phases

### Phase 1 — Output Package
1. Create `pkg/output/` with `Writer` and `OutputFormat`
2. Add `--output` persistent flag to root command
3. Add tests

### Phase 2 — Shell Completions
1. Create `pkg/cmd/completion/` command
2. Register with root command
3. Add tests and documentation

### Phase 3 — Doctor Command
1. Create `pkg/cmd/doctor/` with check framework
2. Implement individual checks
3. Support both text and JSON output
4. Register with root command

### Phase 4 — Migrate Existing Commands
1. Identify commands that benefit from JSON output (version, config dump)
2. Add `output.Writer` usage to those commands
3. Add tests for JSON output paths

---

## Verification

```bash
go build ./...
go test -race ./pkg/output/... ./pkg/cmd/completion/... ./pkg/cmd/doctor/...
go test ./...
golangci-lint run --fix

# Manual verification
go run . completion bash > /dev/null     # should produce bash completion
go run . completion zsh > /dev/null      # should produce zsh completion
go run . doctor                          # should show check results
go run . doctor --output json            # should show JSON report
go run . version --output json           # should show JSON version
```
