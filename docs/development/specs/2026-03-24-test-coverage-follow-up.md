---
title: "Test Coverage Follow-Up Specification"
description: "Address remaining test coverage gaps in pkg/cmd/update, pkg/chat, pkg/vcs/repo, pkg/vcs/github, internal/cmd/generate, and pkg/docs to meet project coverage targets."
date: 2026-03-24
status: DRAFT
tags:
  - specification
  - testing
  - coverage
  - quality
author:
  - name: Matt Cockayne
    email: matt@phpboyscout.com
  - name: Claude (claude-opus-4-6)
    role: AI drafting assistant
---

# Test Coverage Follow-Up Specification

Authors
:   Matt Cockayne, Claude (claude-opus-4-6) *(AI drafting assistant)*

Date
:   24 March 2026

Status
:   DRAFT

---

## Overview

The initial test coverage gaps specification (2026-03-21, IMPLEMENTED) addressed several `pkg/` packages that were below target. A subsequent review has identified six additional packages that remain below their coverage thresholds. This spec defines a prioritised plan to close those remaining gaps.

### Remaining Coverage Gaps

| Package | Current Coverage | Target | Priority |
|---------|-----------------|--------|----------|
| `pkg/cmd/update` | 7.4% | 90% | High -- self-update is critical path |
| `pkg/chat` | 26.6% | 90% | High -- AI integration is core feature |
| `pkg/vcs/repo` | 13.5% | 90% | High -- VCS abstraction for updates |
| `pkg/vcs/github` | 36.8% | 90% | Medium -- GitHub API operations |
| `internal/cmd/generate` | 7.4% | 60%+ | Medium -- CLI generation entry point |
| `pkg/docs` | 15.1% | 60%+ | Medium -- TUI docs browser |

---

## Design Decisions

**Black-box testing (package_test)**: All new tests use external test packages (`package_test`) to validate the public surface only. This ensures tests remain resilient to internal refactoring.

**Table-driven tests with `t.Parallel()`**: Every test function runs its subtests in parallel where safe, improving test suite speed and catching race conditions early.

**Afero `NewMemMapFs()` for filesystem**: All filesystem interactions are tested against an in-memory filesystem. No test touches the real filesystem except where OS-level operations (e.g., symlinks, exec) require it.

**Mockery-generated mocks for interfaces**: All interface dependencies are mocked using mockery/v3-generated mocks. Hand-rolled mocks are not permitted for interfaces that already have generated counterparts.

**`t.Cleanup()` over `defer`**: Resource cleanup uses `t.Cleanup()` to ensure cleanup runs even when subtests fail, and to keep cleanup registration close to resource creation.

**Context propagation testing**: Every function accepting a `context.Context` is tested with cancelled, deadline-exceeded, and nil-parent contexts to verify proper propagation.

---

## Public API Changes

None. This spec adds tests only.

---

## Internal Implementation

### Priority 1: `pkg/cmd/update` (7.4% to 90%)

The self-update command is a critical user-facing path. Tests must cover:

- **Version comparison logic**: Current version vs latest release, already-up-to-date, downgrade prevention.
- **Binary replacement flow**: Mock the download, checksum verification, and file replacement steps using afero.
- **Error paths**: Network failure during release check, missing permissions for binary replacement, corrupt download, checksum mismatch.
- **Flag handling**: `--force`, `--version`, dry-run behaviour.

```go
func TestUpdateCmd_AlreadyUpToDate(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name           string
        currentVersion string
        latestVersion  string
        expectUpdate   bool
    }{
        {"same version", "1.2.3", "1.2.3", false},
        {"newer available", "1.2.3", "1.3.0", true},
        {"dev version", "dev", "1.0.0", true},
        {"ahead of latest", "2.0.0", "1.9.9", false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            // ... mock release provider, assert update/no-update
        })
    }
}
```

### Priority 2: `pkg/chat` (26.6% to 90%)

The chat package provides AI provider integrations. The prior spec covered some provider paths; this follow-up targets the remaining uncovered surface:

- **Provider initialisation errors**: Missing API keys, invalid base URLs, unsupported model names.
- **Response parsing edge cases**: Empty responses, malformed JSON, partial streaming chunks, unexpected stop reasons.
- **Conversation context management**: Multi-turn history, token limit handling, system prompt injection.
- **Timeout and retry behaviour**: Provider-specific timeout handling, context cancellation mid-stream.
- **Error wrapping**: All errors must be wrapped with `cockroachdb/errors` and carry appropriate sentinel types.

```go
func TestClaudeProvider_Ask_MissingAPIKey(t *testing.T) {
    t.Parallel()
    provider, err := NewClaudeProvider(Config{APIKey: ""})
    assert.Error(t, err)
    // Verify error is a configuration error, not a runtime error
}

func TestClaudeProvider_Ask_MalformedResponse(t *testing.T) {
    t.Parallel()
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"content": "not-an-array"}`))
    }))
    t.Cleanup(server.Close)
    // ... assert appropriate error
}
```

### Priority 3: `pkg/vcs/repo` (13.5% to 90%)

The repo package provides VCS abstraction for repository operations:

- **Clone/pull operations**: Mock git commands, test success and failure paths.
- **Branch management**: Create, switch, delete branches with mock VCS backend.
- **Tag operations**: List, create, verify tags.
- **Remote management**: Add, remove, validate remotes.
- **Working directory state**: Clean, dirty, untracked files detection.

```go
func TestRepo_DetectDirtyState(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name     string
        status   string
        expected bool
    }{
        {"clean repo", "", false},
        {"modified files", " M file.go", true},
        {"untracked files", "?? new.go", true},
        {"staged changes", "A  added.go", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            // ... mock git status output
        })
    }
}
```

### Priority 4: `pkg/vcs/github` (36.8% to 90%)

GitHub API operations need thorough mock HTTP server testing:

- **Release operations**: Create release, upload asset, download asset, list releases.
- **Rate limiting**: 403 responses with `Retry-After` headers, secondary rate limits.
- **Pagination**: Multi-page list responses with Link headers.
- **Authentication**: Token validation, missing token, expired token.
- **Asset download**: Redirect following, content-type validation, partial download recovery.

```go
func TestGitHubClient_DownloadReleaseAsset_RedirectChain(t *testing.T) {
    t.Parallel()
    redirectCount := 0
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if redirectCount < 2 {
            redirectCount++
            http.Redirect(w, r, "/next", http.StatusFound)
            return
        }
        w.Write([]byte("asset-content"))
    }))
    t.Cleanup(server.Close)
    // ... verify asset downloaded after redirects
}

func TestGitHubClient_RateLimited(t *testing.T) {
    t.Parallel()
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Retry-After", "1")
        w.WriteHeader(http.StatusForbidden)
    }))
    t.Cleanup(server.Close)
    // ... verify rate limit error is returned with appropriate type
}
```

### Priority 5: `internal/cmd/generate` (7.4% to 60%+)

The CLI generation entry point orchestrates code generation:

- **Manifest parsing**: Valid manifest, missing fields, malformed YAML.
- **Template execution**: Successful generation, template errors, missing template files.
- **Dry-run mode**: Verify output is printed but no files are written.
- **Debug logging**: Verify debug output when enabled.
- **File conflict handling**: Existing files, permission errors.

```go
func TestGenerateCmd_DryRun(t *testing.T) {
    t.Parallel()
    fs := afero.NewMemMapFs()
    // ... set up manifest and templates in memory
    // Execute with dry-run flag
    // Assert no files written to fs
    // Assert expected output contains generated content
}
```

### Priority 6: `pkg/docs` (15.1% to 60%+)

The TUI docs browser needs broader coverage:

- **MkDocs config parsing**: Valid config, missing nav, empty nav, deeply nested sections.
- **Markdown rendering**: Headings, code blocks, tables, links, images.
- **Navigation state**: Forward/back, breadcrumb generation, section expansion.
- **Search functionality**: Keyword matching, no results, special characters.
- **Filesystem edge cases**: Missing docs directory, unreadable files, broken symlinks.

```go
func TestDocsBrowser_MissingDocsDir(t *testing.T) {
    t.Parallel()
    fs := afero.NewMemMapFs()
    // Do not create docs directory
    browser, err := NewBrowser(fs, "/project")
    assert.Error(t, err)
}

func TestDocsBrowser_DeeplyNestedNav(t *testing.T) {
    t.Parallel()
    fs := afero.NewMemMapFs()
    mkdocsYaml := `nav:
  - Level1:
    - Level2:
      - Level3:
        - Level4: deep/page.md`
    afero.WriteFile(fs, "/project/mkdocs.yml", []byte(mkdocsYaml), 0644)
    // ... verify four-level nesting is parsed correctly
}
```

---

## Project Structure

```
pkg/cmd/update/
├── update_test.go          <- MODIFIED: comprehensive table-driven tests
pkg/chat/
├── claude_test.go          <- MODIFIED: edge cases, error paths
├── openai_test.go          <- MODIFIED: edge cases, error paths
├── gemini_test.go          <- MODIFIED: edge cases, error paths
├── testhelpers_test.go     <- MODIFIED: additional shared helpers
pkg/vcs/repo/
├── repo_test.go            <- NEW/MODIFIED: mock VCS backend tests
pkg/vcs/github/
├── client_test.go          <- MODIFIED: rate limiting, pagination, auth tests
├── release_test.go         <- MODIFIED: asset download, redirect tests
internal/cmd/generate/
├── generate_test.go        <- MODIFIED: manifest parsing, dry-run, debug tests
pkg/docs/
├── docs_test.go            <- MODIFIED: nav parsing, search, rendering tests
```

---

## Testing Strategy

### Test Categories

| Category | Packages | Approach |
|----------|----------|----------|
| CLI commands | `pkg/cmd/update` | Mock release provider, afero for binary replacement |
| AI providers | `pkg/chat` | `httptest.Server` with provider-specific responses |
| VCS abstraction | `pkg/vcs/repo` | Mock git command execution |
| HTTP API client | `pkg/vcs/github` | `httptest.Server` with GitHub API responses |
| Code generation | `internal/cmd/generate` | Afero for filesystem, mock templates |
| TUI browser | `pkg/docs` | Afero for filesystem, mock MkDocs config |

### Testing Patterns

All tests must follow these patterns:

1. **Table-driven**: Each test function defines a `tests` slice of structs with named fields.
2. **Parallel execution**: `t.Parallel()` at both the test function and subtest level.
3. **Black-box**: Use `package_test` naming to test only the exported API.
4. **Cleanup**: `t.Cleanup()` for HTTP servers, temporary resources, and mock teardown.
5. **Context testing**: Every context-accepting function tested with cancelled and timed-out contexts.
6. **Error assertions**: Use `errors.Is` and `errors.As` from `cockroachdb/errors` for error type checking.

### Coverage Targets

| Package | Current | Target |
|---------|---------|--------|
| `pkg/cmd/update` | 7.4% | 90% |
| `pkg/chat` | 26.6% | 90% |
| `pkg/vcs/repo` | 13.5% | 90% |
| `pkg/vcs/github` | 36.8% | 90% |
| `internal/cmd/generate` | 7.4% | 60%+ |
| `pkg/docs` | 15.1% | 60%+ |

---

## Backwards Compatibility

No breaking changes. This spec adds tests only.

---

## Future Considerations

- **Coverage CI gate**: Enforce per-package coverage thresholds in CI, failing the build if any package drops below its target.
- **Fuzz testing**: `pkg/vcs/github` response parsing and `pkg/docs` YAML parsing are strong candidates for Go's native fuzz testing.
- **Integration test tag**: A `//go:build integration` tag for tests that exercise real provider APIs, run only in CI with appropriate secrets.
- **Mutation testing**: Once coverage targets are met, mutation testing can identify tests that pass without meaningful assertions.
- **Benchmark suite**: `pkg/chat` response parsing and `pkg/docs` navigation rendering may benefit from benchmarks as the project scales.

---

## Implementation Phases

### Phase 1 -- Critical Path (pkg/cmd/update)
1. Mock the release provider interface and binary replacement filesystem operations
2. Add table-driven tests for version comparison, update flow, and error paths
3. Test flag handling (`--force`, `--version`)
4. Achieve 90%+ coverage

### Phase 2 -- Core Feature (pkg/chat)
1. Extend shared test helpers in `testhelpers_test.go`
2. Add provider initialisation error tests for all three providers
3. Add response parsing edge case tests (malformed JSON, empty responses, partial chunks)
4. Add context propagation and timeout tests
5. Achieve 90%+ coverage

### Phase 3 -- VCS Packages (pkg/vcs/repo, pkg/vcs/github)
1. Create mock git command executor for `pkg/vcs/repo`
2. Add tests for clone, pull, branch, tag, and remote operations
3. Add rate limiting, pagination, and auth tests for `pkg/vcs/github`
4. Add asset download redirect and error path tests
5. Achieve 90%+ coverage for both

### Phase 4 -- Generation and Docs (internal/cmd/generate, pkg/docs)
1. Add manifest parsing and template execution tests for `internal/cmd/generate`
2. Add dry-run and debug logging verification tests
3. Extend `pkg/docs` tests for nav parsing edge cases and search
4. Achieve 60%+ coverage for both

---

## Verification

```bash
# Full test suite with race detector
just test

# Per-package coverage checks
go test -coverprofile=coverage.out ./pkg/cmd/update/...
go tool cover -func=coverage.out

go test -coverprofile=coverage.out ./pkg/chat/...
go tool cover -func=coverage.out

go test -coverprofile=coverage.out ./pkg/vcs/repo/...
go tool cover -func=coverage.out

go test -coverprofile=coverage.out ./pkg/vcs/github/...
go tool cover -func=coverage.out

go test -coverprofile=coverage.out ./internal/cmd/generate/...
go tool cover -func=coverage.out

go test -coverprofile=coverage.out ./pkg/docs/...
go tool cover -func=coverage.out

# Overall coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1

# Lint
golangci-lint run --fix
```
