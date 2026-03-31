# oct pkg

## Overview

`oct pkg` manages package fetch, cache inspection, and dependency sync.
Package metadata is declared in `manifest.oct`.
Dependency sync is explicit and command-driven.

## Rules

- Manifest metadata is declared by `fn Manifest() -> PackageManifest` in `manifest.oct`.
- `PackageManifest` includes package identity fields and `Dependencies`.
- `Dependencies` is a `Dependency[]` describing direct dependencies.
- Current model is direct-dependency fetch/sync from declared sources.
- `oct pkg get <git-url>` fetches one source into cache (or reports cache hit).
- `oct pkg get` reports source, cache path, cache key, dependency count, and manifest identity when present.
- `oct pkg list` shows cached package entries.
- `oct pkg sync` reads the current project's manifest and syncs direct dependencies.
- `oct pkg sync` operates on the current directory as the project root.
- Sync output includes project path, manifest path, dependency count, per-dependency status, and completion line.
- Usage is strict: `oct pkg <get|list|sync>`.
- Present limitations: package operations are manifest-driven and direct-dependency oriented; no additional package-manager surfaces are defined here.

See also [13 Packages](../language/13-packages.md) for language-level `package` / `import` rules.

## Examples

Valid:

```text
oct pkg get https://example.com/repo.git
oct pkg list
oct pkg sync
```

Invalid:

```text
oct pkg sync extra-arg
```
