---
title: gRPC
description: Secure-by-default gRPC server components.
date: 2026-03-24
tags: [components, grpc, networking, security]
authors: [Matt Cockayne <matt@phpboyscout.com>]
---

# gRPC

The `pkg/grpc` package provides a standard gRPC server implementation that integrates with the `controls` package for lifecycle management and observability.

## Features

- **Standard Observability**: Implements the standard gRPC Health Checking Protocol.
- **Named Probes**: Supports `liveness` and `readiness` service names for orchestrator integration.
- **Reflection**: Built-in support for gRPC reflection (enabled by default).

## Functions

- **`NewServer(cfg config.Containable, opt ...grpc.ServerOption) (*grpc.Server, error)`**: Returns a new `*grpc.Server` with reflection registered.
- **`RegisterHealthService(srv *grpc.Server, controller controls.Controllable)`**: Wires the gRPC health service to the controller status.
- **`Register(ctx context.Context, id string, controller controls.Controllable, cfg config.Containable, logger logger.Logger, opt ...grpc.ServerOption) (*grpc.Server, error)`**: Creates a server, registers the health service, adds it to the controller, and returns the server instance for further service registration.

## Usage Example

```go
// Automatically registers health service and adds to controller
srv, err := grpc.Register(ctx, "grpc-api", controller, props.Config, props.Logger)
if err != nil {
    return err
}

// Register your custom services
pb.RegisterMyServiceServer(srv, &myServiceImpl{})
```
