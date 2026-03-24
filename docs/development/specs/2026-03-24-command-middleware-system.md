---
title: "Command Middleware System Specification"
description: "Add a middleware chain pattern to the feature registry, enabling cross-cutting concerns like auth checks, telemetry, timing, and error recovery to be registered as pre/post hooks on commands."
date: 2026-03-24
status: DRAFT
tags:
  - specification
  - middleware
  - extensibility
  - feature
author:
  - name: Matt Cockayne
    email: matt@phpboyscout.com
  - name: Claude (claude-opus-4-6)
    role: AI drafting assistant
---

# Command Middleware System Specification

Authors
:   Matt Cockayne, Claude (claude-opus-4-6) *(AI drafting assistant)*

Date
:   24 March 2026

Status
:   DRAFT

---

## Overview

Common cross-cutting concerns -- authentication validation, execution timing, panic recovery, telemetry -- must currently be duplicated in each command's `RunE` function. There is no central mechanism for injecting shared behaviour before or after command execution.

This specification introduces a middleware chain pattern for the feature registry. Middleware functions wrap a cobra `RunE` function with additional behaviour, and are applied automatically during command registration. This allows shared logic to be defined once and applied consistently across all commands, or selectively to specific features.

---

## Design Decisions

**Middleware as function wrappers, not event hooks**: A `func(next RunEFunc) RunEFunc` signature composes naturally, avoids event bus complexity, and gives each middleware full control over whether and when to call the next handler. This is the same pattern used by Go HTTP middleware (`func(next http.Handler) http.Handler`).

**Per-feature middleware registration**: Middleware is registered against a `props.FeatureCmd` identifier rather than globally. This allows feature-specific concerns (e.g., auth checks for commands that need API keys) without polluting commands that do not need them. Global middleware is supported by registering against a sentinel "all features" value.

**Deterministic ordering**: Middleware is applied in registration order, with global middleware always running before feature-specific middleware. This ensures predictable execution: recovery wraps timing, which wraps auth, etc.

**No runtime middleware modification**: Once command registration is complete, the middleware chain is sealed. This prevents race conditions from middleware being added after commands are already executing.

**cobra `RunE` only**: Middleware applies to `RunE` (error-returning) functions only. Commands using `Run` (no error return) are not supported. This is consistent with the GTB convention of always using `RunE` for proper error propagation.

---

## Public API Changes

### New: `pkg/setup/middleware.go`

```go
package setup

import (
    "github.com/spf13/cobra"

    "github.com/phpboyscout/gtb/pkg/props"
)

// Middleware wraps a cobra RunE function with additional behaviour.
// The middleware receives the next handler in the chain and returns
// a new handler that may execute logic before and/or after calling next.
type Middleware func(next cobra.RunEFunc) cobra.RunEFunc

// RegisterMiddleware adds middleware that will be applied to commands
// belonging to the specified feature. Middleware is applied in
// registration order.
func RegisterMiddleware(feature props.FeatureCmd, mw ...Middleware)

// RegisterGlobalMiddleware adds middleware that is applied to all
// feature commands. Global middleware runs before feature-specific
// middleware in the chain.
func RegisterGlobalMiddleware(mw ...Middleware)

// Chain applies all registered middleware (global + feature-specific)
// to the given RunE function and returns the wrapped function.
func Chain(feature props.FeatureCmd, runE cobra.RunEFunc) cobra.RunEFunc
```

### New: Built-in Middleware (`pkg/setup/middleware_builtin.go`)

```go
package setup

import (
    "log/slog"

    "github.com/spf13/cobra"
)

// WithTiming returns middleware that logs command execution duration.
func WithTiming(logger *slog.Logger) Middleware

// WithRecovery returns middleware that catches panics in the command
// handler and converts them to errors. The panic value and stack trace
// are logged at Error level.
func WithRecovery(logger *slog.Logger) Middleware

// WithAuthCheck returns middleware that validates the specified
// configuration keys are non-empty before allowing command execution.
// If any key is empty, a descriptive error is returned without
// executing the command.
func WithAuthCheck(keys ...string) Middleware
```

---

## Internal Implementation

### Middleware Registry

```go
package setup

import (
    "sync"

    "github.com/phpboyscout/gtb/pkg/props"
    "github.com/spf13/cobra"
)

var (
    mu               sync.RWMutex
    globalMiddleware []Middleware
    featureMiddleware = make(map[props.FeatureCmd][]Middleware)
    sealed           bool
)

func RegisterMiddleware(feature props.FeatureCmd, mw ...Middleware) {
    mu.Lock()
    defer mu.Unlock()
    if sealed {
        panic("cannot register middleware after command registration is complete")
    }
    featureMiddleware[feature] = append(featureMiddleware[feature], mw...)
}

func RegisterGlobalMiddleware(mw ...Middleware) {
    mu.Lock()
    defer mu.Unlock()
    if sealed {
        panic("cannot register global middleware after command registration is complete")
    }
    globalMiddleware = append(globalMiddleware, mw...)
}

// seal prevents further middleware registration. Called after all
// commands have been registered.
func seal() {
    mu.Lock()
    defer mu.Unlock()
    sealed = true
}

func Chain(feature props.FeatureCmd, runE cobra.RunEFunc) cobra.RunEFunc {
    mu.RLock()
    defer mu.RUnlock()

    // Build the full chain: global first, then feature-specific.
    chain := make([]Middleware, 0, len(globalMiddleware)+len(featureMiddleware[feature]))
    chain = append(chain, globalMiddleware...)
    chain = append(chain, featureMiddleware[feature]...)

    // Apply in reverse order so that the first registered middleware
    // is the outermost wrapper (executes first).
    wrapped := runE
    for i := len(chain) - 1; i >= 0; i-- {
        wrapped = chain[i](wrapped)
    }
    return wrapped
}
```

### Built-in Middleware Implementations

#### `WithTiming`

```go
func WithTiming(logger *slog.Logger) Middleware {
    return func(next cobra.RunEFunc) cobra.RunEFunc {
        return func(cmd *cobra.Command, args []string) error {
            start := time.Now()
            err := next(cmd, args)
            duration := time.Since(start)
            logger.Info("command completed",
                "command", cmd.Name(),
                "duration", duration,
                "error", err,
            )
            return err
        }
    }
}
```

#### `WithRecovery`

```go
func WithRecovery(logger *slog.Logger) Middleware {
    return func(next cobra.RunEFunc) cobra.RunEFunc {
        return func(cmd *cobra.Command, args []string) (retErr error) {
            defer func() {
                if r := recover(); r != nil {
                    stack := debug.Stack()
                    logger.Error("panic recovered in command",
                        "command", cmd.Name(),
                        "panic", r,
                        "stack", string(stack),
                    )
                    retErr = errors.Newf("panic in command %q: %v", cmd.Name(), r)
                }
            }()
            return next(cmd, args)
        }
    }
}
```

#### `WithAuthCheck`

```go
func WithAuthCheck(keys ...string) Middleware {
    return func(next cobra.RunEFunc) cobra.RunEFunc {
        return func(cmd *cobra.Command, args []string) error {
            for _, key := range keys {
                val := viper.GetString(key)
                if val == "" {
                    return errors.Newf(
                        "required configuration %q is not set; run 'config set %s <value>' first",
                        key, key,
                    )
                }
            }
            return next(cmd, args)
        }
    }
}
```

### Integration with Command Registration

In `pkg/cmd/root/root.go`, the `registerFeatureCommands` function applies middleware during registration:

```go
func registerFeatureCommands(root *cobra.Command, features []Feature) {
    // Register all middleware first (features call RegisterMiddleware in init())

    // Seal the middleware registry
    setup.Seal()

    for _, feature := range features {
        cmd := feature.Command()
        if cmd.RunE != nil {
            cmd.RunE = setup.Chain(feature.Name(), cmd.RunE)
        }
        // Also wrap subcommands
        for _, sub := range cmd.Commands() {
            if sub.RunE != nil {
                sub.RunE = setup.Chain(feature.Name(), sub.RunE)
            }
        }
        root.AddCommand(cmd)
    }
}
```

### Feature-Level Registration Example

```go
// In a feature's init function (e.g., pkg/cmd/chat/chat.go)
func init() {
    setup.RegisterMiddleware(props.FeatureCmdChat,
        setup.WithAuthCheck("chat.api_key"),
    )
}
```

### Global Middleware Registration Example

```go
// In root command setup
func init() {
    setup.RegisterGlobalMiddleware(
        setup.WithRecovery(logger),
        setup.WithTiming(logger),
    )
}
```

---

## Project Structure

```
pkg/setup/
├── middleware.go             <- NEW: Middleware type, registry, Chain function
├── middleware_test.go        <- NEW: registry and chain tests
├── middleware_builtin.go     <- NEW: WithTiming, WithRecovery, WithAuthCheck
├── middleware_builtin_test.go <- NEW: built-in middleware tests
pkg/cmd/root/
├── root.go                  <- MODIFIED: apply middleware during registration
├── root_test.go             <- MODIFIED: test middleware application
```

---

## Testing Strategy

### Registry Tests

| Test | Scenario |
|------|----------|
| `TestRegisterMiddleware_Single` | One middleware registered for a feature |
| `TestRegisterMiddleware_Multiple` | Multiple middleware registered in order |
| `TestRegisterGlobalMiddleware` | Global middleware registered |
| `TestRegisterMiddleware_AfterSeal_Panics` | Registration after seal causes panic |
| `TestChain_GlobalBeforeFeature` | Global middleware executes before feature-specific |
| `TestChain_EmptyRegistry` | No middleware registered -- RunE unchanged |
| `TestChain_ExecutionOrder` | Three middleware execute in registration order |

### Chain Execution Order Test

```go
func TestChain_ExecutionOrder(t *testing.T) {
    t.Parallel()
    // Reset registry for test isolation
    resetRegistry(t)

    var order []string

    mw := func(name string) Middleware {
        return func(next cobra.RunEFunc) cobra.RunEFunc {
            return func(cmd *cobra.Command, args []string) error {
                order = append(order, name+":before")
                err := next(cmd, args)
                order = append(order, name+":after")
                return err
            }
        }
    }

    RegisterGlobalMiddleware(mw("global"))
    RegisterMiddleware(props.FeatureCmdUpdate, mw("feature"))

    wrapped := Chain(props.FeatureCmdUpdate, func(cmd *cobra.Command, args []string) error {
        order = append(order, "handler")
        return nil
    })

    err := wrapped(&cobra.Command{}, nil)
    assert.NoError(t, err)
    assert.Equal(t, []string{
        "global:before",
        "feature:before",
        "handler",
        "feature:after",
        "global:after",
    }, order)
}
```

### WithTiming Tests

| Test | Scenario |
|------|----------|
| `TestWithTiming_LogsDuration` | Successful command logs duration |
| `TestWithTiming_LogsError` | Failed command logs both duration and error |
| `TestWithTiming_CommandName` | Log entry includes correct command name |

### WithRecovery Tests

| Test | Scenario |
|------|----------|
| `TestWithRecovery_NoPanic` | Normal execution passes through |
| `TestWithRecovery_CatchesPanic` | Panic converted to error return |
| `TestWithRecovery_LogsStack` | Stack trace logged at Error level |
| `TestWithRecovery_PanicValueInError` | Error message contains panic value |

### WithAuthCheck Tests

| Test | Scenario |
|------|----------|
| `TestWithAuthCheck_AllKeysPresent` | All config keys set -- command executes |
| `TestWithAuthCheck_MissingKey` | One key missing -- error returned, command not executed |
| `TestWithAuthCheck_EmptyKey` | Key exists but empty string -- error returned |
| `TestWithAuthCheck_MultipleKeys` | Multiple keys checked, first missing triggers error |
| `TestWithAuthCheck_NoKeys` | No keys specified -- command always executes |

### Integration Test

```go
func TestMiddleware_IntegrationWithCobra(t *testing.T) {
    t.Parallel()
    resetRegistry(t)

    var executed bool
    cmd := &cobra.Command{
        Use: "test",
        RunE: func(cmd *cobra.Command, args []string) error {
            executed = true
            return nil
        },
    }

    RegisterGlobalMiddleware(WithRecovery(slog.Default()))

    cmd.RunE = Chain(props.FeatureCmdUpdate, cmd.RunE)
    err := cmd.RunE(cmd, nil)

    assert.NoError(t, err)
    assert.True(t, executed)
}
```

### Test Isolation

The global middleware registry uses package-level state. Tests must reset this state:

```go
func resetRegistry(t *testing.T) {
    t.Helper()
    mu.Lock()
    defer mu.Unlock()
    globalMiddleware = nil
    featureMiddleware = make(map[props.FeatureCmd][]Middleware)
    sealed = false
    t.Cleanup(func() {
        mu.Lock()
        defer mu.Unlock()
        globalMiddleware = nil
        featureMiddleware = make(map[props.FeatureCmd][]Middleware)
        sealed = false
    })
}
```

### Coverage

- Target: 95%+ for `pkg/setup/middleware.go` and `pkg/setup/middleware_builtin.go`.

---

## Backwards Compatibility

- **No existing command changes required**: Middleware is opt-in. Existing commands continue to work without modification. Middleware is only applied if explicitly registered.
- **No signature changes**: The `Feature` interface and `props.FeatureCmd` type are unchanged. Middleware is applied by the registration machinery, not by individual features.
- **RunE convention**: GTB already uses `RunE` exclusively. Commands using `Run` (if any) are unaffected and do not receive middleware.

---

## Future Considerations

- **Plugin middleware**: When the plugin system (spec: 2026-03-21-plugin-extension-system) is implemented, plugins should be able to register middleware for their own commands.
- **Conditional middleware**: Add a `WithCondition(predicate func(*cobra.Command) bool)` wrapper that skips middleware for commands matching a predicate (e.g., skip auth for `help` subcommands).
- **Middleware groups**: Named groups of middleware that can be applied to multiple features at once (e.g., a "network" group with auth + retry + timeout).
- **OpenTelemetry spans**: A `WithTracing` middleware that creates spans for each command execution, enabling distributed tracing across CLI invocations.
- **Rate limiting**: A `WithRateLimit` middleware for commands that make API calls, preventing accidental abuse of external services.
- **Middleware ordering DSL**: If middleware ordering becomes complex, consider a priority-based ordering system instead of registration order.

---

## Implementation Phases

### Phase 1 -- Core Middleware System
1. Create `pkg/setup/middleware.go` with `Middleware` type, registry, and `Chain` function
2. Add registry tests including ordering, sealing, and edge cases
3. Add `resetRegistry` test helper for isolation

### Phase 2 -- Built-in Middleware
1. Implement `WithTiming` with structured logging
2. Implement `WithRecovery` with stack trace capture
3. Implement `WithAuthCheck` with viper config validation
4. Add comprehensive tests for each built-in middleware

### Phase 3 -- Integration
1. Update `registerFeatureCommands` in `pkg/cmd/root/root.go` to apply middleware
2. Register `WithRecovery` and `WithTiming` as global middleware
3. Register `WithAuthCheck` for features requiring API keys (e.g., chat, update)
4. Add integration tests verifying end-to-end middleware application

---

## Verification

```bash
# Build
go build ./...

# Full test suite with race detector
just test

# Targeted tests
go test -race ./pkg/setup/...
go test -race ./pkg/cmd/root/...

# Coverage for new code
go test -coverprofile=coverage.out ./pkg/setup/...
go tool cover -func=coverage.out

# Verify middleware is applied in root command registration
grep -n 'Chain\|middleware\|Middleware' pkg/cmd/root/root.go

# Lint
golangci-lint run --fix
```
