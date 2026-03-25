---
title: VCS
description: Version control subpackages — git operations, GitHub API, GitLab API, and release management.
date: 2026-03-25
tags: [components, vcs, git, github, gitlab, release]
authors: [Matt Cockayne <matt@phpboyscout.com>]
---

# VCS

The `pkg/vcs/` directory is split into focused subpackages. Each has a distinct responsibility and can be used independently.

## Package Overview

| Package | Import path | Purpose |
|---------|-------------|---------|
| **[Release](release.md)** | `pkg/vcs/release` | Backend-agnostic `Provider`, `Release`, and `ReleaseAsset` interfaces |
| **[Repo](repo.md)** | `pkg/vcs/repo` | Git repository operations (local and in-memory) via `go-git` |
| **[GitHub](github.md)** | `pkg/vcs/github` | GitHub Enterprise API client and GitHub release provider |
| **[GitLab](gitlab.md)** | `pkg/vcs/gitlab` | GitLab release provider |

The root `pkg/vcs` package contains only `auth.go` — the shared `ResolveToken` helper used by both the GitHub and GitLab subpackages.

## Authentication

`vcs.ResolveToken(cfg config.Containable, fallbackEnv string) string` resolves a token from a config subtree in this order:

1. `auth.env` — reads the named environment variable
2. `auth.value` — uses the literal value stored in config
3. `fallbackEnv` — falls back to a well-known environment variable (e.g. `"GITHUB_TOKEN"`)

Returns an empty string when nothing is found. Callers decide whether that is an error — public repositories can operate without a token; private repositories will receive a 401.

```go
import "github.com/phpboyscout/go-tool-base/pkg/vcs"

// Resolve a GitHub token from props.Config.Sub("github")
token := vcs.ResolveToken(props.Config.Sub("github"), "GITHUB_TOKEN")
```

## Design Goals

**Interface segregation**
: `RepoLike` (repo operations) and `GitHubClient` (API operations) are separate interfaces. Most features only need one of them.

**Backend agnosticism for releases**
: Both GitHub and GitLab providers implement `release.Provider`. Consuming code (e.g. the auto-update command) depends only on that interface.

**Testability**
: All public interfaces have generated mocks in `mocks/pkg/vcs/`. In-memory git storage (`SourceMemory`) enables offline integration-style tests.

**`afero` integration**
: `Repo.AddToFS` bridges `go-git` object storage into any `afero.Fs`, so file operations are consistent between production (OS filesystem) and tests (memory-mapped filesystem).

## Related Documentation

- **[VCS Concepts](../../concepts/vcs-repositories.md)** — architectural rationale and usage patterns
- **[Auto-Update Lifecycle](../../concepts/auto-update.md)** — how `release.Provider` is used for version checks
- **[Interface Design](../../concepts/interface-design.md)** — `RepoLike` and `GitHubClient` in the interface hierarchy
