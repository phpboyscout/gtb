---
title: Security & Secret Management
description: Security standards for the GTB library and generator.
tags: [security, secrets, library, standards]
---

# Security & Secret Management

As a framework that handles AI keys and VCS authentication, security is baked into the design of GTB.

## Secret Handling in Code

- **No Hardcoding**: Secrets must never be hardcoded into the library or generator templates.
- **Viper Integration**: Use the `pkg/config` container to handle secrets securely from environment variables or encrypted configuration backends.
- **Redaction**: Ensure that diagnostic logs redact sensitive keys by default.

## AI Provider Security

- **Credential Isolation**: Each AI provider implementation in `pkg/chat` must isolate credentials to prevent accidental cross-provider leakage.
- **Key Rotation**: Provide clear error messages that guide users to rotate their keys if an "Unauthorized" error is returned from a provider.

## Runtime Protections

### Path Validation with Symlink Resolution

Agent tools that perform file operations use `isPathAllowed` to validate that requested paths remain within the allowed base directory. This function resolves symlinks via `filepath.EvalSymlinks` before performing the prefix check, preventing symlink bypass attacks where a symlink inside the allowed directory points to a location outside it.

### Filesystem Abstraction

Agent file operations (`ReadFile`, `WriteFile`, `ListDirectory`) use `afero.Fs` instead of direct `os` package calls. This ensures consistency with the rest of the codebase and enables testing with in-memory filesystems.

### Init-Time Security

The `init` command includes two security features:
- **`.gitignore` generation**: Automatically creates a `.gitignore` in the config directory to prevent accidental commit of secret files.
- **API key detection**: Scans config files for common key patterns and warns if the directory is inside a git repository.

## Secure Templates

The generator templates in `internal/generator/templates` must generate code that follows secure defaults (e.g., standard permission masks for files, sanitized input handling).

## Reporting Vulnerabilities

If you discover a security vulnerability in GTB, please report it via the internal security channel as defined in the project's root README.
