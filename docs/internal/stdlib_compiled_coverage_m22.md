# M22 standard-library compiled coverage inventory

Date: 2026-06-03
Scope: `Libraries/*` after M6-M21 Octxiliary/direct-helper wrapper migrations.
Nature: audit/inventory only; no production behavior, public API, wrapper protocol, language lowering, or standard-library tests were changed.

## 1. Executive summary

### Coverage snapshot

- Top-level `Libraries/*` directories discovered and tested individually: **40**.
- Sidecars built into `.tmp/m22-wrappers`: **12/12 requested** (`octxiliary-io`, `octxiliary-hash`, `octxiliary-compression`, `octxiliary-time`, `octxiliary-text`, `octxiliary-archive`, `octxiliary-json`, `octxiliary-csv`, `octxiliary-plot`, `octxiliary-xlsx`, `octxiliary-image`, `octxiliary-pdf`).
- Per-library compiled directory status:
  - **16 pass**: `Archive`, `Artifact`, `Compression`, `Cooking`, `Csv`, `Deployment`, `Distributions`, `Hash`, `Json`, `Markdown`, `Plot`, `String`, `Structures`, `Text`, `Time`, `Wireless`.
  - **19 partial**: `Analysis`, `Geometry`, `IO`, `Image`, `Interpolation`, `LinearAlgebra`, `Mathematics`, `Mechanics`, `Octomata`, `Optimization`, `Pdf`, `Physics`, `RF`, `Random`, `Signal`, `Simulation`, `Statistics`, `Thermofluids`, `UI`.
  - **5 fail**: `ArtifactUsage`, `Complex`, `DifferentialEquations`, `IfErrNotEqualNil`, `Numerics`.
- Per-library compiled facts counted from runner output: **502 passed**, **215 failed**.
- Whole-root `Libraries` commands still exit successfully but discover only the shallow pass-only `.octfail` set; per-library commands remain required for a real standard-library inventory.
- `auto` mode is green for all directories whose interpreted command is green, confirming fallback still masks compiled gaps by design.

### Biggest remaining blocker categories

Approximate failing-test counts are grouped by representative signature, not by unique root cause:

| Category | Approx. failing facts | Packages most affected | Representative symptom |
| --- | ---: | --- | --- |
| `generated_go_type_error` / `generated_go_import_error` | 108 | `Analysis`, `Geometry`, `Interpolation`, `LinearAlgebra`, `Mechanics`, `Octomata`, `RF`, `Random`, `Statistics`, `Thermofluids`, `UI` | invalid generated Go syntax/types, unused imports, invalid `_` expressions, dimension exponent types |
| `remaining_wrapper_gap` / `unsupported_builtin_remaining` | 48 | `IO`, `Pdf`, `UI` | structured/legacy JSON and CSV through `IO`, PDF image drawing, live UI helpers |
| `callback_or_function_value_lowering` | 23 | `DifferentialEquations`, `Mathematics`, `Numerics`, `Optimization` | function parameters/local callback helpers such as `f` and `DerivativeIdentity` are unresolved in compiled mode |
| `compiled_complex_support` | 22 | `Complex`, `Mathematics`, `RF`, `Signal` | `Real` and `Abs` over `Complex` lack compiled lowering |
| `sidecar_environment` | 3 | `Image` | fixture-relative image paths are not found from compiled runner working directory |
| `missing_manifest_or_dependency` | 2 command-level/fact-level blockers | `ArtifactUsage`, `Simulation` | missing package manifest and unknown `Assert` package |
| `unknown_needs_triage` | ~10 | `Mechanics`, `Physics`, `Statistics` | runtime assertion/panic-style failures not yet isolated to a compiler class |

### Improvement since M5g

M5g had **4 fully passing compiled library directories** and **36 failing directories**. M22 has **16 fully passing**, **19 partial**, and **5 fully failing**. The wrapper migration sequence moved the standard-library picture from “compiled numeric subset with broad interpreted-only wrapper edges” to “most wrappers compile, with remaining failures concentrated in compiler/language/codegen hardening plus a few deferred APIs.”

The clearest migrated areas are: `Archive`, `Compression`, `Csv`, `Hash`, `Json` safe subset, `Markdown`, `Plot`, `Text`, `Time`, and the PDF text/page/save subset. `IO` improved materially (18 compiled facts pass) but still contains old structured JSON/CSV APIs that are not routed through the new split package wrappers. `Image` reaches compiled wrapper execution but fails on fixture path setup, not on unsupported image builtins.

### Recommended next implementation milestone

**M23 — generated-Go hardening pass**.

Rationale: generated-Go failures are the largest current blocker by count and spread. They also affect otherwise pure-Oct libraries and obscure later triage for complex, callback, UI, and package-cleanup work. This recommendation is intentionally **one milestone only**; Complex, Einstein, PDF image interop, package-manager sidecar lifecycle, and UI bridge work should remain deferred unless M23 evidence narrows into one of those areas.

## 2. Command matrix

Commands were run from repository root with `OCT_WRAPPER_PATH=$PWD/.tmp/m22-wrappers` for inventory commands after sidecar build. Exit `124` would indicate `timeout`; no inventory command timed out.

### Sidecar build matrix

| Command | Execution mode | Exit | Result |
| --- | --- | ---: | --- |
| `rm -rf .tmp/m22-wrappers && mkdir -p .tmp/m22-wrappers` | setup | 0 | Prepared one temporary sidecar directory. |
| `go build -o .tmp/m22-wrappers/octxiliary-io ./cmd/octxiliary-io` | sidecar build | 0 | Built. |
| `go build -o .tmp/m22-wrappers/octxiliary-hash ./cmd/octxiliary-hash` | sidecar build | 0 | Built. |
| `go build -o .tmp/m22-wrappers/octxiliary-compression ./cmd/octxiliary-compression` | sidecar build | 0 | Built. |
| `go build -o .tmp/m22-wrappers/octxiliary-time ./cmd/octxiliary-time` | sidecar build | 0 | Built. |
| `go build -o .tmp/m22-wrappers/octxiliary-text ./cmd/octxiliary-text` | sidecar build | 0 | Built. |
| `go build -o .tmp/m22-wrappers/octxiliary-archive ./cmd/octxiliary-archive` | sidecar build | 0 | Built. |
| `go build -o .tmp/m22-wrappers/octxiliary-json ./cmd/octxiliary-json` | sidecar build | 0 | Built. |
| `go build -o .tmp/m22-wrappers/octxiliary-csv ./cmd/octxiliary-csv` | sidecar build | 0 | Built. |
| `go build -o .tmp/m22-wrappers/octxiliary-plot ./cmd/octxiliary-plot` | sidecar build | 0 | Built. |
| `go build -o .tmp/m22-wrappers/octxiliary-xlsx ./cmd/octxiliary-xlsx` | sidecar build | 0 | Built. |
| `go build -o .tmp/m22-wrappers/octxiliary-image ./cmd/octxiliary-image` | sidecar build | 0 | Built. |
| `go build -o .tmp/m22-wrappers/octxiliary-pdf ./cmd/octxiliary-pdf` | sidecar build | 0 | Built. |

### Required Go checks

| Command | Execution mode | Exit | Duration | Result |
| --- | --- | ---: | ---: | --- |
| `go test ./internal/octxiliary` | Go test | 0 | 0s | Passed. |
| `go test ./internal/pkgmgr ./internal/project` | Go test | 0 | 1s | Passed. |
| `go test ./cmd/oct -run 'GenericOctxiliary|CompiledOctxiliary|Hash|Compression|Time|Text|Archive|Json|Csv|Markdown|Plot|Xlsx|Image|Pdf|UtilityWrappers'` | Go test | 0 | 106s | Passed. |
| `go test ./cmd/oct -run '^TestPkgWrappers'` | Go test | 0 | 2s | Passed. |
| `go test ./internal/... ./cmd/oct` | Go test | 0 | 204s initial / 171s final | Passed both before and after doc/script changes. |

### Whole-root library commands

| Command | Execution mode | Exit | Duration | Result |
| --- | --- | ---: | ---: | --- |
| `go run ./cmd/oct test Libraries --execution interpreted` | interpreted | 0 | 0s | Passed; shallow whole-root discovery only. |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m22-wrappers go run ./cmd/oct test Libraries --execution compiled` | compiled | 0 | 0s | Passed; shallow whole-root discovery only, not a complete inventory. |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m22-wrappers go run ./cmd/oct test Libraries --execution auto` | auto | 0 | 0s | Passed; shallow whole-root discovery only. |

### Per-library command matrix

Every row used `OCT_WRAPPER_PATH=$PWD/.tmp/m22-wrappers` for compiled/auto sidecar availability.

| Package/Directory | Interpreted command status | Compiled command status | Auto command status | Status classification | Failure category | Primary failing symbol/test | Suggested next milestone | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `Analysis` | pass | fail: 31 pass / 5 fail | pass | partial | `generated_go_type_error` | `Normalize*`; `LocalMaximaDoesNotIncludeEndpoints` | M23 generated-Go hardening | Invalid generated `range` switch/body plus one unsupported MIR terminator. |
| `Archive` | pass | pass: 3 pass / 0 fail | pass | pass | — | — | — | M11 archive wrapper migration is compiled-green. |
| `Artifact` | pass | pass: 1 pass / 0 fail | pass | pass | — | — | — | Dedicated artifact lane still compiles. |
| `ArtifactUsage` | fail | fail: command-level manifest error | fail | fail | `missing_manifest_or_dependency` | package root | M23 generated-Go hardening is still recommended globally; cleanup later | `package manifest missing`; not a compiled lowering failure. |
| `Complex` | pass | fail: 0 pass / 9 fail | pass | fail | `compiled_complex_support` | `ComplexSin`, `ComplexSinh` via `Real` | deferred: Complex support pass | Do not implement in M22. |
| `Compression` | pass | pass: 5 pass / 0 fail | pass | pass | — | — | — | M8 wrapper migration is compiled-green. |
| `Cooking` | pass | pass: 40 pass / 0 fail | pass | pass | — | — | — | Pure-Oct library compiles. |
| `Csv` | pass | pass: 6 pass / 0 fail | pass | pass | — | — | — | M13 CSV wrapper migration is compiled-green. |
| `Deployment` | pass | pass: 6 pass / 0 fail | pass | pass | — | — | — | Pure-Oct/support package compiles. |
| `DifferentialEquations` | pass | fail: 0 pass / 6 fail | pass | fail | `callback_or_function_value_lowering` | `DerivativeIdentity` | deferred: function-value lowering | Callback-style ODE helpers unresolved. |
| `Distributions` | pass | pass: 7 pass / 0 fail | pass | pass | — | — | — | Pure-Oct library compiles. |
| `Geometry` | pass | fail: 1 pass / 8 fail | pass | partial | `dimensioned_type_lowering` | `Planar*`, `Solid*` | M23 generated-Go hardening | Generated Go emits invalid dimension exponent syntax (`^`). |
| `Hash` | pass | pass: 5 pass / 0 fail | pass | pass | — | — | — | M7 hash wrapper migration is compiled-green. |
| `IO` | pass | fail: 18 pass / 18 fail | pass | partial | `remaining_wrapper_gap` | `CsvRead*`, `JsonLoad`, `JsonLoadStructured`, `JsonLower` | deferred: structured IO/JSON/Csv cleanup | Core file/dir/bytes IO passes; old IO CSV/JSON surface remains unsupported. |
| `IfErrNotEqualNil` | fail | fail: parse error | fail | fail | `unknown_needs_triage` | `IfErrNotEqualNil.Core.oct` | deferred: syntax/reference cleanup | Parse blocker exists in all modes. |
| `Image` | fail | fail: 2 pass / 3 fail | fail | partial | `sidecar_environment` | `LoadInspectAndSaveRoundTrip`, fixture files | deferred: artifact/path lane cleanup | Image wrapper executes; missing fixture path causes panic. |
| `Interpolation` | pass | fail: 20 pass / 8 fail | pass | partial | `matrix_array_lowering` | bicubic/spline helpers | M23 generated-Go hardening | Representative generated Go uses `float64` where `[]float64` expected. |
| `Json` | pass | pass: 3 pass / 0 fail | pass | pass | — | — | — | M11 safe JSON subset is compiled-green. |
| `LinearAlgebra` | pass | fail: 35 pass / 7 fail | pass | partial | `matrix_array_lowering` | matrix determinant/multiply/inverse paths | M23 generated-Go hardening | Matrix/list assignments and int/float mismatches. |
| `Markdown` | pass | pass: 6 pass / 0 fail | pass | pass | — | — | — | M14 direct compiled helpers are green. |
| `Mathematics` | pass | fail: 7 pass / 14 fail | pass | partial | mixed: `callback_or_function_value_lowering`, `compiled_complex_support`, `generated_go_type_error` | calculus callbacks, FFT/complex helpers | M23 generated-Go hardening | Multiple distinct language/compiler blockers. |
| `Mechanics` | pass | fail: 48 pass / 17 fail | pass | partial | mixed: `matrix_array_lowering`, `dimensioned_type_lowering`, `unknown_needs_triage` | continuum tensor/material helpers | M23 generated-Go hardening | Matrix return shape and dimension type issues dominate. |
| `Numerics` | pass | fail: 0 pass / 6 fail | pass | fail | `callback_or_function_value_lowering` | root-finder callback `f` | deferred: function-value lowering | Root helpers need callback/function-value compilation. |
| `Octomata` | pass | fail: 80 pass / 12 fail | pass | partial | `generated_go_import_error`, `type_inference_or_empty_literal` | commitment/coordination/trace helpers | M23 generated-Go hardening | Unused `math` imports and empty-list type inference mismatches. |
| `Optimization` | pass | fail: 3 pass / 4 fail | pass | partial | `callback_or_function_value_lowering` | golden-section callback `f` | deferred: function-value lowering | Some non-callback tests pass. |
| `Pdf` | pass | fail: 7 pass / 3 fail | pass | partial | `remaining_wrapper_gap` | `Pdf.DrawImage`, `Pdf.DrawImageSized` | deferred: PDF image interop | M21 text/page/save subset passes. |
| `Physics` | fail | fail: 4 pass / 1 fail | fail | partial | `unknown_needs_triage` | `PhysicalConstantsCallableSurface` | M23 generated-Go hardening after triage | Runtime exits with `0`; interpreted also fails. |
| `Plot` | pass | pass: 6 pass / 0 fail | pass | pass | — | — | — | M16 plot wrapper migration is compiled-green. |
| `RF` | pass | fail: 36 pass / 21 fail | pass | partial | mixed: `generated_go_type_error`, `compiled_complex_support`, `matrix_array_lowering` | path-loss/series/complex S-parameter helpers | M23 generated-Go hardening | Many failures are generated int/float/list mismatches; complex remains separate. |
| `Random` | pass | fail: 21 pass / 1 fail | pass | partial | `generated_go_type_error` | `RandomResultsComposeWithExpressionChain` | M23 generated-Go hardening | Record result assignment type mismatch. |
| `Signal` | pass | fail: 34 pass / 3 fail | pass | partial | `compiled_complex_support` | `MagnitudeSpectrum` via complex `Abs` | deferred: Complex support pass | Most signal helpers compile. |
| `Simulation` | pass | fail: 5 pass / 1 fail | pass | partial | `missing_manifest_or_dependency` | `CanonicalOctomataFlow...` | deferred: manifest/dependency cleanup | Unknown `Assert` package from compiled path. |
| `Statistics` | pass | fail: 4 pass / 31 fail | pass | partial | mixed: `generated_go_type_error`, `unknown_needs_triage` | descriptive stats helpers | M23 generated-Go hardening | Includes generated build errors and one compiled runtime panic. |
| `String` | pass | pass: 5 pass / 0 fail | pass | pass | — | — | — | Still compiled-green since M5g. |
| `Structures` | pass | pass: 3 pass / 0 fail | pass | pass | — | — | — | Still compiled-green since M5g. |
| `Text` | pass | pass: 4 pass / 0 fail | pass | pass | — | — | — | M10 text/regex wrapper migration is compiled-green. |
| `Thermofluids` | pass | fail: 3 pass / 8 fail | pass | partial | `dimensioned_type_lowering` | fluid/thermal dimension helpers | M23 generated-Go hardening | Generated Go emits invalid dimension exponent syntax (`^`). |
| `Time` | pass | pass: 4 pass / 0 fail | pass | pass | — | — | — | M9 time wrapper migration is compiled-green. |
| `UI` | pass | fail: 24 pass / 29 fail | pass | partial | mixed: `unsupported_builtin_remaining`, `type_inference_or_empty_literal` | `UIButton`, `UICanvas`, `UIMount`, `UIGridRows` | deferred: UI bridge/native UI project | Live bridge helpers are not generic wrapper work. |
| `Wireless` | pass | pass: 15 pass / 0 fail | pass | pass | — | — | — | Pure-Oct wireless helpers compile. |

### Focused package probes

| Command | Execution mode | Exit | Result |
| --- | --- | ---: | --- |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m22-wrappers go run ./cmd/oct test Libraries/Pdf/Pdf.Core.octest --execution compiled` | compiled | 1 | Focused PDF core confirms 3 text/page/save passes and 3 image interop failures in that file. |

## 4. Failure detail catalog

### M22-F001 — Generated Go emits invalid `range` syntax for normalization helpers

- Category: `generated_go_type_error`.
- Command: `OCT_WRAPPER_PATH=$PWD/.tmp/m22-wrappers go run ./cmd/oct test Libraries/Analysis --execution compiled`.
- Package/test path: `Libraries/Analysis/Analysis.Core.octest`.
- Failing tests/functions: `NormalizeAndCenterComposeDeterministically`, `NormalizePreservesRelativeOrder`, `NormalizeProducesZeroToOneRange`, `NormalizeRejectsConstantSeries`.
- Error excerpt: `.octbuild/...gen.go:120:6: syntax error: unexpected keyword range, expected name`.
- Likely cause: generated-Go lowering emits malformed range/switch/loop code for this control-flow shape.
- Proposed fix class: generated-Go statement/control-flow hardening.
- Proposed milestone: **M23 — generated-Go hardening pass**.

### M22-F002 — Unsupported MIR terminator for one local maxima path

- Category: `generated_go_type_error`.
- Command: `go run ./cmd/oct test Libraries/Analysis --execution compiled`.
- Package/test path: `Libraries/Analysis/Analysis.Core.octest`.
- Failing test/function: `LocalMaximaDoesNotIncludeEndpoints`.
- Error excerpt: `compiled execution required: unsupported MIR terminator <nil>`.
- Likely cause: compiler lowering reaches a missing terminator/control-flow case.
- Proposed fix class: MIR-to-Go lowering hardening with a targeted regression after behavior is understood.
- Proposed milestone: **M23 — generated-Go hardening pass**.

### M22-F003 — Complex component/extraction and complex `Abs` are not compiled

- Category: `compiled_complex_support`.
- Commands: compiled runs for `Libraries/Complex`, `Libraries/Mathematics`, `Libraries/RF`, `Libraries/Signal`.
- Package/test path: `Complex.Functions.octest`, `Mathematics.Transforms.octest`, `RF.SParameters.octest`, `Signal.Core.octest`.
- Failing tests/functions: `ComplexSin`, `ComplexSinh`, FFT near-complex helpers, RF magnitude helpers, signal magnitude spectrum.
- Error excerpts:
  - `function Complex.ComplexSinh: compiled mode does not yet support builtin Real`
  - `function Signal.MagnitudeSpectrum: compiled mode does not yet support builtin Abs for type Complex`
- Likely cause: generated Go and compiled builtin lowering do not yet represent/lower full complex helper surface.
- Proposed fix class: dedicated complex scalar/builtin lowering pass.
- Proposed milestone: deferred `M23/M24 — compiled Complex support pass` only if selected later; not selected for immediate M23.

### M22-F004 — Callback/function-value lowering leaves test helper identifiers unresolved

- Category: `callback_or_function_value_lowering`.
- Commands: compiled runs for `Libraries/DifferentialEquations`, `Libraries/Mathematics`, `Libraries/Numerics`, `Libraries/Optimization`.
- Package/test path: ODE/root/calculus/optimization tests.
- Failing tests/functions: `DerivativeIdentity`, root-finder `f`, calculus helper `f`, golden-section `f`.
- Error excerpts:
  - `unknown identifier 'DerivativeIdentity'`
  - `unknown identifier 'f'`
- Likely cause: function values/callbacks are accepted by interpreted execution but not carried through compiled name/type/lowering.
- Proposed fix class: callback/function-value representation and call lowering.
- Proposed milestone: deferred function-value lowering milestone.

### M22-F005 — Dimensioned type lowering emits invalid Go type syntax

- Category: `dimensioned_type_lowering`.
- Commands: compiled runs for `Libraries/Geometry`, `Libraries/Thermofluids`, plus some mechanics paths.
- Package/test path: `Geometry.Planar.octest`, `Geometry.Solids.octest`, `Thermofluids.Fluid.octest`, `Thermofluids.Thermal.octest`.
- Failing tests/functions: planar/solid geometry, fluid/thermal helpers.
- Error excerpt: `.octbuild/...gen.go:21:23: syntax error: unexpected ^ in type declaration`.
- Likely cause: dimension exponent syntax is leaking into generated Go instead of lowering to a valid host representation.
- Proposed fix class: dimensioned type erasure/alias/lowering hardening in generated Go.
- Proposed milestone: **M23 — generated-Go hardening pass**.

### M22-F006 — Matrix/list/array shape mismatches in generated Go

- Category: `matrix_array_lowering` / `generated_go_type_error`.
- Commands: compiled runs for `Libraries/Interpolation`, `Libraries/LinearAlgebra`, `Libraries/Mechanics`, `Libraries/RF`, and `Libraries/Statistics`.
- Package/test path: bicubic/spline interpolation, linear algebra matrix helpers, continuum mechanics, RF series helpers.
- Failing tests/functions: matrix determinant/multiply/inverse families, spline helpers, RF series functions.
- Error excerpts:
  - `cannot use _t6 (variable of type float64) as []float64 value in assignment`
  - `cannot use _t8 (variable of type float64) as [][]float64 value in return statement`
  - `cannot use _t3 (variable of type []int) as []float64 value in return statement`
- Likely cause: shape/type inference is too weak or too late for generated Go array/matrix temporaries and returns.
- Proposed fix class: generated-Go expression/temporary typing and list/matrix coercion hardening.
- Proposed milestone: **M23 — generated-Go hardening pass**.

### M22-F007 — Empty literal and wrong inferred collection element type

- Category: `type_inference_or_empty_literal`.
- Commands: compiled runs for `Libraries/Octomata`, `Libraries/UI`, and RF sequence-style helpers.
- Package/test path: `Octomata.*`, `UI.M111.octest`, RF sequence tests.
- Failing tests/functions: `M111ResolveEventValueEmptyTableReturnsEmpty`, several Octomata trace/commitment helpers.
- Error excerpts:
  - `cannot use _t0 (variable of type []int) as []string value in struct literal`
  - `cannot use _ as value or type`
- Likely cause: bare `[]` and placeholder/ignored values lose expected type context during compiled lowering.
- Proposed fix class: expected-type propagation for empty literals and discard expression lowering.
- Proposed milestone: **M23 — generated-Go hardening pass**.

### M22-F008 — Generated Go import errors

- Category: `generated_go_import_error`.
- Command: `go run ./cmd/oct test Libraries/Octomata --execution compiled`.
- Package/test path: `Libraries/Octomata/Octomata.Commitment.octest` and related.
- Failing tests/functions: commitment/mode-change helpers.
- Error excerpt: `.octbuild/...gen.go:5:2: "math" imported and not used`.
- Likely cause: generated import set is conservative and not pruned after lowering.
- Proposed fix class: import usage tracking or post-generation pruning.
- Proposed milestone: **M23 — generated-Go hardening pass**.

### M22-F009 — Legacy `IO` CSV/JSON surface is still unsupported in compiled mode

- Category: `remaining_wrapper_gap` / `unsupported_builtin_remaining`.
- Command: `go run ./cmd/oct test Libraries/IO --execution compiled`.
- Package/test path: `Libraries/IO/IO.CoreWrappers.octest`, `Libraries/IO/IO.Json.octest`.
- Failing tests/functions: `CsvReadRows`, `CsvReadMatrix`, `CsvReadTable`, `JsonParse`, `JsonLoad`, `JsonNormalize`, `JsonLoadStructured`, `JsonLower` through `IO`.
- Error excerpts:
  - `compiled mode does not yet support builtin CsvReadRows`
  - `compiled mode does not yet support builtin JsonLoadStructured`
- Likely cause: split packages `Csv` and safe `Json` compile, but legacy/structured `IO` convenience APIs were not migrated in M6-M21.
- Proposed fix class: decide whether these remain interpreted-only, alias to split wrappers, or receive a structured JSON design.
- Proposed milestone: deferred structured IO/JSON cleanup; not immediate M23.

### M22-F010 — PDF image interop remains unsupported

- Category: `remaining_wrapper_gap`.
- Commands: `go run ./cmd/oct test Libraries/Pdf --execution compiled`; focused `go run ./cmd/oct test Libraries/Pdf/Pdf.Core.octest --execution compiled`.
- Package/test path: `Libraries/Pdf/Pdf.Core.octest`.
- Failing tests/functions: `DrawImageAndSave`, `DrawImageSizedAndStyledText`, `InvalidImageHandleFails`.
- Error excerpts:
  - `function Pdf.DrawImage: compiled mode does not yet support builtin PdfDrawImage`
  - `function Pdf.DrawImageSized: compiled mode does not yet support builtin PdfDrawImageSized`
- Likely cause: M21 delivered PDF text/page/save subset; image-handle interop is still deferred.
- Proposed fix class: PDF/image handle interop design and wrapper command support.
- Proposed milestone: deferred `Pdf image interop design/implementation`.

### M22-F011 — Image wrapper reaches runtime but cannot find fixture-relative files

- Category: `sidecar_environment` / `artifact_lane_failure`.
- Command: `go run ./cmd/oct test Libraries/Image --execution compiled`.
- Package/test path: `Libraries/Image/Image.Core.octest`.
- Failing tests/functions: `LoadInspectAndSaveRoundTrip`, `MetadataMatchesJpegFixture`, `SaveUnsupportedExtensionFails`.
- Error excerpt: `panic: unwrap failed: mx103d_fixture_rect.png: open mx103d_fixture_rect.png: no such file or directory`.
- Likely cause: compiled runner working directory/fixture path setup differs from interpreted execution; sidecar itself is present.
- Proposed fix class: compiled test fixture path attribution or test runner cwd setup.
- Proposed milestone: deferred artifact/path lane cleanup.

### M22-F012 — UI bridge helpers are still not compiled

- Category: `unsupported_builtin_remaining` / `expected_interpreted_only` for live bridge pieces.
- Command: `go run ./cmd/oct test Libraries/UI --execution compiled`.
- Package/test path: `Libraries/UI/UI.M0.octest`, `UI.M103.octest`, `UI.M114.octest`, `UI.M2.octest`, `UI.M95.octest`, `UI.TypedCoordinates.octest`.
- Failing tests/functions: `Button`, `Canvas`, `Mount`, `Column`, `Grid`, `Signature`, `GridRows` paths.
- Error excerpts:
  - `function UI.Button: compiled mode does not yet support builtin UIButton`
  - `function UI.Canvas: compiled mode does not yet support builtin UICanvas`
- Likely cause: live UI/native bridge helpers are a separate backend project, not generic Octxiliary scalar/list/bytes wrappers.
- Proposed fix class: UI bridge/native UI compiled backend design.
- Proposed milestone: deferred UI live bridge/native UI milestone.

### M22-F013 — Manifest/dependency visibility issues still obscure two paths

- Category: `missing_manifest_or_dependency`.
- Commands: compiled runs for `Libraries/ArtifactUsage`, `Libraries/Simulation`.
- Package/test path: `Libraries/ArtifactUsage`, `Libraries/Simulation/Simulation.Core.octest`.
- Failing tests/functions: package root; `CanonicalOctomataFlowDrivesDeterministicThreeStepSimulation`.
- Error excerpts:
  - `test failed: package manifest missing`
  - `unknown package 'Assert'`
- Likely cause: package-root/manifest/dependency resolution, not compiled standard-library semantics.
- Proposed fix class: package manifest and test dependency visibility cleanup.
- Proposed milestone: deferred package/test manifest cleanup.

### M22-F014 — Runtime/panic failures need narrower triage after codegen hardening

- Category: `unknown_needs_triage`.
- Commands: compiled runs for `Libraries/Physics`, `Libraries/Statistics`, and selected `Mechanics` paths.
- Package/test path: `Physics.Constants.octest`, `Statistics.Core.octest`.
- Failing tests/functions: `PhysicalConstantsCallableSurface`, `MedianHandlesOddAndEvenDeterministically`.
- Error excerpts:
  - `compiled test run failed: exit status 1: 0`
  - `panic: runtime error: index out of range [-1]`
- Likely cause: not yet isolated; could be generated control flow/index lowering, assertion/result plumbing, or real semantic mismatch.
- Proposed fix class: re-run after generated-Go hardening and isolate with minimized `.octest` cases.
- Proposed milestone: **M23 — generated-Go hardening pass** first, then triage remaining runtime mismatches.

### M22-F015 — Parse blocker in `IfErrNotEqualNil`

- Category: `unknown_needs_triage` / possible reference-test drift.
- Command: `go run ./cmd/oct test Libraries/IfErrNotEqualNil --execution compiled`.
- Package/test path: `Libraries/IfErrNotEqualNil/IfErrNotEqualNil.Core.oct`.
- Failing test/function: package parse.
- Error excerpt: `expected ')' after parameter list at 6:32 near "!"`.
- Likely cause: syntax in this library is not accepted by current parser in any mode.
- Proposed fix class: compare with `Language/reference`; either fix stale library syntax or document intended syntax.
- Proposed milestone: deferred syntax/reference cleanup, not M23 unless it blocks generated-Go minimization.

## 5. Failure categories used

- `compiled_complex_support`: complex numbers and complex builtins (`Real`, complex `Abs`) are not represented/lowered fully in generated Go.
- `compiled_einstein_support`: no active M22 library failure clearly reported an Einstein-specific diagnostic; keep this category for future language inventory.
- `generated_go_type_error`: invalid generated Go types, assignments, result wrappers, syntax, invalid placeholder `_`, and dimension syntax leaks.
- `generated_go_import_error`: unused `math` imports in generated Go.
- `type_inference_or_empty_literal`: empty list/record arrays inferred as `[]int` or otherwise without expected context.
- `dimensioned_type_lowering`: dimension syntax such as exponent markers leaks into Go types.
- `callback_or_function_value_lowering`: function values/callbacks/helper identifiers are not lowered.
- `matrix_array_lowering`: matrix/vector/list shape mismatches in generated Go.
- `unsupported_builtin_remaining`: UI bridge helpers and complex builtins still lack compiled support.
- `remaining_wrapper_gap`: legacy `IO` CSV/JSON structured APIs and PDF image interop remain outside M6-M21 compiled wrapper coverage.
- `sidecar_environment`: sidecar exists but fixtures/paths are unavailable to compiled execution.
- `artifact_lane_failure`: fixture/path attribution and output cleanup issues; currently folded into Image path failures.
- `missing_manifest_or_dependency`: package manifest and `Assert` dependency resolution issues.
- `expected_interpreted_only`: live UI bridge style tests may remain interpreted-only until a native/UI bridge milestone.
- `unknown_needs_triage`: parse/runtime failures not yet minimized.

## 6. Wrapper migration delta: M5g to M22

### Packages moved from fail to pass/partial due to M6-M21

- `Archive`: M5g wrapper builtin failures moved to compiled pass after M11.
- `Compression`: M5g gzip builtin failures moved to compiled pass after M8.
- `Csv`: M5g CSV wrapper gaps moved to compiled pass after M13.
- `Hash`: M5g hash builtin failures moved to compiled pass after M7.
- `Json`: safe JSON subset moved to compiled pass after M11.
- `Markdown`: direct compiled helpers moved to compiled pass after M14.
- `Plot`: record-argument/wrapper transport moved to compiled pass after M16.
- `Text`: regex/text helpers moved to compiled pass after M10.
- `Time`: time helpers moved to compiled pass after M9.
- `Pdf`: text/page/save subset moved to partial pass after M21; image interop remains.
- `IO`: file/dir/bytes core now passes with `octxiliary-io`; old IO JSON/CSV APIs remain unsupported.
- `Image`: image wrapper is present and compiled execution reaches it; remaining failures are fixture path/environment rather than missing image builtin lowering.

### Wrappers now compiled-supported

- IO file/dir/bytes core.
- Hash.
- Compression.
- Time.
- Text/Regex.
- Archive.
- Json safe subset.
- Csv.
- Markdown direct compiled helpers.
- Plot.
- IO.Xlsx wrapper tests in Go and sidecar build path; no top-level `Libraries/IO/Xlsx` directory exists in this inventory table.
- Image wrapper transport, subject to fixture path issue in library tests.
- Pdf text/page/save subset.

### Remaining wrapper/API gaps

- PDF image interop (`PdfDrawImage`, `PdfDrawImageSized`).
- Structured/legacy JSON graph/table APIs through `IO` (`JsonLoadStructured`, `JsonLower`, broader `JsonLoad`/`JsonNormalize`).
- Legacy `IO` CSV convenience builtins (`CsvReadRows`, `CsvReadMatrix`, `CsvReadTable`) despite split `Csv` package passing.
- Live UI bridge helpers (`UIButton`, `UICanvas`, `UIMount`, `UIGridRows`, etc.).
- Package-manager/build lifecycle for sidecars remains manual for this audit; `.tmp/m22-wrappers` and `OCT_WRAPPER_PATH` were required.

## 7. Standard-library compiled support map

| Library/API area | Intended strategy | Current M22 state | Remaining blocker | Priority |
| --- | --- | --- | --- | --- |
| `Archive` | `generic_octxiliary_wrapper` | pass | none observed | done |
| `Artifact` | `pure_oct_compiled` / artifact lane | pass | whole-root artifact layout remains separate from this inventory | low |
| `Compression` | `generic_octxiliary_wrapper` | pass | none observed | done |
| `Csv` | `generic_octxiliary_wrapper` with `String[][]` support | pass | legacy `IO.Csv*` APIs separate | done for split package |
| `Hash` | `generic_octxiliary_wrapper` | pass | none observed | done |
| `IO` core file/dir/bytes | `generic_octxiliary_wrapper` | partial pass | structured JSON/CSV legacy APIs | medium/deferred |
| `Json` safe subset | `generic_octxiliary_wrapper` | pass | structured graph/table APIs are outside safe subset | done for safe subset |
| `Markdown` | `direct_compiled_helper` | pass | none observed | done |
| `Plot` | `record_arg_octxiliary_wrapper` | pass | package-manager sidecar lifecycle manual | done for API |
| `Text` / Regex | `generic_octxiliary_wrapper` | pass | none observed | done |
| `Time` | `generic_octxiliary_wrapper` | pass | none observed | done |
| `Image` | `handle_octxiliary_wrapper` | partial | fixture/cwd path setup in compiled tests | medium/deferred |
| `Pdf` text/page/save | `handle_octxiliary_wrapper` | partial pass | image interop deferred | medium/deferred |
| `Pdf` image interop | `deferred_api_design` | fail for image tests | image handle transport between Image and Pdf | deferred |
| `UI` live bridge | `expected_interpreted_only` or future native bridge | partial | UI builtins not compiled | deferred |
| `Complex` / FFT / RF complex helpers | `compiled_language_feature_needed` | fail/partial | complex builtins and representation | high after M23 |
| ODE/Numerics/Optimization callbacks | `compiled_language_feature_needed` | fail/partial | function values/callback lowering | high after M23 |
| Geometry/Thermofluids dimensioned helpers | `compiled_language_feature_needed` + generated-Go hardening | partial | dimension syntax/type lowering | high in M23 |
| LinearAlgebra/Interpolation/Mechanics/RF/Statistics matrices | `pure_oct_compiled` with matrix lowering | partial | array/matrix type mismatches | high in M23 |
| Octomata/Random/Statistics generated edges | `pure_oct_compiled` | partial | imports, empty literals, record result assignments | high in M23 |
| ArtifactUsage/Simulation dependency edges | package/test manifest cleanup | fail/partial | manifest and `Assert` visibility | low/deferred |

## 8. Recommended next implementation milestone

**M23 — generated-Go hardening pass**.

Scope should be bounded to compiler/codegen defects that are deterministic and visible in this inventory:

1. invalid generated syntax (`range`, dimension `^` leaks);
2. invalid Go type assignments and returns for arrays/matrices/records;
3. unused/missing imports;
4. empty-literal expected-type propagation;
5. invalid `_` placeholder emission;
6. runtime/index failures only when they reduce to generated code shape bugs.

The milestone should avoid implementing Complex support, callback/function-value lowering, PDF image interop, UI live bridge, or sidecar package-manager lifecycle unless one turns out to be a directly necessary minimized generated-Go regression.

## 9. Explicit non-fix / deferred notes

Do **not** fold the following into the immediate next milestone unless a later decision explicitly changes scope:

- third-party wrapper package-manager build lifecycle and automatic sidecar builds;
- PDF image interop if generated-Go hardening remains the larger blocker;
- structured JSON graph/table APIs if not selected as a dedicated wrapper/API design milestone;
- UI live bridge/native UI runtime;
- broad package-manager native build system;
- interpreted-only/live/bridge tests that should intentionally remain interpreted-only;
- Complex support;
- Einstein notation support (no clear active M22 standard-library failure signature, but still a known compiled feature gap);
- broad test rewrites or weakening tests to hide failures.

## 10. Optional helper script

Added `scripts/dev/stdlib_compiled_inventory.sh` as a non-CI helper. It:

- builds the requested sidecars into `.tmp/m22-wrappers`;
- writes logs and `commands.tsv` under `.tmp/stdlib-compiled-inventory`;
- runs whole-root and per-`Libraries/*` interpreted/compiled/auto commands;
- prints a compact per-library compiled summary;
- changes no repository state except temporary `.tmp` logs and sidecar binaries.

This script is intentionally not wired into CI because this inventory includes expected compiled failures and can take several minutes.
