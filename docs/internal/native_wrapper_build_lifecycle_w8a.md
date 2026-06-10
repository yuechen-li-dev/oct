# W8a native wrapper build lifecycle design

## 1. Executive summary

Oct now has enough wrapper infrastructure for a third-party-style package to describe a native sidecar and for Oct code to call manifest-declared raw wrapper functions when a sidecar is already present. W1 added the public `pkg/octxiliary` SDK for sidecar authors. W2 established manifest-first wrapper metadata. W4 and W5 added deterministic `oct new wrapper-library <Name>` scaffolding. W6 hardened the current manifest validation boundary. W7a/W7b added interpreted generic wrapper dispatch. Compiled mode already has generic Octxiliary lowering for manifest wrappers.

The missing piece is the native sidecar build/install lifecycle. Today, a wrapper package can say what sidecar command and package-local Go module directory it expects, but users still build and place that executable manually. That gap is acceptable for early tests, but it is not acceptable as the primary third-party wrapper path because native code must be explicit, inspectable, deterministic where possible, and tied to package identity. Oct must not grow npm-style postinstall behavior, silent package-sync builds, arbitrary build scripts, broadened runtime `PATH` lookup, or implicit native sidecar execution.

Recommended next implementation milestone:

```text
W8b — implement local Go-module wrapper build M0 using existing GoModuleDir metadata, explicit --allow-native, project-local .oct/wrappers/<goos>-<goarch>/ output, deterministic build summaries, and no runtime discovery change beyond env guidance.
```

W8b should introduce one explicit command:

```sh
oct pkg build-wrappers --allow-native
```

It should build only Go-module sidecars declared by the current project package's manifest in M0. It should not build during `oct pkg sync`, `oct pkg wrappers`, `oct new`, interpretation, compilation, registry rendering, or package inspection. It should not execute sidecars. It should not fetch package dependencies solely for wrapper builds. It should not add lockfiles, package-cache discovery, registry/federation/P2P behavior, permission prompts, sandboxing, arbitrary build scripts, cross-compilation, or Oct language syntax.

Deferred features include package graph build selection, package-cache/user-cache install locations, package digest binding, lockfiles/checksum policy, registry-provided wrapper build metadata, sandboxing, system dependency declarations, non-Go build systems, runtime package-aware discovery, stdlib sidecar migration, and trusted binary distribution.

## 2. Current wrapper lifecycle inventory

### 2.1 Scaffolding

`oct new wrapper-library <Name>` creates a package-local wrapper scaffold and sidecar reference source under `sidecars/octxiliary-<kebab>/`. The generated manifest is current-schema compatible and includes `Kind: "wrapper"`, `Wrappers`, `SidecarCommand`, `GoModuleDir`, and one manifest-only raw wrapper function. The scaffold intentionally does not build or run native code, generate lockfiles, alter dispatch, or install sidecars.

The generated Oct test only checks that ordinary package source loads before native build lifecycle support exists. The generated raw wrapper metadata is intentionally not called by scaffold tests. The README tells authors that native sidecar build and dispatch lifecycle support is future work and that no native code was built or run by scaffolding.

### 2.2 Manifest and registry planning

`oct pkg wrappers [--registry-out <path>]` is planning-only. It loads wrapper metadata for the current project package and dependencies with explicit `Source` metadata, renders a deterministic summary, and can write an inert `.octagon` registry artifact. It does not build Go modules, run `go mod download`, run `go build`, generate sidecar binaries, generate lockfiles, discover runtime sidecars, lower wrappers, execute sidecars, or change the Octxiliary protocol.

The current planning model records:

- package name, version, source, cache path, package kind, and wrapper count;
- wrapper name, family, protocol, sidecar command, `GoModuleDir`, resolved Go module path, functions, and transport types;
- whether native wrappers exist;
- whether future native build permission is required.

Planning validates deterministic conflicts such as duplicate sidecar commands, duplicate wrapper families, and duplicate Go module paths in the computed plan. The command ends by printing that no wrapper sidecars were built or executed.

### 2.3 Interpreted wrapper dispatch

Interpreted execution indexes `project.Program.Packages[*].Wrappers`. When ordinary source-function lookup fails, manifest-only raw wrapper functions can be resolved by package and `WrapperFunction.OctName`. A sidecar process is started only when a wrapper call actually executes. Merely loading a project, planning wrappers, compiling, or inspecting metadata does not start sidecars.

The interpreted generic client caches sidecar processes by `SidecarCommand`, performs the Octxiliary handshake when the first call starts the sidecar, sends typed generic transport requests, validates responses, and closes clients when the interpreter finishes. W7c/W7d improved test runtime lifecycle by sharing built test sidecars where safe and by isolating missing-sidecar tests.

### 2.4 Compiled wrapper dispatch

Compiled mode has generic Octxiliary lowering for manifest-declared wrapper functions. Generated Go emits a typed generic Octxiliary client that starts the sidecar only when a wrapper call executes. The compiled path also relies on discovery finding an executable sidecar; compilation itself does not build or install wrapper sidecars.

### 2.5 Runtime discovery today

Current interpreted and compiled sidecar discovery is intentionally narrow:

1. sibling executable location;
2. `OCT_WRAPPER_PATH`, either as a directory containing `SidecarCommand` or as an explicit executable path whose basename equals `SidecarCommand`;
3. no `PATH` lookup;
4. no package cache lookup;
5. no project-local `.oct/wrappers/<platform>` lookup.

This is a runtime execution discovery surface, not a build lifecycle. `OCT_WRAPPER_PATH` is the practical test/development bridge today. It should not become the permanent package-managed install story.

### 2.6 Standard-library sidecars

Current repository-owned sidecars live under top-level `cmd/octxiliary-*` directories. They are not package-local `sidecars/` directories generated by W5, and they are not installed through package-manager wrapper outputs. Tests generally build selected sidecars into temporary directories and point `OCT_WRAPPER_PATH` at those directories. W8b must not migrate stdlib sidecars.

## 3. Native build lifecycle goals

The native wrapper build lifecycle should satisfy these goals:

- native sidecar builds happen only through an explicit user command;
- package sync never builds native wrappers;
- `oct pkg wrappers` remains inspection/registry-only and never builds or executes sidecars;
- build does not execute sidecars unless a future validation mode explicitly opts into sidecar execution;
- no arbitrary postinstall or install hooks;
- no arbitrary shell scripts or manifest-provided build commands in M0;
- build output path is deterministic and inspectable;
- output is platform-aware;
- output names follow manifest-declared `SidecarCommand`, with platform executable suffix handling;
- the first implementation works with W5 generated wrapper packages;
- local development works before package federation, registries, caches, digests, and lockfiles exist;
- later cache/registry integration can bind outputs to package identity, digest, platform, and toolchain;
- CI/noninteractive behavior is deterministic and does not depend on prompts;
- runtime discovery remains intentionally narrow until package-aware discovery is designed.

## 4. Proposed command surface

### 4.1 Command family placement

The build command should live under `oct pkg` because wrapper sidecars are package metadata, package-local source, and package lifecycle artifacts. A top-level `oct build-wrappers` would be less clearly tied to package manifests. A nested `oct pkg wrappers build` would overload the current `oct pkg wrappers` inspection command, which has already been documented and tested as planning-only. Keeping build as a sibling command preserves a clean separation:

- `oct pkg wrappers`: inspect wrapper metadata and optionally render inert registry artifacts;
- `oct pkg build-wrappers`: explicitly build native wrapper sidecars.

### 4.2 Recommended M0 command

Recommended W8b M0 command:

```sh
oct pkg build-wrappers --allow-native
```

`--allow-native` should be required even though the command name says build. The redundancy is intentional: it makes CI scripts, copied commands, and audit logs explicitly show that native code compilation was authorized. It also leaves room for future non-native dry-run or metadata-only variants without weakening the native permission model.

If `--allow-native` is omitted, M0 should fail before running any build command and print a clear explanation such as:

```text
oct pkg build-wrappers failed: native wrapper builds require --allow-native; use oct pkg wrappers to inspect sidecars without building
```

M0 should not add interactive prompts. Prompts are hard to test, not CI-friendly, and can make native execution policy less deterministic.

### 4.3 Flags

M0 should require exactly one flag:

```text
--allow-native
```

Explicitly defer these flags:

- `--dry-run`: deferred because `oct pkg wrappers` is already the inspection/dry-run lane, and a second dry-run surface risks divergence in M0.
- `--package <name>`: deferred until dependency/package-graph build selection is implemented.
- `--clean`: deferred until stale-binary and output metadata policy exists.
- `--all`, `--deps`, `--current-only`: deferred until the package graph build model is broadened.
- `--platform`, `--goos`, `--goarch`: deferred because M0 should not cross-compile.
- `--registry`, `--cache`, `--install`: deferred until package federation/cache/lockfile design.

### 4.4 Build selection recommendation

W8b M0 should build wrappers for the current project package only.

Rationale:

- it avoids surprising package fetches or dependency builds;
- it aligns with local development of W5 generated wrapper packages;
- it keeps output ownership simple: one project root writes `.oct/wrappers/<platform>`;
- it avoids treating package-cache dependencies as trusted build sources before package identity/digest/lockfile policy exists;
- it keeps test fixtures focused.

A later milestone can extend selection to already-synced local/source dependencies, but W8b should not fetch dependencies solely to build wrappers.

## 5. M0 scope: Go-module sidecars only

W8b should support only the current production manifest field:

```oct
GoModuleDir: String
```

Conceptually, this is `BuildKind: "go-module"`, but M0 does not need to add that field. The build input for each sidecar is:

- current project package root;
- wrapper metadata from the current package manifest;
- `GoModuleDir`, validated as a package-local relative path;
- `SidecarCommand`;
- host platform `runtime.GOOS` and `runtime.GOARCH`.

Recommended build operation for each sidecar:

```sh
go build -o <project-root>/.oct/wrappers/<goos>-<goarch>/<sidecar-command>[.exe] .
```

Recommended working directory:

```text
<project-root>/<GoModuleDir>
```

Using `.` from the module directory avoids shell expansion, avoids arbitrary manifest-provided package arguments, and matches W5's package-local Go module shape. If a later package wants a module directory with multiple commands, schema must be extended intentionally rather than inferred through shell commands.

The `go` executable may be resolved from the user's `PATH` for this build command because the user explicitly requested native compilation through `oct pkg build-wrappers --allow-native`. This is distinct from runtime sidecar discovery: runtime must still not search `PATH` for sidecar executables.

M0 explicitly does not support:

- arbitrary `BuildCommand` fields;
- shell scripts;
- postinstall hooks;
- Make, CMake, Ninja, Bazel, Cargo, Python, or system package managers;
- system library installation;
- network policy enforcement beyond ordinary Go tool behavior;
- cross-compilation;
- signing, provenance, or binary trust;
- running the built sidecar as validation;
- changing Octxiliary wire protocol.

## 6. Output/install location

### 6.1 Options considered

A. Project-local output:

```text
.oct/wrappers/<goos>-<goarch>/<sidecar-command>
```

B. Package-local output:

```text
<package>/.oct/wrappers/<goos>-<goarch>/<sidecar-command>
```

C. User cache output:

```text
~/.oct/wrappers/<package-digest>/<goos>-<goarch>/<sidecar-command>
```

### 6.2 Recommendation

W8b should use project-local output first:

```text
<project-root>/.oct/wrappers/<goos>-<goarch>/<sidecar-command>[.exe]
```

For the current-package-only M0, project-local and package-local are the same physical tree when run at the package root. The design should still call it project-local because the directory is an execution/build artifact of the current checkout, not source that should be committed and not a stable package artifact.

Do not use a user cache in W8b. A user cache needs package identity, digest, lockfile, provenance, and invalidation policy that package federation has not designed yet. Do not invent a package digest in W8b.

### 6.3 Interaction with runtime discovery

W8b can build into `.oct/wrappers/<platform>`, but current runtime discovery will not search that path automatically. W8b should print a deterministic env hint after success, for example:

```sh
OCT_WRAPPER_PATH=.oct/wrappers/linux-amd64 oct test .
```

A follow-up W8c should design package-aware runtime discovery. If W8c adds project-local lookup, it must specify how interpreted runs find the project root and how compiled binaries behave when launched outside the project root. Until then, relying on an explicit `OCT_WRAPPER_PATH` keeps runtime execution visible and avoids silently changing discovery semantics.

## 7. Platform naming

Use the platform key:

```text
runtime.GOOS + "-" + runtime.GOARCH
```

Examples:

- `linux-amd64`
- `linux-arm64`
- `darwin-amd64`
- `darwin-arm64`
- `windows-amd64`

Windows output should append `.exe` when the declared `SidecarCommand` does not already end in `.exe`. The manifest `SidecarCommand` should remain the protocol/runtime command identity without `.exe` in ordinary cross-platform manifests; the build output layer handles host executable suffixes. Runtime discovery on Windows should be audited in W8c so command identity and executable suffix behavior remain consistent.

## 8. Build selection

W8b M0 should build only sidecars declared by the current project package's manifest. It should not build fetched dependencies, non-fetchable dependencies, or package-cache contents. It should not invoke `oct pkg sync` or `pkgmgr.Get` as part of build.

Future build selection can consider:

- current package plus already-loaded local/source dependencies;
- explicit `--package <name>`;
- explicit `--deps` or `--all`;
- package-cache builds bound to package digest and lockfile identity.

Until those policies exist, current-package-only is the safest useful slice and directly serves local W5-generated wrapper development.

## 9. Native permission model

M0 behavior should be exact and noninteractive:

- `oct pkg build-wrappers` without `--allow-native` fails before building anything.
- `oct pkg wrappers` remains the dry-run/inspection command and never builds or executes sidecars.
- `oct pkg build-wrappers --allow-native` prints the sidecars to be built before invoking `go build`.
- The build summary includes package, wrapper name, family, sidecar command, `GoModuleDir`, resolved source directory, output path, protocol, and function count.
- Build starts only after the explicit `--allow-native` flag is present.
- Build never runs sidecars.
- Build never broadens runtime discovery.
- Build never searches runtime `PATH` for sidecar commands.
- Build may use system `go` from `PATH` because the command is an explicit native build request.

Recommended summary shape:

```text
Wrapper native build plan:
platform: linux-amd64
output dir: .oct/wrappers/linux-amd64
sidecars: 1
requires native build permission: yes
native build permission: granted by --allow-native

* package OpenCV 0.1.0
  wrapper: open_cv
  family: OpenCV
  command: octxiliary-open-cv
  protocol: octxiliary.v0
  module: sidecars/octxiliary-open-cv
  source dir: /abs/project/OpenCV/sidecars/octxiliary-open-cv
  output: /abs/project/OpenCV/.oct/wrappers/linux-amd64/octxiliary-open-cv
  functions: 1

Built wrapper sidecars: 1
Set OCT_WRAPPER_PATH=.oct/wrappers/linux-amd64 to use these sidecars with current runtime discovery.
```

## 10. Build manifest/schema implications

### 10.1 Current production schema

The current production schema requires `GoModuleDir` on each `Wrapper`. It is validated as non-empty and package-local relative metadata. W5 scaffolds this field. `oct pkg wrappers` and registry rendering already include it.

### 10.2 W2 future schema concepts

Earlier design work anticipated future fields such as:

- `SourceDir`;
- `BuildKind`;
- `OutputName`;
- `RequiresNativePermission`;
- platform/system dependency fields.

These are still useful, but W8b should not add them unless implementation reveals a hard blocker. Adding schema fields would require loader validation, reference docs, scaffold changes, registry decisions, and compatibility policy. That is too much scope for a minimal local Go-module builder.

### 10.3 Recommendation

Use `GoModuleDir` only in W8b. Treat it as the current production field and the implicit M0 equivalent of `BuildKind: "go-module"` plus `SourceDir: <GoModuleDir>`. Do not break existing manifests. Do not require W5 scaffold changes.

Future schema migration can either:

- keep `GoModuleDir` as a backwards-compatible alias for `SourceDir` when `BuildKind == "go-module"`; or
- introduce optional `BuildKind` and `SourceDir` while continuing to accept `GoModuleDir` for existing wrapper packages.

Do not add arbitrary build commands to the manifest. If non-Go build systems are supported later, each build kind should be a bounded, inspectable native implementation rather than shell text from package metadata.

## 11. Runtime discovery integration plan

Options:

A. W8b only builds and tells users to set:

```sh
OCT_WRAPPER_PATH=.oct/wrappers/<goos>-<goarch>
```

B. W8b also changes interpreted and compiled runtime discovery to search project-local `.oct/wrappers/<platform>`.

C. W8b writes an env hint file or build summary only.

Recommendation: choose A for W8b, with summary guidance from C, and leave runtime discovery changes to W8c.

Reasons:

- W8a/W8b are build lifecycle work; runtime discovery has separate correctness concerns.
- Interpreted execution can often know a project root, but compiled binaries may run outside a project root.
- Searching the current working directory from compiled binaries could be surprising and brittle.
- Adding project-local lookup before package identity/digest policy could mask missing-sidecar tests or accidentally pick stale binaries.
- `OCT_WRAPPER_PATH` keeps the execution opt-in explicit for M0.

W8c should design package-aware runtime discovery. A possible search order to evaluate then is:

1. package/project-managed `.oct/wrappers/<platform>` path when a trustworthy project root is available;
2. sibling executable path;
3. `OCT_WRAPPER_PATH`;
4. no `PATH`.

However, W8c must decide whether project-local lookup should precede sibling executable lookup. Sibling-first preserves current packaged-binary behavior. Project-local-first makes local development outputs win. This is a multi-signal decision and should use the repository's judgment pattern rather than an unexamined nested-if order.

## 12. Registry and package federation implications

W8b should not design full federation, but the build output shape should not block it.

Future integration requirements:

- build outputs should eventually be tied to package name, version, source identity, package digest, platform, and toolchain;
- registry metadata should expose native wrapper source/build metadata, not silently trusted binaries;
- `.octpkg` artifacts should include source first, not built executables, until binary trust/provenance is designed;
- built binaries may be cached later, but cache hits must be governed by digest/provenance and invalidation policy;
- P2P fetch should deliver package artifacts, not executable trust identity;
- native build artifacts need lockfile/checksum policy before package-cache discovery or user-cache install becomes default;
- registry/federation should not introduce postinstall hooks or arbitrary native execution.

Project-local `.oct/wrappers/<platform>` is therefore an intentionally local, disposable build output. It is not a package identity contract.

## 13. Test plan for W8b

Recommended W8b tests:

1. Scaffold a generated wrapper package:

   ```sh
   oct new wrapper-library OpenCV
   ```

2. Patch the generated sidecar `go.mod` in the test fixture with a test-local replace:

   ```text
   replace github.com/yuechen-li-dev/oct => <repo-root>
   ```

3. Run:

   ```sh
   oct pkg build-wrappers --allow-native
   ```

4. Assert that the binary exists at:

   ```text
   .oct/wrappers/<goos>-<goarch>/octxiliary-open-cv[.exe]
   ```

5. Assert `oct pkg build-wrappers` without `--allow-native` fails before creating output.

6. Assert `oct pkg wrappers` still does not create `.oct/wrappers`, does not invoke `go build`, and still prints that no wrapper sidecars were built or executed.

7. Assert invalid or missing `GoModuleDir` reports a clear manifest/build-plan error.

8. Assert a missing Go module directory reports the package, wrapper, declared `GoModuleDir`, and resolved source directory.

9. Assert no sidecar execution during build. A fixture sidecar can write a sentinel file only from `main`; after build the sentinel must not exist.

10. Assert deterministic summary content includes package, wrapper, family, sidecar command, source directory, output path, platform, and function count.

11. If W8b keeps discovery unchanged, run an interpreted wrapper fixture with:

    ```sh
    OCT_WRAPPER_PATH=.oct/wrappers/<goos>-<goarch> oct test .
    ```

12. If a later W8c adds discovery, add a separate fixture proving the built sidecar is found without `OCT_WRAPPER_PATH` only when the discovery preconditions are satisfied.

Suggested Go test coverage:

- `internal/pkgmgr`: plan-to-build-input selection for current package only and output path calculation if implemented there;
- `internal/newpkg`: generated scaffold remains buildable after test-local `replace`;
- `cmd/oct`: CLI flag validation, permission failure, successful build summary, inert `oct pkg wrappers`, and missing module diagnostics.

## 14. Candidate next milestones

Candidates considered:

A. W8b — implement local Go-module wrapper build M0.

B. W8b — first add schema fields `BuildKind` / `SourceDir` design-only.

C. W8b — implement runtime discovery for `.oct/wrappers/<platform>` first.

D. PM1 — package federation registry index design.

Recommendation: choose exactly A.

```text
W8b — implement local Go-module wrapper build M0 with existing GoModuleDir metadata, oct pkg build-wrappers --allow-native, current-package-only selection, project-local .oct/wrappers/<goos>-<goarch>/ output, deterministic summaries, and no runtime discovery change except OCT_WRAPPER_PATH guidance.
```

Rationale:

- it removes the immediate local-development blocker for W5 wrapper packages;
- it preserves the no-silent-native-execution product principle;
- it uses the schema already implemented and validated;
- it avoids premature federation/cache/lockfile coupling;
- it keeps runtime discovery changes separate and auditable.

## 15. Risks and mitigations

| Risk | Mitigation in W8b M0 |
| --- | --- |
| Silent native execution | Build only through `oct pkg build-wrappers --allow-native`; no build in sync, wrappers, new, run, test, or build. |
| Build scripts / arbitrary command execution | Support only built-in `go build` for package-local `GoModuleDir`; no manifest build commands or shell scripts. |
| Go module network downloads | Document that the Go tool may download modules; do not fetch Oct packages as part of build; tests should use `replace` for repo-local SDK. Future policy can add vendoring/offline controls. |
| System dependency failures | Report `go build` failure with package, wrapper, source dir, and output path; do not install system dependencies. |
| PATH hijacking | Runtime sidecar discovery still does not search `PATH`; build may use `go` from `PATH` only after explicit native build authorization. |
| Platform naming bugs | Centralize platform key as `runtime.GOOS + "-" + runtime.GOARCH`; test at least host platform path rendering. |
| Stale binaries | Output path is deterministic and overwritten by `go build -o`; defer clean/invalidation metadata until lockfile/cache design. |
| Package identity/digest absent | Keep output project-local and disposable; do not put it in user cache or package cache. |
| Cross-platform builds | Do not support cross-compilation in M0; build only host platform. |
| Wrapper build output masking missing-sidecar tests | Do not add automatic runtime discovery in W8b; tests must opt in with `OCT_WRAPPER_PATH`. |
| Running sidecars during build | Never execute built binaries; add sentinel test. |
| CI flakiness | No prompts; deterministic command; tests patch `go.mod` with local `replace`; keep dependency selection current-package-only. |
| Windows executable extension | Append `.exe` to output on Windows; audit runtime suffix behavior in W8c. |
| Duplicate sidecar commands | Reuse existing wrapper plan conflict validation before building. |
| Overbroad package builds | M0 builds current project package only; dependency builds deferred. |

## 16. Final recommendation

W8b exact scope:

- add `oct pkg build-wrappers --allow-native`;
- fail without `--allow-native` before any build work;
- build current project package wrapper sidecars only;
- support only current `GoModuleDir` Go-module sidecars;
- run `go build -o <project-root>/.oct/wrappers/<goos>-<goarch>/<sidecar-command>[.exe] .` from `<project-root>/<GoModuleDir>`;
- create deterministic project-local output directories;
- print package/wrapper/family/command/source/output/platform summaries;
- never execute sidecars;
- print `OCT_WRAPPER_PATH=.oct/wrappers/<platform>` guidance after successful builds;
- add focused tests for permission failure, successful build, inert `oct pkg wrappers`, missing module errors, and no sidecar execution.

W8b exact non-goals:

- no `oct pkg wrappers` behavior change;
- no sidecar execution;
- no runtime discovery change;
- no package-cache discovery;
- no dependency/package-graph builds;
- no package fetches solely for build;
- no lockfiles;
- no registry/federation/P2P;
- no permission prompts;
- no sandboxing;
- no arbitrary build scripts;
- no runtime `PATH` sidecar lookup;
- no Octxiliary wire protocol changes;
- no Oct syntax changes;
- no `@extern` or `EXTERNAL { ... }`;
- no stdlib sidecar migration;
- no package-manager architecture rewrite.

Deferred features:

- W8c package-aware runtime discovery for project-local wrapper outputs;
- dependency/package-closure build selection;
- schema migration to optional `BuildKind`, `SourceDir`, `OutputName`, and native/system dependency metadata;
- clean/stale binary policy;
- lockfile/checksum/provenance policy;
- user-cache/package-cache install paths;
- registry/federation/P2P integration;
- trusted binary distribution;
- non-Go build kinds;
- cross-compilation;
- explicit validation modes that may run sidecars under future opt-in policy.
