# PM1 package registry MVP design

## 1. Executive summary

PM1 designs the smallest package registry and source-resolution flow that can be implemented in PM2 without changing Oct syntax, wrapper runtime discovery, native build lifecycle, or the package-manager architecture. The target is a source-oriented registry MVP: a project configures one or more explicit registries, adds an exact package dependency by name and version, syncs source into the existing package loading path, and then runs ordinary Oct commands.

PM1 should stay MVP-first because the current package stack is already useful for local authoring, but the next missing capability is not federation, signing, artifacts, binary distribution, or lockfiles. The missing capability is simply: **given `SignalTools@0.1.0`, find a source directory or Git checkout, validate its manifest, place it somewhere the project loader can see, and run tests**.

Recommended PM2 scope:

```text
PM2 — implement local registry/source-resolution MVP
```

PM2 should implement:

- project-local registry configuration;
- one inspectable registry index format;
- `oct pkg registry add/list/remove`;
- `oct pkg add <Name>@<exact-version>`;
- registry-backed `oct pkg sync` for direct dependencies;
- exact-version resolution only;
- local source entries only for the first implementation slice;
- project-local synced source cache at `.oct/packages/<Name>/<Version>/`;
- minimal source metadata beside each synced package;
- manifest validation that checks synced package name/version/kind against the registry entry and dependency request.

PM2 should explicitly not implement federation. A registry MVP is a deterministic source index owned by the current project. Federation is a future distribution and trust layer involving multiple authorities, mirrors, P2P discovery, signed metadata, digest policies, conflict policy, and lockfile semantics. PM2 should keep its model simple enough that a later federated registry can generate or consume the same entry shape, but PM2 should not pretend to solve federated identity or trust.

## 2. Current package manager inventory

### Package manifests

Current package manifests are ordinary `manifest.oct` files with `package Manifest`, a `PackageManifest` record, a `Dependency` record, and `fn Manifest() -> PackageManifest`. The package-manager loader requires `Name`, `Version`, `Description`, and `Dependencies`, and recognizes optional `Kind`, `EntryMilestone`, and `Wrappers` fields.

The current manifest kind vocabulary is normalized by Go implementation code rather than Oct user code. Omitted or empty `Kind` is treated as pure/library-style package metadata; explicit supported kinds include `experiment` and `wrapper`. Existing docs sometimes call pure packages “pure,” while the user-facing scaffold command calls them “library.” PM2 should surface this vocabulary mismatch rather than silently inventing a new manifest kind. Recommended registry `Kind` values for PM2 are `library`, `experiment`, and `wrapper`, where `library` maps to an omitted/normalized pure manifest kind during validation.

### Dependencies

`Dependency` currently requires:

```oct
Name: String
VersionRequirement: String
```

and optionally accepts:

```oct
Source: String
```

This is a documentation/product gap for PM2: `VersionRequirement` implies ranges may be meaningful, but current sync does not solve versions. PM2 should require exact version text in `oct pkg add` and write that exact value into `VersionRequirement` until the manifest schema is deliberately renamed or extended.

### Local package source handling and cache behavior

There is already a package-manager cache, but it is not a registry cache. `pkg get` and current `pkg sync` fetch dependencies from explicit `Dependency.Source` values. Sources must have a URL scheme; supported schemes are `https`, `http`, `ssh`, `git`, and `file`. Fetching is implemented by `git clone --depth 1`, so `file:` currently means “clone a local Git repository,” not “copy an arbitrary local directory.”

The current cache defaults to a user cache directory, overrideable with `OCT_PKG_CACHE_DIR`, and stores cloned repositories under `repos/<cache-key>` plus an `index.json`. Cache entries record source, cache key, path, name, version, dependencies, git HEAD, and a fetched timestamp. The timestamp is fine for a user cache listing, but PM2 project-local source metadata should avoid timestamps in golden tests unless a test explicitly normalizes them.

### `oct pkg wrappers`

`oct pkg wrappers` is planning-only. It reads the current package manifest and direct dependencies that already declare fetchable `Source`, reports wrapper sidecar metadata, and can write a deterministic `.octagon` registry artifact. It does not build Go modules, execute sidecars, download native modules beyond dependency source fetches that are already explicit, change runtime discovery, or install wrappers.

### `oct pkg build-wrappers`

`oct pkg build-wrappers --allow-native` builds current-package wrapper sidecars only. It derives targets from the current package manifest, validates package-local `GoModuleDir`, runs `go build -o <project>/.oct/wrappers/<platform>/<sidecar> .` from that module directory, and never executes the resulting sidecar. It does not build dependency sidecars.

### `oct new library/experiment/wrapper-library`

`oct new` currently scaffolds deterministic package directories:

```sh
oct new library <Name>
oct new experiment <Name>
oct new wrapper-library <Name>
```

Names are strict PascalCase with a reserved-name list. The scaffold command creates `./<Name>` and fails if the target exists. Library manifests omit `Kind`, experiment manifests set `Kind: "experiment"` and `EntryMilestone: "M0"`, and wrapper-library manifests set `Kind: "wrapper"` with manifest-declared wrapper metadata and a package-local Go sidecar scaffold.

### Package loading in interpreted/compiled/test paths

Project loading resolves package imports from the active root, experiment family root where applicable, and repository-level `Libraries`/`Packages` directories. In manifest mode, imported package names must be declared in the importing package manifest. If the package is not found in local search roots, the loader falls back to cached dependency paths by reading the package-manager cache index by package name.

Current package loading therefore already has a path PM2 can reuse, but it is user-cache oriented and name-only for cached dependencies. PM2 should make synced packages visible deterministically to the project loader, preferably by extending the loader to include `.oct/packages` before or instead of falling back to the user cache for registry-synced dependencies.

### Existing `Source` metadata behavior

`Dependency.Source` is the only current source-resolution mechanism. `oct pkg sync` requires every dependency to have non-empty `Source`, calls `Get(Source)`, and fails if source metadata is missing. `oct pkg wrappers` is more lenient and skips dependencies without `Source` so wrapper metadata for the current package remains inspectable.

### Current limitations

The current package manager does **not** have:

- a registry index;
- `oct registry` or `oct pkg registry` configuration;
- name/version source resolution;
- `oct pkg add`;
- registry-backed `oct pkg sync`;
- local arbitrary directory copy/symlink source acquisition;
- project-local package cache semantics;
- lockfiles;
- content-addressed `.octpkg` artifacts;
- registry signing;
- binary sidecar distribution;
- semver solving;
- publish/auth/namespace workflows.

## 3. MVP product goal

MVP user story:

```text
A package author can publish package source in a Git repo or local directory and create a registry entry.
A consumer can configure a registry, add a dependency by name/version, sync it, and use/import it.
A wrapper package can be acquired the same way, then built locally with the existing W8b wrapper build command when it is the current package.
```

Library golden path:

```sh
oct new library SignalTools
# author records SignalTools in a local registry index

# consumer project
oct pkg registry add local ../oct-registry
oct pkg add SignalTools@0.1.0
oct pkg sync
oct test .
```

Wrapper source acquisition golden path:

```sh
oct pkg registry add local ../oct-registry
oct pkg add EchoWrapper@0.1.0
oct pkg sync
```

Current-package wrapper build golden path remains:

```sh
oct pkg build-wrappers --allow-native
OCT_WRAPPER_PATH=.oct/wrappers/<platform> oct test .
```

The MVP should prove this chain:

```text
registry index
→ dependency declaration
→ source fetch/copy
→ manifest validation
→ package cache/source layout
→ project loading
→ test/run
```

The MVP should not prove network registry hosting, federation, P2P, binary trust, package publishing, or dependency solving.

## 4. Registry model options

### A. Local directory registry with one file per package

```text
registry/
  packages/
    SignalTools.octreg
    EchoWrapper.octreg
```

Pros:

- package entries are easy to review independently;
- duplicate package file names can be obvious;
- future large registries can avoid rewriting one large file.

Cons:

- PM2 needs directory traversal, file ordering, duplicate handling, and error aggregation;
- file extension and per-file grammar are new conventions;
- simple tests require more fixture files.

### B. Single index file

```text
registry.oct
```

Pros:

- smallest loader surface;
- deterministic one-file fixtures;
- easy to diff and commit;
- easy for a Git checkout or local directory to host;
- duplicate detection can happen while scanning one literal array.

Cons:

- editing one file can become noisy later;
- very large registries will eventually want sharding;
- PM2 needs a new manifest-like parser for a record that is not `PackageManifest`.

### C. Git repository registry

```text
oct-registry.git
  registry.oct
```

Pros:

- no HTTP service needed;
- registry history/review comes from Git;
- the same `registry.oct` shape can be hosted later by HTTP.

Cons:

- PM2 should not require registry cloning before it can prove local source resolution;
- auth/submodules/network policy are out of scope;
- mutable branches can mislead users without a lockfile.

### D. HTTP registry

```text
https://registry.example/index.oct
```

Pros:

- aligns with common package manager mental models;
- central hosting is easy to explain.

Cons:

- requires network policy, caching, retries, server format, and security decisions too early;
- introduces dependency-confusion and trust concerns before lockfiles/signing exist;
- not needed for the local author/consumer golden path.

### PM2 recommendation: exact M0 shape

Use a **project-configured local directory registry** containing one **single index file**:

```text
<registry-root>/registry.oct
```

This covers both local registries and Git-backed registries checked out locally:

```sh
git clone https://example.invalid/oct-registry.git ../oct-registry
oct pkg registry add local ../oct-registry
```

PM2 should not require an HTTP server and should not clone registry repositories itself. A Git-backed registry is just a local checkout in PM2.

## 5. Registry entry schema

Recommended PM2 schema:

```oct
record Registry {
    Packages: PackageEntry[]
}

record PackageEntry {
    Name: String
    Version: String
    Kind: String
    SourceKind: String
    Source: String
    Path: String
    Description: String
}

fn Registry() -> Registry {
    return Registry {
        Packages: [
            PackageEntry {
                Name: "SignalTools"
                Version: "0.1.0"
                Kind: "library"
                SourceKind: "local"
                Source: "../SignalTools"
                Path: "."
                Description: "Signal helpers"
            }
        ]
    }
}
```

Recommended required fields:

- `Name`: exact package name, case-sensitive, same spelling as package manifest `Name`;
- `Version`: exact version string, same as package manifest `Version`;
- `Kind`: `library`, `experiment`, or `wrapper`;
- `SourceKind`: `local` in PM2;
- `Source`: path interpreted relative to the registry root for `local` entries unless absolute;
- `Path`: package root within `Source`; use `"."` for the common case;
- `Description`: human-readable summary, also useful in `oct pkg add` output later.

Deferred fields:

- `Ref`: defer until Git source support;
- `Digest`: defer until lockfile/content-addressed artifact policy;
- `License`: optional later;
- `WrapperCount`: derive from package manifest when needed;
- `Dependencies`: manifest remains the source of dependency truth;
- `OctVersion`: useful later, but not needed for source acquisition M0;
- binary platform metadata: explicitly deferred.

## 6. Source model

Source-kind options:

- `local`: copy package source from a local path into the project-local package cache;
- `git`: clone/fetch a Git repository and checkout `Ref`;
- future `artifact`: download or read content-addressed `.octpkg` archives.

PM2 should support **local only** first.

Rationale:

- the current `pkgmgr.Get` path already supports Git URL cloning, but it does not checkout a requested ref and does not copy arbitrary local directories;
- the requested golden path starts with a local registry checkout and can be proven without network;
- local copy support is the fastest way to validate registry index → sync → project loading;
- adding Git without a lockfile/ref/digest policy invites mutable-source confusion.

PM3 can add `SourceKind: "git"` with narrow rules:

- `git clone` only;
- explicit `Ref` required;
- checkout `Ref`;
- no auth;
- no submodules;
- no publish;
- no shallow clone unless it is easy and does not complicate ref checkout;
- no network activity except during explicit `oct pkg sync`.

For PM2 local source acquisition, use copy rather than symlink. Copying gives deterministic project-local inputs, avoids accidental edits through symlinks, and works on Windows without requiring symlink privileges. PM2 should skip build outputs and VCS internals at minimum: `.git`, `.oct/wrappers`, and existing `.oct/packages` inside the source package.

## 7. Registry configuration

Recommended command surface:

```sh
oct pkg registry add <name> <path>
oct pkg registry list
oct pkg registry remove <name>
```

Prefer `oct pkg registry` over a new top-level `oct registry` in PM2. Registry management is package-manager behavior, and the existing top-level command list is already broad. A top-level alias can be added later if the registry becomes a major user-facing area.

Recommended config location:

```text
.oct/registries.oct
```

inside the current project root.

Recommended PM2 format:

```oct
record RegistryConfig {
    Registries: RegistrySource[]
}

record RegistrySource {
    Name: String
    Path: String
}

fn Registries() -> RegistryConfig {
    return RegistryConfig {
        Registries: [
            RegistrySource { Name: "local" Path: "../oct-registry" }
        ]
    }
}
```

Reasons to keep registry config project-local:

- deterministic tests;
- no user-global state or hidden machine dependency;
- explicit registry order in version control if the project chooses to commit it;
- avoids dependency-confusion surprises from a user-global default registry.

PM2 should create `.oct/` if needed. It should not add global config unless the repository already has user config infrastructure in a future milestone.

## 8. Dependency add flow

Recommended command:

```sh
oct pkg add SignalTools@0.1.0
```

Optional disambiguation flag:

```sh
oct pkg add SignalTools@0.1.0 --registry local
```

PM2 behavior:

1. Parse `<Name>@<Version>`; require both name and exact version.
2. Load project-local `.oct/registries.oct`.
3. Resolve a matching registry entry by name/version.
4. Validate the registry entry shape and source kind.
5. Edit the current package `manifest.oct` dependency list.
6. Do **not** sync automatically.

Recommended manifest edit for PM2:

```oct
Dependency { Name: "SignalTools" VersionRequirement: "0.1.0" }
```

Do not write `Source` into the dependency for registry-added dependencies. The registry should be the acquisition source in PM2, and project-local sync metadata should record the resolved source. This avoids turning the manifest into a lockfile substitute and prevents local absolute paths from leaking into package manifests.

Compatibility note: current `oct pkg sync` requires `Dependency.Source`. PM2 must relax that rule by resolving dependencies without `Source` through configured registries. Dependencies that still have explicit `Source` can continue to use the legacy source path during migration.

If manifest editing is too risky for the first implementation slice, PM2 may start with a conservative text edit that supports the canonical scaffolded manifest shape and errors on unsupported manifest formatting. That limitation must be explicit in the command error. Do not implement a brittle formatter-wide rewrite.

## 9. Sync flow

Recommended PM2 `oct pkg sync` behavior with registry support:

1. Read the current project manifest.
2. For each dependency:
   - if `Source` is present, preserve current legacy behavior for that dependency;
   - otherwise resolve `Name` + exact `VersionRequirement` through configured registries.
3. For each registry-resolved dependency:
   - read the registry entry;
   - acquire source into `.oct/packages/<Name>/<Version>/`;
   - validate the acquired package has `manifest.oct`;
   - load manifest metadata;
   - verify manifest `Name` equals entry/request name;
   - verify manifest `Version` equals entry/request version;
   - verify manifest kind is compatible with registry `Kind`;
   - write minimal source metadata.
4. Make synced packages visible to project loading.
5. Print deterministic sync output.

Recommended cache path:

```text
.oct/packages/<Name>/<Version>/
```

Use project-local cache only for registry-resolved dependencies. Do not use a global cache in PM2. The existing user cache can remain for explicit `Source` dependencies until a future migration unifies behavior.

Update handling:

- exact name/version path is replaced atomically on `oct pkg sync` if the registry source changed;
- implementation can copy into a temporary sibling then rename;
- no incremental update logic in PM2;
- no timestamp-based skip logic in PM2 tests.

Conflicts:

- two direct dependencies with the same `Name` and different versions should be a PM2 error;
- two direct dependencies with identical `Name` and version should be a duplicate error;
- no conflict solver.

Transitive dependencies:

- PM2 should support direct dependencies only unless extending the existing sync loop recursively is clearly small and deterministic;
- if a synced dependency manifest has dependencies without source metadata, PM2 should report that transitive registry resolution is deferred rather than silently ignoring them;
- PM3 can add recursive resolution once direct dependency layout is proven.

## 10. Version semantics

PM2 should support exact versions only:

```text
SignalTools@0.1.0
```

No ranges, no `latest`, no prerelease interpretation, no semver sorting, no solver, and no registry-side default version.

Validation rules:

- `oct pkg add SignalTools` fails with usage text;
- `oct pkg add SignalTools@latest` fails because `latest` is not an exact version;
- `oct pkg add SignalTools@^0.1.0` fails;
- manifest dependencies added by registry use exact `VersionRequirement` text.

The existing manifest field name `VersionRequirement` remains a schema wart. PM2 should document that only exact values are accepted for registry resolution.

## 11. Registry resolution order and safety

Names should be case-sensitive and should follow the existing strict PascalCase package-name convention used by `oct new`. Registry loading should reject names that would not pass the scaffold/package naming policy unless a legacy compatibility exception is explicitly added later.

Resolution policy:

- registry order is the order stored in `.oct/registries.oct`;
- duplicate `Name` + `Version` entries inside one registry are errors;
- duplicate `Name` + `Version` across registries are errors by default;
- `--registry <name>` narrows lookup to one registry and avoids cross-registry ambiguity;
- missing package and missing version errors should list configured registry names searched, not network guesses.

This chooses correctness over first-wins simplicity. First-wins would be easier, but it makes dependency confusion a default behavior before lockfiles/digests exist. PM2 can still be simple: scan all configured registries, collect exact matches, error if count is zero or greater than one.

Security statement for PM2:

- PM2 does not solve dependency confusion;
- PM2 makes registry order/config project-local and ambiguity visible;
- future `oct.lock` plus digest policy is required for stronger source identity.

## 12. Package identity and cache metadata

Recommended PM2 metadata file:

```text
.oct/packages/<Name>/<Version>/.oct-package-source.oct
```

Recommended format:

```oct
record PackageSource {
    Name: String
    Version: String
    Registry: String
    RegistryPath: String
    SourceKind: String
    Source: String
    Path: String
}

fn PackageSource() -> PackageSource {
    return PackageSource {
        Name: "SignalTools"
        Version: "0.1.0"
        Registry: "local"
        RegistryPath: "../oct-registry"
        SourceKind: "local"
        Source: "../SignalTools"
        Path: "."
    }
}
```

Do not include fetched timestamps in PM2 metadata. Do not call this a lockfile. It is diagnostic metadata attached to a copied source tree.

Future lockfile note:

```text
oct.lock
```

should later pin resolved registry, source identity, Git ref, digest, transitive dependency graph, and possibly artifact provenance. PM2 should not create `oct.lock`.

## 13. Wrapper package integration

Registry MVP should resolve wrapper packages like any other package source. The registry entry `Kind: "wrapper"` should be validated against the package manifest `Kind: "wrapper"`, and the copied package source should retain sidecar source directories such as `sidecars/octxiliary-echo-wrapper`.

However, W8b currently builds sidecars for the **current package only**. It deliberately does not build dependency sidecars and does not add runtime lookup for project-local `.oct/wrappers`. Therefore PM2 should not promise that adding a wrapper dependency automatically makes dependency sidecars usable.

Recommended PM2 story:

- wrapper packages can be indexed, added, synced, loaded, and inspected;
- `oct pkg sync` never builds sidecars;
- `oct pkg build-wrappers --allow-native` remains current-package only;
- dependency wrapper sidecar build selection is deferred to W8c/W9 or PM3;
- registry cache layout should preserve enough source metadata for that future command to build dependency wrapper sidecars from `.oct/packages/<Name>/<Version>/`.

Practical golden path for wrapper authors remains:

```sh
cd EchoWrapper
oct pkg build-wrappers --allow-native
OCT_WRAPPER_PATH=.oct/wrappers/<platform> oct test .
```

Practical golden path for wrapper consumers in PM2 is source acquisition and inspection, not automatic dependency-sidecar execution:

```sh
oct pkg add EchoWrapper@0.1.0
oct pkg sync
oct pkg wrappers
# dependency-sidecar build/use is deferred
```

This is slightly awkward, but it preserves convergence: PM2 proves package acquisition without expanding the native trust/build surface.

## 14. CLI surface summary for PM2

Recommended commands:

```sh
oct pkg registry add <name> <path>
oct pkg registry list
oct pkg registry remove <name>
oct pkg add <name>@<version> [--registry <name>]
oct pkg sync
```

Existing commands continue:

```sh
oct pkg get <git-url>          # legacy explicit source fetch/cache
oct pkg list                   # legacy cache listing
oct pkg wrappers [--registry-out <path>]
oct pkg build-wrappers --allow-native
```

Usage boundaries:

- no top-level `oct registry` in PM2;
- no `oct pkg publish`;
- no auth;
- no registry mirror commands;
- no global registry commands;
- no implicit sync during `pkg add`.

## 15. Registry file format

Use Oct manifest-like syntax for PM2, not JSON.

Reasons:

- aligns with existing package identity files;
- gives authors a familiar record-literal shape;
- keeps fixtures readable and diffable;
- avoids introducing another user-facing data format while manifests are already Oct-shaped.

Exact PM2 registry file:

```text
<registry-root>/registry.oct
```

Exact schema:

```oct
package Registry

record RegistryIndex {
    Packages: PackageEntry[]
}

record PackageEntry {
    Name: String
    Version: String
    Kind: String
    SourceKind: String
    Source: String
    Path: String
    Description: String
}

fn Registry() -> RegistryIndex {
    return RegistryIndex {
        Packages: [
            PackageEntry {
                Name: "SignalTools"
                Version: "0.1.0"
                Kind: "library"
                SourceKind: "local"
                Source: "../SignalTools"
                Path: "."
                Description: "Signal helper functions"
            },
            PackageEntry {
                Name: "EchoWrapper"
                Version: "0.1.0"
                Kind: "wrapper"
                SourceKind: "local"
                Source: "../EchoWrapper"
                Path: "."
                Description: "Echo wrapper package"
            }
        ]
    }
}
```

The function return type is `RegistryIndex` instead of `Registry` to avoid a type/function name collision. The package is `Registry` to make parse errors clear.

PM2 parser recommendation:

- implement a small registry loader alongside `internal/pkgmgr`, analogous to manifest metadata loading;
- validate record declarations and literal values strictly;
- keep all fields string literals for M0;
- sort entries by `Name`, `Version`, then registry order only for display, not for resolution ambiguity detection.

## 16. PM2 tests

Recommended tests:

1. Create temp package source with `oct new library SignalTools`.
2. Create temp registry directory with `registry.oct` pointing to `SignalTools` via `SourceKind: "local"`.
3. Create temp consumer project.
4. Run `oct pkg registry add local <registry>`.
5. Run `oct pkg registry list` and verify deterministic output.
6. Run `oct pkg add SignalTools@0.1.0`.
7. Verify manifest dependency was added without `Source`.
8. Run `oct pkg sync`.
9. Verify `.oct/packages/SignalTools/0.1.0/manifest.oct` exists.
10. Add a consumer `.octest` importing/using `SignalTools`.
11. Run `oct test .`.
12. Verify `oct pkg registry remove local` updates config.
13. Duplicate entry in the same registry errors.
14. Duplicate exact entry across two registries errors unless `--registry` is supplied.
15. Missing package errors.
16. Missing version errors.
17. Invalid source kind errors.
18. Source path missing errors.
19. Source path traversal outside the copied package root errors when resolving `Path`.
20. Fetched package manifest name mismatch errors.
21. Fetched package manifest version mismatch errors.
22. Registry kind/manifest kind mismatch errors.
23. No network is used by local source tests.
24. No `oct.lock` is generated.
25. `oct pkg sync` does not build native wrapper sidecars.
26. Wrapper registry entries can be synced and shown by `oct pkg wrappers` without building sidecars.
27. Legacy explicit `Dependency.Source` sync behavior still works.

## 17. Future milestones

Recommended sequence:

### PM2 — implement local registry/source-resolution MVP

Scope:

- project-local `.oct/registries.oct`;
- local directory registry at `<registry-root>/registry.oct`;
- registry index loader;
- `oct pkg registry add/list/remove`;
- `oct pkg add <Name>@<Version>`;
- registry-backed `oct pkg sync`;
- exact-version direct dependencies;
- local source copy into `.oct/packages/<Name>/<Version>/`;
- package source metadata;
- project loader integration for `.oct/packages`;
- wrapper packages acquired as source, not built as dependency sidecars.

Non-goals:

- no federation/P2P;
- no HTTP registry;
- no registry Git clone;
- no package publishing;
- no auth;
- no lockfile;
- no content-addressed artifact;
- no signing;
- no binary sidecar distribution;
- no dependency solver;
- no namespace reservation;
- no wrapper build lifecycle change.

### PM3 — Git source entries and recursive direct+transitive planning

Add `SourceKind: "git"` with explicit `Ref`, checkout validation, no auth, no submodules, and deterministic error messages. Revisit transitive dependency resolution after direct dependencies are stable.

### PM4 — lockfile and digest/provenance design

Design `oct.lock`, package digests, registry entry pinning, dependency graph pinning, and migration from source metadata to lockfile-backed identity.

### PM5 — artifact format and binary/native trust design

Design `.octpkg`, content-addressed artifacts, optional binary sidecar distribution, signing, trust policy, and build-vs-download selection.

Federation/P2P should remain after lockfile/digest/trust semantics, not before them.

## 18. Risks and mitigations

| Risk | PM2 mitigation |
|---|---|
| Dependency confusion | Project-local registry config, ambiguity errors by default, no global default registry. |
| Ambiguous registry entries | Same-registry duplicates error; cross-registry duplicates error unless `--registry` narrows. |
| Missing lockfile | Explicitly label `.oct-package-source.oct` as diagnostic metadata, not a lock; defer strong pinning to PM4. |
| Mutable local sources | Copy source at sync time; require manifest name/version validation; document that local edits require re-sync. |
| Mutable Git sources | Defer Git to PM3; require `Ref` when added. |
| Package name/version mismatch | Validate synced manifest against registry entry and requested dependency. |
| Registry config location | Use project-local `.oct/registries.oct`; no user-global hidden state. |
| Source path traversal | Clean and validate `Source` + `Path`; reject paths that escape the intended local source root. |
| Symlink behavior | Copy rather than symlink; either preserve ordinary files/dirs or define symlink rejection explicitly. |
| Deleting local package cache | `oct pkg sync` can recreate `.oct/packages/<Name>/<Version>/` from configured registry. |
| Wrapper packages as dependencies | Sync source only; dependency sidecar build selection deferred. |
| No binary trust | Do not distribute binaries in PM2; local native builds remain explicitly gated by `--allow-native`. |
| Future migration to artifacts | Keep registry entry fields small and source-oriented; add digest/artifact fields later without changing the direct dependency story. |

## 19. Final recommendation

PM2 should implement exactly this MVP:

```text
PM2 — implement local registry/source-resolution MVP
```

Exact PM2 CLI:

```sh
oct pkg registry add <name> <path>
oct pkg registry list
oct pkg registry remove <name>
oct pkg add <name>@<version> [--registry <name>]
oct pkg sync
```

Exact registry config:

```text
.oct/registries.oct
```

Exact registry file:

```text
<registry-root>/registry.oct
```

Exact registry format: Oct manifest-like `package Registry`, `record RegistryIndex`, `record PackageEntry`, and `fn Registry() -> RegistryIndex` with required string fields:

```text
Name, Version, Kind, SourceKind, Source, Path, Description
```

Exact source support:

```text
SourceKind: "local" only in PM2
```

Exact cache path:

```text
.oct/packages/<Name>/<Version>/
```

Exact metadata path:

```text
.oct/packages/<Name>/<Version>/.oct-package-source.oct
```

Exact dependency behavior:

- `oct pkg add` edits the current manifest only;
- it writes exact `VersionRequirement` without `Source`;
- `oct pkg sync` resolves dependencies without `Source` through configured registries;
- legacy explicit `Dependency.Source` remains supported during transition;
- no automatic sync during add.

Exact version/ambiguity policy:

- exact versions only;
- no ranges/latest/solver;
- names are case-sensitive and should follow strict PascalCase;
- duplicate exact entries in one registry error;
- duplicate exact entries across registries error unless `--registry` narrows resolution.

Exact wrapper policy:

- registry can acquire wrapper package source;
- sync does not build native sidecars;
- W8b `oct pkg build-wrappers --allow-native` remains current-package only;
- dependency wrapper sidecar build selection is deferred.

Exact PM2 non-goals:

- no production remote registry behavior;
- no federation or P2P;
- no lockfile;
- no content-addressed `.octpkg` artifacts;
- no registry signing;
- no binary sidecar distribution;
- no package publishing/auth/namespace reservation;
- no registry mirrors;
- no semver solver sophistication;
- no wrapper runtime discovery changes;
- no wrapper build lifecycle changes;
- no Oct syntax changes.

Exact PM2 test plan is the list in section 16, with the first golden test proving:

```text
local registry.oct
→ oct pkg registry add
→ oct pkg add SignalTools@0.1.0
→ oct pkg sync
→ .oct/packages/SignalTools/0.1.0
→ oct test . imports SignalTools
```
