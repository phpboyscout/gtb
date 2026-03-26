---
title: Migration Guides
description: Step-by-step guides for upgrading between GTB versions.
---

# Migration Guides

Each guide covers the breaking changes introduced in a specific release and
provides before/after code examples with a clear migration path.

## Available guides

| From | To | Guide |
|---|---|---|
| v0.x | v1.0 | [Migrating to v1.0](v0.x-to-v1.0.md) |

## Writing a new guide

Use the `_template.md` file in this directory as a starting point:

1. Copy it to `docs/migration/vX.Y-to-vX.Z.md`.
2. Replace all placeholder text.
3. Group changes by package.
4. Include before/after code blocks and a prose migration path for each change.
5. Remove the template warning admonition at the top.
