---
title: HTTP
description: Secure-by-default HTTP server and client components.
date: 2026-03-24
tags: [components, http, networking, security]
authors: [Matt Cockayne <matt@phpboyscout.com>]
---

# HTTP

The `pkg/http` package provides hardened HTTP components for both server-side and client-side operations. It enforces secure TLS defaults, provides built-in observability endpoints, and mirrors the security posture required for production environments.

## Server Control

The HTTP server implementation integrates seamlessly with the `controls` lifecycle management.

### Features

- **Standardized Endpoints**: Automatically mounts `/healthz`, `/livez`, and `/readyz`.
- **Production Timeouts**: Pre-configured Read (5s), Write (10s), and Idle (120s) timeouts.
- **Secure TLS**: Enforces TLS 1.2 minimum with curated AEAD-based cipher suites and X25519 preference.

### Functions

- **`NewServer(ctx context.Context, cfg config.Containable, handler http.Handler) (*http.Server, error)`**: Returns a pre-configured `*http.Server`.
- **`Register(ctx context.Context, id string, controller controls.Controllable, cfg config.Containable, logger logger.Logger, handler http.Handler) (*http.Server, error)`**: Creates, configures, and registers the server with a `Controller`.

### Usage Example

```go
mux := http.NewServeMux()
mux.HandleFunc("/api/data", myDataHandler)

// Automatically registers observability endpoints and your mux
srv, err := http.Register(ctx, "http-api", controller, props.Config, props.Logger, mux)
```

## Client Factory

The `pkg/http` package provides a factory for creating hardened `http.Client` instances for outbound requests.

### Features

- **Mandatory Timeouts**: Default 30s timeout to prevent blocked goroutines.
- **Secure Transport**: Uses the same hardened TLS configuration as the server.
- **Scheme Protection**: Redirect policy rejects HTTPS-to-HTTP downgrades.
- **Connection Limits**: Pre-configured idle connection pooling and timeouts.

### Functions

- **`NewClient(opts ...ClientOption) *http.Client`**: Returns a hardened HTTP client.
- **`NewTransport(tlsCfg *tls.Config) *http.Transport`**: Returns a pre-configured secure transport for custom client needs.

### Options

- `WithTimeout(d time.Duration)`
- `WithMaxRedirects(n int)`
- `WithTLSConfig(cfg *tls.Config)`
- `WithTransport(rt http.RoundTripper)`

### Usage Example

```go
// Simple secure client
client := http.NewClient()

// Custom secure client for an SDK
githubClient := github.NewClient(http.NewClient(http.WithTimeout(10 * time.Second)))

// Power user: custom client with secure transport
customClient := &http.Client{
    Transport: http.NewTransport(nil),
}
```
