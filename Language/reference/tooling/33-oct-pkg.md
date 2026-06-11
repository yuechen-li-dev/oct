# oct pkg

## Overview

`oct pkg` manages package fetch, cache inspection, local registry configuration, dependency addition/sync, and planning-only wrapper inspection.
Package metadata is declared in `manifest.oct`.
Dependency sync is explicit and command-driven.


## Canonical registry and v0.1 package boundaries

The canonical first-party registry exists at `Registry/registry.oct` in this repository. PM7 registry use is local/source-controlled, not hosted. A consumer project configures a checkout with:

```sh
oct pkg registry add oct <path-to-oct-repo>/Registry
```

`Mathematics` is the canonical math package name in that registry. There is no `Math` alias.

Use exact dependencies and explicit sync:

```sh
oct pkg add Mathematics@0.1.0
oct pkg sync
```

Optional lockfile use is explicit:

```sh
oct pkg lock
oct pkg sync --locked
```

Wrapper package sync copies source and manifest metadata only. It does not build sidecars, run sidecars, or create `.oct/wrappers`; native wrapper sidecars require `oct pkg build-wrappers --allow-native`.

## Manifest schema

`manifest.oct` declares package metadata in package `Manifest`.
The current schema is:

```oct
package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Dependencies: Dependency[]

    // Optional metadata
    Kind: String
    EntryMilestone: String
    Wrappers: Wrapper[]
}

record Dependency {
    Name: String
    VersionRequirement: String

    // Optional metadata
    Source: String
}

record Wrapper {
    Name: String
    Family: String
    Protocol: String
    SidecarCommand: String
    GoModuleDir: String
    Functions: WrapperFunction[]
}

record WrapperFunction {
    OctName: String
    WireName: String
    Args: String[]
    Return: String
    Fallible: Bool
}
```

Required `PackageManifest` fields are `Name`, `Version`, `Description`, and `Dependencies`.
Optional `PackageManifest` fields are `Kind`, `EntryMilestone`, and `Wrappers`.
Required `Dependency` fields are `Name` and `VersionRequirement`.
Optional `Dependency` fields are `Source`.
Optional fields are allowed by the manifest schema, but record literals must still match the fields declared in the local `manifest.oct` record definitions. `Wrapper` and `WrapperFunction` records are required only when `PackageManifest` declares `Wrappers: Wrapper[]`.

## Package kinds

Missing `Kind` or `Kind: ""` defaults to `pure`.
Allowed package kind values are:

- `pure`: an ordinary Oct source package.
- `experiment`: an experiment package; it may specify `EntryMilestone`.
- `wrapper`: a package with Oct source plus native wrapper metadata declared in `manifest.oct`.

`EntryMilestone` is valid only for `Kind: "experiment"`; `EntryMilestone: ""` is treated as omitted.
`Wrappers` must be omitted or empty for `pure` and `experiment` packages.
`Kind: "wrapper"` requires `Wrappers: Wrapper[]` to be declared and supplied as a non-empty array in the returned `PackageManifest` literal.


## Local package registries

`oct pkg` supports a PM2 local, source-oriented package registry flow. Registries are configured per project in:

```text
.oct/registries.oct
```

There is no user-global registry config. Manage configured registries with:

```text
oct pkg registry add <name> <path>
oct pkg registry list
oct pkg registry remove <name>
```

Registry names must be simple stable names matching `[A-Za-z][A-Za-z0-9_-]*`. Relative registry paths are stored as provided and resolved relative to the project root. `oct pkg registry list` succeeds with a clear message when no registries are configured.

A local registry root contains `registry.oct` with this shape:

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

fn Registry() -> RegistryIndex {
    return RegistryIndex {
        Packages: [
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
        ]
    }
}
```

PM4 supports `SourceKind: "local"` and `SourceKind: "git"`. `Ref` is optional in the record shape for compatibility with PM2 local registries, but its value must be omitted or empty for local sources and must be non-empty for Git sources. Git sources are acquired with the installed `git` executable using `git clone <Source>` followed by a detached checkout of `Ref`; supported source strings are whatever local Git accepts, including HTTPS URLs, SSH URLs when the user environment supports them, local repository paths, and `file://` repository URLs. Submodules, LFS, auth storage, registry cloning, publishing, mirrors, federation, P2P, signing, lockfiles, `oct.lock`, and content-addressed `.octpkg` artifacts are not part of PM4.

Registry `Kind` values are `library`, `experiment`, and `wrapper`; `library` maps to ordinary manifests with omitted/empty/normalized pure `Kind`. Versions are exact text only. There is no `latest`, range solving, semver interpretation, solver, or backtracking behavior.

### Canonical first-party registry

The Oct repository includes a PM7 canonical first-party registry at:

```text
Registry/registry.oct
```

This registry is local, source-controlled project data. It is not a hosted registry, does not imply publishing or auth support, and can be configured from a checkout with:

```sh
oct pkg registry add oct <repo-root>/Registry
oct pkg add Mathematics@0.1.0
oct pkg sync
```

From examples or tests nested inside the Oct repository, use a relative path that reaches the repository registry, for example:

```sh
oct pkg registry add oct ../../Registry
```

Canonical registry entries point at first-party library package source under `Libraries/`. Wrapper packages can be synced as source, but `oct pkg sync` does not build sidecars and does not create `.oct/wrappers`. Use `oct pkg build-wrappers --allow-native` only for current-package wrapper sidecars. `lock.octagon` remains optional; use `oct pkg lock` and `oct pkg sync --locked` when a project wants lockfile-backed sync.

The canonical math package is `Mathematics`; PM7 does not define a `Math` alias.

Add an exact registry dependency with:

```text
oct pkg add <Name>@<exact-version> [--registry <name>]
```

`oct pkg add` resolves the exact package/version first, rejects duplicate dependency names, and writes a `Dependency` entry without `Source`. It does not sync automatically. If multiple registries contain the same exact package/version, the command reports ambiguity and asks for `--registry <name>`.

`oct pkg sync` preserves explicit `Dependency.Source` behavior. Dependencies without `Source` are resolved through the project's configured registries and recursively synced as an exact-version dependency graph. Each graph node is `<Name>@<exact-version>`; duplicate identical nodes are synced once, while the same package name with different exact versions is an error. Cycles, missing transitive dependencies, ambiguous transitive registry entries, and non-exact transitive `VersionRequirement` values fail with dependency-chain diagnostics such as `App -> SignalTools@0.1.0 -> MathCore@0.2.0`.

Synced registry packages are copied into:

```text
.oct/packages/<Name>/<Version>/
```

Each synced package receives `.oct-package-source.oct` metadata with no timestamp. PM4 metadata includes diagnostic-only `Ref` and `ResolvedCommit` fields; local sources write both as empty strings, and Git sources record the requested ref plus the checked-out commit. If a Git ref is not a full 40-character commit SHA, sync warns and records the resolved commit, but this is not lockfile pinning. The project loader uses the exact manifest dependency version when searching `.oct/packages`; it does not fall back by name only. Sync copies wrapper package source, including `sidecars/...`, but it does not build sidecars and does not create `.oct/wrappers`. Use `oct pkg build-wrappers --allow-native` only for current-package wrapper sidecar builds.

## Wrapper package source contract

A wrapper package declares `Kind: "wrapper"` and supplies `Wrappers: Wrapper[]` in `manifest.oct`.
The metadata is validated and exposed to package-manager metadata. Planning and registry output are supported, but sidecar build commands, lockfiles, runtime sidecar resolution, wrapper execution, and native build lifecycle hardening remain future work.

Each `Wrapper` must declare non-empty `Name`, `Family`, `Protocol`, `SidecarCommand`, and `GoModuleDir` fields plus a non-empty `Functions` array.
`Protocol` must be exactly `octxiliary.v0`.
`GoModuleDir` is a package-local relative path to the Go module directory; absolute paths and `..` parent traversal are rejected.
Within one package, duplicate `Wrapper.Name` values and duplicate `Wrapper.Family` values are rejected.

Each `WrapperFunction` must declare non-empty `OctName`, non-empty `WireName`, `Args: String[]`, non-empty `Return`, and `Fallible: Bool`.
`Args` elements and `Return` must use one of the supported transport type strings:

- `Void`
- `Int`
- `Float`
- `Bool`
- `String`
- `String[]`
- `Bytes`

Within a single wrapper, duplicate `WrapperFunction.OctName` values and duplicate `WrapperFunction.WireName` values are rejected.
Do not declare sidecar build commands or runtime registry behavior beyond this source-level manifest contract.

## Wrapper build planning

Wrapper manifests can be inspected as native wrapper build plans for the current package and synced direct dependencies. A plan identifies which packages are `Kind: "wrapper"`, each package-local `GoModuleDir`, the resolved package-local Go module path, sidecar command names, wrapper families, protocols, exposed function metadata, and declared transport type metadata.

Wrapper build planning is inspection-only. It does not build Go modules, run `go mod download`, run `go build`, generate sidecar binaries, generate lockfiles, discover runtime sidecars, lower generic wrappers, execute sidecars, or change the Octxiliary protocol.

Planning treats wrapper packages as native-code packages and marks them as requiring explicit future native build permission. Permission prompts, native build execution, runtime registry consumption, and `.octagon` lockfiles remain future work.

## Octxiliary registry artifact

Wrapper build plans can be rendered as deterministic `.octagon` text. The registry is inert metadata for future build, runtime, and compiler integration; writing it does not build, download, execute, discover, or register sidecars. It also does not generate lockfiles and does not add compiled wrapper lowering.

The registry version is the stable string `octxiliary.registry.v0`. A registry lists the resolved sidecars from the wrapper build plan, including package name, wrapper name, family, protocol, sidecar command, Go module directory, resolved Go module path, and function metadata. Function metadata includes the Oct name, wire name, argument transport type strings, return transport type string, and fallibility.

The `.octagon` artifact uses data-only record names `OctxiliaryRegistry`, `OctxiliarySidecar`, and `OctxiliaryFunction`; no package declaration is emitted. Registry output is deterministic, uses safe string quoting, and preserves the Go module path exactly as resolved by wrapper build planning. The artifact is a resolved planning artifact, not a build lockfile and not a final install/cache layout contract.

## Wrapper planning command

Wrapper build planning is exposed through:

```text
oct pkg wrappers [--registry-out <path>]
```

`oct pkg wrappers` runs from the current directory as the package or project root. It reads the current `manifest.oct`, inspects wrapper metadata from the current package, fetches only direct dependencies that declare `Source`, and ignores non-fetchable dependencies such as local standard-library names. This keeps local wrapper metadata inspectable even when the manifest also names non-fetchable dependencies such as `OctStd`. The command prints a deterministic human-readable summary. The summary reports whether native wrappers are present, whether future native build permission would be required, the number of planned sidecars, and stable sidecar fields in plan order.

`oct pkg wrappers --registry-out <path>` builds the same plan, converts it to the inert Octxiliary registry artifact, and writes deterministic `.octagon` text to `<path>`. The path must end in `.octagon`; parent directories may be created by the writer. The command reports the written registry path.

Wrapper planning itself does not download Go modules, run `go mod download`, run `go build`, generate sidecar binaries, execute wrapper sidecars, discover sidecars at runtime, generate lockfiles, change the Octxiliary protocol, or add compiler/runtime registry consumption. It may fetch or reuse direct dependency package metadata only when a dependency declares `Source`. Native build permission prompts and any real native build execution remain future work. Existing `oct pkg get` and `oct pkg sync` commands do not implicitly write wrapper registries. The command prints `No wrapper sidecars were built or executed.` after summary/registry output.

## Related package scaffolding

`oct new` creates deterministic package scaffolds that can be inspected by package tooling:

```text
oct new library <Name>
oct new experiment <Name>
oct new wrapper-library <Name>
```

The current scaffold command has no flags. `<Name>` must be strict PascalCase (`[A-Z][A-Za-z0-9]*`) and is rejected rather than normalized when it contains whitespace, separators, dots, underscores, or reserved names. The target directory is always `./<Name>` relative to the current working directory, and the command fails if that target already exists.

`oct new wrapper-library <Name>` writes wrapper manifest metadata and package-local sidecar reference files, but it does not build or run the generated sidecar scaffold. The generated wrapper-library package can be inspected with `oct pkg wrappers`, and deterministic inert registry metadata can be written with `oct pkg wrappers --registry-out <path>`.

## Rules

- Manifest metadata is declared by `fn Manifest() -> PackageManifest` in `manifest.oct`.
- `PackageManifest` includes package identity fields and `Dependencies`.
- `Dependencies` is a `Dependency[]` describing direct dependencies.
- Current model is direct-dependency fetch/sync from declared sources or exact local registry entries.
- `oct pkg get <git-url>` fetches one source into cache (or reports cache hit).
- `oct pkg get` reports source, cache path, cache key, dependency count, and manifest identity when present.
- `oct pkg list` shows cached package entries.
- `oct pkg registry add/list/remove` manages project-local `.oct/registries.oct`.
- `oct pkg add <Name>@<exact-version> [--registry <name>]` adds a resolved registry dependency without `Source`.
- `oct pkg sync` reads the current project's manifest and syncs direct/source plus exact registry dependencies.
- `oct pkg sync` operates on the current directory as the project root.
- `oct pkg lock` writes an optional project-root `lock.octagon`; `oct pkg sync --locked` requires and syncs that locked graph.
- Sync output includes project path, manifest path, dependency count, per-dependency status, and completion line.
- `oct pkg wrappers` reads the current package wrapper metadata plus direct dependencies that declare `Source`, then prints a deterministic wrapper build plan summary.
- `oct pkg wrappers --registry-out <path>` writes an inert Octxiliary registry artifact to a `.octagon` path.
- `oct pkg wrappers` always reports that no wrapper sidecars were built or executed.
- Usage is strict: `oct pkg <get|list|sync|lock|registry|add|wrappers|build-wrappers>`, with wrapper usage `oct pkg wrappers [--registry-out <path>]`.
- Present limitations: package operations are manifest-driven and exact-version oriented; lockfiles do not add package tree digests or artifact integrity; wrapper planning has no runtime consumption, native build prompts, implicit native build execution, or sidecar execution; third-party wrapper manifest hardening and broader native build lifecycle work remain future work.

See also [13 Packages](../language/13-packages.md) for language-level `package` / `import` rules.

## Examples

Valid:

```text
oct pkg get https://example.com/repo.git
oct pkg list
oct pkg registry add local ../oct-registry
oct pkg registry list
oct pkg add SignalTools@0.1.0
oct pkg sync
oct pkg wrappers
oct pkg wrappers --registry-out .oct/octxiliary-registry.octagon
oct new wrapper-library OpenCV
```

Invalid:

```text
oct pkg sync extra-arg
oct pkg wrappers extra
oct pkg wrappers --registry-out
oct pkg wrappers --registry-out registry.txt extra
```
