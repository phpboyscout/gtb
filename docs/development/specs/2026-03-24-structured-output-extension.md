---
title: "SPEC 8: Structured Output Extension"
description: "Extend the --output json flag to all built-in commands with a standard response envelope and helper utilities"
date: 2026-03-24
status: IMPLEMENTED
tags:
  - specification
  - output
  - cli
  - ux
  - ci-cd
author:
  - name: Matt Cockayne
    email: matt@phpboyscout.com
  - name: Claude (claude-opus-4-6)
    role: AI drafting assistant
---

# SPEC 8: Structured Output Extension

Authors
:   Matt Cockayne, Claude (claude-opus-4-6) *(AI drafting assistant)*

Date
:   24 March 2026

Status
:   DRAFT

---

## Overview

The `--output` flag is defined on the root command (`pkg/cmd/root/root.go:401`) and supports `text` and `json` formats. The `doctor` command already consumes this flag to emit structured JSON output. However, no other built-in commands honour the flag, meaning CI/CD pipelines and scripts that need machine-readable output from GTB-based tools must parse human-readable text.

This spec standardises a response envelope, provides helper utilities in `pkg/output/`, and extends JSON output support to the `version`, `update`, `init`, and `config` (Spec 7) commands.

---

## Design Decisions

### Standard response envelope

All JSON output uses a consistent envelope so consumers can rely on a single schema regardless of which command produced the output. The envelope includes a status field for quick success/failure checks, the command name for context, a data payload for command-specific content, and an optional error message.

### Emit helper centralises formatting

Rather than having each command implement its own JSON marshalling, a single `output.Emit()` function handles format detection (reading the `--output` flag), serialisation, and writing to stdout. This ensures consistent behaviour and makes it trivial to add new output formats (e.g. YAML) in the future.

### Text output unchanged

When `--output` is `text` (the default), commands behave exactly as they do today. The `Emit` function falls through to a no-op for text mode, and the command uses its existing logger/fmt output. This ensures zero disruption for interactive users.

### Doctor command migrated to envelope

The existing `doctor` JSON output is migrated to use the standard `Response` envelope. This is a minor breaking change to the JSON schema but is acceptable because the feature is new and not yet widely consumed.

---

## Public API Changes

### `pkg/output/` additions

```go
// Response is the standard JSON envelope for all command output.
type Response struct {
    Status  string `json:"status"`            // "success", "error", "warning"
    Command string `json:"command"`           // command name, e.g. "version"
    Data    any    `json:"data,omitempty"`    // command-specific payload
    Error   string `json:"error,omitempty"`   // error message when status is "error"
}

// Emit writes the response to stdout in the format specified by the --output flag.
// When the format is "text", Emit is a no-op and returns nil, allowing the caller
// to fall through to its normal text rendering.
// When the format is "json", Emit marshals the Response and writes it to stdout.
// Returns an error if marshalling or writing fails.
func Emit(cmd *cobra.Command, resp Response) error

// IsJSONOutput returns true if the --output flag is set to "json" on the given command.
// Commands can use this to skip text-only rendering when JSON output is requested.
func IsJSONOutput(cmd *cobra.Command) bool

// EmitError is a convenience function that builds an error Response and emits it.
func EmitError(cmd *cobra.Command, commandName string, err error) error

// StatusSuccess, StatusError, StatusWarning are constants for the Status field.
const (
    StatusSuccess = "success"
    StatusError   = "error"
    StatusWarning = "warning"
)
```

---

## Internal Implementation

### `output.Emit`

1. Read the `--output` flag from the command's flag set via `cmd.Flags().GetString("output")`.
2. If the value is `"text"` or the flag is not set, return `nil` (no-op).
3. If the value is `"json"`, marshal the `Response` using `encoding/json.Marshal` with indentation.
4. Write the marshalled JSON to `cmd.OutOrStdout()` followed by a newline.
5. Return any marshalling or write errors wrapped with `cockroachdb/errors`.

### `output.IsJSONOutput`

1. Read the `--output` flag value.
2. Return `true` if the value is `"json"`, `false` otherwise.

### `version` command integration

When `IsJSONOutput` returns true:

```go
output.Emit(cmd, output.Response{
    Status:  output.StatusSuccess,
    Command: "version",
    Data: map[string]any{
        "current_version": currentVersion,
        "latest_version":  latestVersion,
        "update_available": updateAvailable,
        "build_date":      buildDate,
        "go_version":      goVersion,
    },
})
```

Skip the normal text rendering when `Emit` succeeds.

### `update` command integration

When `IsJSONOutput` returns true:

```go
output.Emit(cmd, output.Response{
    Status:  output.StatusSuccess,
    Command: "update",
    Data: map[string]any{
        "previous_version": previousVersion,
        "new_version":      newVersion,
        "updated":          true,
    },
})
```

On error:

```go
output.EmitError(cmd, "update", err)
```

### `init` command integration

When `IsJSONOutput` returns true:

```go
output.Emit(cmd, output.Response{
    Status:  output.StatusSuccess,
    Command: "init",
    Data: map[string]any{
        "config_path":      configPath,
        "steps_completed":  stepsCompleted,
        "already_initialised": alreadyInitialised,
    },
})
```

### `config` command integration (Spec 7)

The `config` subcommands are designed with JSON output from the start:

- `config get` — `Data` contains `{"key": "...", "value": "..."}`
- `config set` — `Data` contains `{"key": "...", "value": "...", "previous_value": "..."}`
- `config list` — `Data` contains `{"entries": [{"key": "...", "value": "...", "masked": true}, ...]}`
- `config validate` — `Data` contains `{"valid": true/false, "diagnostics": [...]}`
- `config edit` — `Data` contains `{"changes": [{"key": "...", "old": "...", "new": "..."}, ...]}`

### `doctor` command migration

Replace the existing JSON output logic in `doctor` with a call to `output.Emit`, wrapping the current diagnostic data in the standard `Response` envelope.

---

## Project Structure

```
pkg/output/
    output.go          # Existing file — add Response, Emit, IsJSONOutput, EmitError
    output_test.go     # Existing or new — tests for all new functions
    response.go        # Alternative: separate file for Response type if output.go is large
    response_test.go
```

Changes to existing command files:

```
pkg/cmd/version/version.go    # Add JSON output branch
pkg/cmd/update/update.go      # Add JSON output branch
pkg/cmd/init/init.go          # Add JSON output branch
pkg/cmd/doctor/doctor.go      # Migrate to Response envelope
pkg/cmd/config/*.go           # (Spec 7) Use Emit from the start
```

---

## Testing Strategy

- **`output.Emit` unit tests:** verify JSON output for success, error, and warning responses; verify no-op behaviour for text mode; verify indented JSON formatting; verify error wrapping on marshal failure.
- **`output.IsJSONOutput` unit tests:** verify true for `"json"`, false for `"text"`, false when flag is absent.
- **`output.EmitError` unit tests:** verify correct error envelope construction.
- **Command integration tests:** for each command (`version`, `update`, `init`, `doctor`), run with `--output json` and assert:
    - Output is valid JSON.
    - Output deserialises into a `Response` with correct `Status` and `Command` fields.
    - `Data` payload contains expected keys.
- **Regression tests:** run commands with `--output text` (or no flag) and assert output is unchanged from current behaviour.
- **Mocks:** mockery/v3 for any interfaces; `bytes.Buffer` as `cmd.OutOrStdout()` for capturing output in tests.
- **Coverage target:** 90%+ for `pkg/output/` additions; integration test coverage for each command's JSON branch.

---

## Backwards Compatibility

- **Text output:** completely unchanged. The `--output` flag defaults to `text` and all existing behaviour is preserved.
- **`doctor` JSON output:** the JSON schema changes from a flat structure to the `Response` envelope. This is a minor breaking change. Since the `--output json` feature is recent and not yet widely adopted, this is acceptable. Document the change in release notes.
- **No new flags:** the existing `--output` flag is reused; no new flags are introduced.

---

## Future Considerations

- **YAML output format:** add `--output yaml` support by extending `Emit` with a YAML marshaller. The `Response` struct and `Emit` architecture make this trivial.
- **Streaming output:** for long-running commands (e.g. `update`), support newline-delimited JSON (NDJSON) for streaming progress events.
- **Output filtering:** add `--query` flag with jq-like syntax for filtering JSON output on the command line.
- **Custom formatters:** allow tools built on GTB to register custom output formatters for their domain-specific commands.
- **Schema generation:** auto-generate JSON Schema from the `Response` type and command-specific `Data` types for documentation and client code generation.

---

## Implementation Phases

### Phase 1: Core output utilities

- Add `Response`, `Emit`, `IsJSONOutput`, `EmitError` to `pkg/output/`.
- Write comprehensive unit tests for all new functions.
- Verify existing tests still pass.

### Phase 2: Version and doctor commands

- Integrate `Emit` into the `version` command.
- Migrate `doctor` command to use the `Response` envelope.
- Write integration tests for both commands with `--output json`.

### Phase 3: Update and init commands

- Integrate `Emit` into the `update` command.
- Integrate `Emit` into the `init` command.
- Write integration tests.

### Phase 4: Config command (Spec 7 dependency)

- Ensure all `config` subcommands use `Emit` for JSON output.
- Write integration tests for `config` commands with `--output json`.

---

## Verification

- [ ] `output.Emit` writes valid indented JSON to stdout when format is `"json"`.
- [ ] `output.Emit` returns nil and writes nothing when format is `"text"`.
- [ ] `output.IsJSONOutput` correctly detects the flag value.
- [ ] `output.EmitError` produces a well-formed error envelope.
- [ ] `gtb version --output json` emits a valid `Response` with version data.
- [ ] `gtb doctor --output json` emits a valid `Response` with diagnostic data (envelope migration).
- [ ] `gtb update --output json` emits a valid `Response` with update result.
- [ ] `gtb init --output json` emits a valid `Response` with initialisation data.
- [ ] All commands with `--output text` produce identical output to current behaviour.
- [ ] All tests pass: `just test`.
- [ ] Coverage is 90%+ for `pkg/output/` additions.
- [ ] JSON output from every command can be piped to `jq` without errors.
