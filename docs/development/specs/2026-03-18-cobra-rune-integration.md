---
title: "Deep Cobra Integration for ErrorHandler Specification"
description: "Replace explicit ErrorHandler.Fatal() calls in command Run functions with idiomatic RunE error returns, routing all errors through a centralized Execute wrapper that feeds the StandardErrorHandler pipeline."
date: 2026-03-18
status: IMPLEMENTED
tags:
  - specification
  - cobra
  - error-handling
  - errorhandling
  - runE
author:
  - name: Matt Cockayne
    email: matt@phpboyscout.com
  - name: Claude (claude-sonnet-4-6)
    role: AI drafting assistant
---

# Deep Cobra Integration for ErrorHandler Specification

Authors
:   Matt Cockayne, Claude (claude-sonnet-4-6) *(AI drafting assistant)*

Date
:   18 March 2026

Status
:   IMPLEMENTED

---

## Overview

GTB currently requires every command to explicitly call `props.ErrorHandler.Fatal()` in its `Run` function body — a boilerplate pattern that adds noise to every command and creates inconsistency where some commands bypass `ErrorHandler` entirely (e.g., using `Logger.Fatalf` directly).

Two additional problems exist:

1. `PersistentPreRunE` in the root command calls `props.Logger.Fatalf()` directly on config-load failures, completely bypassing the `ErrorHandler` pipeline (no hints, no Slack help, no stack traces).
2. Cobra's default error output runs in parallel with GTB's — when `Execute()` returns an error, Cobra may print it in its own format before GTB's handler gets a chance.

The goal is to make commands return errors idiomatically (via `RunE`) while routing all errors — including flag errors, `PersistentPreRunE` errors, and runtime errors — through `StandardErrorHandler`, which provides stack traces, hints, details, special sentinel handling, and Slack help.

### Terminology

**`RunE`**
:   The Cobra command field that accepts a function returning `error`. Cobra passes the returned error to `Execute()` for the caller to handle.

**Execute wrapper**
:   A new `Execute(*cobra.Command, *props.Props)` function in `pkg/cmd/root` that silences Cobra's own error output and routes any error from `rootCmd.Execute()` through `ErrorHandler.Check`.

**`SilenceErrors` / `SilenceUsage`**
:   Cobra fields that suppress default error/usage printing, giving GTB's `ErrorHandler` full control of terminal output.

---

## Design Decisions

**`RunE` + Execute wrapper, not middleware**: Commands use `RunE` and return errors naturally. An `Execute()` wrapper function in `pkg/cmd/root` silences Cobra's output, runs the command, and routes any returned error through `ErrorHandler.Check`. This is simpler than middleware/hook-chaining approaches.

**Non-fatal errors**: Commands that need to log a warning and continue call `props.ErrorHandler.Warn()` inside their body and return `nil`. No new error wrapping types are introduced.

**`ErrRunSubCommand` usage output**: `PreRunE` continues to call `props.ErrorHandler.SetUsage(cmd.Usage)` at its start. When `RunE` returns `ErrRunSubCommand`, `handleSpecialErrors` falls back to `h.Usage` (already set). This requires no change to the `ErrorHandler` interface.

**Flag errors**: `SetFlagErrorFunc` adds a "Run `--help` for usage" hint (using `errors.WithHintf`) and returns the error to propagate to `Execute()`. The wrapper then routes it through `ErrorHandler`.

**`handleOutdatedVersion`**: Currently calls `props.ErrorHandler.Fatal()` directly inside the `PersistentPreRunE` chain. Refactored to set `result.Error` and return; the caller in `newRootPreRunE` already checks `updateResult.Error != nil` and returns it, propagating to the Execute wrapper.

**No `ErrorHandler` interface changes**: The interface is unchanged. The `Check(err, prefix, level, cmd...)` signature is preserved.

---

## Public API

### New: `pkg/cmd/root.Execute`

```go
// Execute runs the root command with centralized error handling.
// It silences Cobra's default error output and routes any error returned by
// the command tree through ErrorHandler.Check at Fatal level.
func Execute(rootCmd *cobra.Command, props *p.Props)
```

This replaces the pattern:
```go
if err := rootCmd.Execute(); err != nil {
    os.Exit(1)
}
```

With:
```go
pkgRoot.Execute(rootCmd, p)
```

### Modified: `internal/cmd/root.NewCmdRoot`

```go
// Before:
func NewCmdRoot(v ver.Info) *cobra.Command

// After:
func NewCmdRoot(v ver.Info) (*cobra.Command, *props.Props)
```

Returns `props` so `main.go` can pass it to `Execute`.

---

## Internal Implementation

### `pkg/cmd/root/execute.go` (NEW)

```go
package root

import (
    "github.com/cockroachdb/errors"
    "github.com/phpboyscout/go-tool-base/pkg/errorhandling"
    p "github.com/phpboyscout/go-tool-base/pkg/props"
    "github.com/spf13/cobra"
)

func Execute(rootCmd *cobra.Command, props *p.Props) {
    rootCmd.SilenceErrors = true
    rootCmd.SilenceUsage = true

    rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
        return errors.WithHintf(err, "Run '%s --help' for usage.", cmd.CommandPath())
    })

    if err := rootCmd.Execute(); err != nil {
        props.ErrorHandler.Check(err, "", errorhandling.LevelFatal)
    }
}
```

### `pkg/cmd/root/root.go` — Fix `newRootPreRunE`

Two `props.Logger.Fatalf(...)` calls become `return errors.Wrap(...)`:

```go
// Before:
flags, err := extractFlags(cmd)
if err != nil {
    props.Logger.Fatalf("%s", err)
}

// After:
flags, err := extractFlags(cmd)
if err != nil {
    return errors.Wrap(err, "failed to read command flags")
}
```

Same pattern for the `loadAndMergeConfig` call:

```go
// Before:
cfg, err := loadAndMergeConfig(...)
if err != nil {
    props.Logger.Fatalf("%s", err)
}

// After:
cfg, err := loadAndMergeConfig(...)
if err != nil {
    return errors.Wrap(err, "failed to load configuration")
}
```

### `pkg/cmd/root/root.go` — Fix `handleOutdatedVersion`

```go
// Before:
if err := update.Update(ctx, props, "", false); err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        props.ErrorHandler.Fatal(errors.New("Update timed out: ..."))
    }
    props.ErrorHandler.Fatal(err)
}

// After:
if err := update.Update(ctx, props, "", false); err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        result.Error = errors.WithHint(
            errors.New("update timed out"),
            "Check your internet connection or try again later.")
        return
    }
    result.Error = err
    return
}
```

`newRootPreRunE` already checks `updateResult.Error != nil` — no change needed there.

---

## Project Structure

```
pkg/cmd/root/
├── execute.go          ← NEW: Execute() wrapper
├── root.go             ← MODIFIED: fix newRootPreRunE and handleOutdatedVersion
├── root_test.go        ← MODIFIED: new Execute tests, updated PreRunE tests
pkg/cmd/version/
├── version.go          ← MODIFIED: Run → RunE
pkg/cmd/update/
├── update.go           ← MODIFIED: Run → RunE
pkg/cmd/initialise/
├── init.go             ← MODIFIED: Run → RunE
pkg/cmd/docs/
├── docs.go             ← MODIFIED: Run → RunE, remove os.Exit(1)
internal/cmd/root/
├── root.go             ← MODIFIED: NewCmdRoot returns (*cobra.Command, *props.Props)
cmd/gtb/
├── main.go             ← MODIFIED: use pkgRoot.Execute(rootCmd, p)
internal/generator/templates/
├── command.go          ← MODIFIED: Run→RunE, PreRun→PreRunE, PersistentPreRun→PersistentPreRunE
```

---

## Error Handling

- **Flag parse errors**: Wrapped with `errors.WithHintf` hint before being returned. `Execute` wrapper routes through `ErrorHandler.Check` at `LevelFatal`.
- **Config load errors**: `newRootPreRunE` returns `errors.Wrap(err, "failed to load configuration")`. Stack trace is preserved.
- **Update timeout**: `result.Error` set with `errors.WithHint(errors.New("update timed out"), "Check your internet connection...")`.
- **Command `RunE` errors**: Returned from command, propagated to `rootCmd.Execute()`, caught by `Execute` wrapper, routed to `ErrorHandler.Check` at `LevelFatal`.
- **`ErrRunSubCommand`**: `handleSpecialErrors` in `ErrorHandler` detects via `errors.Is` and calls `h.Usage()`. The usage func is registered via `PreRunE`'s call to `props.ErrorHandler.SetUsage(cmd.Usage)` — no change required.
- **`ErrNotImplemented`**: Detected in `handleSpecialErrors`, logs a warning. No process exit.

---

## Phase 1 — Infrastructure

### 1.1 — Add `Execute` to `pkg/cmd/root/execute.go` (NEW FILE)

Create `pkg/cmd/root/execute.go` with the `Execute` wrapper (see Internal Implementation above).

### 1.2 — Fix `newRootPreRunE` in `pkg/cmd/root/root.go`

Replace both `props.Logger.Fatalf("%s", err)` calls with `return errors.Wrap(err, "...")`.

### 1.3 — Fix `handleOutdatedVersion` in `pkg/cmd/root/root.go`

Replace `props.ErrorHandler.Fatal(...)` calls with `result.Error = ...; return`.

### 1.4 — Update `internal/cmd/root/root.go`

Refactor `NewCmdRoot` to return `(*cobra.Command, *props.Props)`:

```go
func NewCmdRoot(v ver.Info) (*cobra.Command, *props.Props) {
    // ... construct p (unchanged) ...
    rootCmd := root.NewCmdRoot(p)
    // ... add subcommands (unchanged) ...
    return rootCmd, p
}
```

### 1.5 — Update `cmd/gtb/main.go`

```go
func main() {
    rootCmd, p := root.NewCmdRoot(version.Get())
    pkgRoot.Execute(rootCmd, p)
}
```

---

## Phase 2 — Migrate GTB's Own Commands to `RunE`

Migrate the four GTB-owned command files from `Run` + explicit `Fatal` to `RunE` + error return.

| File | Change |
|------|--------|
| `pkg/cmd/version/version.go` | `Run` → `RunE`, return `errors.Wrap(err, "prefix")` instead of `ErrorHandler.Fatal(err, "prefix")` |
| `pkg/cmd/update/update.go` | `Run` → `RunE`, return errors instead of `ErrorHandler.Fatal` / `Logger.Fatalf` |
| `pkg/cmd/initialise/init.go` | `Run` → `RunE`, return `errors.Wrap(err, "...")` instead of `Logger.Fatalf` |
| `pkg/cmd/docs/docs.go` | `Run` → `RunE`, return `errors.WithHint(err, "...")` instead of `ErrorHandler.Fatal` / `os.Exit(1)` |

Prefix context that was in `ErrorHandler.Fatal(err, "prefix")` becomes `errors.Wrap(err, "prefix")`.

---

## Phase 3 — Update Generator Templates

**File**: `internal/generator/templates/command.go`

### `generateCommandFields` — Run → RunE

```go
// Before:
jen.Id("Run").Op(":").Func()...Block(
    jen.Id("props").Dot("ErrorHandler").Dot("Fatal").Call(
        jen.Id("Run"+data.PascalName).Call(...),
        jen.Lit("failed to run "+data.Name),
    ),
)

// After:
jen.Id("RunE").Op(":").Func()...Block(
    jen.Return(
        jen.Id("Run"+data.PascalName).Call(...),
    ),
)
```

### `generateCommandFields` — PersistentPreRun → PersistentPreRunE

Cobra does not auto-chain `PersistentPreRunE`. Generated child commands must explicitly call the parent's hook:

```go
// After:
jen.Id("PersistentPreRunE").Op(":").Func()...Block(
    jen.If(jen.Id("cmd").Dot("Parent").Call().Op("!=").Nil().
        Op("&&").Id("cmd").Dot("Parent").Call().Dot("PersistentPreRunE").Op("!=").Nil()).Block(
        jen.If(jen.Id("err").Op(":=").Id("cmd").Dot("Parent").Call().
            Dot("PersistentPreRunE").Call(jen.Id("cmd"), jen.Id("args")).Op(";").
            Id("err").Op("!=").Nil()).Block(
            jen.Return(jen.Id("err")),
        ),
    ),
    jen.Return(
        jen.Id("PersistentPreRun"+data.PascalName).Call(...),
    ),
)
```

### `generatePreRunField` — PreRun → PreRunE

```go
// After:
jen.Id("PreRunE").Op(":").Func()...Block(
    // SetUsage call preserved (unchanged):
    jen.Id("props").Dot("ErrorHandler").Dot("SetUsage").Call(jen.Id("cmd").Dot("Usage")),
    // Fatal replaced with return:
    jen.Return(jen.Id("PreRun"+data.PascalName).Call(...)),
    // When data.PreRun is false:
    jen.Return(jen.Nil()),
)
```

### `CommandInitializer` — Run → RunE

```go
// After:
jen.Id("RunE").Op(":").Func()...Block(
    jen.Return(jen.Id("Init"+data.PascalName).Call(...))
)
```

Generated `cmd.go` files no longer need to import `errorhandling` — remove from generated imports.

---

## Backwards Compatibility

- Existing consumer commands using `Run` + `ErrorHandler.Fatal()` continue to work. `Fatal` calls `os.Exit(1)` directly; `Execute()` never returns a non-nil error for those commands (process exits first).
- The `ErrorHandler` interface is **unchanged**.
- Mocks (`mocks/pkg/errorhandling/ErrorHandler.go`) require **no changes**.
- Consumer tools calling `rootCmd.Execute()` directly in their `main.go` still work; they just don't get the centralized routing until they adopt `pkgRoot.Execute(rootCmd, props)`.

---

## Testing Strategy

### `pkg/cmd/root` — New Tests

| Test | Scenario |
|------|----------|
| `TestExecute_NilError` | Command returns nil → `ErrorHandler.Check` not called |
| `TestExecute_RuntimeError` | `RunE` returns error → mock `ErrorHandler.Check` called with `LevelFatal` |
| `TestExecute_FlagError` | Invalid flag → routed through handler with hint containing `--help` |
| `TestExecute_SilenceErrors` | Verify Cobra does not write to stderr |
| `TestNewRootPreRunE_FlagExtractError` | Returns error (was: called Fatalf, untestable) |
| `TestNewRootPreRunE_ConfigLoadError` | Returns error (was: called Fatalf, untestable) |
| `TestHandleOutdatedVersion_UpdateTimeout` | `result.Error` set with hint (was: called Fatal, untestable) |

### Generator (`internal/generator`)

- Update `commands_lifecycle_unit_test.go` expectations: field names `RunE`/`PreRunE`/`PersistentPreRunE`
- Assert generated `cmd.go` does not contain `errorhandling` import
- Assert `PersistentPreRunE` body contains parent-chaining block when `PersistentPreRun: true`

### `pkg/cmd/version`, `update`, `initialise`, `docs`

- Existing tests exercise command logic; `RunE` migration needs no new test cases
- Any test asserting `os.Exit` behavior should be updated to assert the returned error

---

## Migration & Compatibility

Consumer tools built on GTB that call `rootCmd.Execute()` directly in `main.go` continue to work without changes — the migration to `pkgRoot.Execute` is opt-in. Documentation will be updated to recommend the new pattern for new tools.

---

## Future Considerations

- **Middleware / hooks**: If cross-cutting concerns (e.g., telemetry, audit logging) are needed in the future, the `Execute` wrapper is the natural integration point — add pre/post hooks there rather than in individual commands.
- **Structured exit codes**: Currently `LevelFatal` always exits with code 1. Future work could map error types to specific exit codes via `ErrorHandler`.

---

## Implementation Phases Summary

| Phase | Scope | Files Changed |
|-------|-------|---------------|
| 1 | Infrastructure | `pkg/cmd/root/execute.go` (new), `pkg/cmd/root/root.go`, `internal/cmd/root/root.go`, `cmd/gtb/main.go` |
| 2 | Migrate GTB commands | `pkg/cmd/version/version.go`, `pkg/cmd/update/update.go`, `pkg/cmd/initialise/init.go`, `pkg/cmd/docs/docs.go` |
| 3 | Generator templates | `internal/generator/templates/command.go` |

## Verification

```bash
just build                        # confirm binary compiles
just test                         # full suite
go test -race ./pkg/cmd/...       # migrated commands
go test -race ./internal/...      # generator tests
golangci-lint run --fix
go run ./ generate test-cmd -p /tmp  # verify generator output uses RunE
```

Check generated `/tmp/` command file contains `RunE:` not `Run:`, and contains no `ErrorHandler.Fatal`.
