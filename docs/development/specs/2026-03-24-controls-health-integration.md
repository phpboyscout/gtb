---
title: "Controls Package Health Check Integration"
description: "Standardized health check handlers for HTTP and gRPC services leveraging the Controller status."
date: 2026-03-24
status: DRAFT
tags:
  - specification
  - controls
  - health-check
  - observability
author:
  - name: Gemini CLI
    role: AI drafting assistant
---

# Controls Package Health Check Integration

Authors
:   Gemini CLI *(AI drafting assistant)*

Date
:   24 March 2026

Status
:   DRAFT

## 1. Overview

The `controls` package manages the lifecycle of registered services (`Start`, `Stop`, `Status`). While basic status functions exist, there is currently no standardized way to expose these health indicators externally to orchestrators like Kubernetes or monitoring systems.

This specification proposes adding standardized health check handlers for HTTP and gRPC services that leverage the `Controller`'s knowledge of all registered services. This ensures that a single health check endpoint can report the health of the entire application or specific components.

## 2. Problem Statement

Currently, each service registered with the `Controller` has a `StatusFunc`, but:
1. There is no public API on the `Controller` to aggregate these statuses into a single report.
2. HTTP servers created via `pkg/controls/http` do not automatically provide a `/healthz` endpoint.
3. gRPC servers created via `pkg/controls/grpc` do not implement the standard gRPC Health Checking Protocol.
4. Consumers have to manually wire up health checks, leading to inconsistency.

## 3. Goals & Non-Goals

### Goals
- Add a public `Status()` method to `Controller` to aggregate service health.
- Provide a standard `/healthz` HTTP handler in `pkg/controls/http`.
- Integrate the standard gRPC Health Checking Protocol in `pkg/controls/grpc`.
- Ensure health checks are non-blocking and do not cause service disruption.
- Automate the registration of these health checks where possible (transparently).

### Non-Goals
- Implementing complex liveness vs. readiness probe logic (this spec focuses on a general health check).
- Persistent storage of health history.
- Automatic restarts of failing services (this remains the responsibility of the orchestrator).

## 4. Public API

### 4.1 `pkg/controls`

Update `Controller` to expose health information:

```go
type ServiceStatus struct {
    Name   string `json:"name"`
    Status string `json:"status"` // "OK", "ERROR"
    Error  string `json:"error,omitempty"`
}

type HealthReport struct {
    OverallHealthy bool            `json:"overall_healthy"`
    Services       []ServiceStatus `json:"services"`
}

// Status returns an aggregate health report for all registered services.
func (c *Controller) Status() HealthReport
```

### 4.2 `pkg/controls/http`

Add a health handler:

```go
// HealthHandler returns an http.HandlerFunc that responds with the controller's health report.
// It returns 200 OK if all services are healthy, and 503 Service Unavailable otherwise.
func HealthHandler(controller controls.Controllable) http.HandlerFunc
```

Update `Register` to optionally (or by default) include the health endpoint.

### 4.3 `pkg/controls/grpc`

Add a health service registration helper:

```go
// RegisterHealthService registers the standard gRPC health service with the provided server,
// wired to the controller's status.
func RegisterHealthService(srv *grpc.Server, controller controls.Controllable)
```

## 5. Internal Implementation

### 5.1 `Controller.Status()`
The `Status()` method will iterate through `c.services.services`, calling each `Status()` function. Since `StatusFunc` now returns an `error` (as per recent updates), the results will be collected into a `HealthReport`.

### 5.2 HTTP Integration
The `HealthHandler` will encode the `HealthReport` as JSON.
If `Register` is called, it should wrap the provided `http.Handler` (if it's a mux) or use a internal mux to ensure `/healthz` is handled.

### 5.3 gRPC Integration
Using `google.golang.org/grpc/health`, we will implement a periodic or on-demand update of the health status. Since `grpc/health` expects a `Watch` or simple `Check`, we can either:
1. Update the health server state every time `Status()` is queried.
2. Run a background ticker that updates the health server state by calling `Controller.Status()`.

## 6. Project Structure

- `pkg/controls/controller.go`: Implementation of `Status()`.
- `pkg/controls/http/handlers.go`: New file for health handlers.
- `pkg/controls/grpc/health.go`: New file for gRPC health service integration.

## 7. Error Handling

- If a `StatusFunc` panics, it should be recovered and reported as an error in the `HealthReport`.
- Connectivity errors in health checks should be wrapped using `github.com/cockroachdb/errors`.

## 8. Testing Strategy

Implementation must follow the **Test-Driven Development (TDD)** approach as defined in `docs/development/specs/index.md`.

### 8.1 Unit Tests
- **Package `pkg/controls`**:
    - Test `Status()` with various service combinations:
        - All services healthy (returns `overall_healthy: true`).
        - One or more services returning errors (returns `overall_healthy: false`).
        - Services with `nil` `StatusFunc` (should be treated as healthy by default or handled gracefully).
    - Verify thread-safety of `Status()` when called concurrently with service start/stop.
- **Package `pkg/controls/http`**:
    - Test `HealthHandler` with mock controllers.
    - Assert correct HTTP status codes (200 for healthy, 503 for unhealthy).
    - Assert JSON response body matches the `HealthReport` structure.
- **Package `pkg/controls/grpc`**:
    - Test gRPC health service integration using `grpc_health_v1`.
    - Verify `Check` returns `SERVING` or `NOT_SERVING` based on controller status.

### 8.2 Integration Tests
- **HTTP Server Integration**:
    - Start a real HTTP server using `http.Register`.
    - Perform HTTP GET requests to `/healthz` and verify full end-to-end connectivity and reporting.
- **gRPC Server Integration**:
    - Start a real gRPC server using `grpc.Register`.
    - Use a gRPC health client to query the health status.

### 8.3 Quality Gates
- **Code Coverage**: New code in `pkg/` must achieve at least **90% coverage**.
- **Race Detection**: All tests must pass with `go test -race ./...`.
- **Linting**: Must pass `golangci-lint run --fix` with no outstanding issues.

## 9. Documentation Maintenance

Documentation is a first-class citizen in GTB. The following updates are required:

- **Library Documentation**: Update `docs/components/controls.md` to document the new `Status()` method and health registration helpers.
- **Concept Documentation**: Update `docs/concepts/service-orchestration.md` to reflect the new health checking capabilities.
- **API Reference**: Ensure all new public methods, types, and constants are fully documented with GoDoc comments.
- **Examples**: Add or update examples in `pkg/controls/example_test.go` (if exists) or in the documentation to demonstrate how to use health checks.

## 10. Leveraged Workflows

Implementation MUST leverage the following workflows from `.agent/workflows/`:

- `/gtb-library-contribution`: For adding the core logic to `pkg/`.
- `/gtb-verify`: To ensure all tests pass, race conditions are absent, and linting is clean.
- `/gtb-lint`: To resolve any complex linting issues.
- `/gtb-docs`: For updating the markdown documentation in `docs/`.

## 11. Migration & Compatibility

- This is a feature addition and should be backward compatible.
- Existing `StatusFunc` implementations are already compatible with the new signature (`func() error`).

## 12. Implementation Phases

### Phase 1: Core Controller
- Implement `Controller.Status()`.
- Add `ServiceStatus` and `HealthReport` types.
- Update `Services.status()` to return results.

### Phase 2: HTTP Health Check
- Implement `http.HealthHandler`.
- Update `http.Register` to optionally inject the health handler.

### Phase 3: gRPC Health Check
- Implement `grpc.RegisterHealthService`.
- Wire it into the `grpc.Register` flow.
