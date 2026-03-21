---
title: "Trivial Code Improvements Specification"
description: "Consolidate mechanical, low-risk code quality improvements including error wrapping migration, godoc additions, config interface documentation, and chat provider default tuning."
date: 2026-03-21
status: DRAFT
tags:
  - specification
  - code-quality
  - cleanup
  - documentation
author:
  - name: Matt Cockayne
    email: matt@phpboyscout.com
  - name: Claude (claude-opus-4-6)
    role: AI drafting assistant
---

# Trivial Code Improvements Specification

Authors
:   Matt Cockayne, Claude (claude-opus-4-6) *(AI drafting assistant)*

Date
:   21 March 2026

Status
:   DRAFT

---

## Overview

This specification consolidates nine mechanical, low-risk improvements identified during a code review. Each task is independently deployable and introduces no breaking changes. Grouping them into a single spec reduces overhead while ensuring nothing is forgotten.

### Tasks at a Glance

| # | Summary | Files |
|---|---------|-------|
| T1 | Complete `fmt.Errorf` → `cockroachdb/errors` migration | 4+ non-test files |
| T2 | Document `GetViper()` as intentional power-user escape hatch | `pkg/config/container.go` |
| T3 | Refactor `Dump()` to accept `io.Writer` | `pkg/config/container.go` |
| T4 | Fix misleading "YAML" comment (actually JSON) | `pkg/config/container.go` |
| T5 | Extract `maxSteps = 20` into `chat.Config` | `pkg/chat/client.go`, providers |
| T6 | Per-provider `MaxTokens` defaults | `pkg/chat/client.go`, providers |
| T7 | Remove deprecated `Enable`/`Disable` fields | `pkg/props/tool.go` |
| T8 | Add `doc.go` files with package-level godoc | All packages |
| T9 | Minimal config key validation | `pkg/cmd/root/root.go` |

---

## Design Decisions

**Incremental migration (T1)**: Continue the pattern established by the already-IMPLEMENTED `2026-02-18-cockroachdb-errors-migration` spec. Replace `fmt.Errorf("...: %w", err)` with `errors.Wrap(err, "...")` or `errors.Newf("...: %w", err)`. Test files are excluded — `fmt.Errorf` is acceptable in tests.

**GetViper intentional (T2)**: The `GetViper()` method on `Containable` is a deliberate power-user escape hatch for accessing Viper's full API. Rather than removing it, we document this intent with clear godoc explaining the trade-off.

**Dump accepts writer (T3)**: Library code should not write to stdout. Change `Dump()` to accept an `io.Writer` parameter. This is a minor breaking change to the `Containable` interface but is considered acceptable for this low-usage method.

**Config fields in chat.Config (T5, T6)**: Rather than per-provider constants, add `MaxSteps` and `MaxTokens` fields to the shared `chat.Config` struct. Providers check for zero values and apply their own defaults. This gives consumers control without requiring them to know provider internals.

**Remove Enable/Disable (T7)**: The `Enable`/`Disable` fields on the `Tool` struct are superseded by `Features`. Rather than a deprecation cycle, remove them now along with any logic that reads them. The `Features` field and its `IsEnabled()`/`IsDisabled()` methods are the sole mechanism for feature gating.

---

## Public API Changes

### T3: `Containable` Interface — `Dump` Signature

```go
// Before:
Dump()

// After:
Dump(w io.Writer)
```

### T5: `chat.Config` — New Fields

```go
type Config struct {
    // ... existing fields ...

    // MaxSteps limits the number of ReAct loop iterations in Chat().
    // Zero means use the provider default (20).
    MaxSteps int

    // MaxTokens sets the maximum tokens per response.
    // Zero means use the provider default (OpenAI: 4096, Claude: 8192, Gemini: 8192).
    MaxTokens int
}
```

### T6: New Per-Provider Default Constants

```go
const (
    DefaultMaxTokensOpenAI  = 4096
    DefaultMaxTokensClaude  = 8192
    DefaultMaxTokensGemini  = 8192
    DefaultMaxSteps         = 20
)
```

---

## Internal Implementation

### T1: `fmt.Errorf` Migration

**Files to migrate** (non-test, non-docs):

| File | Occurrences | Pattern |
|------|-------------|---------|
| `internal/agent/tools.go` | 12 | `fmt.Errorf("...: %w", err)` → `errors.Wrap(err, "...")` |
| `internal/generator/docs.go` | 3 | Same pattern |
| `pkg/cmd/root/root.go` | 3 | Same pattern |
| `pkg/errorhandling/helpers.go` | 1 | Same pattern |

Import `"github.com/cockroachdb/errors"` where not already present. Remove `"fmt"` import if no longer used.

### T2: `GetViper()` Documentation

```go
// GetViper returns the underlying Viper instance for advanced operations
// not exposed by the Containable interface. This is an intentional escape
// hatch for power users who need Viper's full API (e.g., MergeConfig,
// BindPFlag, or direct access to config file watching).
//
// Prefer the typed accessor methods (GetString, GetInt, etc.) for standard
// configuration reads. Use GetViper only when the Containable interface
// does not cover your use case.
GetViper() *viper.Viper
```

### T3: `Dump()` Refactor

```go
// Before:
func (c *Container) Dump() {
    fmt.Println(c.ToJSON())
}

// After:
func (c *Container) Dump(w io.Writer) {
    _, _ = fmt.Fprintln(w, c.ToJSON())
}
```

Update all callers (search for `.Dump()` calls) to pass `os.Stdout` or the appropriate writer.

### T4: Comment Fix

```go
// Before (line 172):
c.logger.Error("unable to marshal config to YAML", ...)

// After:
c.logger.Error("unable to marshal config to JSON", ...)
```

### T5: `maxSteps` Extraction

Each provider's `Chat()` method changes from:

```go
const maxSteps = 20
for step := range maxSteps {
```

To:

```go
maxSteps := c.cfg.MaxSteps
if maxSteps <= 0 {
    maxSteps = DefaultMaxSteps
}
for step := range maxSteps {
```

### T6: Per-Provider `MaxTokens`

Claude provider changes from:

```go
MaxTokens: int64(DefaultMaxTokensPerChunk),
```

To:

```go
maxTokens := c.cfg.MaxTokens
if maxTokens <= 0 {
    maxTokens = DefaultMaxTokensClaude
}
// ...
MaxTokens: int64(maxTokens),
```

Same pattern for OpenAI and Gemini with their respective defaults. Remove the old `DefaultMaxTokensPerChunk` constant.

### T7: Remove `Enable`/`Disable` Fields

Remove the following fields from the `Tool` struct:

```go
// REMOVE:
Disable []FeatureCmd `json:"disable" yaml:"disable"`
Enable  []FeatureCmd `json:"enable" yaml:"enable"`
```

Remove any logic in `IsEnabled()`, `IsDisabled()`, or other methods that reads from these fields. The `Features` map is the sole source of truth for feature gating. Update or remove any tests that exercise the legacy fields.

### T8: `doc.go` Files for Package Godoc

Create a `doc.go` file in every package under `pkg/` and `internal/` that lacks a package-level godoc comment. Following Go convention, `doc.go` contains only the package comment and the `package` declaration — no code. Examples:

```go
// pkg/props/doc.go
// Package props provides the central dependency injection container for GTB applications.
package props
```

```go
// pkg/chat/doc.go
// Package chat provides a unified multi-provider AI client supporting Claude, OpenAI, and Gemini.
package chat
```

```go
// pkg/controls/doc.go
// Package controls provides service lifecycle management with message-based coordination.
package controls
```

### T9: Minimal Config Validation

In `newRootPreRunE` after config is loaded, add optional validation:

```go
func validateConfig(cfg config.Containable, logger *log.Logger) {
    // Warn on commonly misconfigured keys
    if cfg.IsSet("github.token") && cfg.GetString("github.token") == "" {
        logger.Warn("github.token is set but empty — GitHub operations will fail")
    }
}
```

This is a lightweight first step. A full schema validation system is deferred to future work.

---

## Project Structure

```
pkg/config/container.go          ← T2, T3, T4: godoc, Dump(io.Writer), comment fix
pkg/chat/client.go               ← T5, T6: Config fields, constants
pkg/chat/constants.go            ← T6: per-provider defaults
pkg/chat/claude.go               ← T5, T6: use Config.MaxSteps/MaxTokens
pkg/chat/openai.go               ← T5, T6: use Config.MaxSteps/MaxTokens
pkg/chat/gemini.go               ← T5, T6: use Config.MaxSteps/MaxTokens
pkg/props/tool.go                ← T7: remove Enable/Disable fields and related logic
pkg/cmd/root/root.go             ← T1, T9: error migration, config validation
internal/agent/tools.go          ← T1: error migration
internal/generator/docs.go       ← T1: error migration
pkg/errorhandling/helpers.go     ← T1: error migration
(all packages)/doc.go            ← T8: new doc.go files with package godoc
```

---

## Testing Strategy

### T1: Error Migration
- No new tests required — existing tests validate behaviour. Run full suite to confirm no regressions.

### T3: `Dump(io.Writer)`
- Update existing `Dump` test to pass `&bytes.Buffer{}` and assert output content.

### T5–T6: Config Fields
| Test | Scenario |
|------|----------|
| `TestConfig_MaxSteps_Zero` | Zero value → uses DefaultMaxSteps (20) |
| `TestConfig_MaxSteps_Custom` | Custom value → used by provider |
| `TestConfig_MaxTokens_Zero` | Zero value → uses provider default |
| `TestConfig_MaxTokens_Custom` | Custom value → overrides provider default |

### T7: Enable/Disable Removal
- Remove or update any tests that reference `Enable`/`Disable` fields.
- Verify `IsEnabled`/`IsDisabled` work correctly using only `Features`.

### T9: Config Validation
- Test with empty token → warning logged.
- Test with valid token → no warning.

### Coverage
- Target: 90%+ for all modified `pkg/` files.

---

## Linting

- `golangci-lint run --fix` must pass after all changes.
- No new `nolint` directives unless justified with a comment.
- The `fmt.Errorf` migration (T1) will resolve any existing `wrapcheck` lint warnings in migrated files.

---

## Documentation

- **T2**: Godoc on `GetViper()` explaining intentional design (see Internal Implementation).
- **T5–T6**: Godoc on new `Config` fields.
- **T7**: Update any user-facing docs referencing `Enable`/`Disable` to use `Features`.
- **T8**: `doc.go` files provide package-level godoc for all packages in `pkg/` and `internal/`.
- Update `docs/components/chat.md` to document `MaxSteps` and `MaxTokens` config options.
- Update `docs/components/config.md` to document `Dump(io.Writer)` change.

---

## Backwards Compatibility

- **T3** (`Dump` signature change): Minor breaking change. All callers must pass an `io.Writer`. Low impact — `Dump` is rarely called externally.
- **T5–T6** (new Config fields): Non-breaking. Zero values preserve existing behaviour.
- **T7** (`Enable`/`Disable` removal): Breaking change. Any external code using these fields must migrate to `Features`. This is considered acceptable as `Features` has been the recommended mechanism.
- All other tasks: Non-breaking.

---

## Future Considerations

- **Full config schema validation (T9)**: This spec implements a minimal warning-based approach. A full JSON Schema or struct-tag validation system could be added later.
- **Migration guide (T7)**: If external consumers exist, a short migration note in release notes pointing from `Enable`/`Disable` to `Features` may be helpful.
- **Streaming token counting (T6)**: When streaming is implemented (separate spec), MaxTokens may need additional per-chunk semantics.

---

## Implementation Phases

### Phase 1 — Config Package (T2, T3, T4)
1. Fix comment in `container.go:172` (T4)
2. Add godoc to `GetViper()` (T2)
3. Change `Dump()` to `Dump(io.Writer)` and update all callers (T3)

### Phase 2 — Chat Provider Tuning (T5, T6)
1. Add `MaxSteps` and `MaxTokens` to `chat.Config`
2. Add per-provider default constants
3. Update Claude, OpenAI, Gemini to read from Config
4. Remove `DefaultMaxTokensPerChunk`

### Phase 3 — Error Migration (T1)
1. Migrate `internal/agent/tools.go`
2. Migrate `internal/generator/docs.go`
3. Migrate `pkg/cmd/root/root.go`
4. Migrate `pkg/errorhandling/helpers.go`

### Phase 4 — Deprecation & Documentation (T7, T8, T9)
1. Remove `Enable`/`Disable` fields and related logic from `tool.go` (T7)
2. Create `doc.go` files for all packages (T8)
3. Add minimal config validation (T9)

---

## Verification

```bash
# Build
go build ./...

# Full test suite
go test ./...

# Race detector on modified packages
go test -race ./pkg/config/... ./pkg/chat/... ./pkg/props/...

# Lint
golangci-lint run --fix

# Verify no fmt.Errorf remains in non-test Go files (excluding docs/)
grep -rn 'fmt\.Errorf' --include='*.go' --exclude='*_test.go' pkg/ internal/ | grep -v 'docs/'

# Verify all packages have godoc comments
go doc ./pkg/... 2>&1 | head -100
```
