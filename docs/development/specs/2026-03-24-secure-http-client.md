---
title: "Secure HTTP Client Specification"
description: "Add a hardened HTTP client factory to pkg/controls/http with security-focused defaults, shared TLS configuration, and migration of all existing bare http.DefaultClient usages."
date: 2026-03-24
status: DRAFT
tags:
  - specification
  - security
  - http
  - networking
author:
  - name: Matt Cockayne
    email: matt@phpboyscout.com
  - name: Claude (claude-opus-4-6)
    role: AI drafting assistant
---

# Secure HTTP Client Specification

Authors
:   Matt Cockayne, Claude (claude-opus-4-6) *(AI drafting assistant)*

Date
:   24 March 2026

Status
:   DRAFT

---

## Overview

Go's default `http.Client{}` has no timeouts, no TLS minimum version, follows redirects unconditionally, and imposes no connection limits. GTB already provides `NewServer()` in `pkg/controls/http` with secure TLS defaults, but there is no equivalent factory for outbound HTTP clients.

This specification adds a `NewClient()` factory function that mirrors the server's security posture for outbound connections, extracts shared TLS configuration into a reusable module, and migrates all existing usages of `http.DefaultClient` and uninitialised SDK clients across the codebase.

### Problem

The following security risks exist with the current approach:

1. **No timeouts**: `http.DefaultClient` has zero timeouts. A slow or unresponsive upstream can block goroutines indefinitely.
2. **No TLS floor**: Connections may negotiate TLS 1.0 or 1.1, which have known vulnerabilities.
3. **Unrestricted redirects**: The default client follows up to 10 redirects with no cross-scheme protection, allowing HTTPS-to-HTTP downgrades that leak credentials.
4. **No connection pooling limits**: Unbounded idle connections can exhaust file descriptors under load.
5. **Inconsistent posture**: The server enforces TLS 1.2+ with curated cipher suites, but outbound clients use whatever Go's defaults happen to be.

---

## Design Decisions

**Mirror server TLS configuration**: The client must use the same TLS minimum version, cipher suites, and curve preferences as `NewServer()`. This ensures a consistent security posture across all network boundaries.

**Shared TLS module**: Rather than duplicating TLS constants between server and client, extract them into a `tls.go` file within the same package. Both `NewServer` and `NewClient` consume the shared configuration.

**Functional options pattern**: `ClientOption` functions follow the established GTB pattern for optional configuration (consistent with other builders in the project).

**Redirect policy with scheme protection**: The default policy allows up to 10 redirects (matching Go's default count) but rejects any redirect that downgrades from HTTPS to HTTP. This prevents credential leakage through redirect-based attacks.

**SDK client injection**: Third-party SDK clients (GitHub, GitLab, Anthropic, OpenAI) accept custom `http.Client` instances. Passing `NewClient()` to these SDKs ensures all outbound traffic inherits GTB's security defaults without forking or wrapping the SDKs.

---

## Public API Changes

### New: `pkg/controls/http.NewClient`

```go
package http

import (
    "crypto/tls"
    "net/http"
    "time"
)

// NewClient returns an *http.Client with security-focused defaults:
// TLS 1.2 minimum, curated cipher suites, timeouts, connection limits,
// and redirect policy that rejects HTTPS-to-HTTP downgrades.
func NewClient(opts ...ClientOption) *http.Client

// ClientOption configures the secure HTTP client.
type ClientOption func(*clientConfig)

// WithTimeout sets the overall request timeout. Default: 30s.
func WithTimeout(d time.Duration) ClientOption

// WithMaxRedirects sets the maximum number of redirects to follow. Default: 10.
// Set to 0 to disable redirect following entirely.
func WithMaxRedirects(n int) ClientOption

// WithTLSConfig overrides the default TLS configuration.
// The caller is responsible for ensuring the provided config meets
// security requirements.
func WithTLSConfig(cfg *tls.Config) ClientOption

// WithTransport overrides the entire HTTP transport.
// When set, transport-level options (TLS config, connection limits) are ignored.
func WithTransport(rt http.RoundTripper) ClientOption
```

### Modified: `pkg/controls/http` internal structure

The existing TLS configuration in `server.go` is extracted to `tls.go` and shared.

---

## Internal Implementation

### Shared TLS Configuration (`tls.go`)

```go
package http

import (
    "crypto/tls"
)

// defaultTLSConfig returns the shared TLS configuration used by both
// NewServer and NewClient. It enforces TLS 1.2 minimum with curated
// cipher suites and curve preferences.
func defaultTLSConfig() *tls.Config {
    return &tls.Config{
        MinVersion: tls.VersionTLS12,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
            tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
            tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
            tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
        },
        CurvePreferences: []tls.CurveID{
            tls.X25519,
            tls.CurveP256,
        },
    }
}
```

The existing `NewServer` function in `server.go` is updated to call `defaultTLSConfig()` instead of inlining the same values.

### Client Factory (`client.go`)

```go
package http

import (
    "crypto/tls"
    "net"
    "net/http"
    "time"

    "github.com/cockroachdb/errors"
)

type clientConfig struct {
    timeout      time.Duration
    maxRedirects int
    tlsConfig    *tls.Config
    transport    http.RoundTripper
}

func defaultClientConfig() *clientConfig {
    return &clientConfig{
        timeout:      30 * time.Second,
        maxRedirects: 10,
        tlsConfig:    defaultTLSConfig(),
    }
}

func NewClient(opts ...ClientOption) *http.Client {
    cfg := defaultClientConfig()
    for _, opt := range opts {
        opt(cfg)
    }

    var transport http.RoundTripper
    if cfg.transport != nil {
        transport = cfg.transport
    } else {
        transport = &http.Transport{
            TLSClientConfig:       cfg.tlsConfig,
            MaxIdleConns:          100,
            MaxIdleConnsPerHost:   10,
            IdleConnTimeout:       90 * time.Second,
            TLSHandshakeTimeout:  10 * time.Second,
            ExpectContinueTimeout: 1 * time.Second,
            ResponseHeaderTimeout: 30 * time.Second,
            DialContext: (&net.Dialer{
                Timeout:   30 * time.Second,
                KeepAlive: 30 * time.Second,
            }).DialContext,
        }
    }

    return &http.Client{
        Timeout:   cfg.timeout,
        Transport: transport,
        CheckRedirect: redirectPolicy(cfg.maxRedirects),
    }
}

// redirectPolicy returns a CheckRedirect function that limits the number
// of redirects and rejects HTTPS-to-HTTP downgrades.
func redirectPolicy(maxRedirects int) func(*http.Request, []*http.Request) error {
    return func(req *http.Request, via []*http.Request) error {
        if len(via) >= maxRedirects {
            return errors.Newf("stopped after %d redirects", maxRedirects)
        }
        if len(via) > 0 && via[0].URL.Scheme == "https" && req.URL.Scheme == "http" {
            return errors.New("refused redirect: HTTPS to HTTP downgrade")
        }
        return nil
    }
}
```

### Server Update (`server.go`)

Replace the inline TLS configuration in `NewServer` with a call to `defaultTLSConfig()`:

```go
// Before (in server.go):
// tlsConfig := &tls.Config{
//     MinVersion: tls.VersionTLS12,
//     CipherSuites: []uint16{...},
//     CurvePreferences: []tls.CurveID{...},
// }

// After:
tlsConfig := defaultTLSConfig()
```

### Migration of Existing Usages

| File | Current Usage | Migration |
|------|---------------|-----------|
| `pkg/vcs/github/client.go:180` | `http.DefaultClient` in `DownloadReleaseAsset` | Replace with `gtbhttp.NewClient()` |
| `pkg/vcs/github/release.go:135` | `http.DefaultClient` in `DownloadReleaseAsset` | Replace with `gtbhttp.NewClient()` |
| `pkg/vcs/gitlab/release.go:172` | `http.DefaultClient.Do(req)` | Replace with `gtbhttp.NewClient().Do(req)` |
| `pkg/vcs/github/client.go:222` | `github.NewClient(nil)` | `github.NewClient(gtbhttp.NewClient())` |
| `pkg/vcs/gitlab/release.go:94-97` | `gitlab.NewClient(token)` | Add `gitlab.WithHTTPClient(gtbhttp.NewClient())` |
| `pkg/chat/claude.go:43-45` | `anthropic.NewClient()` | Add `option.WithHTTPClient(gtbhttp.NewClient())` |
| `pkg/chat/openai.go:53-58` | `openai.NewClient()` | Add `option.WithHTTPClient(gtbhttp.NewClient())` |
| `pkg/chat/gemini.go:41-44` | `genai.NewClient()` | Investigate transport injection; use `option.WithHTTPClient` if supported, otherwise wrap via custom `RoundTripper` |

Note: Files importing the new client will use a qualified import alias (e.g., `gtbhttp`) to avoid collision with the standard library `net/http` package.

---

## Project Structure

```
pkg/controls/http/
├── tls.go              <- NEW: shared TLS configuration
├── tls_test.go         <- NEW: TLS config validation tests
├── client.go           <- NEW: secure HTTP client factory
├── client_test.go      <- NEW: client tests
├── server.go           <- MODIFIED: use defaultTLSConfig()
├── server_test.go      <- MODIFIED: verify server still uses shared config
pkg/vcs/github/
├── client.go           <- MODIFIED: use NewClient()
├── release.go          <- MODIFIED: use NewClient()
pkg/vcs/gitlab/
├── release.go          <- MODIFIED: use NewClient()
pkg/chat/
├── claude.go           <- MODIFIED: inject NewClient()
├── openai.go           <- MODIFIED: inject NewClient()
├── gemini.go           <- MODIFIED: inject NewClient() or transport
```

---

## Testing Strategy

### Unit Tests for `NewClient`

| Test | Scenario |
|------|----------|
| `TestNewClient_DefaultTimeout` | Default client has 30s timeout |
| `TestNewClient_WithTimeout` | Custom timeout is applied |
| `TestNewClient_DefaultTLS` | TLS config matches `defaultTLSConfig()` |
| `TestNewClient_WithTLSConfig` | Custom TLS config overrides default |
| `TestNewClient_WithTransport` | Custom transport replaces default |
| `TestNewClient_WithMaxRedirects_Zero` | No redirects followed when set to 0 |
| `TestNewClient_TransportDefaults` | Connection pool limits, handshake timeout, etc. are set |

### Redirect Policy Tests

| Test | Scenario |
|------|----------|
| `TestRedirectPolicy_WithinLimit` | 3 redirects with limit 10 -- allowed |
| `TestRedirectPolicy_ExceedsLimit` | 11 redirects with limit 10 -- rejected |
| `TestRedirectPolicy_HTTPStoHTTP` | HTTPS origin redirected to HTTP -- rejected |
| `TestRedirectPolicy_HTTPtoHTTPS` | HTTP origin redirected to HTTPS -- allowed |
| `TestRedirectPolicy_SameScheme` | HTTPS to HTTPS redirect -- allowed |

### Shared TLS Config Tests

| Test | Scenario |
|------|----------|
| `TestDefaultTLSConfig_MinVersion` | MinVersion is TLS 1.2 |
| `TestDefaultTLSConfig_CipherSuites` | All cipher suites are AEAD-based |
| `TestDefaultTLSConfig_CurvePreferences` | X25519 is preferred, P256 is fallback |
| `TestDefaultTLSConfig_ServerAndClientMatch` | Server and client use identical TLS config |

### Integration-Level Tests

```go
func TestNewClient_RealHTTPSRequest(t *testing.T) {
    t.Parallel()
    // Start a local TLS server with TLS 1.2
    server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    t.Cleanup(server.Close)

    client := NewClient(WithTLSConfig(server.TLS))
    resp, err := client.Get(server.URL)
    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    resp.Body.Close()
}

func TestNewClient_RejectsHTTPSDowngrade(t *testing.T) {
    t.Parallel()
    httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    t.Cleanup(httpServer.Close)

    httpsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        http.Redirect(w, r, httpServer.URL, http.StatusFound)
    }))
    t.Cleanup(httpsServer.Close)

    client := NewClient(WithTLSConfig(httpsServer.TLS))
    _, err := client.Get(httpsServer.URL)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "HTTPS to HTTP downgrade")
}
```

### Migration Verification

Each migrated file must have a test confirming the secure client is used. For SDK clients, verify the custom HTTP client is passed through by checking that requests go through the expected transport (e.g., using a recording `RoundTripper`).

### Coverage

- Target: 95%+ for `pkg/controls/http/client.go` and `pkg/controls/http/tls.go`.
- Existing `server.go` coverage must not regress.

---

## Backwards Compatibility

- **`NewServer` behaviour unchanged**: Extracting TLS config to a shared function does not change any server behaviour. The same cipher suites, TLS version, and curve preferences are used.
- **No public signature changes**: `NewServer` retains its existing signature. `NewClient` is purely additive.
- **SDK client behaviour**: Injecting a custom HTTP client into SDK constructors is the documented and supported configuration mechanism for all three providers. No SDK internals are modified.

---

## Future Considerations

- **Client-side mTLS**: Add a `WithClientCert` option for services requiring mutual TLS authentication.
- **HTTP/2 support**: The default transport supports HTTP/2 via Go's automatic upgrade. A future option could force HTTP/2-only or HTTP/1.1-only modes.
- **Circuit breaker integration**: Wrap the transport with a circuit breaker for resilience against cascading failures.
- **Request signing**: Add middleware support for AWS Signature V4 or similar request signing schemes.
- **Observability**: Add a `WithTracing` option that injects OpenTelemetry spans into outbound requests.
- **Proxy support**: Add explicit proxy configuration options beyond Go's default `HTTP_PROXY` environment variable handling.

---

## Implementation Phases

### Phase 1 -- Shared TLS Configuration
1. Create `pkg/controls/http/tls.go` with `defaultTLSConfig()`
2. Update `server.go` to use `defaultTLSConfig()`
3. Add tests verifying server behaviour is unchanged
4. Verify all existing tests pass

### Phase 2 -- Client Factory
1. Create `pkg/controls/http/client.go` with `NewClient` and all options
2. Implement `redirectPolicy` with scheme downgrade protection
3. Add comprehensive unit tests for all options and the redirect policy
4. Add integration tests with `httptest.NewTLSServer`

### Phase 3 -- Migration
1. Update `pkg/vcs/github/client.go` and `release.go`
2. Update `pkg/vcs/gitlab/release.go`
3. Update `pkg/chat/claude.go`, `openai.go`, and `gemini.go`
4. Add or update tests for each migrated file
5. Verify no remaining usages of `http.DefaultClient` or uninitialised SDK clients

---

## Verification

```bash
# Build
go build ./...

# Full test suite with race detector
just test

# Targeted tests
go test -race ./pkg/controls/http/...
go test -race ./pkg/vcs/github/...
go test -race ./pkg/vcs/gitlab/...
go test -race ./pkg/chat/...

# Coverage for new code
go test -coverprofile=coverage.out ./pkg/controls/http/...
go tool cover -func=coverage.out

# Verify no remaining http.DefaultClient usages
grep -rn 'http\.DefaultClient' pkg/ internal/

# Verify no uninitialised SDK clients (nil http client)
grep -rn 'NewClient(nil)' pkg/ internal/
grep -rn 'github\.NewClient(nil)' pkg/ internal/

# Lint
golangci-lint run --fix
```
