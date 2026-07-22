# Oct 1.0 Surface Manifest

This is the authoritative Oct 1.0 public-surface classification. A surface is
stable only when it participates in the documented compatibility and compiled
conformance promises in `OCT_1_0_CONTRACT.md`.

## Stable language and tooling

| Surface | Status | Compatibility / execution promise |
| --- | --- | --- |
| Syntax, types, functions, arrays/ranges, records/enums, control flow, errors, units, named function values, vectors/matrices, and Octomata as documented in `Language/reference/` | Stable | Meaning is preserved in 1.x; valid stable programs interpret and compile through GoOct. |
| Core compiler-owned builtins in `Language/reference/language/09-builtins.md` | Stable | Same language compatibility promise; compiled conformance covers the applicable core forms. |
| `oct run`, `build`, `test`, `fmt`, `new`, `init`, `pkg`, `artifact`, `bench`, and `version` | Stable essential tooling | Compatible command behavior in 1.x, subject to documented environmental prerequisites. |
| Local/source-controlled package registry, exact dependency sync, and optional `lock.octagon` | Stable tooling | Current documented MVP behavior is compatible; hosted publishing, solver ranges, federation, and integrity guarantees are not promised. |

## Stable standard-library APIs

The following first-party modules are stable when used through their documented
API and runtime prerequisites: `Analysis`, `Artifact`, `Complex`,
`DifferentialEquations`, `Distributions`, `Geometry`, `Interpolation`,
`LinearAlgebra`, `Loop`, `Mathematics`, `Mechanics`, `Numerics`, `Octomata`,
`Optimization`, `Physics`, `Random`, `RF`, `Signal`, `Simulation`,
`Statistics`, `String`, `Structures`, `Tensor2D`, `Thermofluids`, `Time`,
`Uncertainty`, `Units`, and `Wireless`. The 28-package compiled sweep records
native execution for every package with zero interpreter fallback.

Wrapper-backed stable module families are `Archive`, `Compression`, `Csv`,
`Hash`, `Image`, `IO`, `Json`, `Pdf`, `Plot`, and `Text`. Their compiled path
requires the documented Octxiliary sidecars; absence is an explicit error, not
an interpreter fallback. `ArtifactUsage`, `Cooking`, `HelloScience`, and
`Deployment` are examples/reference packages, not separately versioned APIs.

## Experimental, internal, or separately governed

| Surface | Classification | Contract |
| --- | --- | --- |
| `oct sdslv`, `oct prometheus-sgemm`, `oct prometheus-m1-async`; SDSL-V and Prometheus APIs | Separately governed / experimental | No Oct 1.x language or API compatibility guarantee; excluded from Oct conformance. |
| `oct make` and `Libraries/Make*` | Experimental | No compatibility guarantee during 1.x; excluded from stable conformance. |
| `Libraries/UI` and Machina UI runtime/host integrations | Experimental | UI authoring is dogfooding evidence, not stable Oct 1.0 API. |
| `oct exp run` | Experimental | Explicit remote-execution opt-in; excluded from release conformance. |
| `Backends/ClrOct`, generated Go/MIR, `.octbuild`, test harness layouts, sidecar protocols | Internal | No external compatibility promise. |

No currently public 1.0 surface is deprecated. Experimental surfaces must not
be used as evidence for stable interpreted or compiled parity.

## Conformance

Run the positive stable language gate with:

```powershell
go build -o .tmp\oct-rc2.exe ./cmd/oct
.\tools\Test-Oct10Conformance.ps1 -OctPath .tmp\oct-rc2.exe -Execution interpreted
.\tools\Test-Oct10Conformance.ps1 -OctPath .tmp\oct-rc2.exe -Execution compiled
```

The driver checks source existence, command success, per-target summaries,
positive native-case discovery, and zero compiled fallback. Invalid diagnostic
contracts remain a separate negative gate: `go run ./cmd/oct test Language
--execution interpreted`.
