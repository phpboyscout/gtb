---
title: "Security Hardening Specification"
description: "Fix symlink bypass in path validation, migrate agent tools from os to afero, and add API key protection for git repositories."
date: 2026-03-21
status: IMPLEMENTED
tags:
  - specification
  - security
  - agent
  - file-operations
author:
  - name: Matt Cockayne
    email: matt@phpboyscout.com
  - name: Claude (claude-opus-4-6)
    role: AI drafting assistant
---

# Security Hardening Specification

Authors
:   Matt Cockayne, Claude (claude-opus-4-6) *(AI drafting assistant)*

Date
:   21 March 2026

Status
:   DRAFT

---

## Overview

Three security concerns were identified during code review:

1. **Symlink bypass in path validation**: `isPathAllowed` in `internal/agent/tools.go` uses `filepath.Abs` which does not resolve symlinks. An attacker can create a symlink inside the allowed base path pointing to `/etc/passwd` or similar, bypassing the `strings.HasPrefix` check.

2. **Agent tools use `os` package directly**: All file operations in `internal/agent/tools.go` use `os.ReadFile`, `os.WriteFile`, and `os.ReadDir` instead of the project's `afero.Fs` abstraction. This makes the tools untestable with in-memory filesystems and inconsistent with the rest of the codebase.

3. **API keys in git repositories**: During `init`, config files may contain API keys. If the project directory is a git repo, these keys could be committed accidentally. No `.gitignore` template is generated and no warnings are issued.

---

## Design Decisions

**`filepath.EvalSymlinks` before prefix check**: This is the standard Go approach for canonicalising paths before security-sensitive comparisons. It resolves all symlinks in the path and returns the real absolute path.

**Afero migration for agent tools**: The agent tool constructors currently receive only a `basePath string`. They need an `afero.Fs` parameter to use the filesystem abstraction. Since Props already carries `afero.Fs`, the natural approach is to pass `props.FS` through to the tool constructors.

**Gitignore template during init**: The `init` command already writes config files. Adding a `.gitignore` in the config directory is a natural extension. A warning log when API keys are detected in config within a git repo provides defence in depth.

---

## Public API Changes

### Modified: Agent Tool Constructors

```go
// Before:
func NewReadFileTool(basePath string) Tool
func NewWriteFileTool(basePath string) Tool
func NewListDirectoryTool(basePath string) Tool

// After:
func NewReadFileTool(fs afero.Fs, basePath string) Tool
func NewWriteFileTool(fs afero.Fs, basePath string) Tool
func NewListDirectoryTool(fs afero.Fs, basePath string) Tool
```

### Modified: `isPathAllowed`

```go
// Before:
func isPathAllowed(basePath, requestedPath string) (string, error)

// After:
func isPathAllowed(fs afero.Fs, basePath, requestedPath string) (string, error)
```

---

## Internal Implementation

### Symlink Resolution

```go
func isPathAllowed(fs afero.Fs, basePath, requestedPath string) (string, error) {
    absBase, err := filepath.Abs(basePath)
    if err != nil {
        return "", errors.Wrap(err, "resolving base path")
    }

    absRequested, err := filepath.Abs(requestedPath)
    if err != nil {
        return "", errors.Wrap(err, "resolving requested path")
    }

    // Resolve symlinks to get the real path
    realBase, err := filepath.EvalSymlinks(absBase)
    if err != nil {
        return "", errors.Wrap(err, "evaluating symlinks in base path")
    }

    realRequested, err := filepath.EvalSymlinks(absRequested)
    if err != nil {
        // File may not exist yet (write operations) — resolve parent
        parentDir := filepath.Dir(absRequested)
        realParent, err2 := filepath.EvalSymlinks(parentDir)
        if err2 != nil {
            return "", errors.Wrap(err2, "evaluating symlinks in parent directory")
        }
        realRequested = filepath.Join(realParent, filepath.Base(absRequested))
    }

    if !strings.HasPrefix(realRequested, realBase+string(filepath.Separator)) && realRequested != realBase {
        return "", errors.Newf("path %q is outside allowed base %q", requestedPath, basePath)
    }

    return realRequested, nil
}
```

### Afero Migration for Agent Tools

```go
type readFileTool struct {
    fs       afero.Fs
    basePath string
}

func NewReadFileTool(fs afero.Fs, basePath string) Tool {
    return &readFileTool{fs: fs, basePath: basePath}
}

func (t *readFileTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
    // ...
    allowed, err := isPathAllowed(t.fs, t.basePath, params.Path)
    if err != nil {
        return "", err
    }

    data, err := afero.ReadFile(t.fs, allowed)
    if err != nil {
        return "", errors.Wrap(err, "reading file")
    }
    return string(data), nil
}
```

Same pattern for `NewWriteFileTool` (using `afero.WriteFile`) and `NewListDirectoryTool` (using `afero.ReadDir`).

### Caller Updates

All callers that construct agent tools must pass the `afero.Fs` instance:

| File | Change |
|------|--------|
| `internal/generator/verifier/agent.go` | Pass `props.FS` to tool constructors |
| Any other agent tool registration sites | Same |

### Gitignore Template

In `pkg/setup/init.go`, after writing config files:

```go
func (i *initialiser) writeGitignore(configDir string) error {
    gitignorePath := filepath.Join(configDir, ".gitignore")

    // Don't overwrite existing .gitignore
    exists, err := afero.Exists(i.fs, gitignorePath)
    if err != nil {
        return errors.Wrap(err, "checking .gitignore existence")
    }
    if exists {
        return nil
    }

    content := "# Ignore files that may contain secrets\n*.env\n*.secret\n*.key\n"
    return afero.WriteFile(i.fs, gitignorePath, []byte(content), 0644)
}
```

### API Key Detection Warning

```go
func warnIfAPIKeysInGitRepo(logger *slog.Logger, fs afero.Fs, configDir string) {
    // Check if we're in a git repo
    _, err := fs.Stat(filepath.Join(filepath.Dir(configDir), ".git"))
    if err != nil {
        return // not a git repo, no warning needed
    }

    // Scan config files for common API key patterns
    patterns := []string{"sk-", "api_key", "token", "secret"}
    _ = afero.Walk(fs, configDir, func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return nil
        }
        data, readErr := afero.ReadFile(fs, path)
        if readErr != nil {
            return nil
        }
        content := string(data)
        for _, pattern := range patterns {
            if strings.Contains(content, pattern) {
                logger.Warn("config file may contain API keys — ensure it is gitignored",
                    "file", path)
                return filepath.SkipDir
            }
        }
        return nil
    })
}
```

---

## Project Structure

```
internal/agent/
├── tools.go           ← MODIFIED: afero migration, symlink fix
├── tools_test.go      ← MODIFIED: test with afero.MemMapFs
internal/generator/verifier/
├── agent.go           ← MODIFIED: pass afero.Fs to tool constructors
pkg/setup/
├── init.go            ← MODIFIED: gitignore template, API key warning
├── init_test.go       ← MODIFIED: test gitignore and warning
```

---

## Testing Strategy

| Test | Scenario |
|------|----------|
| `TestIsPathAllowed_SymlinkBypass` | Create symlink inside base pointing outside → rejected |
| `TestIsPathAllowed_SymlinkWithinBase` | Symlink resolving to path inside base → allowed |
| `TestIsPathAllowed_NonexistentTarget` | Write to new file in allowed dir → parent resolved |
| `TestReadFileTool_UsesAfero` | Read from `afero.MemMapFs` → returns content |
| `TestWriteFileTool_UsesAfero` | Write to `afero.MemMapFs` → file written |
| `TestListDirectoryTool_UsesAfero` | List from `afero.MemMapFs` → entries returned |
| `TestWriteGitignore_NewDir` | No existing .gitignore → created |
| `TestWriteGitignore_ExistingPreserved` | Existing .gitignore → not overwritten |
| `TestWarnAPIKeys_InGitRepo` | Config with "sk-" in git repo → warning logged |
| `TestWarnAPIKeys_NotGitRepo` | Config with "sk-" outside git repo → no warning |

### Symlink Test Setup

```go
func TestIsPathAllowed_SymlinkBypass(t *testing.T) {
    // This test uses the real filesystem since symlinks need OS support
    baseDir := t.TempDir()
    outsideDir := t.TempDir()
    secretFile := filepath.Join(outsideDir, "secret.txt")
    os.WriteFile(secretFile, []byte("secret"), 0644)

    // Create symlink inside base pointing outside
    symlink := filepath.Join(baseDir, "escape")
    os.Symlink(outsideDir, symlink)

    _, err := isPathAllowed(afero.NewOsFs(), baseDir, filepath.Join(symlink, "secret.txt"))
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "outside allowed base")
}
```

### Coverage
- Target: 90%+ for `internal/agent/` and `pkg/setup/`.

---

## Linting

- `golangci-lint run --fix` must pass.
- No new `nolint` directives.
- The afero migration removes direct `os` package usage from agent tools, resolving any `forbidigo` warnings if configured.

---

## Documentation

- Godoc for updated `isPathAllowed` explaining symlink resolution.
- Godoc for updated tool constructors noting the `afero.Fs` parameter.
- Internal documentation for the gitignore template behaviour.
- No user-facing documentation changes.

---

## Backwards Compatibility

- **Agent tool constructor signatures change**: Internal API — not expected to be used externally. Low risk.
- **`isPathAllowed` signature change**: Internal function. No external impact.
- **Gitignore generation**: Only for new `init` runs. Does not modify existing directories.

---

## Future Considerations

- **Content-based API key detection**: Use regex patterns for known API key formats (OpenAI `sk-...`, Anthropic `sk-ant-...`, etc.) for more precise detection.
- **Git hooks**: Could add a pre-commit hook that prevents committing files matching sensitive patterns.
- **Sandboxed agent execution**: For higher-security deployments, agent tools could run in a restricted namespace.

---

## Implementation Phases

### Phase 1 — Symlink Fix
1. Update `isPathAllowed` to use `filepath.EvalSymlinks`
2. Add symlink bypass tests

### Phase 2 — Afero Migration
1. Update tool struct types to include `afero.Fs`
2. Update constructors to accept `afero.Fs`
3. Replace `os.ReadFile`/`os.WriteFile`/`os.ReadDir` with afero equivalents
4. Update callers to pass `props.FS`
5. Add afero-based tests

### Phase 3 — API Key Protection
1. Add `.gitignore` template to `init` command
2. Add API key detection warning
3. Add tests for both features

---

## Verification

```bash
go build ./...
go test -race ./internal/agent/... ./pkg/setup/...
go test ./...
golangci-lint run --fix

# Verify no os.ReadFile/WriteFile/ReadDir in agent tools
grep -n 'os\.\(ReadFile\|WriteFile\|ReadDir\)' internal/agent/tools.go  # should return nothing

# Verify symlink resolution is present
grep -n 'EvalSymlinks' internal/agent/tools.go  # should return matches
```
