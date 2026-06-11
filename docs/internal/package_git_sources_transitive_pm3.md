# PM3 Git source entries and transitive dependency planning design

## 1. Executive summary

PM2 established the first useful registry/source-resolution slice for Oct packages:

- project-local registry configuration in `.oct/registries.oct`;
- local registry indexes in `<registry-root>/registry.oct`;
- `oct pkg registry add/list/remove`;
- `oct pkg add <Name>@<exact-version> [--registry <name>]`;
- `oct pkg sync` for direct manifest dependencies;
- `SourceKind: "local"` registry entries only;
- project-local synced package copies in `.oct/packages/<Name>/<Version>/`;
- per-package diagnostic source metadata in `.oct-package-source.oct`;
- exact-version package loading from `.oct/packages/<Name>/<Version>/`;
- continued support for legacy explicit `Dependency.Source` fetch/cache behavior.

Git source entries are the next practical capability because they let a small local registry point at real package repositories without introducing federation, publishing, signing, artifact formats, remote registry hosting, mirrors, auth, lockfiles, or binary sidecar distribution. This keeps the PM2 model intact: a registry entry names exact package metadata and points to source, while `oct pkg sync` materializes source into the loader-visible project cache.

Transitive dependency planning must be designed now because Git packages will immediately make package reuse more realistic. A directly synced package can have its own manifest dependencies. Without an explicit graph policy, PM4 would either silently leave transitive imports unresolved, accidentally depend on first-wins registry order, or create loader behavior that cannot later be locked deterministically.

Recommended PM4 scope:

```text
PM4 — implement Git source entries plus deterministic dependency graph planning; include recursive transitive sync if the implementation audit confirms the code surface stays small.
```

PM4 should not implement lockfiles. It should record the resolved Git commit in package source metadata as diagnostic provenance only. PM5 should then design lockfile/digest policy using evidence from PM4's Git and graph-planning behavior.

## 2. Current PM2 inventory

### Registry config file

Configured package registries are project-local and stored at:

```text
.oct/registries.oct
```

There is no user-global registry configuration. `oct pkg registry add <name> <path>` creates or updates this file, rejects duplicate names, validates names with `[A-Za-z][A-Za-z0-9_-]*`, and stores relative paths as provided. Paths are resolved relative to the project root when loaded.

### Registry index schema

A registry root contains:

```text
registry.oct
```

PM2 registry indexes use this shape:

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
            }
        ]
    }
}
```

Registry `Kind` values are `library`, `experiment`, and `wrapper`. `library` maps to ordinary pure package manifests whose `Kind` is omitted, empty, normalized to `pure`, or otherwise treated as library-compatible by registry validation.

### `SourceKind: "local"` behavior

PM2 supports only:

```text
SourceKind: "local"
```

For local entries:

- `Source` is a local source root path;
- relative `Source` values are resolved relative to the registry root;
- `Path` is a package-root path inside `Source`;
- `Path` must be relative, non-empty, and must not escape `Source`;
- copied source rejects symlinks;
- copied source skips `.git`, `.oct/wrappers`, and `.oct/packages`.

### Synced package layout

Registry-resolved dependencies are copied into:

```text
.oct/packages/<Name>/<Version>/
```

Existing synced package directories are replaced through a temporary directory under:

```text
.oct/packages/<Name>/.tmp-*
```

then renamed into the final version directory. This reduces the chance of leaving a half-written final package directory.

### Source metadata file

PM2 writes diagnostic source metadata at:

```text
.oct/packages/<Name>/<Version>/.oct-package-source.oct
```

Current metadata shape:

```oct
package PackageSource

record PackageSource {
    Name: String
    Version: String
    Registry: String
    RegistryPath: String
    SourceKind: String
    Source: String
    Path: String
}
```

The metadata has no timestamp and is not a lockfile.

### Exact version behavior

`oct pkg add <Name>@<exact-version>` and `oct pkg sync` reject empty versions, `latest`, ranges, comparison constraints, wildcards, and versions containing spaces. The exact version text is written into the manifest dependency's `VersionRequirement` field.

The project loader searches exact project-local synced package paths using the version declared by the importing package manifest:

```text
<project-root>/.oct/packages/<Name>/<Version>/
```

It does not fall back to another version under `.oct/packages/<Name>/`.

### Direct dependency sync

`oct pkg sync` currently reads the current project manifest, validates direct dependencies, skips the built-in/non-fetchable `OctStd` dependency, and syncs each direct dependency:

- dependencies with explicit `Dependency.Source` use the existing legacy package cache/fetch path;
- dependencies without `Source` are resolved from configured registries by exact `Name` and `VersionRequirement`;
- registry ambiguity is an error unless `oct pkg add` was given `--registry <name>` at add time for direct dependency insertion.

### Current handling of transitive dependencies

PM2 does not perform generalized transitive registry resolution or recursive sync. It syncs the current project's direct manifest dependencies only.

The project loader already records dependency declarations per loaded package and resolves imports by consulting the manifest of the importing package. Because synced packages are searched under the root project's `.oct/packages/<Name>/<Version>/`, the loader can potentially load a transitive package if it has already been synced there and the importing package's manifest declares the exact version. PM2 does not ensure that package is present.

### Legacy explicit `Dependency.Source` behavior

Legacy explicit `Dependency.Source` remains supported. The package manager normalizes sources with URL schemes (`https`, `http`, `ssh`, `git`, and `file`) and fetches them through `git clone --depth 1` into the user package cache. `file:` means a local Git repository URL in that legacy path, not an arbitrary directory copy. The user cache records source, cache key, path, name, version, dependencies, Git HEAD, and fetch time in `index.json`.

PM3 should not remove or reinterpret this legacy path. Registry-backed Git entries should be implemented as registry sync behavior, not by changing manifest `Dependency.Source` semantics.

## 3. MVP Git source goal

PM4's golden path should be:

```sh
oct pkg registry add local ../oct-registry
oct pkg add SignalTools@0.1.0
oct pkg sync
oct test .
```

where the registry contains a Git-backed package entry:

```oct
PackageEntry {
    Name: "SignalTools"
    Version: "0.1.0"
    Kind: "library"
    SourceKind: "git"
    Source: "https://github.com/example/oct-signal-tools.git"
    Ref: "v0.1.0"
    Path: "."
    Description: "Signal helper functions"
}
```

After sync, the project should contain:

```text
.oct/packages/SignalTools/0.1.0/manifest.oct
.oct/packages/SignalTools/0.1.0/.oct-package-source.oct
```

and ordinary Oct loading/testing should use the same exact-version loader path PM2 introduced.

## 4. Registry schema extension

### Current PM2 fields

```text
Name
Version
Kind
SourceKind
Source
Path
Description
```

### PM3/PM4 addition

```text
Ref
```

### Recommended registry format

PM4 should accept both PM2 local entries without `Ref` and PM4 entries with `Ref` by treating `Ref` as an optional field in the schema, then validating it by source kind.

Recommended schema:

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
    Ref: String
    Path: String
    Description: String
}
```

Recommended local entry:

```oct
PackageEntry {
    Name: "SignalTools"
    Version: "0.1.0"
    Kind: "library"
    SourceKind: "local"
    Source: "../SignalTools"
    Ref: ""
    Path: "."
    Description: "Signal helper functions"
}
```

Recommended Git entry:

```oct
PackageEntry {
    Name: "SignalTools"
    Version: "0.1.0"
    Kind: "library"
    SourceKind: "git"
    Source: "https://github.com/example/oct-signal-tools.git"
    Ref: "v0.1.0"
    Path: "."
    Description: "Signal helper functions"
}
```

### Validation decisions

- `Ref` should be required and non-empty for `SourceKind: "git"`.
- `Ref` should be optional and must be empty if present for `SourceKind: "local"`.
- The registry loader should accept PM2 entries whose `PackageEntry` record does not declare `Ref`, as a backwards-compatibility concession for local registries.
- The in-memory `PackageEntry` model should include `Ref string`, defaulting to `""` when omitted.
- `SourceKind` should become a two-value MVP enum: `local` or `git`.
- A non-empty `Ref` on `SourceKind: "local"` should be rejected rather than ignored. Rejecting it prevents a registry author from believing a local source is pinned when PM4 would not use the field.

This keeps the schema additive while still making source-kind semantics explicit.

## 5. Git source semantics

### Source

For `SourceKind: "git"`, `Source` should be a Git clone source accepted by `git clone`.

PM4 should support:

- remote HTTPS Git URLs;
- remote SSH Git URLs if the user's Git installation can access them without PM4-specific auth support;
- local repository paths;
- `file://` repository URLs.

Supporting local repository paths is important for deterministic PM4 tests and for private local package development. This differs from legacy `Dependency.Source`, which requires URL schemes, but it is acceptable because registry `SourceKind: "git"` is an explicit source model and goes through registry validation.

PM4 should not add auth prompts, credential helpers, token configuration, registry credential storage, or any Oct-specific authentication behavior.

### Ref

`Ref` is the exact checkout target PM4 passes to Git. It may be a tag, branch, full commit SHA, short commit SHA, or any other commit-ish accepted by Git checkout/rev-parse.

Recommended PM4 behavior:

1. clone the repository into a temporary directory;
2. checkout exactly `Ref`;
3. resolve `HEAD` after checkout with `git rev-parse HEAD`;
4. record that resolved commit SHA in `.oct-package-source.oct`.

### Mutable refs

PM4 should allow any checkout target for usability, including branch-like refs. However:

- if `Ref` is not a full 40-character commit SHA, PM4 should warn in sync output or metadata diagnostics that the ref may be mutable;
- the warning should be informational, not a failure;
- the resolved commit should be recorded so users can diagnose what was actually synced;
- PM5 lockfile/digest design should decide how to pin and verify future syncs.

Requiring tags or commit SHAs only would improve reproducibility, but it would make the first Git-source milestone less useful for local and early package development. Allowing any Git checkout target while recording the resolved commit is the right pre-lockfile compromise.

### Explicit non-goals

PM4 Git source entries should not implement:

- auth;
- submodule checkout;
- Git LFS fetch policy;
- recursive Git dependencies beyond manifest-declared package dependencies;
- shallow clone unless the implementation can prove the requested `Ref` is fetched safely;
- automatic registry clone;
- package publish;
- lockfile pinning;
- content digest verification.

For MVP reliability, prefer a normal clone followed by checkout over clever shallow-fetch behavior. Shallow clone can be revisited once lockfile/digest semantics exist.

## 6. Git checkout/cache layout

### Options

#### A. Clone directly into `.oct/packages/<Name>/<Version>/`

Pros:

- simplest file movement;
- the checked-out repository remains available for Git diagnostics.

Cons:

- final package cache contains `.git` unless special cleanup runs in-place;
- failed checkout can leave a half-written final package directory;
- local and Git source entries produce visibly different final layouts;
- package tests might accidentally depend on Git metadata.

#### B. Clone into a temporary directory, checkout, then copy `Path` into final package directory

Pros:

- final package copy matches PM2 local-copy behavior;
- `.git` does not leak into `.oct/packages/<Name>/<Version>/`;
- `Path` can select a package subdirectory in a mono-repo;
- existing copy validation, symlink rejection, manifest validation, and temp-to-final replacement remain applicable;
- failed clone/checkout does not disturb the final synced package.

Cons:

- repeated syncs reclone without a persistent Git object cache;
- Git diagnostics require metadata rather than inspecting `.git` in the final package.

#### C. Use a user cache clone, then copy

Pros:

- avoids repeated network fetches;
- can support offline/later lockfile workflows.

Cons:

- expands the cache model before lockfile/digest policy exists;
- risks coupling registry Git entries to the legacy `Dependency.Source` cache;
- raises cache invalidation, mutable ref, and stale checkout questions too early.

### Recommendation

PM4 should implement option B:

1. create a temp clone directory under `.oct/packages/<Name>/.tmp-git-*` or another package-local temp path;
2. run `git clone <Source> <tmp-repo>`;
3. run `git -C <tmp-repo> checkout --detach <Ref>` when possible;
4. read `ResolvedCommit` with `git -C <tmp-repo> rev-parse HEAD`;
5. resolve registry `Path` inside the checked-out repo;
6. copy that package root into a separate temp install directory under `.oct/packages/<Name>/.tmp-*`;
7. skip `.git`, `.oct/wrappers`, and `.oct/packages` while copying;
8. reject symlinks as PM2 does;
9. validate the copied manifest against registry name/version/kind;
10. write source metadata with `SourceKind`, `Source`, `Ref`, `ResolvedCommit`, and `Path`;
11. atomically replace `.oct/packages/<Name>/<Version>/`;
12. remove clone/install temporary directories.

The final synced package should be a clean source tree, equivalent in shape to a local registry copy.

## 7. Source metadata extension

### Current PM2 metadata

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
```

### PM4 metadata additions

```text
Ref
ResolvedCommit
```

Possible future field:

```text
FetchedBy
```

PM4 should not add `FetchedBy` yet unless an implementation need appears. It would be descriptive and could be confused with lockfile/toolchain semantics.

### Recommended PM4 metadata

```oct
package PackageSource

record PackageSource {
    Name: String
    Version: String
    Registry: String
    RegistryPath: String
    SourceKind: String
    Source: String
    Ref: String
    ResolvedCommit: String
    Path: String
}

fn PackageSource() -> PackageSource {
    return PackageSource {
        Name: "SignalTools"
        Version: "0.1.0"
        Registry: "local"
        RegistryPath: "../oct-registry"
        SourceKind: "git"
        Source: "https://github.com/example/oct-signal-tools.git"
        Ref: "v0.1.0"
        ResolvedCommit: "0123456789abcdef0123456789abcdef01234567"
        Path: "."
    }
}
```

For `SourceKind: "local"`, write:

```text
Ref: ""
ResolvedCommit: ""
```

No timestamp should be added. This metadata is diagnostic provenance, not `oct.lock`, not an integrity proof, and not a reproducibility contract.

## 8. Transitive dependency planning

### PM2 baseline

PM2 syncs only the current project's direct dependencies. If direct dependency `A` imports dependency `B`, PM2 succeeds at syncing `A` but does not ensure `B` is present in `.oct/packages/B/<Version>/`.

### PM3 graph model

PM4 should plan dependencies as a graph before final sync execution.

Each graph node is:

```text
<Name>@<exact-version>
```

Each directed edge records the dependency chain:

```text
Importer@Version -> Dependency@VersionRequirement
```

The root project is a synthetic root node used only for diagnostics.

### Planning algorithm

1. Load the root project manifest.
2. Validate all root direct dependencies:
   - dependency name non-empty;
   - exact version requirement;
   - duplicate direct names rejected unless identical duplicate policy is deliberately changed;
   - `OctStd` without `Source` is skipped as built-in.
3. Enqueue root dependencies in deterministic order by `Name`, then version text, preserving stable diagnostics.
4. For each dependency:
   - if it has explicit `Source`, use the legacy source plan path and record that it is source-explicit;
   - if it has no `Source`, resolve it through configured registries by exact name/version;
   - if already planned with the same name and version, deduplicate;
   - if the same package name appears with a different exact version, fail with a conflict diagnostic including both chains.
5. To discover transitive dependencies, obtain the dependency package manifest:
   - for local registry entries, read the manifest from the local source/path before copying, or from the copied temp install directory;
   - for Git entries, clone/checkout into a temp repo and read the manifest from `Path` during planning/execution;
   - for legacy explicit `Dependency.Source`, either use the existing fetched cache metadata or fetch first, then inspect manifest dependencies.
6. Add manifest dependencies to the graph with the current package as parent.
7. Detect cycles with a visiting/visited traversal state and report the cycle path.
8. Execute sync in topological order or another deterministic order that guarantees dependencies are available before importer tests run.

### Exact versions only

PM4 should not implement semver ranges, `latest`, version solving, compatibility rules, or backtracking. Every dependency edge must name an exact version. If a manifest still uses `VersionRequirement` text that looks like a range, PM4 should fail with the existing exact-version validation.

### Deduplication

If two chains require the same `Name@Version`, PM4 should sync it once and share the result:

```text
App -> SignalTools@0.1.0 -> MathCore@0.2.0
App -> AnalysisKit@0.1.0 -> MathCore@0.2.0
```

`MathCore@0.2.0` is one graph node.

### Conflicting versions

If two chains require the same package name at different exact versions, PM4 should fail:

```text
conflicting exact dependency versions for MathCore:
- App -> SignalTools@0.1.0 -> MathCore@0.2.0
- App -> AnalysisKit@0.1.0 -> MathCore@0.3.0
```

This matches the current loader's package map by package name and avoids pretending Oct can load multiple versions of the same package simultaneously.

### Cycles

Cycles should fail before copying final packages when possible:

```text
dependency cycle detected: App -> A@0.1.0 -> B@0.1.0 -> A@0.1.0
```

Cycle diagnostics should include the package/version path, not just the repeated package name.

### Should PM4 execute recursive sync?

Recommendation:

- PM4 should implement recursive direct+transitive sync if the implementation can reuse the planned graph and existing copy/sync functions without a broad architecture rewrite.
- If the audit shows loader context or source-fetch staging is too risky, PM4 should at least implement graph planning and fail clearly when transitive dependencies are discovered but not synced.

Because the existing loader can already use the importing package manifest's exact version and look under root `.oct/packages/<Name>/<Version>/`, recursive sync appears feasible. The main risk is not loader lookup; it is building a clean graph planner around current direct-sync functions.

## 9. Registry resolution for transitive dependencies

The same project-local registry config applies to direct and transitive dependencies:

```text
.oct/registries.oct
```

Rules:

- Direct `oct pkg add <Name>@<Version> --registry <name>` should choose the registry for the direct dependency insertion/resolution only.
- That direct `--registry` choice should not permanently force all transitive dependencies to use the same registry.
- Transitive dependencies without explicit `Source` should resolve by scanning all configured registries.
- Zero matches should be an unresolved dependency error with the dependency chain.
- Multiple matches should be an ambiguity error with the dependency chain.
- There should be no first-wins registry behavior for transitive dependencies.

Example ambiguity diagnostic:

```text
ambiguous registry dependency MathCore@0.2.0 required by:
App -> SignalTools@0.1.0 -> MathCore@0.2.0
found in registries: local, vendor
```

Example unresolved diagnostic:

```text
unresolved registry dependency MathCore@0.2.0 required by:
App -> SignalTools@0.1.0 -> MathCore@0.2.0
searched registries: local, vendor
```

## 10. Project loader and transitive packages

The project loader must support these cases for PM4 to feel complete:

1. root package imports a direct dependency declared in the root manifest;
2. a synced dependency package imports its own dependency declared in its own manifest;
3. the imported dependency is found at `.oct/packages/<Name>/<Version>/` using the importing package manifest's exact `VersionRequirement`;
4. both direct and transitive synced packages may live under the root project's `.oct/packages` tree;
5. no fallback by package name only is allowed.

Current loader behavior is mostly aligned with this because it stores manifest dependencies per package and, when resolving an import in manifest mode, consults the importing package's dependency map. It then searches the root project's `.oct/packages/<ImportName>/<Version>/` path. PM4 should verify this with integration tests where `A` imports `B`, the root imports only `A`, and both `A` and `B` are synced from the registry.

Risks:

- The loader's loaded package map is keyed by package name, so multiple exact versions of the same package cannot coexist. PM4 graph planning must reject exact-version conflicts before loader ambiguity.
- Existing legacy package-cache fallback is keyed by package name, not version. PM4 should not rely on that fallback for registry transitive dependencies.
- If a dependency package's manifest has an import dependency but PM4 did not sync it, loader errors will occur late during `oct test .`. PM4 should catch that earlier in graph planning.
- If future package aliasing or multi-version loading is desired, that is a separate language/tooling design and not part of PM4.

## 11. Git source safety and errors

PM4 should validate and report:

- missing `git` binary;
- clone failure, including source and captured stderr/stdout summary;
- checkout failure, including `Ref` and source;
- `git rev-parse HEAD` failure after checkout;
- missing package `Path` inside the checked-out repository;
- `Path` absolute or escaping repository root;
- missing `manifest.oct` at package path;
- manifest name/version/kind mismatch against registry entry;
- unsupported `SourceKind`;
- `SourceKind: "git"` with empty `Ref`;
- `SourceKind: "local"` with non-empty `Ref`;
- duplicate package/version entries inside one registry;
- ambiguous package/version entries across registries;
- unresolved transitive dependencies;
- cycle path;
- conflicting exact versions with both dependency chains;
- symlink rejection during package copy;
- accidental `.git` leakage into final package copy.

Git command errors should include enough context to act on them without dumping unbounded output. A useful shape:

```text
dependency SignalTools 0.1.0 from registry local: git checkout ref "v0.1.0" failed for https://github.com/example/oct-signal-tools.git: <trimmed git output>
```

No credentials/auth support should be added. PM4 should let Git use the user's existing environment for SSH keys or credential helpers, but Oct should not store or prompt for credentials.

## 12. CLI behavior

Current commands should remain the PM4 surface:

```sh
oct pkg registry add <name> <path>
oct pkg registry list
oct pkg registry remove <name>
oct pkg add <Name>@<Version> [--registry <name>]
oct pkg sync
```

Potential future flags:

```text
oct pkg sync --plan
oct pkg sync --offline
oct pkg sync --no-git
oct pkg sync --registry <name>
```

Recommendation:

- Do not add new flags for PM4 unless `oct pkg sync --plan` falls out naturally from the graph planner.
- Do not add `--offline` before there is a persistent Git source cache or lockfile.
- Do not add `--no-git`; a registry entry with `SourceKind: "git"` should either sync or fail clearly.
- Do not add `oct pkg sync --registry <name>`; sync should honor project dependencies and registry resolution rules rather than making a global override that can hide transitive ambiguity.

If `--plan` is cheap, it should print the deterministic graph, source kind for each node, registry selected for registry nodes, explicit-source nodes, and conflicts without writing final package directories. It is useful but not required for PM4 acceptance.

## 13. Test plan for PM4

### Git source tests

1. Local Git repo package source:
   - initialize a Git repo in a temp directory;
   - commit a scaffolded package;
   - tag `v0.1.0`;
   - create registry entry with `SourceKind: "git"`, `Source: <repo path or file URL>`, `Ref: "v0.1.0"`;
   - run `oct pkg sync`;
   - assert `.oct/packages/<Name>/<Version>` exists;
   - assert `.git` was not copied;
   - assert `.oct-package-source.oct` includes `Ref` and `ResolvedCommit`;
   - run `oct test .`.
2. Checkout failure reports the requested ref and source.
3. Package path inside repo works, e.g. `Path: "packages/SignalTools"`.
4. Manifest mismatch errors for wrong name, wrong version, and wrong kind.
5. Missing Git binary test using a controlled `PATH`, if feasible and stable.
6. Symlink inside Git checkout is rejected consistently with local copy behavior.
7. Submodule behavior is not fetched by default; a dedicated no-submodule test may be deferred if setup cost is high.

### Registry schema tests

1. PM2 registry entry without `Ref` remains valid for `SourceKind: "local"`.
2. Local entry with `Ref: ""` remains valid.
3. Local entry with non-empty `Ref` is rejected.
4. Git entry with missing or empty `Ref` is rejected.
5. Unsupported `SourceKind` is rejected with a message listing `local` and `git`.

### Transitive dependency graph tests

1. `A` depends on `B`; registry contains `A` and `B`; root depends on `A`; sync materializes both `A` and `B`; `A` imports/uses `B`; `oct test .` passes.
2. `A` and `C` both depend on `B@0.1.0`; sync deduplicates `B`.
3. `A` depends on `B@0.1.0`, `C` depends on `B@0.2.0`; sync errors with both chains.
4. `A` depends on `B`, `B` depends on `A`; sync errors with cycle path.
5. Transitive dependency found in two registries errors with dependency chain and registry names.
6. Transitive dependency missing from all registries errors with dependency chain and searched registries.
7. Legacy explicit `Dependency.Source` still works for direct dependencies.
8. If legacy explicit `Dependency.Source` is encountered transitively, PM4 either fetches it through the legacy path and includes its manifest dependencies, or fails with an explicit scoped message if that slice is deferred.

### Loader integration tests

1. Root manifest declares only `A`; `A` manifest declares `B`; `A` imports `B`; root tests exercise `A`; loader finds `B` from root `.oct/packages/B/<Version>/`.
2. Version conflict is caught by planner before loader sees two packages with the same name.
3. Wrong synced version is not used as fallback.

## 14. Documentation updates for PM4

PM4 should update user-facing package docs to document:

- `SourceKind: "git"` registry entries;
- `Ref` required for Git and empty/omitted for local;
- supported `Source` forms for Git entries;
- exact versions only;
- `Ref` may be mutable before lockfiles;
- `ResolvedCommit` metadata is diagnostic only;
- `.oct-package-source.oct` is not a lockfile;
- no auth/submodules/LFS behavior;
- transitive exact dependency sync or plan/fail behavior, depending on final PM4 scope;
- conflict, cycle, ambiguity, and missing-dependency errors;
- no lockfile, no `oct.lock`, no content-addressed artifacts, no federation, no publishing, no signing, no binary sidecar distribution.

Wrapper lifecycle docs should not be changed except possibly to state that Git source sync copies wrapper package source but does not build dependency sidecars and does not change runtime wrapper discovery.

## 15. PM4 recommendation

Options considered:

```text
Option A: PM4 — implement Git source entries and transitive sync
Option B: PM4 — implement Git source entries only; transitive sync deferred
Option C: PM4 — design lockfile/digest before Git
```

Recommendation: choose a bounded version of Option A.

```text
PM4 — implement Git source entries plus dependency graph planning; include recursive transitive sync if the audit confirms low implementation risk.
```

Rationale:

- Git source entries are the next useful capability after local registry copies.
- Git without transitive planning will fail common real-world package reuse cases.
- Lockfile/digest design needs concrete source metadata and graph behavior to lock.
- Federation, publishing, signing, mirrors, and P2P remain too broad and should not be pulled into PM4.

If PM4 must reduce scope, the fallback should be:

```text
PM4 — implement Git source entries and graph planning that fails clearly on transitive dependencies not yet synced/executed.
```

It should not skip the graph design entirely.

## 16. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Mutable Git refs before lockfile | Allow them for usability, warn when ref is not a full commit SHA, record `ResolvedCommit`, design lockfile/digest in PM5. |
| Missing lockfile | Be explicit that `.oct-package-source.oct` is diagnostic only; do not call it a lockfile or promise reproducibility. |
| Dependency confusion | Treat registry ambiguity as an error; do not use first-wins for transitive dependencies; include dependency chains in errors. |
| Transitive ambiguity | Use same configured registries but fail on multiple matches with chain and registry names. |
| Exact version conflicts | Reject same package name with different exact versions before loader execution. |
| Cycles | Detect with graph visiting state and report a clear cycle path. |
| Loader context complexity | Add integration tests proving importer-manifest version lookup works for synced transitive packages. |
| Git unavailable | Check `git` execution failure and report installation/PATH context. |
| Path traversal inside repo | Reuse safe relative `Path` validation; reject absolute paths and `..` escape. |
| `.git` leakage into package cache | Clone to temp repo, copy package root while skipping `.git`, and assert no `.git` in tests. |
| Local Git path vs remote URL semantics | Document that registry Git `Source` is any `git clone` source, including local paths; keep legacy `Dependency.Source` URL-scheme semantics unchanged. |
| No auth/submodules/LFS | State as PM4 non-goals; let user Git environment handle access, but do not add Oct credential policy. |
| Persistent cache absence | Reclone per sync for PM4; defer user cache/offline policy until lockfile/digest design. |
| Partial sync after graph error | Prefer graph planning before final installs; use temp directories and final renames for execution. |

## 17. Final recommendation

### Exact PM4 scope

PM4 should implement:

- registry schema extension with optional `Ref` field;
- `SourceKind: "git"` validation;
- Git clone/checkout of registry entries into temp directories;
- checkout of exactly `Ref`;
- resolved commit capture with `git rev-parse HEAD`;
- copy of registry `Path` from checked-out repo into `.oct/packages/<Name>/<Version>/`;
- final package copy without `.git`;
- source metadata extension with `Ref` and `ResolvedCommit`;
- deterministic dependency graph planning for direct and transitive manifest dependencies;
- exact-version-only transitive resolution through configured registries;
- conflict, cycle, ambiguity, and missing-dependency diagnostics with chains;
- recursive transitive sync if implementation remains small and well-contained;
- user docs and tests for Git sources and graph behavior.

### Exact PM4 non-goals

PM4 should not implement:

- lockfiles;
- `oct.lock` generation;
- content-addressed `.octpkg` artifacts;
- registry signing;
- P2P;
- package publishing;
- auth;
- namespace reservation;
- registry mirrors;
- HTTP registry hosting;
- binary sidecar distribution;
- native wrapper dependency build selection;
- wrapper runtime discovery changes;
- wrapper build lifecycle changes;
- semver ranges;
- solver behavior;
- `latest`;
- Oct syntax changes;
- package-manager architecture rewrite;
- federation.

### Exact registry schema extension

Add optional `Ref: String` support to `PackageEntry` loading:

- accepted/empty for `local`;
- required/non-empty for `git`;
- rejected/non-empty for `local`;
- omitted allowed for PM2 local registry compatibility.

### Exact source metadata extension

Add:

```text
Ref
ResolvedCommit
```

Write empty strings for local sources and concrete checkout data for Git sources. Do not add timestamps. Do not call metadata a lockfile.

### Exact transitive policy

- Build a deterministic graph of exact `Name@Version` nodes.
- Resolve dependencies without `Source` through the same project registry config.
- Preserve legacy explicit `Dependency.Source` behavior.
- Deduplicate identical `Name@Version` nodes.
- Reject same package name with different exact versions.
- Reject cycles with cycle paths.
- Reject ambiguous transitive registry matches; no first-wins.
- Include dependency chains in all transitive resolution errors.
- Keep semver ranges and solver behavior deferred.

### Exact test plan

PM4 tests should cover:

- local Git repo source sync by tag;
- checkout failure diagnostics;
- package subpath inside repo;
- manifest mismatch errors;
- missing Git binary if feasible;
- `.git` exclusion from final package;
- metadata `Ref` and `ResolvedCommit`;
- PM2 local registry compatibility without `Ref`;
- local non-empty `Ref` rejection;
- Git missing/empty `Ref` rejection;
- `A -> B` transitive sync and `oct test .`;
- duplicate same-version transitive dedupe;
- conflicting exact versions;
- cycles;
- transitive ambiguity;
- missing transitive dependency;
- legacy explicit `Dependency.Source` compatibility.

PM5 should design lockfile/digest behavior after PM4 proves Git source sync and graph planning in the real path.
