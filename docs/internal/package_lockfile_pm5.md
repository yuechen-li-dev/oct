# PM5 — optional `lock.octagon` package lockfile design

PM5 is design-only. It does not implement package-manager behavior, generate a lockfile, add a digest format, or change the current PM4 package sync path.

## 1. Executive summary

`lock.octagon` should solve one narrow problem: preserving a fully resolved package dependency graph when a project deliberately wants reproducible package acquisition. The lockfile records exact `Name@Version` nodes plus source identity and resolution metadata, especially Git `ResolvedCommit`, so CI, research artifacts, production releases, demos, and controlled experiments can replay the same graph instead of following the current registry state.

The lockfile should be optional because Oct's default package-manager posture is source-oriented, rolling, and development-friendly. A normal project should be able to edit `manifest.oct`, update a local registry entry, retag a Git dependency during active development, and run:

```sh
oct pkg sync
```

without receiving or maintaining generated lockfile churn. This is similar in spirit to ecosystems where package locks are valuable when chosen but are not the default semantic center of package management.

Default package behavior should remain rolling: `manifest.oct` declares user intent, `.oct/registries.oct` selects registries, `registry.oct` describes currently available package sources, and plain `oct pkg sync` resolves from those registries each time. This means transitive dependencies can intentionally drift when registry entries drift.

Recommended PM6 scope:

- implement `oct pkg lock` to resolve the current exact-version registry graph and write project-root `lock.octagon`;
- implement `oct pkg sync --locked` to materialize exactly the graph in `lock.octagon`;
- keep plain `oct pkg sync` rolling and non-lock-generating;
- require Git lock entries to contain full `ResolvedCommit` values and check out those commits detached;
- allow local source entries but mark them mutable/non-reproducible;
- explicitly defer tree digests, `.octpkg` artifacts, signing, federation, publishing, mirrors, P2P, auth, binary sidecar distribution, semver ranges, `latest`, and solver/backtracking behavior.

## 2. Current PM4 inventory

### Registry config behavior

PM4 uses project-local registry configuration at:

```text
.oct/registries.oct
```

There is no user-global registry config. Registry commands are:

```sh
oct pkg registry add <name> <path>
oct pkg registry list
oct pkg registry remove <name>
```

Registry names are stable simple names matching `[A-Za-z][A-Za-z0-9_-]*`. Relative registry paths are stored as provided and resolved relative to the project root when used.

### Registry index behavior

A registry root contains:

```text
<registry-root>/registry.oct
```

The PM4 registry index shape is:

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

`Kind` must be one of `library`, `experiment`, or `wrapper`. `library` maps to ordinary pure package manifests. Versions are exact text only; there is no `latest`, semver range solving, solver, or backtracking behavior.

### Local and Git source behavior

PM4 supports:

- `SourceKind: "local"`
- `SourceKind: "git"`

For local sources:

- `Source` names a local source root relative to the registry root unless absolute;
- `Path` selects a package root inside the source;
- `Ref` must be omitted or empty.

For Git sources:

- `Source` is passed to local `git clone`;
- `Ref` is required and non-empty;
- sync checks out `Ref` detached;
- sync records `git rev-parse HEAD` as `ResolvedCommit`;
- mutable refs are allowed in PM4, but a non-full-SHA `Ref` prints a warning and records the resolved commit.

PM4 source copying rejects symlinks, skips `.git`, `.oct/wrappers`, and `.oct/packages`, validates copied manifest name/version/kind, and replaces the final package directory atomically after a temporary install directory is prepared.

### Recursive exact-version sync

`oct pkg sync` resolves direct and transitive source-less dependencies declared in `manifest.oct`. It is exact-version only. Graph rules are:

- graph nodes are `<Name>@<exact-version>`;
- identical nodes are deduped;
- same package name with different exact versions is a conflict;
- cycles are errors;
- missing dependencies and ambiguous registry entries include dependency-chain diagnostics;
- final sync order is dependency-before-importer for registry nodes discovered through manifests;
- explicit legacy `Dependency.Source` behavior remains supported separately.

### Synced package location

Registry-resolved packages sync into:

```text
.oct/packages/<Name>/<Version>/
```

The project loader resolves a manifested import with an exact dependency declaration by looking first under this project-local exact-version path. That path is therefore the real runtime/test path for PM4 synced registry packages.

### Source metadata

Every synced registry package receives:

```text
.oct/packages/<Name>/<Version>/.oct-package-source.oct
```

The metadata is Oct data written next to one installed copy. PM4 fields are:

```text
Name
Version
Registry
RegistryPath
SourceKind
Source
Ref
ResolvedCommit
Path
```

For local sources, `Ref` and `ResolvedCommit` are empty. For Git sources, `Ref` is the registry request and `ResolvedCommit` is the commit actually checked out.

This metadata is diagnostic-only. It is not a project lockfile and must not be converted into one.

### Current lack of lockfile/digest/artifact/signing

PM4 intentionally does not have:

- `lock.octagon`;
- `oct.lock`;
- content-addressed `.octpkg` artifacts;
- package tree digests;
- source digests;
- registry index digests;
- signatures;
- federation, P2P, publishing, mirrors, or auth;
- binary sidecar distribution;
- semver ranges, `latest`, solver, or backtracking behavior.

## 3. Lockfile philosophy

The package-manager data roles should stay distinct:

- Manifest expresses user intent.
- Registry expresses available package sources.
- `.oct-package-source.oct` expresses diagnostic source metadata for one synced package copy.
- `lock.octagon` expresses the fully resolved graph.

Recommended user-facing language:

```text
manifest.oct says: “I depend on SignalTools.”
registry.oct says: “SignalTools can be acquired from here.”
.oct-package-source.oct says: “This synced copy came from here.”
lock.octagon says: “Use exactly this resolved graph.”
```

Consequences:

- `manifest.oct` remains authored source metadata.
- `registry.oct` remains registry-maintained source metadata.
- `.oct-package-source.oct` remains per-copy diagnostic metadata.
- `lock.octagon` is generated project data that can be committed when desired.
- No lockfile behavior should require a semver range solver.
- No lockfile behavior should reinterpret mutable registry defaults as a project guarantee.

## 4. File name, format, and location

### Recommended file name

PM6 should use:

```text
lock.octagon
```

Rationale:

- the lockfile is generated data;
- the lockfile is not executable/source Oct;
- `.octagon` already denotes data/artifact shape in the repository;
- the name avoids implying that users should edit or run it as Oct code;
- the name avoids `oct.lock`, which is not part of current PM4 behavior and would add a second convention.

### Recommended location

PM6 should place the lockfile at the project root:

```text
<project-root>/lock.octagon
```

Rationale:

- it is a project artifact intended to be committed when the project chooses reproducibility;
- it should be visible in ordinary review and CI workflows;
- project-root placement matches user expectations for generated dependency lock data;
- it separates the lockfile from `.oct/`, which currently contains local package-manager state and synced package copies.

Tradeoff: `.oct/lock.octagon` would reduce project-root clutter and align with `.oct/registries.oct`, but it would make a deliberately committed artifact easier to miss and easier to confuse with local cache state. Because PM5's core principle is optional but explicit freezing, visibility is more important than hiding generated data.

## 5. Default behavior: rolling mode

Plain sync remains rolling mode:

```sh
oct pkg sync
```

Design rules:

- `oct pkg sync` ignores `lock.octagon` unless the user explicitly requests locked behavior.
- `oct pkg sync` continues resolving from project registries.
- `oct pkg sync` does not generate `lock.octagon` automatically.
- Package dependencies can drift as registry entries change.
- Git refs can resolve to newer commits in rolling mode if the registry still points at mutable refs.
- This drift is intentional for normal development and local experimentation.

### Should rolling sync warn when `lock.octagon` exists?

Recommendation: no warning by default in PM6.

Reasoning:

- if lockfiles are optional, a project may intentionally keep a lockfile for release/CI while developers run rolling sync locally;
- a warning on every plain sync would teach users to ignore package-manager output;
- the command name remains explicit: `oct pkg sync` means rolling, `oct pkg sync --locked` means locked.

Deferred PM6.1 idea: add a concise opt-in check, such as `oct pkg lock --check`, that reports whether the current registry resolution differs from the committed lock. Avoid making ordinary rolling mode noisy.

## 6. Locked behavior

### Command surface considered

Candidate A:

```sh
oct pkg lock
oct pkg sync --locked
```

Candidate B:

```sh
oct pkg sync --write-lock
oct pkg sync --locked
```

Recommendation: use Candidate A for PM6.

```sh
oct pkg lock
oct pkg sync --locked
```

Rationale:

- writing a lockfile is a different user intent than syncing packages;
- `oct pkg lock` is easier to explain in docs and CI scripts;
- plain `oct pkg sync` can remain stable and rolling;
- `--write-lock` risks suggesting that lockfile writes are a variant of normal sync rather than an explicit freeze operation.

### `oct pkg lock`

PM6 M0 behavior:

- resolve the current direct and transitive exact-version graph using the PM4 graph planner/resolver;
- require the graph to be valid: no version conflicts, cycles, missing registry entries, or ambiguous registry entries;
- acquire enough source information to populate lock entries;
- for Git entries, resolve and record a full `ResolvedCommit`;
- write deterministic project-root `lock.octagon`;
- print warnings for mutable local source entries;
- print warnings during lock creation for Git refs that are not full commits, while still recording the resolved commit.

Recommended output:

```text
Resolved package graph: 3 packages
Wrote lock.octagon
```

Should `oct pkg lock` also sync packages?

PM6 M0 recommendation: `oct pkg lock` resolves and writes the lock, but does not promise to install packages as its primary output.

Implementation note: the simplest PM6 implementation may reuse PM4 source acquisition to validate package manifests and discover transitive dependencies. That is acceptable if documented, but the command contract should be “write/update `lock.octagon`,” not “sync the package cache.” If the initial implementation happens to populate `.oct/packages` while resolving, output should make that incidental effect explicit rather than relying on it.

### `oct pkg sync --locked`

PM6 M0 behavior:

- load project-root `lock.octagon`;
- fail if it is missing;
- fail if it is malformed or has unsupported `LockVersion`;
- use lock graph entries as the authority for locked packages;
- fail if the current manifest root dependencies are not exactly represented by the lock graph;
- fail if locked graph dependencies are inconsistent, cyclic, or stale/unreachable;
- for Git entries, check out `ResolvedCommit` detached, not mutable `Ref`;
- for local entries, sync from the recorded local source/path and report mutability limitations;
- write `.oct-package-source.oct` metadata into each synced package just like rolling sync;
- avoid consulting registries for locked packages unless a future lock schema explicitly says a registry lookup is necessary.

Recommended missing-lock diagnostic:

```text
pkg sync: lock.octagon is required for --locked; run oct pkg lock to create it
```

Recommended output:

```text
Loaded lock.octagon
Synced SignalTools 0.1.0 from locked git commit 0123456789abcdef0123456789abcdef01234567
Package sync complete: 3 packages
```

## 7. Lockfile content model

`lock.octagon` should be graph data, not source code and not an executable manifest.

### Root fields to consider

- `LockVersion`
- `GeneratedBy`
- `RootPackage`
- `RootVersion`
- `Packages`

`GeneratedBy` should be informational and deterministic enough not to cause needless churn. PM6 can use a stable value such as `"oct pkg lock"` or include a tool version only if Oct has a stable version source.

PM6 should not include timestamps. Timestamps make repeated lock writes non-deterministic.

### PM6 minimal package entry

Recommended PM6 M0 package entry fields:

```text
Name
Version
Kind
SourceKind
Source
Ref
ResolvedCommit
Path
Registry
RegistryPath
Dependencies
Mutable
```

The user's minimum field list did not require `Mutable`, but PM5 recommends adding it in PM6 M0 because local-source mutability is a core design risk. If implementation simplicity requires omitting `Mutable` initially, local mutability must still be represented by a deterministic convention and CLI warning. The preferred convention is explicit `Mutable: true` for local entries and `Mutable: false` for Git entries pinned to full commits.

### Dependency entry

Dependencies should be exact graph edges:

```text
Name
Version
```

No source, range, registry, or solver data belongs in edge entries. Source identity belongs to package nodes.

### Git entries

For `SourceKind: "git"`:

- `Ref` records the original registry/user-facing request;
- `ResolvedCommit` records the full commit SHA used by locked sync;
- locked sync must check out `ResolvedCommit`, not `Ref`;
- lock writing must fail if `ResolvedCommit` is missing or not a full SHA;
- mutable-ref warnings become lock-creation diagnostics, not locked-sync behavior.

### Local entries

For `SourceKind: "local"`:

- record `Source` and `Path`;
- set `ResolvedCommit` to an empty string;
- set `Mutable: true`;
- allow local entries for development convenience;
- warn that local lock entries are not reproducible until tree digests or artifacts exist.

Recommended warning when writing a lock:

```text
warning: local source SignalTools@0.1.0 is mutable; lock.octagon records source path but not content digest
```

## 8. `.octagon` schema sketch

Existing `.octagon` files in the repository are data literals, often a single record literal or array literal, without `package`, `record`, or function declarations. `lock.octagon` should follow that data-artifact style.

Recommended conceptual shape:

```text
OctPackageLock {
    LockVersion: 1
    GeneratedBy: "oct pkg lock"
    Root: LockRoot {
        Name: "MyProject"
        Version: "0.1.0"
    }
    Packages: [
        LockPackage {
            Name: "MathCore"
            Version: "0.2.0"
            Kind: "library"
            SourceKind: "git"
            Source: "https://github.com/example/oct-math-core.git"
            Ref: "v0.2.0"
            ResolvedCommit: "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
            Path: "."
            Registry: "local"
            RegistryPath: "../oct-registry"
            Mutable: false
            Dependencies: []
        },
        LockPackage {
            Name: "SignalTools"
            Version: "0.1.0"
            Kind: "library"
            SourceKind: "git"
            Source: "https://github.com/example/oct-signal-tools.git"
            Ref: "v0.1.0"
            ResolvedCommit: "0123456789abcdef0123456789abcdef01234567"
            Path: "."
            Registry: "local"
            RegistryPath: "../oct-registry"
            Mutable: false
            Dependencies: [
                LockDependency { Name: "MathCore" Version: "0.2.0" }
            ]
        }
    ]
}
```

This is a schema sketch, not a PM5 implementation requirement. PM6 should use the existing internal Octagon reader/writer conventions rather than adding source-Oct parsing for the lockfile.

## 9. Lock graph semantics

- Lock graph nodes are exact `Name@Version` packages.
- `Name@Version` identity is deduped.
- The same package name at multiple versions is invalid for PM6 M0, matching PM4 conflict behavior.
- Cycles are invalid; the lock writer should reject cycles even if they should already be rejected by the PM4 planner.
- The root project is not copied and should not appear as a package node unless a future schema explicitly models workspace roots.
- Package edges are represented by each node's `Dependencies` list.
- Locked sync validates dependencies from lock entries and must not recompute locked package dependencies from registries.
- Locked sync may validate each acquired package's `manifest.oct` against the locked node, but registry metadata is not authoritative in locked mode.

### Deterministic ordering

Recommendation: write `Packages` in deterministic dependency-before-importer order when the graph planner already has that order. Within equal graph levels, sort by `Name`, then `Version`.

If PM6 cannot conveniently reuse planner order, sorting the full package list by `Name`, then `Version` is acceptable because dependencies are explicit edges. However, dependency-before-importer order is friendlier for review because lower-level packages appear before packages that use them.

Repeated `oct pkg lock` runs with unchanged inputs should produce byte-identical `lock.octagon`.

## 10. Commands and CLI behavior

### PM6 M0 commands

```sh
oct pkg lock
oct pkg sync --locked
```

### Deferred PM6.1 command

```sh
oct pkg lock --check
```

`oct pkg lock --check` would verify that the current manifest plus current registry resolution matches `lock.octagon` without writing. It is useful for CI, but PM6 M0 should stay focused on lock writer plus locked sync.

### CLI details

`oct pkg lock`:

- accepts no arguments in PM6 M0;
- writes `lock.octagon` in the current project root;
- resolves direct and transitive graph from the current manifest;
- reports package count;
- reports local mutability warnings;
- exits non-zero on graph errors.

Example:

```text
Resolved package graph: 3 packages
warning: local source Fixtures@0.1.0 is mutable; lock.octagon records source path but not content digest
Wrote lock.octagon
```

`oct pkg sync --locked`:

- accepts no additional arguments in PM6 M0;
- fails without `lock.octagon`;
- syncs direct and transitive packages from lock entries;
- reports locked Git commit for Git entries;
- reports local mutability limitations for local entries;
- writes `.oct-package-source.oct` metadata;
- exits non-zero on manifest drift, stale lock, missing source, Git checkout failure, or invalid lock graph.

Example:

```text
Loaded lock.octagon
Synced MathCore 0.2.0 from locked git commit abcdefabcdefabcdefabcdefabcdefabcdefabcd
Synced SignalTools 0.1.0 from locked git commit 0123456789abcdef0123456789abcdef01234567
Package sync complete: 2 packages
```

## 11. Interaction with PM4 rolling sync

- `oct pkg sync` continues PM4 behavior.
- `oct pkg lock` uses the PM4 graph planner/resolver to create a lock graph.
- `oct pkg sync --locked` uses `lock.octagon` as authority.
- If a registry Git `Ref` points to a newer commit after lock creation, `--locked` ignores the registry state and uses the locked `ResolvedCommit`.
- If a registry entry is deleted after lock creation, `--locked` should still sync if the lock contains enough source information to acquire the source.
- If a source remote is unavailable, locked sync still fails unless the source is otherwise locally reachable. PM6 does not add an offline source cache.
- Rolling sync and locked sync can both continue writing `.oct-package-source.oct`; the metadata should reflect the actual source used for that sync.

This preserves the PM4 real path while adding an explicit reproducible path.

## 12. Manifest drift behavior

Locked sync must compare the current root manifest with `lock.octagon` before syncing.

### Manifest adds a dependency

If `manifest.oct` adds a direct dependency that is not represented in the lock graph, locked sync fails:

```text
pkg sync --locked: manifest dependency SignalTools@0.1.0 is not locked; run oct pkg lock
```

### Manifest removes a dependency

Recommendation: fail if packages remain in the lock but are no longer reachable from the current manifest root dependency set.

Rationale:

- stale lock entries can hide accidental dependency drift;
- strict failure teaches users that `lock.octagon` represents the current project graph, not a historical append-only cache;
- users can repair by running `oct pkg lock`.

### Manifest changes a version

If a direct dependency changes version, locked sync fails because the lock no longer exactly satisfies manifest intent:

```text
pkg sync --locked: manifest dependency SignalTools@0.2.0 is not locked; lock contains SignalTools@0.1.0; run oct pkg lock
```

### Transitive graph changes upstream

- rolling `oct pkg sync` sees the new graph because it resolves current registries and source manifests;
- locked `oct pkg sync --locked` keeps the old locked graph;
- `oct pkg lock` intentionally refreshes the graph.

### Strict policy summary

PM6 locked sync should require the lock graph to exactly satisfy current manifest root dependencies. Extra packages in the lock are allowed only if reachable from locked root dependencies. Unreachable packages are a stale-lock error.

## 13. Local source lock policy

Options considered:

A. Allow local lock entries with `Mutable: true`.
B. Refuse to lock local sources until tree digests exist.
C. Lock local source by copying current source metadata only, with no reproducibility guarantee.

Recommendation: A.

Allow local entries for local development convenience, but make mutability explicit:

- set `Mutable: true`;
- record `Source` and `Path`;
- leave `ResolvedCommit` empty;
- print a warning when writing the lock;
- optionally print a concise note during locked sync when syncing mutable local entries.

This supports demos and local experiments while refusing to overstate reproducibility. PM7 should add tree digest/content hashing or content-addressed artifacts for stronger local-source reproducibility.

## 14. Git source lock policy

Git sources are the strongest PM6 M0 reproducibility story because Git already provides commit identity.

Rules:

- `oct pkg lock` uses PM4's resolved Git commit data.
- Each Git lock entry must contain a full `ResolvedCommit` SHA.
- `Ref` remains in the lock as diagnostic/user-facing original request.
- `oct pkg sync --locked` checks out `ResolvedCommit` directly with detached checkout behavior equivalent to:

  ```sh
  git checkout --detach <ResolvedCommit>
  ```

- locked sync does not follow mutable `Ref` values.
- if checkout of `ResolvedCommit` fails, locked sync fails clearly with package, version, source, ref, commit, and operation.
- if `ResolvedCommit` is missing or not full-length, lock loading or locked sync fails before source acquisition.

## 15. Registry dependency and lockfile authority

The lockfile should include enough data to sync locked packages without registry lookup.

### Stored fields

Each package entry should store:

- original `Registry` name for diagnostics;
- original `RegistryPath` for diagnostics and traceability;
- `SourceKind`;
- `Source` as the acquisition source string used by PM4 resolution;
- `Path` as the package-root path inside the source;
- `Ref` and `ResolvedCommit` for Git entries.

### Local path policy

Path portability is the hardest PM6 M0 issue.

Recommendation:

- preserve `Source` in the form PM4 resolution used for acquisition;
- preserve `Registry` and `RegistryPath` for diagnostics;
- for local entries, document that relative local source paths are portable only when the same registry/project relative layout exists;
- do not require registry lookup for locked sync, but allow PM6 implementation to resolve legacy relative local paths using the stored registry path when needed for compatibility;
- surface path resolution in diagnostics so users can see whether a path was resolved relative to project root, registry root, or absolute source.

Future PM7 tree digests and `.octpkg` artifacts should reduce reliance on local filesystem layout.

### Registry deletion

If a registry entry disappears after lock creation, locked sync should still work for locked entries when the lock's source fields are sufficient. Registry configuration should not be mandatory for already locked Git packages.

## 16. Integrity/digest deferral

PM5 explicitly defers:

- package tree digest;
- source digest;
- registry index digest;
- content-addressed `.octpkg` artifact digest;
- reproducible archive format;
- package signatures;
- signing key management;
- federation;
- P2P transport;
- publishing;
- auth;
- registry mirrors;
- binary sidecar provenance;
- binary sidecar distribution.

PM6 M0 `lock.octagon` pins package identity and source identity. For Git sources, it pins a Git commit. It does not prove content integrity beyond Git commit identity and whatever trust the user places in their Git remote/local clone. For local sources, it does not prove content integrity at all without future tree digests.

## 17. Wrapper package interaction

- Lockfile records wrapper packages like any other package node.
- Lockfile generation does not build wrapper sidecars.
- Locked sync does not build wrapper sidecars.
- W8b current-package wrapper build remains unchanged.
- `oct pkg build-wrappers --allow-native` remains explicit and current-package-focused in PM6 M0.
- Future dependency wrapper build selection can use the lock graph to know which wrapper package source to build.
- No binary sidecar lock, sidecar digest, sidecar provenance, or native artifact distribution is part of PM6.

This preserves W8a/W8b's lifecycle separation: source acquisition is not native build execution, and runtime wrapper discovery remains unchanged.

## 18. Test plan for PM6

PM6 implementation should add tests for:

1. Git registry package with mutable tag/ref:
   - `oct pkg lock` writes `ResolvedCommit`;
   - update tag/ref to a new commit;
   - `oct pkg sync --locked` still checks out the old commit.
2. Missing lock:
   - `oct pkg sync --locked` fails with guidance to run `oct pkg lock`.
3. Manifest drift:
   - dependency added -> locked sync fails;
   - dependency removed -> stale lock fails;
   - dependency version changed -> locked sync fails.
4. Local source:
   - lock writes local entry with `Mutable: true`;
   - lock command prints the mutable-source warning.
5. Transitive graph:
   - lock includes direct and transitive packages;
   - locked sync materializes both;
   - locked sync can avoid registry lookup when lock contains source information.
6. Deterministic lock ordering:
   - repeated lock writes produce identical bytes.
7. File naming:
   - no `oct.lock` is created;
   - no `lock.oct` is created.
8. Rolling mode separation:
   - plain `oct pkg sync` does not generate `lock.octagon`;
   - plain `oct pkg sync` ignores an existing lockfile.
9. Wrapper packages:
   - locking a wrapper package records it as a node;
   - locked sync does not build sidecars.
10. Metadata:
    - locked sync still writes `.oct-package-source.oct` into synced package directories.
11. Lock validation:
    - Git entry missing `ResolvedCommit` fails;
    - unsupported `LockVersion` fails;
    - cyclic lock graph fails;
    - duplicate/conflicting package nodes fail.

PM5 design-only validation should run the existing package-manager/project/CLI tests to confirm no production behavior changed.

## 19. PM6 recommendation

Choose:

```text
PM6 — implement optional lock.octagon M0
```

PM6 should implement both lock writer and locked sync M0. Implementing only a lock writer would leave the artifact unvalidated by the real motivating path. Designing tree digests before any lockfile would be useful but would delay the simpler Git-commit reproducibility improvement already enabled by PM4's `ResolvedCommit` metadata.

Recommended sequencing:

1. Refactor PM4 graph planning enough to expose deterministic resolved graph data without changing rolling sync behavior.
2. Add Octagon lock encode/decode types.
3. Implement `oct pkg lock`.
4. Implement `oct pkg sync --locked` using lock graph authority.
5. Add strict manifest drift validation.
6. Add PM6 tests.
7. Defer PM7 digest/artifact design.

## 20. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Users assume lockfile is default | Keep command surface explicit: `oct pkg sync` is rolling, `oct pkg sync --locked` is locked. Document this prominently. |
| Stale lockfile confusion | Locked sync fails on manifest drift and unreachable stale nodes. Add `oct pkg lock --check` in PM6.1. |
| Local source mutability | Allow local entries only with explicit `Mutable: true` and warnings. Defer integrity claims until tree digest/artifact support. |
| Git remote unavailable | Locked sync pins commits but still needs source availability unless future cache/artifact support exists. Diagnostics must name source/ref/commit. |
| Mutable refs before lock | `oct pkg lock` records full `ResolvedCommit`; warnings belong at lock creation. |
| Lock path portability | Store source, path, registry, and registry path; document local path limitations; PM7 artifacts/digests improve this. |
| No content digest yet | State that PM6 pins identity, not full content integrity, except Git commit identity. |
| Transitive graph drift | Rolling sync intentionally drifts; locked sync uses lock edges and manifest drift checks. |
| Wrapper native sidecar confusion | Lock records wrapper packages as source nodes only. Native build remains explicit and separate. |
| CI behavior unclear | Recommend `oct pkg sync --locked` for CI and defer `oct pkg lock --check` to PM6.1. |
| Generated lockfile merge conflicts | Deterministic ordering and no timestamps reduce churn. Users can regenerate with `oct pkg lock`. |
| Lockfile mistaken for Oct source | Use `.octagon`, not `.oct`; do not include `package` or functions. |

## 21. Final recommendation

PM6 should implement optional `lock.octagon` M0.

Exact PM6 scope:

- project-root `lock.octagon`;
- `oct pkg lock`;
- `oct pkg sync --locked`;
- deterministic lock graph encode/decode;
- strict manifest drift validation;
- Git lock entries pinned by full `ResolvedCommit`;
- local lock entries allowed with explicit mutability warning;
- locked sync writes `.oct-package-source.oct` metadata just like rolling sync;
- no automatic lock generation from plain `oct pkg sync`.

Exact PM6 non-goals:

- content-addressed `.octpkg` artifacts;
- package tree digests;
- source digests;
- registry index digests;
- signing;
- federation;
- P2P;
- publishing;
- auth;
- registry mirrors;
- binary sidecar distribution;
- wrapper sidecar build changes;
- wrapper runtime discovery changes;
- semver ranges;
- `latest`;
- solver/backtracking behavior;
- Oct syntax changes;
- package-manager rewrite;
- changing `.oct-package-source.oct` into a lockfile.

File name/location:

```text
<project-root>/lock.octagon
```

Command surface:

```sh
oct pkg lock
oct pkg sync --locked
```

Lock schema:

- root: `LockVersion`, `GeneratedBy`, `Root`, `Packages`;
- package: `Name`, `Version`, `Kind`, `SourceKind`, `Source`, `Ref`, `ResolvedCommit`, `Path`, `Registry`, `RegistryPath`, `Mutable`, `Dependencies`;
- dependency edge: `Name`, `Version`.

Rolling vs locked behavior:

- `oct pkg sync` is rolling and ignores the lockfile by default;
- `oct pkg lock` freezes the current resolved graph;
- `oct pkg sync --locked` uses exactly the locked graph and fails on missing/stale/incomplete locks.

Local source policy:

- allowed for convenience;
- marked mutable;
- not claimed reproducible without future digest/artifact support.

Git source policy:

- `Ref` is diagnostic original intent;
- `ResolvedCommit` is authoritative for locked sync;
- locked sync checks out the commit detached.

PM6 test plan:

- Git mutable-ref pinning;
- missing lock failure;
- manifest drift failures;
- local mutable warnings;
- direct plus transitive graph lock/sync;
- deterministic lock output;
- no lock from plain sync;
- wrapper package source-only locking;
- `.oct-package-source.oct` still written by locked sync;
- invalid lock schema/graph diagnostics.

This reaches PM5 success: it defines the optional lockfile model while leaving PM4 production behavior unchanged.
