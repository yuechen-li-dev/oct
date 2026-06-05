# W2 Third-party native wrapper manifest design

## 1. Executive summary

W2 solves the declaration gap between the new public Go sidecar SDK and usable third-party Oct wrapper packages. W1 made native sidecar authoring easier by adding `pkg/octxiliary`, but it deliberately did not define how a package declares its Oct API, wire ABI, native sidecar source, build lifecycle, safety policy, interpreted dispatch, compiled dispatch, or runtime discovery. This document is a repo-grounded design/audit milestone only: it recommends the package and manifest model for those declarations without changing production behavior.

The recommendation is **manifest-first wrapper metadata**. Oct already has wrapper package metadata machinery in `manifest.oct`: `Kind: "wrapper"`, `Wrappers`, `Wrapper`, `WrapperFunction`, optional `WrapperTransportType`, and optional `WrapperTransportField`. Third-party wrappers should extend and stabilize that path before adding syntax. `@extern`, extern body elision, or other language sugar should remain deferred until manifest validation, package-manager planning, interpreter dispatch, compiler consumption, sidecar build/install, and registry output all agree on a stable manifest workflow.

Relationship to W1:

- W1 SDK handles the Octxiliary process protocol for native authors: handshake, framing, request parsing, response construction, typed argument helpers, and dispatcher behavior.
- W2 manifest design handles the package ABI declaration: which Oct functions map to which wire functions, which sidecar family/command handles them, which transport types are legal, and which native source directory/build metadata exists.
- Future package-manager work handles explicit native build/install. Future interpreter/compiler work consumes the same manifest metadata for dispatch/lowering.

Recommended next implementation milestone after W2: **W4 — wrapper package manifest validation / registry artifact hardening**. The current code already has substantial manifest extraction, wrapper planning, and deterministic `.octagon` registry rendering, but the schema is still Go-module-specific, does not validate Oct stubs against manifest functions, and does not expose native-sidecar source/build metadata in registry output. Hardening the manifest/registry contract first gives W3 interpreted dispatch a reliable ABI to consume and prevents third-party wrappers from becoming a collection of special cases.

Deferred features for W2 and the next milestone:

- No `@extern` and no Oct language syntax changes.
- No production package-manager sidecar build lifecycle.
- No third-party interpreted wrapper dispatch implementation.
- No registry/federation/P2P architecture beyond noting required wrapper fields.
- No wire protocol change.
- No sidecar discovery behavior change.
- No native permission prompts or lockfiles.
- No stdlib sidecar migration.
- No broader default PATH lookup.

## 2. Current stdlib wrapper manifest inventory

### 2.1 Manifest shape

The current package manifest model is ordinary Oct code in `package Manifest`. `PackageManifest` requires `Name`, `Version`, `Description`, and `Dependencies`; it optionally accepts `Kind`, `EntryMilestone`, and `Wrappers`. `Dependency` requires `Name` and `VersionRequirement` and may include `Source`. Project and package-manager manifest validation both recognize wrapper records through `internal/manifestwrapper`.

A wrapper package is represented by:

```oct
Kind: "wrapper"
Wrappers: [
    Wrapper {
        Name: "..."
        Family: "..."
        Protocol: "octxiliary.v0"
        SidecarCommand: "..."
        GoModuleDir: "..."
        TransportTypes: [...]
        Functions: [...]
    }
]
```

`Kind: "wrapper"` has real validation semantics: if the kind is wrapper, `Wrappers` must be present and non-empty; if kind is `pure` or `experiment`, non-empty wrapper metadata is rejected. This is generally reusable for third-party packages.

### 2.2 Current audited stdlib wrappers

The requested inventory shows these manifests in the current repository:

| Package manifest | Wrapper(s) | Family | Command | GoModuleDir | Transport metadata | Functions |
|---|---:|---|---|---|---|---:|
| `Libraries/IO/manifest.oct` | `io-csv`, `xlsx` | `Csv`, `Xlsx` | `octxiliary-csv`, `octxiliary-xlsx` | `octxiliary` | `IO.Workbook` handle for Xlsx | 2 CSV + 5 Xlsx |
| `Libraries/Hash/manifest.oct` | `hash` | `Hash` | `octxiliary-hash` | `octxiliary` | none | 3 |
| `Libraries/Compression/manifest.oct` | `compression` | `Compression` | `octxiliary-compression` | `octxiliary` | none | 4 |
| `Libraries/Time/manifest.oct` | `time` | `Time` | `octxiliary-time` | `octxiliary` | none | 5 |
| `Libraries/Text/manifest.oct` | `text` | `Text` | `octxiliary-text` | `octxiliary` | none | 4 |
| `Libraries/Archive/manifest.oct` | `archive` | `Archive` | `octxiliary-archive` | `octxiliary` | none | 3 |
| `Libraries/Json/manifest.oct` | `json` | `Json` | `octxiliary-json` | `octxiliary` | none | 2 |
| `Libraries/Csv/manifest.oct` | `csv` | `Csv` | `octxiliary-csv` | `octxiliary` | none | 2 |
| `Libraries/Plot/manifest.oct` | `plot` | `Plot` | `octxiliary-plot` | `octxiliary` | `Plot.Size`, `Plot.Labels` records | 3 |
| `Libraries/Image/manifest.oct` | `image` | `Image` | `octxiliary-image` | `octxiliary` | `Image.ImageHandle` handle | 6 |
| `Libraries/Pdf/manifest.oct` | `pdf` | `Pdf` | `octxiliary-pdf` | `octxiliary` | `Pdf.PdfPage` handle, `Pdf.TextStyle` record | 6 |

Inventory inconsistency surfaced: there is no top-level `Libraries/Xlsx/manifest.oct` in this checkout. Xlsx wrapper metadata currently lives under the IO package manifest as the `xlsx` wrapper family, and the public Oct files are `Libraries/IO/IO.Xlsx.oct` and `Libraries/IO/IO.Xlsx.octest`. If future docs refer to a separate Xlsx package, that is a documentation/package-layout gap rather than current repository behavior.

### 2.3 Metadata fields and validation now

Current required wrapper fields:

- `Name`: package-local wrapper name.
- `Family`: Octxiliary request family.
- `Protocol`: currently exactly `octxiliary.v0`.
- `SidecarCommand`: executable command name expected at runtime/build planning.
- `GoModuleDir`: package-local relative Go module directory; absolute paths and `..` traversal are rejected.
- `Functions`: non-empty `WrapperFunction[]`.

Current optional wrapper fields:

- `TransportTypes`: `WrapperTransportType[]`, currently for structured records and handles.

Current function fields:

- `OctName`
- `WireName`
- `Args`
- `Return`
- `Fallible`

Current primitive/generic transport type strings:

- `Void`
- `Int`
- `Int<unit>` style unit-qualified ints
- `Float`
- `Bool`
- `String`
- `String[]`
- `String[][]`
- `Float[]`
- `Bytes`

Current structured transport constraints:

- `WrapperTransportType.Kind` must be `record` or `handle`.
- A handle schema must have exactly one field: `Handle: Int`.
- Transport fields may use supported primitive transport types except `Void`, plus `Int<unit>`.
- Function arguments may use primitive transport strings or declared transport types.
- Function returns may use primitive transport strings or declared handle transport types.
- Declared record returns are rejected today, even though `pkg/octxiliary` exposes an `OkRecord` constructor; records are currently supported as arguments/fields, not as manifest-declared returns. This is a documentation/API gap to keep visible for future work.

### 2.4 How compiler lowering consumes manifest metadata

Generic compiled wrapper lowering is already metadata-driven for manifest-declared wrappers. The compiler builds generic calls from sidecar command, family, wire function, argument values, and expected response type. It includes generic host helpers for records and handles, validates handle family/type/positive IDs, and maps missing sidecars to diagnostics naming the sidecar command.

Current runtime search in generated compiled code differs from the future policy recommended below. It checks a sibling executable location and `OCT_WRAPPER_PATH`; current stdlib-specific IO legacy helpers also hard-code `octxiliary-io`. This behavior should not be broadened in W2.

### 2.5 How `oct pkg wrappers` and registry output expose metadata

`oct pkg wrappers` is planning-only. It syncs the current project/dependencies, builds a wrapper plan for manifests whose `Kind` is `wrapper`, prints whether native wrappers exist, whether native build permission would be required, the number of sidecars, package/wrapper/family/command/protocol/module/function counts, and always reports that no wrapper sidecars were built or executed.

The package manager can also render a deterministic `.octagon` wrapper registry with `--registry-out <path>`. The registry currently records:

- registry version `octxiliary.registry.v0`;
- package name;
- wrapper name;
- family;
- protocol;
- sidecar command;
- Go module dir/path;
- transport types and fields;
- function `OctName`, `WireName`, `Args`, `Return`, and `Fallible`.

Generally reusable pieces:

- manifest parsing and wrapper record validation;
- `Kind: "wrapper"` rules;
- wrapper build plan as inert metadata;
- conflict detection for duplicate sidecar commands, families, and module paths;
- deterministic registry rendering;
- generic compiler call generation once package/project loading exposes the metadata.

Stdlib-specific or too narrow pieces:

- `GoModuleDir` names the sidecar source as a Go module rather than general native source metadata;
- compiled runtime discovery is sibling/`OCT_WRAPPER_PATH`, not deterministic package-cache discovery;
- interpreter wrapper registry still has internal builtin behavior;
- no manifest/stub agreement validator exists;
- no native build lifecycle exists;
- no explicit third-party sidecar package cache/build output path exists.

## 3. Third-party wrapper package shape

Recommended canonical layout:

```text
oct-opencv/
  manifest.oct
  OpenCV/
    OpenCV.Core.oct
    OpenCV.Core.octest
  sidecars/
    octxiliary-opencv/
      go.mod
      main.go
      README.md
```

Roles:

- `manifest.oct` declares the package metadata, wrapper ABI, sidecar command, transport schemas, and native sidecar source/build metadata.
- `OpenCV/*.oct` contains the public Oct API surface. These are ordinary Oct files and remain the source of user-facing docs, types, examples, and tests.
- `OpenCV/*.octest` contains ordinary Oct behavior tests for the public API. W2 does not require semantics to move into Go tests.
- `sidecars/octxiliary-opencv` contains native sidecar source. For W1/W2 examples this is Go code importing `github.com/yuechen-li-dev/oct/pkg/octxiliary`, but the manifest should not force all future sidecars to be Go.
- The manifest connects public Oct functions/stubs to sidecar family/wire names. Package-manager work later builds and installs sidecars explicitly into deterministic wrapper locations.

The package manager should later support inspecting this package without building or executing `sidecars/octxiliary-opencv`. Native source metadata must be declarative and inert during package sync.

## 4. Oct API stub contract

Preferred current pattern:

```oct
package OpenCV

fn ReadImage(path: String) -> Bytes ! Error {
    return OpenCVReadImage(path)?
}

fn Resize(bytes: Bytes, width: Int, height: Int) -> Bytes ! Error {
    return OpenCVResize(bytes, width, height)?
}
```

Contract:

- Public functions are ordinary typed Oct functions.
- Raw wire builtin names are package-private by convention until the language has privacy. Use names that are clearly internal, e.g. `OpenCVReadImage`, `_OpenCVReadImage`, or a package-local naming convention selected by future style guidance.
- Process-boundary APIs should generally be fallible (`! Error`) because sidecars can be missing, fail to start, reject arguments, encounter OS/library errors, or return protocol errors.
- Dimensions, unit-qualified ints, records, handles, enums, strings, bytes, lists, and numeric arrays are allowed only when the transport model supports them and manifest validation can prove agreement.
- API docs/tests remain ordinary Oct files. The manifest declares ABI; it should not replace source-level API docs or behavior tests.
- Raw wrapper calls should be thin. User-facing validation, overload-like naming, convenience conversions, and semantic grouping belong in ordinary Oct API functions where possible.

Future sugar evaluation:

- Extern stub body elision could reduce boilerplate once manifest/stub agreement is robust, e.g. a raw wire declaration that has no body because the manifest owns its sidecar binding. This is future sugar, not W2/W3 scope.
- `@extern` should not be introduced first. Starting with syntax would couple language design to an unstable package-manager/build/discovery story. If added later, it should compile down to the same manifest ABI and should be optional sugar, not a parallel source of truth.

## 5. Manifest schema proposal

### 5.1 Design principles

- Build from current wrapper metadata instead of inventing a parallel schema.
- Keep M0 fields close to what the package manager can plausibly validate and expose soon.
- Separate native source metadata from runtime ABI metadata.
- Keep all metadata inert: parsing a manifest must not run native code.
- Preserve deterministic registry rendering.
- Avoid over-specifying lockfiles, checksums, signatures, registry trust, prompts, or package federation in W2.

### 5.2 M0 required fields

Package-level M0:

```oct
record PackageManifest {
    Name: String
    Version: String
    Description: String
    Kind: String
    Dependencies: Dependency[]
    Wrappers: Wrapper[]
}
```

M0 package rules:

- `Kind` must be `"wrapper"` for packages with native wrapper sidecars.
- `Wrappers` must be present and non-empty for `Kind: "wrapper"`.
- `Dependencies` remains an ordinary package dependency list.

Wrapper-level M0:

```oct
record Wrapper {
    Name: String
    Family: String
    Protocol: String
    SidecarCommand: String
    SourceDir: String
    BuildKind: String
    TransportTypes: WrapperTransportType[]
    Functions: WrapperFunction[]
}
```

M0 wrapper semantics:

- `Name`: package-local wrapper name.
- `Family`: Octxiliary family used in request frames and sidecar dispatcher creation.
- `Protocol`: currently `"octxiliary.v0"` only.
- `SidecarCommand`: executable name used by build/install/discovery diagnostics.
- `SourceDir`: package-local native source directory, replacing or aliasing current `GoModuleDir` for third-party packages. M0 implementation may keep `GoModuleDir` for backwards compatibility, but public docs should move toward language-neutral naming.
- `BuildKind`: bounded enum such as `"go-module"`, initially the only supported build kind.
- `TransportTypes`: explicit schemas; allow empty arrays.
- `Functions`: non-empty function ABI declarations.

Function-level M0:

```oct
record WrapperFunction {
    OctName: String
    WireName: String
    Args: String[]
    Return: String
    Fallible: Bool
}
```

Function semantics:

- `OctName`: raw Oct stub/builtin name the compiler/interpreter recognizes.
- `WireName`: sidecar dispatcher function name.
- `Args`: ordered transport type strings.
- `Return`: transport type string.
- `Fallible`: whether the Oct raw function returns `T ! Error` rather than `T`.

Transport schema M0:

```oct
record WrapperTransportType {
    Name: String
    Kind: String
    Fields: WrapperTransportField[]
}

record WrapperTransportField {
    Name: String
    Type: String
}
```

M0 supported type categories:

- primitives: `Void`, `Int`, `Int<unit>`, `Float`, `Bool`, `String`;
- lists/arrays currently supported by transport: `String[]`, `String[][]`, `Float[]`;
- binary: `Bytes`;
- records: declared `Kind: "record"` schemas as arguments/fields;
- handles: declared `Kind: "handle"` schemas with `Handle: Int`;
- explicit unsupported type diagnostics for anything outside the supported set, including nested lists other than `String[][]`, arbitrary maps, general JSON values, callbacks, streams, function values, and undeclared structs.

Native sidecar M0 metadata:

```oct
record WrapperNativeSource {
    SourceDir: String
    Language: String
    BuildKind: String
    BuildCommand: String
    OutputName: String
    SupportedPlatforms: String[]
    SystemDependencies: String[]
    RequiresNetwork: Bool
    RequiresNativePermission: Bool
    Environment: String[]
}
```

M0 should not require every field above immediately. A plausible first implementation is:

- required: `SourceDir`, `Language`, `BuildKind`, `OutputName`, `RequiresNativePermission`;
- optional or future: `BuildCommand`, `SupportedPlatforms`, `SystemDependencies`, `RequiresNetwork`, `Environment`.

`BuildCommand` should be treated carefully. If supported, it must be displayed before execution and run only under explicit native-build permission. Prefer `BuildKind` for common deterministic flows such as Go modules so package manager code can own the command line.

### 5.3 Future fields

Future fields that should not block M0:

- artifact checksums for built sidecars;
- package release content digests;
- source archive hashes;
- signature metadata;
- lockfile pins;
- toolchain version constraints;
- sandbox profiles;
- runtime permission categories;
- generated C headers or FFI metadata;
- registry federation locations;
- package-cache install locations.

These belong to later package-manager/registry milestones, not W2.

## 6. Native permission / safety policy

Policy design:

- Native sidecars must not build or run silently during `oct pkg sync`.
- Wrapper packages must be inspectable before native build. `oct pkg wrappers` should continue to expose native-code status, sidecar commands, source directories, functions, and transport schemas without executing package code.
- Native builds require explicit user action, for example:

```sh
oct pkg build-wrappers --allow-native
```

- No NPM-style postinstall execution. Fetching/syncing package bytes and evaluating inert manifests must not run build scripts.
- Runtime sidecar execution is explicit in the sense that calling a wrapper function starts/uses a native sidecar; diagnostics should identify the sidecar command and package.
- Manifest metadata should make native-code status visible in `oct pkg wrappers` and future registry views.

Local dev vs verified cache:

- Local development should allow explicit overrides, such as building from a local package checkout or using `OCT_WRAPPER_PATH` in tests/CI.
- Verified package cache installs should be keyed by package digest, platform, build kind/toolchain, and sidecar output name.
- A local override should be visibly marked as unverified/dev and should not silently masquerade as a verified package-cache artifact.

PATH policy:

- PATH lookup should not be the default trust boundary for third-party native wrappers. PATH is mutable, process-environment-dependent, vulnerable to hijacking/dependency confusion, and not tied to package identity or digest.
- PATH can be an explicit dev/CI opt-in in a future flag or environment setting, but default discovery should prefer deterministic project/package locations.

Deferred safety features:

- runtime prompts;
- OS sandbox profiles;
- capability-based permissions;
- signature verification policies;
- remote attestation;
- centralized trust stores.

## 7. Interpreted vs compiled dispatch model

Current stdlib model:

- Interpreted execution may use internal wrapper builtin registry behavior.
- Compiled execution uses Octxiliary for manifest-declared wrapper paths.

Third-party target model:

- Interpreted execution uses Octxiliary through manifest-declared wrapper metadata.
- Compiled execution uses Octxiliary through the same metadata.
- The user-facing Oct call should look identical in interpreted and compiled modes.

Expected behavior:

- Public Oct APIs call raw stubs just like ordinary functions.
- Raw stub names are resolved against manifest wrapper metadata.
- The interpreter routes manifest-declared raw wrapper calls through Octxiliary instead of requiring repo-internal builtin registration. This is future W3 work.
- The compiler generic wrapper lowering may already be close for non-stdlib packages if package/project loading exposes the relevant manifests to the lowering phase; W2 does not assert complete third-party support.
- Missing sidecar diagnostics should identify the sidecar command, package, family, and raw/wire function where possible.
- Process-boundary functions should be fallible, and sidecar error responses should map to Oct `Error` values rather than panics.

W2 does not implement third-party interpreted dispatch.

## 8. Sidecar discovery policy

Preferred future discovery order:

1. Project-local wrapper build dir, for example `.oct/wrappers/<platform>/<sidecar>`.
2. Verified package cache wrapper dir, keyed by package digest/platform/toolchain/sidecar.
3. Sibling of compiled `.octbin` or executable.
4. `OCT_WRAPPER_PATH` as explicit dev/CI override.
5. PATH only if explicitly enabled, not default.

Rationale:

- Project-local build dirs make active development deterministic and easy to inspect.
- Verified package cache dirs tie executables to package identity, content digests, and platform.
- Sibling-of-executable remains useful for packaged applications and current compiled stdlib flows.
- `OCT_WRAPPER_PATH` remains useful for tests and CI but should be visibly an override.
- PATH as default is dangerous because it can select an unrelated executable with the same name. Native wrapper commands are security-sensitive and should resolve through package-aware locations first.

Current stdlib discovery differs: generated compiled helpers look beside the executable and at `OCT_WRAPPER_PATH`, with some legacy IO-specific behavior. W2 must not change that behavior.

## 9. Relationship to package federation / registry

Wrapper manifests should fit future package federation by carrying inert, inspectable metadata:

- registry index entries should expose whether a package contains native wrappers;
- wrapper metadata should be visible without executing code;
- package artifacts should be content-addressed;
- sidecar source/build metadata should become part of package release metadata;
- built sidecar artifacts should eventually carry platform, toolchain, output name, and checksum metadata;
- P2P fetch is a transport for package bytes, not a trust mechanism;
- trust comes from hashes, signatures, policies, provenance, and explicit user/admin choices.

W2 does not design a full registry, federation protocol, or P2P trust system. It only requires that wrapper ABI and native source/build fields be serializable into deterministic registry/package metadata.

## 10. Relationship to W1 SDK

Expected sidecar author workflow:

```go
package main

import (
    "os"

    "github.com/yuechen-li-dev/oct/pkg/octxiliary"
)

func main() {
    d := octxiliary.NewDispatcher("OpenCV")
    d.HandleFunc("ReadImage", func(req octxiliary.Request) octxiliary.Response {
        // Decode args, do native work, return an octxiliary response.
        return octxiliary.ErrUnsupported(req.ID, req.Function)
    })
    os.Exit(octxiliary.Main(os.Stdin, os.Stdout, d.HandleRequest))
}
```

Separation of responsibilities:

- SDK handles wire protocol.
- Manifest handles ABI declaration.
- Package manager handles build/install later.
- Oct source handles public API, docs, and tests.

The sidecar dispatch family should match the manifest `Family`, and handler function names should match manifest `WireName` values.

## 11. Validation plan

Existing for stdlib/current manifests:

- `oct pkg wrappers` prints planning metadata and native-code status without building/executing sidecars.
- `oct pkg wrappers --registry-out <path.octagon>` renders deterministic wrapper ABI registry output.
- Manifest parsing validates record shapes, wrapper kind rules, protocol value, package-local `GoModuleDir`, non-empty functions, duplicate wrapper names/families, duplicate function Oct/wire names, supported transport types, handle shape, and record field support.
- Wrapper build-plan validation detects duplicate sidecar commands, families, and module paths.

Future W3/W4 validation commands/tools:

- `oct pkg wrapper-check` for manifest/stub agreement.
- Stub/function agreement checks:
  - manifest `OctName` exists as a raw Oct function or declared raw wrapper stub;
  - argument count matches;
  - argument types match transport type declarations;
  - return type matches;
  - fallibility matches `! Error` vs non-fallible function type;
  - records and handles have matching Oct record definitions where appropriate;
  - unsupported transport types fail with clear diagnostics.
- Sidecar family naming checks:
  - manifest family should match sidecar dispatcher family when testable;
  - a future optional sidecar introspection/test command could assert supported wire function names without executing arbitrary package install scripts.
- Registry rendering checks:
  - deterministic `.octagon` output for wrapper ABI;
  - inclusion of native source/build metadata;
  - content-addressable fields once package federation exists.

W3 should consume a hardened manifest ABI for interpreter dispatch. W4 should harden validation/registry before broad third-party use.

## 12. Example package design

### 12.1 Example manifest

```oct
package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Kind: String
    Dependencies: Dependency[]
    Wrappers: Wrapper[]
}

record Dependency {
    Name: String
    VersionRequirement: String
}

record Wrapper {
    Name: String
    Family: String
    Protocol: String
    SidecarCommand: String
    SourceDir: String
    BuildKind: String
    TransportTypes: WrapperTransportType[]
    Functions: WrapperFunction[]
}

record WrapperTransportType {
    Name: String
    Kind: String
    Fields: WrapperTransportField[]
}

record WrapperTransportField {
    Name: String
    Type: String
}

record WrapperFunction {
    OctName: String
    WireName: String
    Args: String[]
    Return: String
    Fallible: Bool
}

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: "oct-echo"
        Version: "0.1.0"
        Description: "Example third-party Octxiliary echo wrapper"
        Kind: "wrapper"
        Dependencies: [Dependency { Name: "OctStd" VersionRequirement: "0.1.0" }]
        Wrappers: [
            Wrapper {
                Name: "echo"
                Family: "Echo"
                Protocol: "octxiliary.v0"
                SidecarCommand: "octxiliary-echo"
                SourceDir: "sidecars/octxiliary-echo"
                BuildKind: "go-module"
                TransportTypes: []
                Functions: [
                    WrapperFunction { OctName: "EchoStringRaw" WireName: "EchoString" Args: ["String"] Return: "String" Fallible: true },
                    WrapperFunction { OctName: "ByteLengthRaw" WireName: "ByteLength" Args: ["Bytes"] Return: "Int" Fallible: true }
                ]
            }
        ]
    }
}
```

Current implementation note: today's parser expects `GoModuleDir`, not `SourceDir`/`BuildKind`. The example above is the proposed third-party schema, not production-compatible W2 code.

### 12.2 Example Oct API stub

```oct
package Echo

fn EchoString(text: String) -> String ! Error {
    return EchoStringRaw(text)?
}

fn ByteLength(data: Bytes) -> Int ! Error {
    return ByteLengthRaw(data)?
}
```

### 12.3 Example Go sidecar skeleton

```go
package main

import (
    "os"

    "github.com/yuechen-li-dev/oct/pkg/octxiliary"
)

func main() {
    d := octxiliary.NewDispatcher("Echo")

    d.HandleFunc("EchoString", func(req octxiliary.Request) octxiliary.Response {
        text, err := octxiliary.ArgString(req, 0)
        if err != nil {
            return octxiliary.Err(req.ID, err)
        }
        return octxiliary.OkString(req.ID, text)
    })

    d.HandleFunc("ByteLength", func(req octxiliary.Request) octxiliary.Response {
        data, err := octxiliary.ArgBytes(req, 0)
        if err != nil {
            return octxiliary.Err(req.ID, err)
        }
        return octxiliary.OkInt(req.ID, len(data))
    })

    os.Exit(octxiliary.Main(os.Stdin, os.Stdout, d.HandleRequest))
}
```

Unsupported functions are handled by the dispatcher default response, which returns an `ErrUnsupported` response for unknown wire function names.

## 13. Candidate next milestones

### A. W3 — interpreted generic wrapper dispatch

Scope:

- Interpreter reads manifest wrapper metadata.
- Raw/unresolved wrapper calls route through Octxiliary.
- Third-party wrappers become usable in interpreted mode.
- No package-manager build lifecycle yet.

Pros:

- Highest immediate user-visible leverage if manifests are already reliable.
- Aligns interpreted and compiled behavior.
- Exercises W1 SDK with real third-party-style sidecars.

Cons:

- Risky if manifest/stub agreement remains under-validated.
- Could bake in ad hoc ABI assumptions before registry/schema hardening.
- Missing sidecar/build lifecycle may still make interpreted demos awkward.

### B. W4 — wrapper package manifest validation / registry artifact hardening

Scope:

- Strengthen manifest/stub agreement checks.
- Normalize third-party native source/build metadata.
- Harden deterministic `.octagon` wrapper ABI output.
- Keep package-manager behavior inspection-only.

Pros:

- Builds on existing manifest extraction, wrapper planning, and registry output.
- Gives interpreter/compiler/package-manager a shared reliable ABI.
- Reduces risk of manifest/stub drift and third-party special cases.
- Keeps W2 non-goals intact.

Cons:

- Does not by itself make third-party wrappers callable in interpreted mode.
- Requires careful schema compatibility with current stdlib manifests.

### C. W5 — package-manager native wrapper build lifecycle

Scope:

- Add explicit `oct pkg build-wrappers --allow-native` or equivalent.
- Build/install sidecars into deterministic locations.

Pros:

- Removes manual sidecar build friction.
- Enables deterministic project/package-cache wrapper installs.

Cons:

- Too much safety/build/toolchain surface before schema and validation are stable.
- Risks scope creep into lockfiles, toolchains, prompts, and registry trust.

### D. PM1 — package federation registry index design

Scope:

- Broader registry/P2P/package artifact architecture.

Pros:

- Aligns wrapper metadata with future package distribution.

Cons:

- Too broad for wrapper usability right now.
- Trust and federation should not block local manifest/schema hardening.

### Recommended next milestone

Recommend exactly one: **W4 — wrapper package manifest validation / registry artifact hardening**.

Reason: current codebase readiness points to metadata hardening as the safest next convergence step. The repository already parses wrapper manifests, plans wrapper sidecars, exposes native-wrapper status, and writes deterministic `.octagon` registry output. But the third-party schema is not yet language-neutral, manifest/stub drift is not checked, record-return support is inconsistent with SDK constructors, and native source/build metadata is not represented in registry artifacts. W4 removes these blockers and gives W3 interpreted dispatch a validated ABI rather than asking the interpreter to consume partially specified manifests.

## 14. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Native code supply-chain risk | Never build/run during sync; expose native metadata; require explicit native permission; use hashes/signatures later. |
| Dependency confusion | Resolve sidecars through package-aware build/cache paths before any dev override; avoid default PATH lookup. |
| Silent postinstall risk | No postinstall hooks; build only through explicit commands such as `oct pkg build-wrappers --allow-native`. |
| PATH hijacking | PATH disabled by default for wrappers; if enabled, make it explicit and diagnostic-visible. |
| Manifest/stub drift | Add `oct pkg wrapper-check`; validate `OctName`, args, return, fallibility, records, handles, and transport support. |
| SDK compatibility ossification | Keep W1 SDK focused on protocol-facing helpers; version protocol/registry metadata separately. |
| Third-party wrappers becoming second-class | Use same manifest ABI for stdlib and third-party packages; avoid interpreter-only or stdlib-only special cases. |
| Too-early `@extern` syntax | Defer syntax until manifest workflow stabilizes; make future syntax optional sugar over manifest ABI. |
| Package manager scope creep | Stage work: W4 validation/registry, W3 dispatch, W5 explicit builds, PM1 federation. |
| Cross-platform sidecar builds | Add supported-platform and output-name metadata; build into platform-keyed dirs. |
| System library dependency pain | Declare system dependencies inertly; display before build; do not auto-install OS packages. |
| Record/handle schema mismatch | Validate manifest schemas against Oct records where possible and SDK request/response expectations. |
| Missing sidecar diagnostics | Include package, command, family, wire name, and discovery paths in errors. |

## 15. Final recommendation

Recommended next milestone after W2: **W4 — wrapper package manifest validation / registry artifact hardening**.

Scope:

- preserve manifest-first wrapper ABI;
- stabilize third-party-compatible native source/build metadata;
- validate manifest/stub agreement;
- harden deterministic `.octagon` wrapper ABI output;
- keep existing stdlib behavior compatible;
- keep package-manager behavior inspection-only.

Non-goals:

- no `@extern`;
- no language syntax change;
- no sidecar build lifecycle;
- no interpreted third-party dispatch implementation;
- no registry federation/P2P implementation;
- no runtime discovery behavior change;
- no native permission prompts;
- no PATH broadening.

Deferred features:

- W3 interpreted generic wrapper dispatch;
- W5 explicit native build/install lifecycle;
- PM1 registry/federation design;
- checksum/signature/lockfile policy;
- sandbox/runtime permission prompts;
- extern syntax sugar after manifest workflow stability.
