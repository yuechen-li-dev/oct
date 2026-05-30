# oct pkg

## Overview

`oct pkg` manages package fetch, cache inspection, and dependency sync.
Package metadata is declared in `manifest.oct`.
Dependency sync is explicit and command-driven.

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

## Wrapper package source contract

M5c defines wrapper package metadata validation only. A wrapper package declares `Kind: "wrapper"` and supplies `Wrappers: Wrapper[]` in `manifest.oct`.
The metadata is validated and exposed to package-manager metadata, but sidecar build commands, registry generation, lockfiles, runtime sidecar resolution, wrapper execution, generic wrapper lowering, and handle-backed wrapper support are future work.

Each `Wrapper` must declare non-empty `Name`, `Family`, `Protocol`, `SidecarCommand`, and `GoModuleDir` fields plus a non-empty `Functions` array.
`Protocol` must be exactly `octxiliary.v0`.
`GoModuleDir` is a package-local relative path to the Go module directory; absolute paths and `..` parent traversal are rejected.
Within one package, duplicate `Wrapper.Name` values and duplicate `Wrapper.Family` values are rejected.

Each `WrapperFunction` must declare non-empty `OctName`, non-empty `WireName`, `Args: String[]`, non-empty `Return`, and `Fallible: Bool`.
`Args` elements and `Return` must use one of the supported M5c transport type strings:

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

M5d adds internal package-manager planning support for wrapper manifests. Wrapper manifests can now be inspected as native wrapper build plans for fetched packages and synced dependency graphs. A plan identifies which packages are `Kind: "wrapper"`, each package-local `GoModuleDir`, the resolved package-local Go module path, sidecar command names, wrapper families, protocols, and exposed function metadata.

Wrapper build planning is inspection-only. It does not build Go modules, run `go mod download`, run `go build`, generate sidecar binaries, generate lockfiles, discover runtime sidecars, lower generic wrappers, execute sidecars, or change the Octxiliary protocol.

Planning treats wrapper packages as native-code packages and marks them as requiring explicit future native build permission. Permission prompts, native build execution, runtime registry consumption, and `.octagon` lockfiles remain future work.

## Octxiliary registry artifact

M5e adds an internal package-manager artifact model that can render wrapper build plans as deterministic `.octagon` text. The registry is inert metadata for future build, runtime, and compiler integration; writing it does not build, download, execute, discover, or register sidecars. It also does not generate lockfiles and does not add compiled wrapper lowering.

The registry version is the stable string `octxiliary.registry.v0`. A registry lists the resolved sidecars from the wrapper build plan, including package name, wrapper name, family, protocol, sidecar command, Go module directory, resolved Go module path, and function metadata. Function metadata includes the Oct name, wire name, argument transport type strings, return transport type string, and fallibility.

The `.octagon` artifact uses data-only record names `OctxiliaryRegistry`, `OctxiliarySidecar`, and `OctxiliaryFunction`; no package declaration is emitted. Registry output is deterministic, uses safe string quoting, and preserves the Go module path exactly as resolved by wrapper build planning. The artifact is a resolved planning artifact, not a build lockfile and not a final install/cache layout contract.

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
