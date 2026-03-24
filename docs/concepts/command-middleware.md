---
title: Command Middleware System
description: Understanding the middleware chain pattern for cross-cutting CLI command concerns.
date: 2026-03-24
tags: [concepts, middleware, extensibility, cobra]
authors: [Matt Cockayne <matt@phpboyscout.uk>]
---

# Command Middleware System

The Command Middleware System provides a powerful way to inject shared behavior across your CLI command tree. Instead of duplicating logic in every command's `RunE` function, you can define "middlewares" that wrap your commands to handle cross-cutting concerns.

## The Chain Pattern

GTB uses a functional "Chain" pattern for middleware, similar to those found in modern web frameworks like Gin or Echo. A middleware is a function that receives the "next" handler in the chain and returns a new handler.

```go
type Middleware func(next func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error
```

This pattern is superior to simple lifecycle hooks (like `PreRun`) because it allows the middleware to "wrap" the entire execution. A middleware can:
1.  Execute logic **before** the command runs.
2.  Execute logic **after** the command runs.
3.  **Recover** from panics within the command.
4.  **Decide** whether to call the next handler at all (e.g., for auth checks).
5.  **Transform** the error returned by the command.

## Global vs. Feature Middleware

GTB distinguishes between two scopes of middleware:

**Global Middleware**
: Applied to **every** command in the tool. Ideal for universal concerns like panic recovery, execution timing, and global telemetry.

**Feature Middleware**
: Applied only to commands belonging to a specific **Feature**. Ideal for domain-specific concerns like verifying API keys for AI commands or checking for a valid git repository.

## Execution Order

Middleware is applied in a deterministic order to ensure predictable behavior:

1.  **Global Middleware** executes first, in the order they were registered.
2.  **Feature Middleware** executes second, in the order they were registered.
3.  **The Command Handler** executes last.

Because it is a wrapping chain, the "before" logic runs in registration order (outermost to innermost), and the "after" logic (and `defer` blocks) runs in reverse registration order (innermost to outermost).

## The Registry Lifecycle

To ensure thread safety and architectural consistency, the middleware registry follows a strict lifecycle:

1.  **Registration**: Occurs during the `init()` phase of your packages.
2.  **Sealing**: The registry is "sealed" during the root command registration. No further middleware can be added once the command tree is being built.
3.  **Execution**: The `Chain()` function is called recursively for every command and subcommand, baking the middleware into the `RunE` field of the `cobra.Command`.

## Manual Registration

If you add commands to your CLI tree *after* the root command has been initialized (e.g., dynamically or in a non-standard `main.go`), you must ensure middleware is applied manually.

While `setup.Seal()` prevents adding *new* middleware definitions, already registered middleware can still be applied to new commands using the helpers provided in `pkg/cmd/root`:

- `root.AddCommandWithMiddleware(parent, cmd, feature)`: Adds a command to a parent and recursively applies middleware.
- `root.ApplyMiddlewareRecursively(cmd, feature)`: Applies middleware to an existing command tree.

Using these helpers ensures that even manually registered commands benefit from global concerns like recovery and timing.

---

!!! tip "Middleware vs. Hooks"
    Use **Hooks** (`PersistentPreRunE`) for environmental setup like loading config files. Use **Middleware** for operational concerns that need to wrap the execution, like timing, logging, or error recovery.
