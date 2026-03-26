---
title: "Config Schema Validation Specification"
description: "Add a validation layer to pkg/config that checks configuration values at load time against a schema, catching typos, missing required fields, and type mismatches before they cause runtime errors."
date: 2026-03-26
status: DRAFT
tags:
  - specification
  - config
  - validation
  - feature
author:
  - name: Matt Cockayne
    email: matt@phpboyscout.com
  - name: Claude (claude-opus-4-6)
    role: AI drafting assistant
---

# Config Schema Validation Specification

Authors
:   Matt Cockayne, Claude (claude-opus-4-6) *(AI drafting assistant)*

Date
:   26 March 2026

Status
:   DRAFT

---

## Overview

`pkg/config` supports hierarchical merging from multiple sources (files, embedded assets, environment variables, CLI flags) but performs no structural validation. A typo in a config key (e.g., `github.tokne` instead of `github.token`) silently produces an empty value, discovered only at runtime when an API call fails. Missing required fields, type mismatches (string where int is expected), and unrecognised keys go undetected.

This specification adds a schema validation layer that runs at config load time and on hot-reload, producing actionable error messages with hints. Two schema definition strategies are supported: Go struct tags (for tools that define their config shape in code) and JSON Schema documents (for tools that ship a schema file alongside their config).

---

## Design Decisions

**Dual schema sources (struct tags + JSON Schema)**: Go struct tags are the simplest path for tools that already define config structs. JSON Schema documents support external tooling (IDE completion, CI validation) and tools that don't define Go structs. Both compile to the same internal representation.

**Validation at the `Containable` boundary**: Validation runs inside `Load`, `LoadFilesContainer`, and `LoadEmbed` after merging completes, rather than inside individual `Get*` calls. This catches all issues upfront rather than lazily. The `Container.Validate()` method is also exposed for on-demand re-validation.

**Integration with `Observable` for hot-reload**: When `watchConfig` detects a change, validation runs before notifying observers. If validation fails, observers receive the error via the existing `chan error` and the previous valid config remains in effect.

**Functional options pattern**: Schema configuration uses the same `Option` pattern as `pkg/http/client.go`, keeping the API extensible without breaking changes.

**Warnings vs errors**: Unknown keys produce warnings (logged, not fatal) to support forward-compatible config files. Missing required fields and type mismatches produce errors that prevent startup.

---

## Public API Changes

### New Types in `pkg/config`

```go
// Schema defines the expected structure and constraints for configuration values.
type Schema struct {
    fields   map[string]FieldSchema
    required []string
}

// FieldSchema describes a single configuration field.
type FieldSchema struct {
    // Type is the expected Go type: "string", "int", "float64", "bool", "duration", "time".
    Type        string
    // Required indicates the field must be present and non-zero.
    Required    bool
    // Description is used in validation error messages and JSON Schema export.
    Description string
    // Default is the default value if the field is absent and not required.
    Default     any
    // Enum restricts the field to a set of allowed values.
    Enum        []any
    // Children defines nested fields for map/object types.
    Children    map[string]FieldSchema
}

// ValidationError contains details about a single validation failure.
type ValidationError struct {
    Key     string // dot-separated config key
    Message string // human-readable description
    Hint    string // actionable fix suggestion
}

// ValidationResult holds the outcome of schema validation.
type ValidationResult struct {
    Errors   []ValidationError
    Warnings []ValidationError
}

// Valid returns true if no errors were found. Warnings do not affect validity.
func (r *ValidationResult) Valid() bool

// Error returns a formatted multi-line error string, or empty string if valid.
func (r *ValidationResult) Error() string
```

### Schema Construction

```go
// SchemaOption configures schema validation behaviour.
type SchemaOption func(*schemaConfig)

// WithStrictMode treats unknown keys as errors instead of warnings.
func WithStrictMode() SchemaOption

// WithJSONSchema loads a schema from a JSON Schema document (Draft 2020-12).
func WithJSONSchema(reader io.Reader) SchemaOption

// WithStructSchema derives a schema from a tagged Go struct.
// Supported tags: `config:"key" validate:"required" enum:"a,b,c" default:"value"`.
func WithStructSchema(v any) SchemaOption

// NewSchema creates a Schema from the provided options.
func NewSchema(opts ...SchemaOption) (*Schema, error)
```

### Validation on Containable

```go
// Extended Containable interface (new method):
type Containable interface {
    // ... existing methods ...

    // Validate checks the current configuration against the provided schema.
    // Returns a ValidationResult; callers should check result.Valid().
    Validate(schema *Schema) *ValidationResult
}
```

### Integration with Load Functions

```go
// LoadFilesContainerWithSchema loads config files and validates against the schema.
// Returns an error wrapping all validation errors if the config is invalid.
func LoadFilesContainerWithSchema(l logger.Logger, fs afero.Fs, schema *Schema, configFiles ...string) (Containable, error)
```

### Usage Example

```go
type AppConfig struct {
    Github struct {
        Token string `config:"github.token" validate:"required"`
    }
    Log struct {
        Level  string `config:"log.level" enum:"debug,info,warn,error" default:"info"`
        Format string `config:"log.format" enum:"json,logfmt,text" default:"text"`
    }
}

schema, err := config.NewSchema(config.WithStructSchema(AppConfig{}))
if err != nil {
    return err
}

cfg, err := config.LoadFilesContainerWithSchema(logger, fs, schema, "config.yaml")
if err != nil {
    // err contains actionable messages:
    // "config validation failed:\n  github.token: required field is missing (hint: set GITHUB_TOKEN or add github.token to your config file)\n"
    return err
}
```

---

## Internal Implementation

### Schema Compilation

Both `WithStructSchema` and `WithJSONSchema` compile to the internal `Schema` struct. Struct tag parsing uses `reflect` to walk the struct and extract `config`, `validate`, `enum`, and `default` tags. JSON Schema parsing uses a lightweight subset parser supporting `type`, `required`, `enum`, `default`, `description`, and `properties`.

### Validation Engine

```go
func (c *Container) Validate(schema *Schema) *ValidationResult {
    result := &ValidationResult{}

    for key, field := range schema.fields {
        value := c.viper.Get(key)
        validateField(key, field, value, result)
    }

    if schema.strict {
        detectUnknownKeys(c.viper.AllKeys(), schema.fields, result)
    }

    return result
}

func validateField(key string, field FieldSchema, value any, result *ValidationResult) {
    // Check required
    // Check type matches expected
    // Check enum membership
    // Recurse into Children for nested fields
}
```

### Hot-Reload Integration

The existing `watchConfig` method in `Container` is extended: if a `Schema` has been set on the container, validation runs on the updated config before observers are notified. If validation fails, the error is sent to the observer error channel and the reload is rejected.

```go
func (c *Container) watchConfig() {
    c.viper.OnConfigChange(func(e fsnotify.Event) {
        if c.schema != nil {
            result := c.Validate(c.schema)
            if !result.Valid() {
                c.logger.Error("config reload rejected: validation failed", "errors", result.Error())
                return // do not notify observers
            }
        }
        // ... existing observer notification ...
    })
    c.viper.WatchConfig()
}
```

### Error Formatting with Hints

Validation errors integrate with `pkg/errorhandling` via `errors.WithHint`:

```go
errors.WithHint(
    errors.Newf("config validation: %s is required but missing", key),
    fmt.Sprintf("Add %s to your config file or set the %s environment variable", key, envKey),
)
```

---

## Project Structure

```
pkg/config/
├── config.go          ← UNCHANGED
├── container.go       ← MODIFIED: add schema field, Validate method, watchConfig gate
├── load.go            ← MODIFIED: add LoadFilesContainerWithSchema
├── observer.go        ← UNCHANGED
├── schema.go          ← NEW: Schema, FieldSchema, SchemaOption, NewSchema
├── schema_struct.go   ← NEW: WithStructSchema implementation (reflect-based)
├── schema_json.go     ← NEW: WithJSONSchema implementation (JSON Schema parser)
├── validate.go        ← NEW: validation engine, ValidationResult, ValidationError
├── validate_test.go   ← NEW: validation tests
├── schema_test.go     ← NEW: schema construction tests
```

---

## Testing Strategy

| Test | Scenario |
|------|----------|
| `TestValidate_RequiredFieldPresent` | Required field exists and is non-zero &#8594; no error |
| `TestValidate_RequiredFieldMissing` | Required field absent &#8594; error with hint |
| `TestValidate_RequiredFieldEmpty` | Required string field is empty string &#8594; error |
| `TestValidate_TypeMismatch` | Config has string where int expected &#8594; error |
| `TestValidate_EnumValid` | Value is in allowed set &#8594; no error |
| `TestValidate_EnumInvalid` | Value not in allowed set &#8594; error listing allowed values |
| `TestValidate_UnknownKey_Warning` | Key not in schema (non-strict) &#8594; warning only |
| `TestValidate_UnknownKey_Strict` | Key not in schema (strict mode) &#8594; error |
| `TestValidate_NestedFields` | Nested config objects validated recursively |
| `TestValidate_DefaultApplied` | Missing optional field with default &#8594; default is set |
| `TestWithStructSchema_Tags` | Struct tags correctly parsed into Schema |
| `TestWithJSONSchema_Document` | JSON Schema document correctly parsed into Schema |
| `TestHotReload_ValidConfig` | Config change passes validation &#8594; observers notified |
| `TestHotReload_InvalidConfig` | Config change fails validation &#8594; observers not notified, error logged |
| `TestLoadFilesContainerWithSchema` | End-to-end load + validate |
| `TestValidationResult_Error` | Multi-error formatting matches expected output |

### Coverage

- Target: 90%+ for all new files in `pkg/config/`.

---

## Linting

- `golangci-lint run --fix` must pass.
- No new `nolint` directives.

---

## Documentation

- Godoc for all new public types and functions.
- Update `docs/components/config.md` with schema validation usage, struct tag reference, and JSON Schema example.
- Add a "Common Misconfigurations" section showing how validation catches real-world issues.

---

## Backwards Compatibility

- **No breaking changes**. The `Containable` interface gains one new method (`Validate`), which is additive. Existing code that does not use schemas is unaffected.
- `LoadFilesContainer` and `Load` remain unchanged. Schema validation is opt-in via `LoadFilesContainerWithSchema` or explicit `Validate()` calls.
- Hot-reload validation only activates when a schema is attached to the container.

---

## Future Considerations

- **Schema generation CLI**: A `gtb config schema` command that generates a JSON Schema from struct tags for distribution alongside the tool.
- **IDE integration**: Published JSON Schema files can power autocompletion in VS Code, JetBrains, and other editors.
- **Deprecation warnings**: Mark config keys as deprecated with a migration hint, easing version upgrades.
- **Cross-field validation**: Rules like "if provider is gitlab, then gitlab.token is required" using conditional schema logic.

---

## Implementation Phases

### Phase 1 — Schema Definition
1. Define `Schema`, `FieldSchema`, `SchemaOption` types
2. Implement `NewSchema` with functional options
3. Implement `WithStructSchema` (struct tag parsing)
4. Add unit tests for schema construction

### Phase 2 — Validation Engine
1. Implement `ValidationResult`, `ValidationError`
2. Implement `Container.Validate()` with required, type, and enum checks
3. Implement unknown-key detection (strict and non-strict modes)
4. Add unit tests for all validation paths

### Phase 3 — Load Integration
1. Add `LoadFilesContainerWithSchema`
2. Integrate validation into hot-reload (`watchConfig`)
3. Add integration tests for load + validate flow

### Phase 4 — JSON Schema Support
1. Implement `WithJSONSchema` parser
2. Add tests for JSON Schema document loading
3. Verify round-trip: struct tags &#8594; JSON Schema &#8594; validate

---

## Verification

```bash
go build ./...
go test -race ./pkg/config/...
go test ./...
golangci-lint run --fix

# Verify new types exist
grep -n 'type Schema struct' pkg/config/schema.go
grep -n 'func.*Validate' pkg/config/container.go
grep -n 'LoadFilesContainerWithSchema' pkg/config/load.go
```
