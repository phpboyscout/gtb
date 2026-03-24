---
title: Implementing Custom Middleware
description: How to create and register your own custom middleware for GTB commands.
date: 2026-03-24
tags: [how-to, middleware, setup, custom]
authors: [Matt Cockayne <matt@phpboyscout.com>]
---

# Implementing Custom Middleware

While GTB provides several built-in middlewares, you may need to implement your own for tool-specific behaviors. This guide shows you how to create and register custom middleware.

## The Middleware Signature

A middleware is a function that receives the `next` handler and returns a new handler:

```go
type Middleware func(next cobra.RunEFunc) cobra.RunEFunc
```

Where `cobra.RunEFunc` is defined as:
```go
func(cmd *cobra.Command, args []string) error
```

## Basic Structure

The most common pattern is to create a factory function that takes configuration and returns a `Middleware`:

```go
func WithMyCustomLogic(config string) setup.Middleware {
    return func(next cobra.RunEFunc) cobra.RunEFunc {
        return func(cmd *cobra.Command, args []string) error {
            // 1. Logic BEFORE the command runs
            fmt.Printf("Starting: %s with config %s\n", cmd.Name(), config)

            // 2. Call the next handler in the chain
            err := next(cmd, args)

            // 3. Logic AFTER the command runs
            fmt.Printf("Finished: %s\n", cmd.Name())

            return err
        }
    }
}
```

## Advanced: Conditional Execution

Middleware can decide whether to execute the rest of the chain:

```go
func WithRepoCheck() setup.Middleware {
    return func(next cobra.RunEFunc) cobra.RunEFunc {
        return func(cmd *cobra.Command, args []string) error {
            if _, err := os.Stat(".git"); os.IsNotExist(err) {
                // Short-circuit the execution
                return errors.New("this command must be run inside a git repository")
            }
            return next(cmd, args)
        }
    }
}
```

## Advanced: Modifying the Command or Args

Since you have access to `*cobra.Command` and `[]string`, you can modify them before passing them down:

```go
func WithForcedQuiet() setup.Middleware {
    return func(next cobra.RunEFunc) cobra.RunEFunc {
        return func(cmd *cobra.Command, args []string) error {
            // Set a flag value manually
            _ = cmd.Flags().Set("quiet", "true")

            return next(cmd, args)
        }
    }
}
```

## Error Handling

When your middleware encounters an error, use `github.com/cockroachdb/errors` to wrap it:

```go
func WithValidation() setup.Middleware {
    return func(next cobra.RunEFunc) cobra.RunEFunc {
        return func(cmd *cobra.Command, args []string) error {
            if err := validateEnvironment(); err != nil {
                return errors.Wrap(err, "environment validation failed")
            }
            return next(cmd, args)
        }
    }
}
```

## Registration

Once you've defined your custom middleware, register it like any other:

```go
setup.RegisterGlobalMiddleware(WithMyCustomLogic("production"))
```

---

!!! warning "Order Matters"
    Remember that middleware is applied in the order it's registered. If you have a middleware that depends on another (e.g., auth check depending on a config loader), ensure they are registered in the correct sequence.
