---
title: "Controllable Interface Narrowing Specification"
description: "Split the 18-method Controllable interface into focused role-based interfaces while preserving backward compatibility through interface composition."
date: 2026-03-21
status: IMPLEMENTED
tags:
  - specification
  - controls
  - interfaces
  - refactor
author:
  - name: Matt Cockayne
    email: matt@phpboyscout.com
  - name: Claude (claude-opus-4-6)
    role: AI drafting assistant
---

# Controllable Interface Narrowing Specification

Authors
:   Matt Cockayne, Claude (claude-opus-4-6) *(AI drafting assistant)*

Date
:   21 March 2026

Status
:   IMPLEMENTED

---

## Overview

The `Controllable` interface in `pkg/controls/controls.go` has 18 methods spanning runtime control, state access, configuration, and channel management. This violates the Go proverb "the bigger the interface, the weaker the abstraction" — consumers that only need to start/stop a service must depend on the full 18-method contract.

Splitting into focused interfaces lets consumers declare minimal dependencies while the `Controller` struct continues to implement everything.

---

## Design Decisions

**Composition over replacement**: The existing `Controllable` interface becomes a composed interface embedding all narrow interfaces. Existing code continues to compile without changes.

**Role-based split**: Interfaces are grouped by responsibility, not by getter/setter pairs. A consumer that needs to run services gets `Runner`; one that needs to configure channels gets `Configurable`.

**`ControllerOpt` functions accept `Configurable`**: Options like `WithoutSignals`, `WithShutdownTimeout`, and `WithLogger` only need setter methods. Their parameter type narrows from `Controllable` to `Configurable`.

---

## Public API Changes

### New Interfaces

```go
// Runner provides service lifecycle operations.
type Runner interface {
    Start()
    Stop()
    IsRunning() bool
    IsStopped() bool
    IsStopping() bool
    Register(id string, opts ...ServiceOption)
}

// StateAccessor provides read access to controller state and context.
type StateAccessor interface {
    GetState() State
    SetState(state State)
    GetContext() context.Context
    GetLogger() *slog.Logger
}

// Configurable provides controller configuration setters.
type Configurable interface {
    SetErrorsChannel(errs chan error)
    SetMessageChannel(control chan Message)
    SetSignalsChannel(sigs chan os.Signal)
    SetHealthChannel(health chan HealthMessage)
    SetWaitGroup(wg *sync.WaitGroup)
    SetShutdownTimeout(d time.Duration)
    SetLogger(logger *slog.Logger)
}

// ChannelProvider provides access to controller channels.
type ChannelProvider interface {
    Messages() chan Message
    Health() chan HealthMessage
    Errors() chan error
    Signals() chan os.Signal
}
```

### Modified: `Controllable`

```go
// Controllable is the full controller interface, composed of all role-based interfaces.
// Prefer using the narrower interfaces (Runner, Configurable, etc.) where possible.
type Controllable interface {
    Runner
    StateAccessor
    Configurable
    ChannelProvider
}
```

### Modified: `ControllerOpt`

```go
// Before:
type ControllerOpt func(Controllable)

// After:
type ControllerOpt func(Configurable)
```

---

## Internal Implementation

### Compile-Time Satisfaction Checks

Add to `controller.go`:

```go
var (
    _ Runner          = (*Controller)(nil)
    _ StateAccessor   = (*Controller)(nil)
    _ Configurable    = (*Controller)(nil)
    _ ChannelProvider = (*Controller)(nil)
    _ Controllable    = (*Controller)(nil)
)
```

### `ControllerOpt` Migration

```go
// Before:
func WithoutSignals() ControllerOpt {
    return func(c Controllable) {
        c.SetSignalsChannel(nil)
    }
}

// After:
func WithoutSignals() ControllerOpt {
    return func(c Configurable) {
        c.SetSignalsChannel(nil)
    }
}
```

Same for `WithShutdownTimeout` and `WithLogger`.

### `NewController` Update

```go
func NewController(ctx context.Context, opts ...ControllerOpt) *Controller {
    // ... create controller ...
    for _, opt := range opts {
        opt(c)  // Controller satisfies Configurable
    }
    return c
}
```

---

## Project Structure

```
pkg/controls/
├── controls.go      ← MODIFIED: new interfaces, Controllable becomes composed
├── controller.go    ← MODIFIED: compile-time checks, ControllerOpt type change
├── controls_test.go ← MODIFIED: add interface satisfaction tests
├── services.go      ← UNCHANGED
```

---

## Testing Strategy

| Test | Scenario |
|------|----------|
| `TestController_SatisfiesRunner` | Compile-time: `var _ Runner = (*Controller)(nil)` |
| `TestController_SatisfiesStateAccessor` | Compile-time check |
| `TestController_SatisfiesConfigurable` | Compile-time check |
| `TestController_SatisfiesChannelProvider` | Compile-time check |
| `TestController_SatisfiesControllable` | Compile-time check (existing) |
| `TestControllerOpt_WithConfigurable` | `WithoutSignals()` works with `Configurable` parameter |
| Existing tests | All existing `controls_test.go` tests pass unchanged |

### Coverage
- Target: 90%+ for `pkg/controls/`.

---

## Linting

- `golangci-lint run --fix` must pass.
- No new `nolint` directives.
- The `interfacebloat` linter (if enabled) will no longer flag `Controllable` since the narrow interfaces are small.

---

## Documentation

- Godoc for each new interface explaining its role and when to use it.
- Add guidance to `Controllable` godoc: "Prefer the narrower interfaces where possible."
- Update `docs/components/controls.md` with the new interface hierarchy and usage guidance.

---

## Backwards Compatibility

- **No breaking changes for `Controllable` consumers**: The composed interface has the exact same method set.
- **Minor breaking change for `ControllerOpt`**: Parameter type changes from `Controllable` to `Configurable`. Any external code defining custom `ControllerOpt` functions that call non-setter methods will need updating. This is expected to be rare.
- **Mock regeneration**: Mocks for `Controllable` will need regeneration, but the mock implementation is unchanged.

---

## Future Considerations

- **gRPC and HTTP servers**: `pkg/grpc` and `pkg/http` could accept `Runner` instead of `Controllable` if they only need lifecycle methods.
- **Event-driven state**: If state transitions become event-driven, `StateAccessor` is the natural interface to extend.

---

## Implementation Phases

### Phase 1 — Define Interfaces
1. Add `Runner`, `StateAccessor`, `Configurable`, `ChannelProvider` to `controls.go`
2. Redefine `Controllable` as composed interface
3. Add compile-time satisfaction checks

### Phase 2 — Narrow `ControllerOpt`
1. Change `ControllerOpt` type to `func(Configurable)`
2. Update `WithoutSignals`, `WithShutdownTimeout`, `WithLogger`
3. Verify `NewController` still compiles

### Phase 3 — Tests & Docs
1. Add interface satisfaction tests
2. Run full test suite
3. Update documentation

---

## Verification

```bash
go build ./...
go test -race ./pkg/controls/...
go test ./...
golangci-lint run --fix
```
