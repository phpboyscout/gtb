---
title: Command Middleware
description: Technical reference for the command middleware system in the setup package.
date: 2026-03-24
tags: [components, setup, middleware, cobra]
authors: [Matt Cockayne <matt@phpboyscout.com>]
---

# Command Middleware

The middleware system in `pkg/setup` provides a mechanism for wrapping `cobra.Command` execution with cross-cutting concerns. It uses a functional chain pattern that allows logic to be executed before and after a command's `RunE` function.

## Core API

### Middleware Type

```go
type Middleware func(next cobra.RunEFunc) cobra.RunEFunc
```

A `Middleware` is a higher-order function that takes a `cobra.RunEFunc` and returns a new `cobra.RunEFunc`. This allows for nesting and composition.

### Registration Functions

#### `RegisterGlobalMiddleware`
```go
func RegisterGlobalMiddleware(mw ...Middleware)
```
Adds middleware that will be applied to **all** commands registered via the root command. Global middleware is executed before feature-specific middleware.

#### `RegisterMiddleware`
```go
func RegisterMiddleware(feature props.FeatureCmd, mw ...Middleware)
```
Adds middleware that will be applied only to commands associated with a specific `props.FeatureCmd`.

#### `Seal`
```go
func Seal()
```
Locks the middleware registry. This must be called before `Chain()` is used, typically during root command initialization. Attempting to register middleware after sealing will cause a panic.

### Application Functions

#### `Chain`
```go
func Chain(feature props.FeatureCmd, runE cobra.RunEFunc) cobra.RunEFunc
```
Applies all registered global and feature-specific middleware to the provided `RunE` function, returning the final wrapped function.

## Integration Helpers

While `pkg/setup` provides the core registry, the `pkg/cmd/root` package provides higher-level helpers for applying middleware to `cobra.Command` structures:

#### `root.AddCommandWithMiddleware`
Adds a command to a parent and recursively applies middleware to the command and all its children.

#### `root.ApplyMiddlewareRecursively`
Applies middleware to an existing command tree. Use this if the command has already been added to a parent via standard cobra methods.

## Built-in Middleware

The `setup` package provides several production-ready middlewares in `middleware_builtin.go`.

### `WithTiming`
```go
func WithTiming(logger *slog.Logger) Middleware
```
Logs the execution duration of the command.
- **Log Level**: `Info`
- **Fields**: `command`, `duration`, `error` (if any)

### `WithRecovery`
```go
func WithRecovery(logger *slog.Logger) Middleware
```
Catches panics during command execution and converts them into returned errors.
- **Log Level**: `Error` (on panic)
- **Fields**: `command`, `panic`, `stack`

### `WithAuthCheck`
```go
func WithAuthCheck(keys ...string) Middleware
```
Verifies that the specified configuration keys are set (non-empty) before executing the command. If any key is missing, it returns an error and prevents command execution.

## Implementation Details

### Execution Order
When `Chain` is called, it constructs a sequence:
`Global MW 1` -> `Global MW 2` -> `Feature MW 1` -> `Feature MW 2` -> `Actual Command`

Because each middleware "wraps" the next, the "before" logic executes in the order above, while "after" logic (and `defer` statements) executes in reverse order.

### Thread Safety
The middleware registry uses a `sync.RWMutex` to ensure safe concurrent access, although registration typically happens during single-threaded `init()` phases.

### Error Handling
Middleware should generally return the error from the `next()` call unless they are specifically designed to transform or suppress errors. GTB recommends using `github.com/cockroachdb/errors` for wrapping errors within middleware.

## Example: Custom Middleware

```go
func WithCustomHeader(header string) setup.Middleware {
    return func(next cobra.RunEFunc) cobra.RunEFunc {
        return func(cmd *cobra.Command, args []string) error {
            fmt.Println(header)
            return next(cmd, args)
        }
    }
}
```
