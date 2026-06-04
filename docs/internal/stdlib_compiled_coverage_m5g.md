# M5g Standard-library compiled coverage inventory

Date: 2026-05-30  
Scope: `Libraries/*`, top-level `Language`, compiled test runner behavior, and a small artifact-lane probe.

This is an audit milestone only. No compiler lowering, standard-library APIs, generic wrapper lowering, sidecar build behavior, or broad manifests were changed.

## 1. Executive summary

### Coverage snapshot

- Top-level `Libraries/*` directories discovered: **40**.
- Per-library compiled runs completed: **40**.
- Per-library compiled directories/packages fully passing: **4** (`Artifact`, `String`, `Structures`, `Wireless`).
- Per-library compiled directories/packages failing: **36**.
- Passing compiled facts across per-library runs, excluding `.octfail` pass-only roots: **382**.
- Failing compiled facts across per-library runs: **311**.
- Whole-root `Libraries` discovery currently only finds 9 `.octfail` tests, even with `--all-packages`; per-library runs are required for this inventory.
- Top-level `Language` discovery is blocked before execution by a malformed `.octfail` expectation header in `Language/Testing/SelectedFileCompiled/sibling_invalid.octfail`.

### Top failure categories from per-library compiled runs

| Category | Counted failing tests | Representative symptom |
| --- | ---: | --- |
| `unsupported_builtin` | 114 | `compiled mode does not yet support builtin JsonLoad`, `UIButton`, `Real`, `Idx`, wrapper-style IO/CSV/JSON/XLSX/plot/hash/time/image/pdf/text builtins |
| `generated_go_error` | 95 | Generated Go syntax/type/import errors such as unit exponent types, int/float mismatches, matrix/list assignment mismatches, unused imports, undefined `IO_Bytes` |
| `missing_manifest` / layout / dependency visibility | 72 | `unknown package 'Assert'`, `package manifest missing`, `no [Fact], [Theory], or .octfail tests found` |
| `unsupported_language_lowering` | 24 | `unsupported MIR terminator <nil>`, local test helper callbacks like `DerivativeIdentity`, root-finder function arguments |
| `compiled_runtime_semantic_failure` | 6 | Sidecar panic in IO, one statistics runtime panic, one zero-assertion/compiled-run failure |

### Immediate highest-leverage next fixes

1. **M6 — generic scalar/list/bytes Octxiliary lowering** is the recommended immediate next implementation milestone. It targets the largest practical gap in wrapper-backed standard libraries (`IO`, `Archive`, `Compression`, `Csv`, `Json`, `Hash`, `Plot`, `Pdf`, `Text`, `Time`, `Image`, `Markdown`, `UI`/bridge-adjacent pieces) without requiring package-wide semantic redesign.
2. Follow M6 with one non-IO wrapper library migration (for example `Hash` or `Time`) to prove the generic registry path outside the M4 IO path.
3. Separately schedule manifest/test-root cleanup for missing `Assert`, missing package manifests, and whole-root discovery gaps; these obscure real compiler gaps but are not compiler-lowering work.
4. Schedule generated-Go hardening after wrapper lowering, because many pure-Oct numerical packages already run partially and then fail on common deterministic codegen issues.

## 2. Command matrix

Commands were run from repository root. Durations are wall-clock seconds measured with shell `date +%s`; `timeout` was used for bounded inventory commands.

| Command | Exit | Duration | Result summary |
| --- | ---: | ---: | --- |
| `go test ./internal/pkgmgr` | 0 | 52s | Passed. |
| `go test ./cmd/oct -run '^TestPkgWrappers'` | 0 | 92s | Passed. |
| `go test ./internal/project` | 0 | 2s | Passed. |
| `go test ./internal/... ./cmd/oct` | 0 | 189s | Passed. |
| `go run ./cmd/oct help` | 0 | ~1s | Confirmed command surface. |
| `go run ./cmd/oct test -h` | 0 | ~1s | Confirmed actual execution flag shape: `--execution <auto|compiled|interpreted>` and `--all-packages`; suggested `--compiled`/`--auto` are unsupported aliases. |
| `go run ./cmd/oct test Libraries` | 0 | 3s | Only 9 `.octfail` tests discovered; no compiled or interpreted facts run. |
| `go run ./cmd/oct test --compiled Libraries` | 1 | 1s | Unsupported flag placement/alias; interpreted as path: `stat --compiled: no such file or directory`. |
| `go run ./cmd/oct test --auto Libraries` | 1 | 0s | Unsupported flag placement/alias; interpreted as path: `stat --auto: no such file or directory`. |
| `go run ./cmd/oct test Libraries --all-packages` | 0 | 1s | Still only 9 `.octfail` tests discovered. |
| `go run ./cmd/oct test Libraries --execution compiled --all-packages` | 0 | 1s | Still only 9 `.octfail` tests discovered; no compiled facts run. |
| `go run ./cmd/oct test Libraries --execution auto --all-packages` | 0 | 0s | Still only 9 `.octfail` tests discovered. |
| `go run ./cmd/oct test Language` | 1 | 1s | Blocked by `Language/Testing/SelectedFileCompiled/sibling_invalid.octfail: malformed expectation header`. |
| `go run ./cmd/oct test Language --all-packages` | 1 | 1s | Same malformed expectation header blocker. |
| `go run ./cmd/oct test Language --execution compiled --all-packages` | 1 | 0s | Same malformed expectation header blocker. |
| `go run ./cmd/oct test Language --execution auto --all-packages` | 1 | 1s | Same malformed expectation header blocker. |
| `go run ./cmd/oct artifact Libraries/Artifact` | 0 | ~1s | Artifact lane passes for the dedicated artifact package: `PASS Artifact.ArtifactLaneCompiles`. |
| `go run ./cmd/oct artifact Libraries` | 1 | ~1s | Artifact root layout failure: `artifact failed: unknown package 'Main'`. |
| Per-library matrix: `timeout 90s go run ./cmd/oct test Libraries/<dir> --execution interpreted|auto|compiled` | mixed | 0-90s per command | Completed all 40 directories in all three modes. No command timed out; several compiled runs took tens of seconds because each fact builds generated Go. |

## 3. Per-library coverage table

`Interpreted status`, `Auto status`, and `Compiled status` are directory-level exit statuses from the per-library matrix. `pass` means the command exited 0. `fail` means the command exited non-zero. `compiled status` includes compiled pass/fail fact counts when the runner reached facts.

| Package/Directory | Interpreted status | Auto status | Compiled status | Failure category | Primary failing symbol/test | Notes | Suggested next milestone |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `Libraries/Analysis` | pass | pass | fail, 29 pass / 7 fail | `missing_manifest`, `unsupported_language_lowering`, `generated_go_error` | `DiffForwardRejectsInvalidInputs`, `LocalMaximaDoesNotIncludeEndpoints`, `Normalize*` | Missing `Assert`; MIR terminator gap; generated Go emits invalid `range` syntax for local names. | M9, M10 |
| `Libraries/Archive` | pass | pass | fail, 0 / 2 | `unsupported_builtin` | `ZipListEntriesAndExtractAllRoundTrip` | `ZipListEntries` is wrapper/builtin-backed and not lowered in compiled mode. | M6 |
| `Libraries/Artifact` | pass | pass | pass, 1 / 0 | none | n/a | Dedicated artifact package compiles/runs in test lane; artifact lane also passes directly. | keep monitored |
| `Libraries/ArtifactUsage` | fail | fail | fail before facts | `missing_manifest` | package root | `package manifest missing`; this is layout/manifest inventory, not compiler lowering. | M9 |
| `Libraries/Complex` | pass | pass | fail, 0 / 9 | `unsupported_builtin` | `ComplexSin`, `ComplexSinh`, `ComplexTan` | Complex helpers depend on `Real`, currently unsupported in compiled lowering. | M8 |
| `Libraries/Compression` | pass | pass | migrated, focused compiled pass | none for focused Compression M8 coverage | `CompressBytes`, `DecompressBytes`, `CompressFile`, `DecompressFile` | Migrated in M8 to generic Octxiliary wrapper lowering through manifest metadata and `octxiliary-compression`; byte and file gzip round-trips are covered with focused compiled tests. | done M8 |
| `Libraries/Cooking` | pass | pass | fail, 38 / 2 | `missing_manifest` | `GramsPerCupUnknownIngredientErrors`, `IsSafeTemperatureUnknownProteinErrors` | Most pure-Oct facts compile; error-path tests require `Assert` package visibility. | M9 |
| `Libraries/Csv` | fail | fail | fail before facts | `missing_manifest` / layout | package root | No fact tests found; package has implementation only. | M9 / add tests if desired |
| `Libraries/Deployment` | pass | pass | fail, 4 / 2 | `missing_manifest` | Gitea validation error-path tests | Pure string helpers mostly compile; `Assert` dependency not visible. | M9 |
| `Libraries/DifferentialEquations` | pass | pass | fail, 0 / 6 | `unsupported_language_lowering` | `DerivativeIdentity` | Local callback/function-argument test helpers are not resolved/lowered in compiled facts. | M8/M10 |
| `Libraries/Distributions` | pass | pass | fail, 3 / 4 | `missing_manifest` | invalid parameter/error tests | Valid math facts compile; error assertions need `Assert`. | M9 |
| `Libraries/Geometry` | pass | pass | fail, 1 / 8 | `generated_go_error`, `missing_manifest` | `PlanarAreaAndCircumferenceHandChecks` | Unit exponent types produce invalid Go like `unexpected ^ in type declaration`; some `Assert` gaps. | M10 |
| `Libraries/Hash` | pass | pass | migrated, focused compiled pass | none for focused Hash M7 coverage | `Sha256Bytes`, `Sha256File`, `Sha256Text` | Migrated in M7 to generic Octxiliary wrapper lowering through manifest metadata and `octxiliary-hash`; package still monitored with focused compiled coverage. | done M7 |
| `Libraries/IO` | pass | pass | focused Xlsx/Csv migrated; broad root may still fail, 1 / 34 | `unsupported_builtin`, `sidecar_environment`, `missing_manifest`, `generated_go_error` | legacy IO root aggregation; file/directory round trips; bytes round trip; migrated CSV/JSON/Xlsx focused paths | File/directory wrappers reach compiled runtime but panic when `OCT_WRAPPER_PATH`/`octxiliary-io` is unavailable. CSV row-major, safe JSON file helpers, and focused IO.Xlsx are now migrated; broader IO root issues remain tracked separately. Bytes alias emits undefined `IO_Bytes`. | M6, M9, M10 |
| `Libraries/IfErrNotEqualNil` | fail | fail | fail before facts | `unsupported_language_lowering` / syntax | `IfErrNotEqualNil.Core.oct` | Parser rejects `err != nil` style: `expected ')' after parameter list ... near "!"`; likely legacy/synthetic fixture. | expected_interpreted_only or cleanup |
| `Libraries/Image` | pass | pass | pass, focused Image M19 coverage | none for focused Image M19 coverage | `ImageLoad`, `ImageSave`, `ImageEncodePng`, `ImageWidth`, `ImageHeight`, `ImageFormat` | M19 migrates Image through `octxiliary-image` with typed `Image.ImageHandle` transport; M30 adds PNG `Bytes` export for Pdf interop. Fixture generation remains a harness concern for focused tests. | done M19 |
| `Libraries/Interpolation` | pass | pass | fail, 16 / 12 | `generated_go_error`, `missing_manifest` | `CubicSpline*` | Generated Go confuses scalar/list variables (`float64` as `[]float64`); error-path `Assert` gaps. | M10/M9 |
| `Libraries/Json` | fail | fail | fail before facts | `missing_manifest` / layout | package root | No fact tests found; package has implementation only. | M9 / add tests if desired |
| `Libraries/LinearAlgebra` | pass | pass | fail, 18 / 24 | `missing_manifest`, `generated_go_error` | `MatrixDimensionMismatchRaisesError`, `JacobiEigen*` | Many core facts compile; generated Go has int/float loop-index confusion in eigen routines; error tests miss `Assert`. | M10/M9 |
| `Libraries/Markdown` | pass | pass | pass, 3 / 0 | none for focused Markdown M14 coverage | `MarkdownH1`, `MarkdownCallout`, `MarkdownReport`, scalar/list/report helpers, table helpers | M14 lowers deterministic Markdown helpers directly in generated Go with no Octxiliary sidecar. `String[][]` report composition is now supported; columnar record tables compile in process without adding record transport. | done M14 |
| `Libraries/Mathematics` | pass | pass | fail, 6 / 15 | `unsupported_language_lowering`, `unsupported_builtin`, `missing_manifest` | `RootFind*`, `IndexOfMax*`, `DerivativePolynomial*` | Function callback lowering and `Idx` builtin are missing; error tests need `Assert`. | M8/M10/M9 |
| `Libraries/Mechanics` | pass | pass | fail, 48 / 17 | `unsupported_builtin`, `generated_go_error` | `Idx`, continuum/matrix stress helpers | Broad pure-Oct mechanics coverage compiles; remaining gaps are `Idx` and generated Go matrix/list arithmetic issues. | M8/M10 |
| `Libraries/Numerics` | pass | pass | fail, 0 / 6 | `unsupported_language_lowering` | `BisectionFindsSquareRoot`, `NewtonFindsSquareRoot` | Root-finder tests use function arguments/callbacks not lowered. | M8 |
| `Libraries/Octomata` | pass | pass | fail, 71 / 21 | `missing_manifest`, `generated_go_error` | `CommitmentAndEdgeComposeForModeChangeDetection`, validation/error tests | Strong compiled coverage already; generated Go has unused `math` import and some type issues; error tests need `Assert`. | M10/M9 |
| `Libraries/Optimization` | pass | pass | fail, 3 / 4 | `unsupported_language_lowering` | objective callback tests | Function argument/callback lowering gap. | M8 |
| `Libraries/Pdf` | pass | pass | focused text/image-bytes pass | legacy image bridge remains compiled-deferred | `PdfNewPage`, `PdfDrawText`, `PdfDrawTextStyled`, `PdfSave`, `PdfDrawImageBytes`, `PdfDrawImageBytesSized` | M21 migrates page/text/save; M30 adds PNG bytes drawing via Oct-mediated Image-to-Pdf transfer. Legacy `PdfDrawImage` / `PdfDrawImageSized` remain interpreted-only bridge APIs. | done M30 |
| `Libraries/Physics` | fail | fail | fail, 4 / 1 | `compiled_runtime_semantic_failure` / test-shape | `PhysicalConstantsCallableSurface` | Same fact fails interpreted due zero assertions; compiled reports `compiled test run failed: exit status 1: 0`. Not a compiler coverage gap. | test cleanup / M9 |
| `Libraries/Plot` | pass | pass | pass, 6 / 0 | none for focused Plot M16 coverage | `PlotRenderLine`, `PlotRenderScatter`, `PlotRenderHistogram` | M16 migrates Line, Scatter, and Histogram through `octxiliary-plot` using `Float[]` plus declared `Plot.Size`/`Plot.Labels` record arguments. | done M16 |
| `Libraries/RF` | pass | pass | fail, 34 / 23 | `generated_go_error`, `unsupported_builtin`, `missing_manifest` | `AwgnHelpersGeneralizeAcrossSequenceLengths`, `DbToLinearSeries*` | Generated Go invalid `_` use; `Idx`/`Abs` gaps; some `Assert` error paths. | M10/M8/M9 |
| `Libraries/Random` | pass | pass | fail, 20 / 2 | `generated_go_error`, `missing_manifest` | `BernoulliAndRangesAndNormalAreNonDegenerate`, `CryptoDiceHelpersSmokeAndValidation` | Mostly compiled; one generated Go record-type variable reuse issue; crypto error-path uses `Assert`. | M10/M9 |
| `Libraries/Signal` | pass | pass | fail, 28 / 9 | `missing_manifest`, `unsupported_builtin` | `MagnitudeSpectrumLengthMatchesInput`, FIR validation tests | Pure filters compile; complex `Abs` and validation `Assert` remain. | M8/M9 |
| `Libraries/Simulation` | pass | pass | fail, 3 / 3 | `missing_manifest` | simulation validation/error facts | Pure happy path compiles; error assertions need `Assert`. | M9 |
| `Libraries/Statistics` | pass | pass | fail, 4 / 31 | `generated_go_error`, `compiled_runtime_semantic_failure`, `missing_manifest` | `Mean*`, `MedianHandlesOddAndEvenDeterministically`, regression/summary helpers | Int/float arithmetic codegen dominates; median also panics with index `-1`; error paths miss `Assert`. | M10/M9 |
| `Libraries/String` | pass | pass | pass, 5 / 0 | none | n/a | String standard-library tests compile successfully. | keep monitored |
| `Libraries/Structures` | pass | pass | pass, 3 / 0 | none | n/a | Structures core compiles successfully. | keep monitored |
| `Libraries/Text` | pass | pass | migrated, focused compiled pass | none for focused Text M10 coverage | `IsMatch`, `FindAll`, `ReplaceAll`, `Split` | Migrated in M10 to generic Octxiliary wrapper lowering through `octxiliary-text`; public regex APIs preserve pattern-then-text order and Go `regexp` behavior. | done M10 |
| `Libraries/Thermofluids` | pass | pass | fail, 3 / 8 | `generated_go_error`, `missing_manifest` | `CylindricalGeometryHandChecks`, validation tests | Unit exponent type syntax errors plus `Assert` gaps. | M10/M9 |
| `Libraries/Time` | pass | pass | pass, 4 / 0 | none | n/a | Migrated in M9 to generic Octxiliary wrapper lowering through `octxiliary-time`; public parse/format APIs preserve normalized RFC3339 string return values. | keep monitored |
| `Libraries/UI` | pass | pass | fail, 23 / 30 | `unsupported_builtin`, `missing_manifest`, `generated_go_error`, `expected_interpreted_only` for live UI lanes | `UIButton`, `UICanvas`, `UIMount`, `UIColumn`, `M111ResolveEventValueEmptyTableReturnsEmpty` | Pure model/layout facts partly compile; widget/canvas/mount bridge should remain a separate reactor/UI project, not generic numeric lowering. | M6 for pure wrappers; reactor_bridge project for live UI |
| `Libraries/Wireless` | pass | pass | pass, 15 / 0 | none | n/a | Wireless pure-Oct library compiles successfully. | keep monitored |

## 4. Failure detail catalog

### M5G-F001 — missing `Assert` package in compiled tests

- Category: `missing_manifest`
- Commands: many per-library compiled commands, e.g. `go run ./cmd/oct test Libraries/Analysis --execution compiled`
- Package/test path: `Libraries/Analysis/Analysis.Core.octest`, `Libraries/Cooking/Cooking.Core.octest`, `Libraries/LinearAlgebra/*`, etc.
- Failing test/function: `Analysis.DiffForwardRejectsInvalidInputs`; many error-path tests.
- Error excerpt: `compiled execution required: function Analysis.DiffForwardRejectsInvalidInputs: unknown package 'Assert'`
- Likely cause: compiled package/test root resolution does not expose the built-in test assertion package for these manifested library test roots, or dependencies are not declared/loaded in a way compiled runner accepts.
- Proposed fix class: manifest/test-root cleanup and assertion package visibility.
- Proposed milestone: M9.

### M5G-F002 — package root layout/no tests/missing manifest

- Category: `missing_manifest`
- Commands: `go run ./cmd/oct test Libraries/ArtifactUsage --execution compiled`; `go run ./cmd/oct test Libraries/Csv --execution compiled`; `go run ./cmd/oct test Libraries/Json --execution compiled`
- Package/test path: `Libraries/ArtifactUsage`, `Libraries/Csv`, `Libraries/Json`
- Failing test/function: root discovery.
- Error excerpts:
  - `test failed: package manifest missing`
  - `test failed: no [Fact], [Theory], or .octfail tests found`
- Likely cause: library/test layout mismatch. `Csv` and `Json` currently have implementation files but no facts; `ArtifactUsage` is missing a manifest at the selected root.
- Proposed fix class: package/test-root cleanup; optionally add tests where appropriate.
- Proposed milestone: M9.

### M5G-F003 — whole-root `Libraries` discovery is misleading

- Category: `missing_manifest` / test-root layout
- Command: `go run ./cmd/oct test Libraries --execution compiled --all-packages`
- Package/test path: `Libraries`
- Failing test/function: discovery did not fail, but inventory coverage is incomplete.
- Error/result excerpt: only invalid guidance tests run: `Result: 9 passed, 0 failed, 0 skipped`; `Execution summary: compiled: 0 interpreted fallback: 0`.
- Likely cause: top-level root does not act as a package workspace for all `Libraries/*` facts. Per-library commands are needed today.
- Proposed fix class: test runner/workspace discovery cleanup, or docs that standard-library lane is per package.
- Proposed milestone: M9.

### M5G-F004 — unsupported wrapper/builtin family: IO/CSV/JSON/XLSX

- Category: `unsupported_builtin` / `unsupported_wrapper`
- Command: `go run ./cmd/oct test Libraries/IO --execution compiled`
- Package/test path: `Libraries/IO/IO.CoreWrappers.octest`, `IO.Json.octest`, `IO.Xlsx.octest`
- Failing symbols/tests: `CsvReadMatrix`, `CsvReadRows`, `CsvReadTable`, `CsvRead`, `JsonParse`, `JsonLoad`, `JsonNormalize`, `JsonLoadStructured`, `JsonLower`, `XlsxCreateWorkbook`, `XlsxAddSheet`, `XlsxSaveWorkbook`.
- Error excerpt: `function IO.Load: compiled mode does not yet support builtin JsonLoad`
- Likely cause: only the file/directory wrapper family has compiled sidecar support; generic wrapper lowering for other scalar/list/bytes helpers is absent.
- Proposed fix class: generic wrapper registry/Octxiliary lowering.
- Proposed milestone: M6.

### M5G-F005 — IO file/directory sidecar unavailable at compiled runtime

- Category: `sidecar_environment`
- Command: `go run ./cmd/oct test Libraries/IO --execution compiled`
- Package/test path: `Libraries/IO/IO.CoreWrappers.octest`
- Failing tests/functions: `DirectoryMakeListAndRemoveAllRoundTrip`, `FileReadWriteTextAndDeleteRoundTrip`, `FileWriteLinesReadLinesPreservesEmptyLines`, `FileWriteTextReadTextRoundTripAndOverwrite`.
- Error excerpt: `panic: unwrap failed: Octxiliary sidecar not found; set OCT_WRAPPER_PATH or place octxiliary-io beside .octbin`
- Likely cause: compiled generated program reaches the sidecar path, but no `octxiliary-io` executable is provided in this audit environment.
- Proposed fix class: sidecar build/install lane, or test runner environment setup for compiled wrapper tests.
- Proposed milestone: M6 for generic path; separate sidecar build lane later.

### M5G-F006 — generated Go undefined bytes alias

- Category: `generated_go_error`
- Command: `go run ./cmd/oct test Libraries/IO --execution compiled`
- Package/test path: `Libraries/IO/IO.CoreWrappers.octest`
- Failing test/function: `IO.FileReadWriteBytesRoundTrip`
- Error excerpt: `.octbuild/...gen.go:22:8: undefined: IO_Bytes`
- Likely cause: package-qualified `Bytes` type alias/codegen path is not emitted for IO wrapper tests.
- Proposed fix class: generated Go type lowering/import hardening.
- Proposed milestone: M10.

### M5G-F007 — unsupported archive/compression/hash/plot/pdf/text/time/image external helpers

- Category: `unsupported_builtin` / `unsupported_wrapper`
- Commands: per-library compiled commands for `Archive`, `Compression`, `Hash`, `Plot`, `Pdf`, `Text`, `Time`, `Image`
- Original failing symbols: `ZipListEntries`, `GzipCompressBytes`, `GzipCompressFile`, `GzipDecompressBytes`, `HashSha256Bytes`, `HashSha256File`, `HashSha256Text`, `PlotRenderLine`, `PlotRenderScatter`, `PlotRenderHistogram`, `PdfNewPage`, `PdfDrawText`, `RegexIsMatch`, `TimeParseIso8601`, `TimeUnixSecondsNow`, `ImageLoad`.
- Current status after M19: Hash was migrated in M7, Compression in M8, Time in M9, Text in M10, Archive in M11, JSON in M11, CSV in M13, Plot in M16, Xlsx in M18, and Image in M19. The remaining handle-centered symbols in this row are Pdf helpers such as `PdfNewPage` and `PdfDrawText`; Image is no longer part of this blocker.
- Error excerpt from original M5g inventory: `function Hash.Sha256Text: compiled mode does not yet support builtin HashSha256Text`
- Likely cause: these wrapper/direct-host builtins have interpreted implementations but no compiled lowering.
- Proposed fix class: generic scalar/list/bytes wrapper lowering first; choose direct helper only where wrapper protocol is inappropriate.
- Proposed milestone: M6/M7/M8 for Hash and Compression; future wrapper milestones for remaining packages.

### M5G-F008 — markdown helpers not compiled

- Category: `unsupported_builtin`
- Command: `go run ./cmd/oct test Libraries/Markdown --execution compiled`
- Package/test path: `Libraries/Markdown/Markdown.Core.octest`
- Current status after M14: `MarkdownBlocksM0`, `MarkdownM1Helpers`, and `MarkdownReportAndTablesM0` pass in compiled mode.
- Implementation note: deterministic Markdown helpers are direct compiled helpers, not Octxiliary wrappers.
- Resolution: M14 added direct compiled helper lowering for Markdown without adding an Octxiliary sidecar.
- Follow-up: keep Markdown in direct-helper coverage; do not route it through generic wrapper lowering.

### M5G-F009 — UI widget/canvas/mount bridge not compiled

- Category: `unsupported_builtin` / `expected_interpreted_only` for live bridge lanes
- Command: `go run ./cmd/oct test Libraries/UI --execution compiled`
- Package/test path: `Libraries/UI/UI.M0.octest`, `UI.M2.octest`, etc.
- Failing symbols: `UIButton`, `UICanvas`, `UIMount`, `UIColumn`, `UIGrid`, `UISignature`, `UIGridRows`.
- Error excerpt: `function UI.Button: compiled mode does not yet support builtin UIButton`
- Likely cause: UI bridge/reactor integration is a distinct compiled backend problem. Pure UI model/layout tests can compile, but live widget/canvas/mount helpers should not be folded into generic numeric lowering.
- Proposed fix class: reactor bridge/native UI compiled project; keep manual/live lanes separate.
- Proposed milestone: reactor_bridge project after core M6/M10 gaps.

### M5G-F010 — complex `Real` builtin unsupported

- Category: `unsupported_builtin`
- Command: `go run ./cmd/oct test Libraries/Complex --execution compiled`
- Package/test path: `Libraries/Complex/Complex.Functions.octest`
- Failing symbols/tests: `ComplexSin`, `ComplexSinh`, `ComplexTan`, Euler/trig helpers.
- Error excerpt: `function Complex.ComplexSin: compiled mode does not yet support builtin Real`
- Likely cause: complex component extraction is missing from compiled builtin lowering.
- Proposed fix class: direct compiled builtin lowering for complex helpers.
- Proposed milestone: M8.

### M5G-F011 — `Idx` and complex `Abs` unsupported

- Category: `unsupported_builtin`
- Commands: `Libraries/Mathematics`, `Libraries/Mechanics`, `Libraries/RF`, `Libraries/Signal`
- Package/test path: multiple pure-Oct numeric libraries.
- Failing symbols/tests: `IndexOfMax*`, mechanics indexing helpers, RF sequence helpers, `MagnitudeSpectrumLengthMatchesInput`.
- Error excerpts:
  - `compiled mode does not yet support builtin Idx`
  - `compiled mode does not yet support builtin Abs for type Complex`
- Likely cause: compiled builtin coverage lacks these core scalar/complex helpers.
- Proposed fix class: direct compiled builtin lowering.
- Proposed milestone: M8.

### M5G-F012 — function callback/lambda-style lowering gap

- Category: `unsupported_language_lowering`
- Commands: `Libraries/DifferentialEquations`, `Libraries/Numerics`, `Libraries/Optimization`, `Libraries/Mathematics`
- Package/test path: ODE/root/optimization tests.
- Failing tests/functions: `EulerStepMatchesExplicitFormula`, `BisectionFindsSquareRoot`, `GoldenSectionFindsQuadraticMinimum`, `RootFind*`.
- Error excerpt: `function DifferentialEquations.EulerStepMatchesExplicitFormula: unknown identifier 'DerivativeIdentity'`
- Likely cause: local helper functions/function-valued parameters are accepted by interpreter tests but not resolved/lowered in compiled runner.
- Proposed fix class: language lowering support for function values/callback references, or test contract split if interpreted-only by design.
- Proposed milestone: M8.

### M5G-F013 — unsupported MIR terminator

- Category: `unsupported_language_lowering`
- Command: `go run ./cmd/oct test Libraries/Analysis --execution compiled`
- Package/test path: `Libraries/Analysis/Analysis.Core.octest`
- Failing test/function: `LocalMaximaDoesNotIncludeEndpoints`
- Error excerpt: `compiled execution required: unsupported MIR terminator <nil>`
- Likely cause: a control-flow/early-exit/lowering edge case emits a nil MIR terminator.
- Proposed fix class: MIR lowering hardening with diagnostics.
- Proposed milestone: M10.

### M5G-F014 — generated Go unit exponent type syntax errors

- Category: `generated_go_error`
- Commands: `Libraries/Geometry`, `Libraries/Thermofluids`
- Package/test path: `Geometry.Planar.octest`, `Thermofluids.Fluid.octest`
- Failing tests/functions: `PlanarAreaAndCircumferenceHandChecks`, `CylindricalGeometryHandChecks`.
- Error excerpt: `.octbuild/...gen.go:21:23: syntax error: unexpected ^ in type declaration`
- Likely cause: dimensional unit exponents are not normalized to valid Go type identifiers in generated code.
- Proposed fix class: generated Go type-name mangling for dimension exponents.
- Proposed milestone: M10.

### M5G-F015 — generated Go int/float arithmetic and loop-index mismatches

- Category: `generated_go_error`
- Commands: `Libraries/Statistics`, `Libraries/LinearAlgebra`, and related numeric libraries.
- Package/test path: `Statistics.Core.octest`, `LinearAlgebra.Eigen.octest`
- Failing tests/functions: `MeanComputesAverageForSimpleArray`, `JacobiEigen*`.
- Error excerpts:
  - `.octbuild/...gen.go:93:11: invalid operation: sum / n (mismatched types float64 and int)`
  - `.octbuild/...gen.go:945:41: cannot use c (variable of type float64) as int value in argument to fn_LinearAlgebra_FlatIndex`
- Likely cause: compiled numeric coercion and loop-variable type inference are weaker than interpreter semantics.
- Proposed fix class: generated Go numeric conversion and variable reuse hardening.
- Proposed milestone: M10.

### M5G-F016 — generated Go scalar/list/matrix confusion

- Category: `generated_go_error`
- Commands: `Libraries/Interpolation`, `Libraries/Mechanics`
- Package/test path: `Interpolation.Core.octest`, `Mechanics.Continuum.octest`
- Failing tests/functions: `CubicSpline*`, `LinearIsotropicStress2DMatchesLambdaMuSkeleton`.
- Error excerpts:
  - `.octbuild/...gen.go:408:8: cannot use _t6 (variable of type float64) as []float64 value in assignment`
  - `.octbuild/...gen.go:214:11: invalid operation: _t4 * identity (mismatched types float64 and [][]float64)`
- Likely cause: list/matrix temporaries and scalar multiplication semantics are not consistently lowered.
- Proposed fix class: generated Go expression/type lowering hardening.
- Proposed milestone: M10.

### M5G-F017 — generated Go invalid `_` value and unused import

- Category: `generated_go_error`
- Commands: `Libraries/RF`, `Libraries/Octomata`
- Package/test path: `RF.Fading.octest`, `Octomata.Commitment.octest`
- Failing tests/functions: `AwgnHelpersGeneralizeAcrossSequenceLengths`, `CommitmentAndEdgeComposeForModeChangeDetection`.
- Error excerpts:
  - `.octbuild/...gen.go:229:8: cannot use _ as value or type`
  - `.octbuild/...gen.go:5:2: "math" imported and not used`
- Likely cause: placeholder/discard lowering and import emission are not pruned consistently.
- Proposed fix class: generated Go cleanup pass.
- Proposed milestone: M10.

### M5G-F018 — generated Go record/list type inference mismatch

- Category: `generated_go_error`
- Commands: `Libraries/Random`, `Libraries/UI`
- Package/test path: `Random.Core.octest`, `UI.M111.octest`
- Failing tests/functions: `BernoulliAndRangesAndNormalAreNonDegenerate`, `M111ResolveEventValueEmptyTableReturnsEmpty`.
- Error excerpts:
  - `.octbuild/...gen.go:311:11: cannot use _t4 (variable of struct type Random_RandBoolResult) as Random_RandFloatResult value in assignment`
  - `.octbuild/...gen.go:212:40: cannot use _t0 (variable of type []int) as []string value in struct literal`
- Likely cause: temporary variable reuse across different record/list types and empty list inference problems.
- Proposed fix class: generated Go temp allocation/type inference hardening.
- Proposed milestone: M10.

### M5G-F019 — compiled runtime semantic panic in statistics median

- Category: `compiled_runtime_semantic_failure`
- Command: `go run ./cmd/oct test Libraries/Statistics --execution compiled`
- Package/test path: `Libraries/Statistics/Statistics.Core.octest`
- Failing test/function: `MedianHandlesOddAndEvenDeterministically`
- Error excerpt: `compiled test run failed: exit status 2: panic: runtime error: index out of range [-1]`
- Likely cause: compiled index/arithmetic semantics differ from interpreter or generated test runner computes an index incorrectly.
- Proposed fix class: generated Go/runtime semantic investigation after codegen type fixes.
- Proposed milestone: M10.

### M5G-F020 — interpreted-only/test-shape failures

- Category: `expected_interpreted_only` or test cleanup
- Commands: `Libraries/Physics`, `Libraries/Image`, `Libraries/IfErrNotEqualNil`
- Package/test path: `Physics.Constants.octest`, `Image.Core.octest`, `IfErrNotEqualNil.Core.oct`
- Failing symbols/tests: `PhysicalConstantsCallableSurface`, image fixture tests, parser fixture.
- Error excerpts:
  - `test completed with zero assertions`
  - `ImageLoad: NotFound: mx103d_fixture_rect.png: open mx103d_fixture_rect.png: no such file or directory`
  - `parse Libraries/IfErrNotEqualNil/IfErrNotEqualNil.Core.oct: expected ')' after parameter list at 6:32 near "!"`
- Likely cause: not compiled-specific. Physics fact has no assertions; image tests need fixture/root handling; `IfErrNotEqualNil` appears to be legacy/synthetic syntax coverage.
- Proposed fix class: test cleanup or explicit lane marking.
- Proposed milestone: M9 or explicit interpreted-only classification.

### M5G-F021 — artifact root lane layout failure

- Category: `artifact_lane_failure`
- Commands: `go run ./cmd/oct artifact Libraries/Artifact`; `go run ./cmd/oct artifact Libraries`
- Package/test path: `Libraries/Artifact`, top-level `Libraries`
- Failing test/function: whole-root artifact discovery.
- Error excerpts:
  - direct package success: `PASS Artifact.ArtifactLaneCompiles`
  - root failure: `artifact failed: unknown package 'Main'`
- Likely cause: artifact command expects a package/root shape, not a multi-package workspace root.
- Proposed fix class: artifact lane workspace behavior or docs.
- Proposed milestone: M9.

### M5G-F022 — top-level `Language` lane blocked before inventory

- Category: `missing_manifest` / test data header issue
- Command: `go run ./cmd/oct test Language --execution compiled --all-packages`
- Package/test path: `Language/Testing/SelectedFileCompiled/sibling_invalid.octfail`
- Failing test/function: discovery/parsing of expectation header.
- Error excerpt: `test failed: Language/Testing/SelectedFileCompiled/sibling_invalid.octfail: malformed expectation header`
- Likely cause: invalid fixture in a broad directory run. This may be intentional for selected-file tests, but it blocks whole-language compiled inventory.
- Proposed fix class: language lane discovery/fixture isolation.
- Proposed milestone: M9.

## 5. Standard-library lowering strategy map

| Package | Function/API area | Current compiled state | Intended strategy | Blocker | Priority |
| --- | --- | --- | --- | --- | --- |
| `Analysis` | numerical series, normalize, integrate, extrema | partial pure-Oct | `pure_oct` | generated Go range/name issue; MIR terminator; `Assert` visibility | high |
| `Archive` | zip list/extract/create | fail | `wrapper_registry_generic_octxiliary` | `ZipListEntries` unsupported | high |
| `Artifact` | artifact-lane smoke | pass | `pure_oct` / artifact runner | none in package lane | low |
| `ArtifactUsage` | artifact usage fixture | fail before facts | `artifact_lane` / layout | missing manifest/root shape | medium |
| `Complex` | complex trig/hyperbolic | fail | `direct_builtin` / `compiled_helper` | `Real` unsupported | medium |
| `Compression` | gzip bytes/files | migrated M8 | `wrapper_registry_generic_octxiliary` | none for focused gzip coverage | done |
| `Cooking` | unit conversions/recipes | mostly pass | `pure_oct` | `Assert` visibility for error paths | medium |
| `Csv` | CSV helpers | no facts | `wrapper_registry_generic_octxiliary` | no tests at package root; CSV builtins unsupported through IO | high |
| `Deployment` | Gitea command/config helpers | mostly pass | `pure_oct` | `Assert` visibility | medium |
| `DifferentialEquations` | Euler/RK4 with derivative callbacks | fail | `pure_oct` plus function-value lowering | callback/local helper lowering | high |
| `Distributions` | probability distributions | partial | `pure_oct` / `direct_builtin` math | `Assert` visibility | medium |
| `Geometry` | planar/solid dimensional math | partial | `pure_oct` | generated Go dimension exponent type names | high |
| `Hash` | sha256 bytes/text/file | migrated M7 | `wrapper_registry_generic_octxiliary` | none for focused sha256 coverage | done |
| `IO` | file/directory | reaches runtime, fails sidecar | `octxiliary_sidecar` | sidecar availability | high |
| `IO` | CSV/JSON/XLSX | fail | `wrapper_registry_generic_octxiliary` | generic wrapper lowering missing | high |
| `IfErrNotEqualNil` | legacy nil/error fixture | fail parse | `expected_interpreted_only` or cleanup | syntax not accepted by current parser | low |
| `Image` | load/save/metadata | migrated M19 | `wrapper_registry_generic_octxiliary` + `handle_transport` | focused fixtures required by harness; no Pdf interop | done |
| `Interpolation` | linear/cubic spline | partial | `pure_oct` | scalar/list generated Go type confusion; `Assert` | high |
| `Json` | JSON package helpers | no facts | `wrapper_registry_generic_octxiliary` | no tests at package root; JSON builtins unsupported through IO | high |
| `LinearAlgebra` | matrix/eigen | partial | `pure_oct` / maybe `compiled_helper` for heavy kernels later | generated Go int/float loop variables; `Assert` | high |
| `Markdown` | report/table/callout helpers | migrated M14 | `compiled_helper` | none for focused scalar/list/report/table coverage; no sidecar | done |
| `Mathematics` | calculus/core/transforms | partial | `pure_oct` plus `direct_builtin` | `Idx`; function callback lowering; `Assert` | high |
| `Mechanics` | continuum/stress/shaft/fatigue | broad partial | `pure_oct` | `Idx`; matrix/list generated Go errors | high |
| `Numerics` | roots | fail | `pure_oct` plus function-value lowering | callback lowering | high |
| `Octomata` | state/control helpers | strong partial | `pure_oct` | generated Go import/type cleanup; `Assert` | high |
| `Optimization` | optimization callbacks | partial | `pure_oct` plus function-value lowering | callback lowering | high |
| `Pdf` | text/page/save PDF helpers | migrated M21 focused subset | `wrapper_registry_generic_octxiliary` + `handle_transport` + `record_transport` | image drawing remains compiled-deferred; no cross-family Image handles | partial |
| `Physics` | constants | test-shape failure | `pure_oct` | zero-assertion fact/test cleanup | low |
| `Plot` | line/scatter/histogram | fail | `wrapper_registry_generic_octxiliary` | plot builtins unsupported | high |
| `RF` | RF math/sequences | partial | `pure_oct` / `direct_builtin` | generated Go `_`; `Idx`; complex `Abs`; `Assert` | high |
| `Random` | deterministic/crypto random | mostly pass | `direct_builtin` for RNG core; wrappers for crypto if needed | generated Go record temp reuse; `Assert` | medium |
| `Signal` | filters/spectrum/windows | mostly pass | `pure_oct` / `direct_builtin` | complex `Abs`; `Assert` | medium |
| `Simulation` | fixed-step traces | partial | `pure_oct` | `Assert` visibility | medium |
| `Statistics` | descriptive/regression/summary | weak partial | `pure_oct` | generated Go int/float semantics; median runtime panic; `Assert` | high |
| `String` | string helpers | pass | `pure_oct` / existing direct builtins | none | low |
| `Structures` | axial structures helpers | pass | `pure_oct` | none | low |
| `Text` | regex | fail | `wrapper_registry_generic_octxiliary` or `compiled_helper` | regex builtins unsupported | medium |
| `Thermofluids` | fluid/thermal dimensional math | partial | `pure_oct` | dimension exponent type names; `Assert` | high |
| `Time` | ISO parsing/format/current time | fail | `wrapper_registry_generic_octxiliary` or `compiled_helper` | time builtins unsupported | medium |
| `UI` | pure model/layout/dispatch | partial | `pure_oct` / `direct_builtin` | empty-list type inference; `Assert` | medium |
| `UI` | live widgets/canvas/mount | fail | `reactor_bridge` / `expected_interpreted_only` for manual lanes | `UIButton`, `UICanvas`, `UIMount` unsupported | separate project |
| `Wireless` | Wi-Fi/RF calculations | pass | `pure_oct` | none | low |

## 6. Recommended next implementation sequence

Recommended sequence after M5g:

1. **M6 — generic scalar/list/bytes Octxiliary lowering**. This is the one recommended immediate next implementation milestone.
2. M7 — migrate one non-IO wrapper library through the generic path, preferably `Hash` or `Time`, because each has a small API surface and clear pass/fail tests.
3. M8 — standard-library compiled coverage pass 1 for direct builtins and language lowering: `Real`, `Idx`, complex `Abs`, and callback/function-value lowering triage.
4. M9 — manifest/test-root cleanup: `Assert` visibility, package manifests, whole-root `Libraries` discovery, `Language` malformed-header isolation, artifact workspace roots, fixture roots.
5. M10 — generated Go hardening: unit exponent type names, int/float conversions, scalar/list/matrix temporary typing, temp variable reuse, unused import pruning, discard placeholder lowering, runtime semantic panics.

Exactly one immediate next milestone: **M6 — generic scalar/list/bytes Octxiliary lowering**.

## 7. Explicit non-fix notes

These failures should not be fixed as part of M5g and should not be conflated with generic compiled standard-library lowering:

- `Libraries/UI` live widget/canvas/mount behavior (`UIButton`, `UICanvas`, `UIMount`) belongs to a UI/reactor bridge project or explicitly interpreted/manual lane, not the generic numeric/wrapper coverage pass.
- `Libraries/ArtifactUsage` and `go run ./cmd/oct artifact Libraries` are root/package-layout or artifact-lane issues, not missing scalar lowering.
- `Libraries/Physics/PhysicalConstantsCallableSurface` fails interpreted too because it completes with zero assertions; this is test-shape cleanup.
- `Libraries/Image` fixture assets such as `mx103d_fixture_rect.png` remain a harness/root concern for direct focused runs; compiled Image support exists, including M30 `ImageEncodePng`.
- `Libraries/IfErrNotEqualNil` is rejected by the current parser before any compiled inventory can start; it appears to be a legacy/synthetic syntax fixture and should be classified/relocated rather than used as standard compiled coverage.
- Top-level `Language` inventory is blocked by `Language/Testing/SelectedFileCompiled/sibling_invalid.octfail`; broad language-lane audit needs fixture isolation first.

## Appendix A. Representative unsupported builtin names

From per-library compiled failures, representative unsupported builtins include:

- Wrapper/external data helpers still not migrated after M9: `ZipListEntries`, `CsvRead`, `CsvReadMatrix`, `CsvReadRows`, `CsvReadTable`, `JsonParse`, `JsonLoad`, `JsonNormalize`, `JsonLoadStructured`, `JsonLower`, `XlsxCreateWorkbook`, `XlsxAddSheet`, `XlsxSaveWorkbook`, `PdfDrawImage`, `PdfDrawImageSized`, `PlotRenderLine`, `PlotRenderScatter`, `PlotRenderHistogram`, `RegexIsMatch`.
- Direct compiled builtin candidates: `Real`, `Idx`, `Abs for type Complex`.
- UI/reactor bridge helpers: `UIButton`, `UICanvas`, `UIMount`, `UIColumn`, `UIGrid`, `UISignature`, `UIGridRows`.

## Appendix B. Inventory logs

Raw command logs were written outside the repository under `/tmp/oct-m5g` during this audit. They are intentionally not committed because the milestone asks for grouped inventory rather than large log dumps.

## M11 update — wrapper sweep

M11 resolved the M5g archive gap by migrating `Libraries/Archive` to generic Octxiliary wrapper lowering and adding `cmd/octxiliary-archive`.

M11 also migrated the safe `Libraries/Json` public file helpers (`Save` and `Load`) to generic wrapper lowering with `cmd/octxiliary-json`. Broader `Libraries/IO` JSON graph/structured helpers remain blocked by record and nested-array transport requirements.

Remaining M5g wrapper candidates are now classified as explicit blockers rather than silently pending:

- `Libraries/Csv`: `needs_nested_array_transport` for `String[][]` row APIs.
- `Libraries/Markdown`: `needs_record_transport` and `needs_nested_array_transport` for table/report helpers.
- `Libraries/Pdf`: text/page/save migrated in M21 using `Pdf.PdfPage` handle transport and `Pdf.TextStyle` record transport; M30 adds PNG bytes drawing. Legacy `PdfDrawImage` / `PdfDrawImageSized` remain deferred because cross-family Image handles are not supported.
- `Libraries/Plot`: migrated in M16 through `Float[]` plus declared non-recursive record argument transport; record returns/handles remain unsupported.
- `Libraries/Image`: handle transport migrated in M19; M30 adds PNG `Bytes` export.

See `docs/internal/octxiliary_m11_wrapper_sweep.md` for the focused M11 table.

## M13 coverage update

`Libraries/Csv` row-major `Read`/`Write` is migrated to generic Octxiliary via the new narrow `String[][]` transport and `octxiliary-csv`. Raw CSV preserves ragged rows. IO row-major aliases have focused compiled coverage. Csv table/matrix helpers, Markdown, Plot, Pdf, Image, XLSX, records, handles, dynamic values, numeric array transports, Complex, and Einstein notation remain future work.
