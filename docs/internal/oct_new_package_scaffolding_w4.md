# W4 `oct new` package scaffolding design audit

## 1. Executive summary

Oct needs a canonical `oct new` command family because package manifests are no longer a small hand-written metadata file. The current manifest model already has package kinds, optional experiment entry milestones, dependency metadata, wrapper ABI metadata, wrapper transport records/handles, sidecar command/family/protocol fields, and deterministic wrapper registry rendering. W1 added the public `pkg/octxiliary` sidecar SDK; W2 recommended manifest-first third-party wrappers. Without scaffolding, new packages are likely to drift from the current manifest schema or from the package layouts that `internal/project`, `internal/pkgmgr`, and `oct pkg wrappers` actually load.

Recommended commands for the first implementation milestone:

```sh
oct new experiment <Name>
oct new library <Name>
oct new wrapper-library <Name>
```

Prefer `new` over `init`: `oct new library SignalTools` reads as creating a new package directory, while `init` suggests mutating the current directory and would raise overwrite and manifest-editing questions that are not needed for M0.

Recommended next milestone after this design/audit is exactly:

```text
W5 — implement oct new experiment/library/wrapper-library
```

W5 should implement deterministic scaffolding only: create directories/files, render valid current `manifest.oct` shapes, and add focused CLI/rendering tests. It should not alter manifest parsing, package-manager sync, wrapper validation, wrapper dispatch, native build lifecycle, registry/federation behavior, lockfile behavior, or existing package layouts.

Deferred features:

- no `oct new init` alias in M0;
- no `--dir`, `--family`, `--sidecar`, `--force`, `--registry`, or native-build flags in M0;
- no interpreted generic wrapper dispatch;
- no native sidecar build/run/download lifecycle;
- no generated lockfiles;
- no registry/federation/P2P metadata;
- no `@extern` or `EXTERNAL { ... }` syntax;
- no changes to existing package directories.

## 2. Current package layout inventory

### 2.1 Manifest and loader facts that constrain scaffolding

The package manager manifest loader expects `manifest.oct` to declare `package Manifest`, define `record PackageManifest`, define `record Dependency`, and define `fn Manifest() -> PackageManifest`. Required `PackageManifest` fields are `Name`, `Version`, `Description`, and `Dependencies`; optional fields recognized by current code are `Kind`, `EntryMilestone`, and `Wrappers`. `Kind` normalizes to `pure` when omitted, and the allowed kinds are `pure`, `experiment`, and `wrapper`.

Current wrapper metadata is manifest-first. If `PackageManifest` declares `Wrappers: Wrapper[]`, the manifest must also define `record Wrapper` and `record WrapperFunction`; `WrapperTransportType` and `WrapperTransportField` are optional record definitions used only when transport metadata is declared. `Kind: "wrapper"` requires non-empty `Wrappers`; `Kind: "pure"` and `Kind: "experiment"` reject non-empty wrapper metadata. `EntryMilestone` is valid only for `Kind: "experiment"`.

The project loader treats a directory with `manifest.oct` as a manifested package root. It loads ordinary `.oct` files and, for tests, `.octest` files. It also descends into milestone directories whose names match `M<number>`, `Mx<number>`, or those forms plus lowercase suffix characters. A milestone directory under an experiment family root can use the parent manifest when the family root has both `manifest.oct` and `REPORT.md`.

### 2.2 Standard library pure package layout

Representative example: `Libraries/Signal`.

```text
Libraries/Signal/
  manifest.oct
  README.md
  Signal.Core.oct
  Signal.Core.octest
```

Audit:

- Example path: `Libraries/Signal/manifest.oct`.
- Manifest fields: `Name`, `Version`, `Description`, `Dependencies`; `Kind` omitted, which means current code treats it as `pure`.
- Dependencies: typically `OctStd` at `0.1.0`; some libraries declare additional direct package dependencies.
- Source file naming convention: files sit directly in the package root and use PascalCase package/module prefixes such as `Signal.Core.oct`, `LinearAlgebra.Eigen.oct`, or `Mechanics.Fatigue.oct`.
- Test file naming convention: colocated `.octest` files mirror source names, such as `Signal.Core.octest`.
- README/docs: most standard library packages include `README.md`, though not every package does (`Libraries/Csv` and `Libraries/Json` are current exceptions in this checkout).
- Artifacts: standard library package roots generally do not generate milestone `.octagon` artifacts; some packages include `testdata` or compiled smoke tests.
- Wrapper metadata: none for pure packages.

### 2.3 Wrapper library layout

Representative examples: `Libraries/Time`, `Libraries/IO`, and `Language/Testing/CompiledOctxiliary/valid`.

```text
Libraries/Time/
  manifest.oct
  README.md
  Time.Core.oct
  Time.Core.octest

cmd/octxiliary-time/
  main.go
```

Audit:

- Example paths: `Libraries/Time/manifest.oct`, `Libraries/IO/manifest.oct`, `Libraries/Pdf/manifest.oct`, and `Language/Testing/CompiledOctxiliary/valid/manifest.oct`.
- Manifest fields: `Name`, `Version`, `Description`, `Kind`, `Dependencies`, `Wrappers`, plus wrapper record definitions.
- Kind: `Kind: "wrapper"`.
- Wrapper fields: current production schema requires `Name`, `Family`, `Protocol`, `SidecarCommand`, `GoModuleDir`, and `Functions`; some packages also include `TransportTypes`.
- Transport records/handles: `Libraries/IO` declares the `IO.Workbook` handle; `Libraries/Image`, `Libraries/Pdf`, and `Libraries/Plot` declare handles or record transport metadata.
- Source file naming convention: wrapper Oct APIs are ordinary `.oct` files in the package root, e.g. `Time.Core.oct`, `IO.Xlsx.oct`, and `Pdf.Core.oct`.
- Test file naming convention: colocated `.octest` files mirror the public API files; wrapper packages also commonly have compiled smoke tests such as `Json.CompiledSmoke.octest`.
- README/docs: most wrapper library packages include a package README; `Libraries/Csv` and `Libraries/Json` currently do not.
- Artifacts: no package-local registry artifact is committed by default; `oct pkg wrappers --registry-out <path>` can render deterministic inert `.octagon` text on demand.
- Wrapper metadata: yes. Current stdlib sidecars live in top-level `cmd/octxiliary-*`, while manifest `GoModuleDir` values commonly point to `octxiliary` even though sidecar command source is not currently package-local. This is a current layout inconsistency and a reason third-party scaffolding should not cargo-cult stdlib sidecar placement.

Current wrapper manifest inventory in this checkout:

| Package manifest | Wrapper family/command highlights | Transport metadata |
|---|---|---|
| `Libraries/Time/manifest.oct` | `Time`, `octxiliary-time` | none |
| `Libraries/Text/manifest.oct` | `Text`, `octxiliary-text` | none |
| `Libraries/Archive/manifest.oct` | `Archive`, `octxiliary-archive` | none |
| `Libraries/Hash/manifest.oct` | `Hash`, `octxiliary-hash` | none |
| `Libraries/Compression/manifest.oct` | `Compression`, `octxiliary-compression` | none |
| `Libraries/Json/manifest.oct` | `Json`, `octxiliary-json` | none |
| `Libraries/Csv/manifest.oct` | `Csv`, `octxiliary-csv` | none |
| `Libraries/IO/manifest.oct` | `Csv`/`Xlsx`, `octxiliary-csv`/`octxiliary-xlsx` | `IO.Workbook` handle |
| `Libraries/Plot/manifest.oct` | `Plot`, `octxiliary-plot` | `Plot.Size`, `Plot.Labels` records |
| `Libraries/Image/manifest.oct` | `Image`, `octxiliary-image` | `Image.ImageHandle` handle |
| `Libraries/Pdf/manifest.oct` | `Pdf`, `octxiliary-pdf` | `Pdf.PdfPage` handle, `Pdf.TextStyle` record |
| `Language/Testing/CompiledOctxiliary/valid/manifest.oct` | `TestWrapper`, `octxiliary-test-wrapper` | record and handle fixtures |

### 2.4 Experiment layout

Representative example: `Experiments/FmBrownNoiseKalman`.

```text
Experiments/FmBrownNoiseKalman/
  manifest.oct
  REPORT.md
  FEEDBACK.md
  Shared/
    shared.oct
  M0/
    fm_brown_noise_kalman_m0.oct
    fm_brown_noise_kalman_m0.octest
  M1/
    fm_brown_noise_kalman_m1.oct
    fm_brown_noise_kalman_m1.octest
```

Audit:

- Example path: `Experiments/FmBrownNoiseKalman/manifest.oct`.
- Manifest fields: most current experiment manifests use only `Name`, `Version`, `Description`, and `Dependencies`; they omit `Kind`, so current code treats them as `pure` even though loader heuristics recognize experiment family roots by `manifest.oct` plus `REPORT.md`.
- Source file naming convention: milestone source files usually use lowercase snake_case derived from the experiment name and milestone, e.g. `fm_brown_noise_kalman_m0.oct`.
- Test file naming convention: milestone test files mirror source names with `.octest`.
- README/docs: current experiments more often include `REPORT.md`, milestone reports, `FINDINGS.md`, `DESIGN.md`, or `FEEDBACK.md` than `README.md`.
- Artifacts: experiments frequently commit milestone `.octagon`, `.csv`, `.json`, and report artifacts.
- Wrapper metadata: none in audited experiment manifests.

Inconsistency surfaced: manifest kind support includes `Kind: "experiment"` and `EntryMilestone`, but current experiment manifests in `Experiments/` generally omit both. Scaffolding should use the documented/current schema (`Kind: "experiment"`, `EntryMilestone: "M0"`) for new packages, and W5 tests should prove the generated shape loads.

### 2.5 Milestone experiment layout

Representative examples: `Experiments/PrometheusSgemmAlgorithmLab/M14` and `Experiments/ContinuumComputabilityBoundary/M29/valid`.

Audit:

- Example path: `Experiments/PrometheusSgemmAlgorithmLab/M14/prometheus_sgemm_algorithm_lab_m14.oct`.
- Manifest fields: the experiment family root has the manifest. Some nested valid fixtures under experiments have their own `valid/manifest.oct` for a smaller package root.
- Source file naming convention: milestone directories are named `M0`, `M1`, `M14`, etc.; source files are usually lowercase snake_case with the milestone suffix.
- Test file naming convention: colocated `.octest` file mirrors the source file.
- README/docs: milestone folders may include `REPORT.md`, `README.md`, `FINDINGS.md`, or milestone-specific markdown.
- Artifacts: milestone folders commonly hold deterministic `.octagon` artifacts and measurement files.
- Wrapper metadata: none observed in milestone experiment manifests.

### 2.6 Test fixture package layout

Representative examples: `Language/Packages/CrossPackageM81/valid/Cooking`, `Language/Testing/CompiledOctxiliary/invalid/record_arg_mismatch`, and `testdata/m24g/valid/Main`.

Audit:

- Example paths: `Language/Packages/CrossPackageM81/valid/Cooking/manifest.oct`, `Language/Testing/CompiledOctxiliary/valid/manifest.oct`, and `testdata/m24g/valid/Main/manifest.oct`.
- Manifest fields: valid language fixtures often use the same minimal manifest shape as pure libraries; wrapper fixtures use compact wrapper manifests to exercise validation and compiled wrapper behavior.
- Source file naming convention: varies by fixture purpose. Language fixtures are semantic contracts and may use domain-specific file names; `testdata` fixtures are synthetic/transitional and must not become canonical language semantics.
- Test file naming convention: `.octest` for valid behavior and `.octfail` for invalid/rejected behavior under `Language/`.
- README/docs: uncommon in small fixtures.
- Artifacts: some language and experiment fixtures include `.octagon` when the fixture is about artifact output.
- Wrapper metadata: present in compiled Octxiliary fixture manifests and invalid wrapper fixtures.

## 3. Command design

### 3.1 Shared M0 command policy

M0 should support exactly these commands and no flags:

```sh
oct new experiment <Name>
oct new library <Name>
oct new wrapper-library <Name>
```

Shared behavior:

- Required args: one subcommand and one `<Name>`.
- Optional flags: none in M0.
- Default target directory: `./<Name>` relative to the current working directory.
- Parent directories: no parent creation is needed in M0 because `--dir` is deferred; if future `--dir` is added, parent directories may be created only when the final target is absent.
- Error behavior: strict usage errors for missing/extra args or unknown subcommands, e.g. `usage: oct new <experiment|library|wrapper-library> <Name>`.
- Overwrite behavior: fail if target directory already exists, even if empty. This is the least surprising M0 rule and avoids having to define partial-directory merge semantics.
- Side effects: write only deterministic scaffold files under the new target directory; no network, no package sync, no native build, no sidecar execution, no lockfiles, no registry output.

### 3.2 Future flags considered but deferred

Considered commands:

```sh
oct new experiment <Name> --milestone M0
oct new library <Name> --dir <path>
oct new wrapper-library <Name> --family <Family> --sidecar <SidecarCommand>
```

Recommendation: do not support these flags in W5/M0.

Rationale:

- `--milestone` is attractive but unnecessary for the first command because M0 should always generate `EntryMilestone: "M0"` and an `M0/` directory. Supporting it would require validation for milestone names and filename derivation before the basic scaffolder exists.
- `--dir` introduces ambiguity between package name and directory name. M0 can avoid this by requiring the target directory name to equal `<Name>`.
- `--family` and `--sidecar` invite custom wrapper ABI choices before validation is hardened. M0 should teach one boring default convention.

Future milestone candidates may add these flags after W5 proves the default scaffold is valid and useful.

### 3.3 `oct new library <Name>`

- Required args: `<Name>`.
- Optional flags: none in M0.
- Defaults: version `0.1.0`, description `"<Name> package"`, dependencies `[OctStd 0.1.0]`, omitted `Kind` to match current pure stdlib convention.
- Generated directory: `./<Name>`.
- Generated files:
  - `manifest.oct`;
  - `README.md`;
  - `<Name>.Core.oct`;
  - `<Name>.Core.octest`.
- Error behavior: invalid name, target exists, current directory cannot be read/written, file write failure.
- Overwrite behavior: never overwrite in M0.

### 3.4 `oct new experiment <Name>`

- Required args: `<Name>`.
- Optional flags: none in M0.
- Defaults: version `0.1.0`, description `"<Name> experiment"`, dependencies `[OctStd 0.1.0]`, kind `experiment`, entry milestone `M0`.
- Generated directory: `./<Name>`.
- Generated files:
  - `manifest.oct`;
  - `README.md`;
  - `REPORT.md`;
  - `M0/<snake_name>_m0.oct`;
  - `M0/<snake_name>_m0.octest`.
- Error behavior: invalid name, target exists, write failure.
- Overwrite behavior: never overwrite in M0.
- Placement clarification: when the user runs the command from the repo root and wants an in-repo experiment, they should run `cd Experiments && oct new experiment BrownNoiseKalman`. M0 should not implicitly write to `Experiments/` because third-party packages outside the repo need the same command.

### 3.5 `oct new wrapper-library <Name>`

- Required args: `<Name>`.
- Optional flags: none in M0.
- Defaults: version `0.1.0`, description `"<Name> wrapper package"`, dependencies `[OctStd 0.1.0]`, kind `wrapper`, family `<Name>`, wrapper name `<kebab-name>`, protocol `octxiliary.v0`, sidecar command `octxiliary-<kebab-name>`, package-local Go module dir `sidecars/octxiliary-<kebab-name>`.
- Generated directory: `./<Name>`.
- Generated files:
  - `manifest.oct`;
  - `README.md`;
  - `<Name>.Core.oct`;
  - `<Name>.Core.octest`;
  - `sidecars/octxiliary-<kebab-name>/go.mod`;
  - `sidecars/octxiliary-<kebab-name>/main.go`;
  - `sidecars/octxiliary-<kebab-name>/README.md`.
- Error behavior: invalid name, target exists, write failure.
- Overwrite behavior: never overwrite in M0.
- Sidecar behavior: generated only; W5 must not build, run, download, or register it.

## 4. Naming rules

Recommended M0 rules are intentionally boring and non-normalizing.

### 4.1 Package and directory name syntax

`<Name>` must be a PascalCase Oct package identifier:

```text
[A-Z][A-Za-z0-9]*
```

Additional constraints:

- length: 1 to 80 characters;
- no whitespace;
- no hyphen, underscore, slash, dot, colon, or path separator;
- must not be `Manifest`, because generated manifests use package `Manifest`;
- must not be `Main` for library and wrapper-library scaffolds unless a future app command exists; `Main` is too easy to confuse with program entry package layout;
- must not be a built-in scalar/type family name such as `String`, `Int`, `Float`, `Bool`, `Void`, `Bytes`, `Error`, `Array`, or `Map`;
- must not be a top-level command family name such as `Pkg`, `Exp`, `New`, `Run`, `Build`, `Test`, `Artifact`, `Bench`, or `Fmt`.

The directory name is exactly `<Name>` in M0. Do not generate `signal-tools`, `signal_tools`, or `Signal Tools` directories from a PascalCase input.

### 4.2 PascalCase, snake_case, and kebab-case derivations

- Package name: preserve `<Name>` exactly after validation.
- Directory name: preserve `<Name>` exactly.
- Library source prefix: `<Name>.Core`.
- Experiment milestone filename stem: split PascalCase/acronyms into lowercase snake_case and append `_m0`.
- Wrapper sidecar command: split PascalCase/acronyms into lowercase kebab-case and prefix `octxiliary-`.
- Wrapper name: use the lowercase kebab-case value without the `octxiliary-` prefix.
- Wire function sample prefix: preserve package/family casing, e.g. `OpenCVEchoString`.

Examples:

| Input `<Name>` | Package | Directory | Snake stem | Kebab stem | Sidecar command |
|---|---|---|---|---|---|
| `OpenCV` | `OpenCV` | `OpenCV` | `open_cv` | `open-cv` | `octxiliary-open-cv` |
| `SignalTools` | `SignalTools` | `SignalTools` | `signal_tools` | `signal-tools` | `octxiliary-signal-tools` |
| `BrownNoiseKalman` | `BrownNoiseKalman` | `BrownNoiseKalman` | `brown_noise_kalman` | `brown-noise-kalman` | `octxiliary-brown-noise-kalman` |

The user prompt suggested `OpenCV -> sidecar octxiliary-opencv`. That is attractive for common acronyms, but it creates special cases and hidden normalization. M0 should prefer deterministic splitting (`octxiliary-open-cv`) unless W5 first adds an explicitly tested acronym policy. If product preference strongly favors `opencv`, make that a documented rule before implementation rather than a silent exception.

### 4.3 No normalization from non-PascalCase input

`oct new wrapper-library oct-opencv` should fail with a name validation error. It should not normalize to `OpenCV` and should not preserve `oct-opencv` as a package name. This avoids surprising directory names and avoids guessing whether `oct-` is a package prefix, sidecar prefix, or branding prefix.

### 4.4 Collision behavior

- If `./<Name>` exists, fail.
- If the generated sidecar directory would collide inside a newly created target, that is impossible in M0 unless the scaffold renderer has duplicate paths; tests should assert unique generated paths.
- Future `--dir` must fail if the target path exists and is non-empty, and should probably fail if any generated file exists unless an explicit `--force` policy is designed.

## 5. `oct new library` canonical output

Recommended output:

```text
SignalTools/
  manifest.oct
  README.md
  SignalTools.Core.oct
  SignalTools.Core.octest
```

Recommended `manifest.oct` template should use valid current syntax and current pure-library convention:

```oct
package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Dependencies: Dependency[]
}

record Dependency {
    Name: String
    VersionRequirement: String
}

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: "SignalTools"
        Version: "0.1.0"
        Description: "SignalTools package"
        Dependencies: [Dependency { Name: "OctStd" VersionRequirement: "0.1.0" }]
    }
}
```

Do not include `Kind: "pure"` in M0. Omitting `Kind` matches the current standard library convention and current code normalizes missing kind to `pure`. If W6 later hardens manifests and prefers explicit kinds, scaffold templates can be updated with exact tests.

Recommended source file:

```oct
package SignalTools

/// Return the input value unchanged.
fn Identity(value: Int) -> Int {
    return value
}
```

Recommended test file:

```oct
package SignalTools

[Fact]
fn IdentityReturnsInput() -> Void {
    Assert.Equal(7, Identity(7), "identity should return the input")
}
```

A tiny example function is preferable to an empty file because it gives generated `.octest` a real, deterministic assertion and catches package loading issues. The example should be semantically bland so Codex/humans do not overfit package design from it.

## 6. `oct new experiment` canonical output

Recommended output:

```text
BrownNoiseKalman/
  manifest.oct
  README.md
  REPORT.md
  M0/
    brown_noise_kalman_m0.oct
    brown_noise_kalman_m0.octest
```

Recommended manifest:

```oct
package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Kind: String
    EntryMilestone: String
    Dependencies: Dependency[]
}

record Dependency {
    Name: String
    VersionRequirement: String
}

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: "BrownNoiseKalman"
        Version: "0.1.0"
        Description: "BrownNoiseKalman experiment"
        Kind: "experiment"
        EntryMilestone: "M0"
        Dependencies: [Dependency { Name: "OctStd" VersionRequirement: "0.1.0" }]
    }
}
```

Recommended M0 source:

```oct
package BrownNoiseKalman

/// Return the starting sample count for the first experiment milestone.
fn M0SampleCount() -> Int {
    return 1
}
```

Recommended M0 test:

```oct
package BrownNoiseKalman

[Fact]
fn M0SampleCountIsPositive() -> Void {
    Assert.True(M0SampleCount() > 0, "M0 sample count should be positive")
}
```

Experiments should be generated in the current directory, not automatically under `Experiments/`. In-repo authors can run the command from `Experiments/`; third-party authors can run it anywhere. This avoids hard-coding repository layout into a user-facing package scaffolder.

## 7. `oct new wrapper-library` canonical output

Recommended output:

```text
OpenCV/
  manifest.oct
  README.md
  OpenCV.Core.oct
  OpenCV.Core.octest
  sidecars/
    octxiliary-open-cv/
      go.mod
      main.go
      README.md
```

Recommended current-compatible manifest:

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

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: "OpenCV"
        Version: "0.1.0"
        Description: "OpenCV wrapper package"
        Kind: "wrapper"
        Dependencies: [Dependency { Name: "OctStd" VersionRequirement: "0.1.0" }]
        Wrappers: [
            Wrapper {
                Name: "open-cv"
                Family: "OpenCV"
                Protocol: "octxiliary.v0"
                SidecarCommand: "octxiliary-open-cv"
                GoModuleDir: "sidecars/octxiliary-open-cv"
                Functions: [
                    WrapperFunction { OctName: "EchoStringRaw" WireName: "OpenCVEchoString" Args: ["String"] Return: "String" Fallible: true }
                ]
            }
        ]
    }
}
```

This uses only currently supported fields. W2-style future `SourceDir`, `BuildKind`, safety policy, registry/federation metadata, and native build lifecycle fields should be documented as future manifest hardening work and not generated until production schema supports them.

Recommended M0 Oct source should be deliberately inert:

```oct
package OpenCV

/// Returns true when the generated package scaffold is loadable.
fn ScaffoldReady() -> Bool {
    return true
}
```

Recommended M0 Oct test should not call the sidecar:

```oct
package OpenCV

[Fact]
fn ScaffoldLoads() -> Void {
    Assert.True(ScaffoldReady(), "wrapper scaffold should load before native sidecar build support")
}
```

Do **not** generate an Oct wrapper stub or an Oct function that calls `EchoStringRaw` in M0. Current manifests can declare wrapper functions and `oct pkg wrappers` can plan them, but `Language/reference` does not document a source-level `wrapper fn` declaration syntax, and third-party interpreted generic wrapper dispatch is explicitly deferred. Because `Kind: "wrapper"` currently requires a non-empty `Functions` array, the manifest should include one primitive `String` wrapper function for planning/registry validation, while generated Oct tests should remain sidecar-free until W7+ dispatch/build semantics exist.

Recommended sidecar `go.mod` for third-party packages:

```go.mod
module example.com/opencv-sidecar

go 1.22

require github.com/yuechen-li-dev/oct v0.0.0
```

Recommended W5 implementation detail: use a placeholder module path such as `example.com/<kebab-name>-sidecar` and document that authors should change it before publishing. Do not include a repo-local `replace` directive by default because generated packages may be outside the Oct repo. In in-repo tests, either avoid running `go build` for the generated sidecar or create a temporary test-only `replace github.com/yuechen-li-dev/oct => <repo-root>` inside the test fixture. The scaffold itself should import the public SDK path:

```go
import "github.com/yuechen-li-dev/oct/pkg/octxiliary"
```

Recommended sidecar `main.go`:

```go
package main

import (
    "os"

    "github.com/yuechen-li-dev/oct/pkg/octxiliary"
)

func main() {
    dispatcher := octxiliary.NewDispatcher("OpenCV")
    dispatcher.HandleFunc("OpenCVEchoString", func(req octxiliary.Request) octxiliary.Response {
        text, err := octxiliary.ArgString(req, 0)
        if err != nil {
            return octxiliary.Err(req.ID, err)
        }
        return octxiliary.OkString(req.ID, text)
    })
    _ = octxiliary.Main(os.Stdin, os.Stdout, dispatcher.HandleRequest)
}
```

Use an Echo sample in the manifest and sidecar for M0 planning metadata. It demonstrates primitive `String` ABI wiring, avoids records/handles, and does not imply real OpenCV/native execution. The generated Oct package test should not call this sample until wrapper dispatch/build milestones make that path real. The README and comments must say the native sidecar is scaffold-only and is not built or run by `oct new`.

## 8. Manifest template design

Considered approaches:

1. Embedded Go string templates in `cmd/oct`.
2. Mutable template files under a repository templates directory.
3. AST/renderer helpers that build manifest syntax structurally.
4. An internal scaffolding package with deterministic render functions.

Recommendation for W5: create an internal package, likely `internal/newpkg`, with deterministic rendering functions and exact-output tests. The CLI in `internal/cli` should only parse `oct new ...`, call `internal/newpkg`, and report results.

Rationale:

- Keeping rendering out of `cmd/oct` and `internal/cli` avoids bloating command dispatch.
- Mutable external templates are easy to drift from schema and harder to test as API.
- Full AST rendering is overbuilt for M0 and may become tangled with formatter/rendering work.
- Deterministic string rendering is acceptable if tests assert exact content, parse/load generated manifests, and run package loader checks.

W5 tests should assert:

- exact file path set for each command;
- exact text or stable golden text for each generated file;
- final newline and LF line endings;
- generated manifest parses through package-manager/project manifest loading;
- no duplicate generated paths.

## 9. Safety and overwrite behavior

Recommended M0 safety rules:

- Fail if target directory exists. Do not special-case empty directories in M0.
- Do not provide `--force` in M0.
- Never overwrite existing files.
- Write files only under the target directory.
- Refuse names that imply paths or normalization.
- Do not read or write package-manager cache.
- Do not run `oct pkg sync` as part of generation.
- Do not access the network.
- Do not run `go mod download`, `go build`, sidecars, or any native code.
- Do not create `.octagon` registries or lockfiles.
- Make output deterministic for a given command.

Future `--force` should remain deferred until there is a clear, tested policy. A safe future option might be `--force-empty-dir` rather than broad `--force`, but W5 should avoid that design space.

## 10. Formatting and determinism

Generated files should:

- use LF line endings;
- include one final newline;
- avoid timestamps;
- avoid usernames, hostnames, absolute paths, temp paths, and machine-specific Go module cache paths;
- use stable field ordering in manifests;
- use stable file creation order in tests, but not depend on filesystem enumeration order at runtime;
- pass `git diff --check`;
- avoid generated comments that mention the current date;
- avoid generated registry or lockfile output.

Manifest field order should follow current examples: identity fields first, `Kind`/`EntryMilestone` before `Dependencies` or immediately after description for experiment manifests, `Wrappers` last for wrapper manifests. Within wrapper metadata, use `Name`, `Family`, `Protocol`, `SidecarCommand`, `GoModuleDir`, optional `TransportTypes`, then `Functions`.

## 11. Validation after generation

W4 does not implement generation, but W5 acceptance should include these post-generation checks:

### 11.1 Library scaffold

- `manifest.oct` parses through `internal/pkgmgr.LoadManifestMetadata` or equivalent test helper.
- `internal/project.LoadForTest(<target>)` loads the package and tests.
- `go run ./cmd/oct test <target> --execution interpreted` passes.
- `go run ./cmd/oct test <target> --execution compiled` passes where compiled mode supports the generated code.

### 11.2 Experiment scaffold

- Manifest parses and normalizes to `Kind: "experiment"`.
- `EntryMilestone` is `M0`.
- Project loader includes the M0 `.oct` and `.octest` files.
- Interpreted tests pass.
- Compiled tests pass where feasible.

### 11.3 Wrapper-library scaffold

- Manifest parses and normalizes to `Kind: "wrapper"`.
- Wrapper metadata contains one sidecar, protocol `octxiliary.v0`, a package-local relative `GoModuleDir`, and one primitive string function.
- The generated package loads.
- `oct pkg wrappers` run from the generated target reports native wrappers and the expected sidecar command.
- `oct pkg wrappers --registry-out <temp>.octagon` writes deterministic registry text.
- No sidecar is built or executed during generation or validation unless a later milestone explicitly adds a build lifecycle.
- Sidecar Go source may be syntax-checked in W5 only if tests add a temporary `replace` directive; otherwise document that sidecar build is out of scope.

## 12. Relationship to W2/W4/W5 naming conflict

W2 recommended “W4 wrapper package manifest validation / registry artifact hardening.” This milestone is now W4 for `oct new` scaffolding design. The sequence should be renamed explicitly to avoid ambiguity:

```text
W1 — public pkg/octxiliary sidecar SDK
W2 — third-party native wrapper manifest design
W4 — oct new package scaffolding design
W5 — implement oct new experiment/library/wrapper-library
W6 — wrapper manifest validation / registry artifact hardening
W7 — interpreted generic wrapper dispatch
W8 — native wrapper build lifecycle
PM1 — package federation registry design
```

Recommendation: implement scaffolding before further wrapper validation hardening. Reason: a canonical scaffold gives validation hardening a concrete “new packages start correct” target and reduces the chance that W6 validates against layouts humans are not expected to write. W6 should still harden wrapper manifests before W7/W8 consume them for dispatch/build.

## 13. Candidate implementation milestone

Recommended next milestone, exactly one:

```text
W5 — implement oct new experiment/library/wrapper-library
```

### Scope

- Add `oct new` command dispatch.
- Add `internal/newpkg` or equivalent deterministic scaffolding package.
- Implement the three M0 commands with no flags.
- Generate the canonical file sets described above.
- Validate names with explicit errors.
- Fail when target directory exists.
- Add unit tests for rendering, path sets, name validation, and overwrite behavior.
- Add CLI tests for usage and successful generation.
- Add integration checks that generated manifests parse and generated packages load/test.
- Add wrapper-plan/registry checks for generated wrapper-library scaffold.

### Non-goals

- No manifest parser changes.
- No package-manager sync changes.
- No changes to existing package layouts.
- No native sidecar build/run/download.
- No lockfiles or registry auto-generation.
- No `@extern` or `EXTERNAL` syntax.
- No interpreted generic wrapper dispatch.
- No registry/federation/P2P.
- No flags beyond the three required command shapes.

### Tests

W5 should add focused tests in addition to the W4 regression tests:

- `go test ./internal/newpkg` if that package is created;
- `go test ./internal/cli ./cmd/oct` for CLI behavior;
- generated-package integration checks through `go run ./cmd/oct test <temp>` where feasible;
- `go test ./internal/pkgmgr ./internal/project` to preserve manifest/package loading behavior.

### Acceptance criteria

- `oct new experiment BrownNoiseKalman` creates the documented experiment scaffold.
- `oct new library SignalTools` creates the documented pure library scaffold.
- `oct new wrapper-library OpenCV` creates the documented wrapper scaffold.
- All generated files are deterministic and pass `git diff --check`.
- Existing required tests still pass.
- No production behavior changes outside the new command.

## 14. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Generated manifests are invalid or drift from schema. | Keep templates in `internal/newpkg`, assert exact text, parse generated manifests in tests, and update templates alongside schema changes. |
| Scaffolding includes too much sample code and users cargo-cult it. | Use tiny neutral examples (`Identity`, `M0SampleCount`, `EchoString`) and README language that labels them placeholders. |
| Wrapper sample implies native execution is ready. | State in generated README and docs that `oct new` never builds/runs sidecars; sample is scaffold-only. |
| Sidecar `go.mod` import/replace confusion. | Import `github.com/yuechen-li-dev/oct/pkg/octxiliary`; do not emit repo-local `replace` by default; test-only replace can be injected by tests. |
| Overwriting user files. | Fail if target exists; no `--force` in M0. |
| Naming normalization surprises. | Accept only PascalCase and fail nonconforming input; derive snake/kebab deterministically. |
| `OpenCV` acronym expectations differ. | Document the M0 split rule and add tests. If product wants `opencv`, define that as an explicit acronym policy before implementation. |
| Package layout diverges between stdlib and third-party packages. | Use package-local `sidecars/` for new third-party scaffolds and document current stdlib top-level `cmd/octxiliary-*` as not canonical for new packages. |
| `Kind: "experiment"` differs from existing experiment manifests. | Surface the inconsistency and test that current code accepts the explicit experiment kind and `EntryMilestone`. |
| Wrapper stub syntax is not documented in reference docs. | W5 must verify against `Language/reference`; update docs or avoid generating stubs if the syntax is unsupported/undocumented. |
| Codex overfits generated examples. | Keep examples small, deterministic, and clearly labeled as replaceable skeleton code. |
| Validation hardening after scaffolding changes generated output. | W6 should treat W5 scaffold as a first-class fixture and update it intentionally when schema changes. |

## 15. Final recommendation

Proceed with:

```text
W5 — implement oct new experiment/library/wrapper-library
```

W5 should implement the three no-flag commands, deterministic package scaffolding, strict PascalCase name validation, fail-if-target-exists safety, exact template tests, manifest/package load checks, and wrapper-plan/registry checks for generated wrapper libraries.

Keep these non-goals explicit for W5:

- no production manifest parser changes;
- no package-manager sync changes;
- no native sidecar build/run/download;
- no lockfiles;
- no automatic registry generation;
- no interpreted generic wrapper dispatch;
- no `@extern` or `EXTERNAL` syntax;
- no registry/federation/P2P;
- no changes to existing package layouts.

Deferred features remain W6+ work: wrapper manifest validation and registry artifact hardening, interpreted generic wrapper dispatch, native wrapper build lifecycle, and package federation/registry design.
