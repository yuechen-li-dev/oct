# W7d compiled fixture consolidation and test runtime reduction

## Scope

W7d focused on the remaining compile-and-run test cost identified by W7c. The target was not language behavior or compiled output semantics; it was test shape. The main change was to preserve several small compiled lowering regressions inside one themed fixture so the suite pays the generated-Go and `go build` cost once instead of once per tiny program.

## Measurement commands

Baseline and after measurements were captured with:

```sh
go test ./internal/build -count=1 -json > /tmp/oct-build-test-w7d-before.json
go test ./cmd/oct -count=1 -json > /tmp/oct-cmd-test-w7d-before.json
go test ./internal/... ./cmd/oct -count=1 -json > /tmp/oct-full-test-w7d-before.json

go test ./internal/build -count=1 -json > /tmp/oct-build-test-w7d-after.json
go test ./cmd/oct -count=1 -json > /tmp/oct-cmd-test-w7d-after.json
go test ./internal/... ./cmd/oct -count=1 -json > /tmp/oct-full-test-w7d-after.json
```

The JSON files were inspected by collecting package-level `pass` events and sorting per-test `Elapsed` values.

## Baseline timings

| Command | Package elapsed |
| --- | ---: |
| `go test ./internal/build -count=1 -json` | 45.811s |
| `go test ./cmd/oct -count=1 -json` | 351.471s |
| `go test ./internal/... ./cmd/oct -count=1 -json` | `cmd/oct` 346.489s; `internal/build` 52.450s |

Note: the full-suite baseline command was part of the requested baseline sequence. The `internal/build` package-level number in that full run includes scheduler/cache noise relative to the focused `internal/build` baseline; the focused `internal/build` baseline is the primary comparison for this W7d change.

## Top slow tests before W7d

### `internal/build`

| Test | Elapsed |
| --- | ---: |
| `TestCompileAndRunNumericArrayLoweringM24` | 4.080s |
| `TestCompileAndRunOctagonRoundTrip` | 3.300s |
| `TestCompileAndRunBatchDeterministicRepeatedRuns` | 3.270s |
| `TestCompileAndRunLogicalOperatorsShortCircuitIndexExpressions` | 3.030s |
| `TestCompileAndRunCrossPackageFallibleAndEnum` | 2.970s |
| `TestCompileAndRunSubsetProgram` | 2.820s |
| `TestCompileAndRunBatchEmptyAndSingleElement` | 2.820s |
| `TestCompileAndRunNamedRecordArraySurface` | 2.810s |
| `TestCompileAndRunLoopLoweringParity` | 2.540s |
| `TestCompileAndRunBatchParameterSweepAndOrder` | 2.530s |

`TestCompileAndRunNumericArrayLoweringM24` was the most direct consolidation candidate because it compiled five independent tiny programs in subtests. `TestCompileAndRunMatrixScalarElementwiseRegression` was a compatible sixth small lowering regression: it was a positive compile-and-run check, did not assert an independent compile-time diagnostic, did not need a unique package layout beyond `Main`, and did not inspect generated Go.

### `cmd/oct`

| Test | Elapsed |
| --- | ---: |
| `TestIOCoreWrappers` | 42.400s |
| `TestIOJsonGoldenWrapper` | 30.270s |
| `TestUtilityWrappers` | 23.170s |
| `TestIOXlsxWrapper` | 19.850s |
| `TestEnumAwareSwitch` | 12.700s |
| `TestStringErgonomics` | 12.210s |
| `TestPdfCoreWrappers` | 10.580s |
| `TestNativeHostBoundaryConditionSwitchContractsRunAsNativeOctTests` | 9.970s |
| `TestCompiledArchiveOctxiliaryWrapper` | 9.750s |
| `TestCompiledPlotOctxiliaryWrapper` | 9.260s |

The slowest `cmd/oct` tests are still user-facing CLI integration and wrapper/Octxiliary coverage. W7d left these in place because the measured slow paths are validating CLI behavior, wrapper diagnostics, broad package behavior, or representative compiled sidecar behavior rather than merely retesting compiler lowering.

## Refactors performed

### Numeric/shape lowering chimera

Replaced six separate compiled runs with one themed compiled fixture in `internal/build`:

| Old regression/category | New chimera check |
| --- | --- |
| mixed `Int`/`Float` arithmetic lowers as `Float` | `MixedIntFloatArithmeticCompilesAsFloat` |
| `Float[]` argument context coerces integer array literal elements | `TypedFloatArrayLiteralCoercesIntegerElements` |
| `Float[]` record field context coerces integer array literal elements | `RecordFloatArrayFieldCoercesIntegerLiteralElements` |
| `for` loop index keeps `Int` shape when a later `Float` local shadows the same name | `LoopIndexKeepsIntShapeWhenLaterFloatLocalShadowsName` |
| the same source-level name can hold distinct record result shapes in separate bindings | `SameLogicalNameCanHoldDistinctRecordResultShapes` |
| matrix/scalar elementwise lowering and stress-style matrix composition work | `MatrixScalarElementwiseRegressionWorks` |

The new `TestCompileAndRunNumericShapeLoweringChimera` writes one `Main` package, compiles it once, runs it once, and maps stable integer failure codes back to the original regression labels. This keeps failures localized without requiring six generated-Go builds.

### Coverage equivalence

Coverage is equivalent because each old positive regression still has a dedicated Oct function and a distinct failure code. The consolidation does not merge invalid/diagnostic tests, generated-Go inspection tests, package-layout tests, CLI behavior tests, or sidecar failure tests. It only combines compatible positive lowering checks that previously differed by source snippet and expected scalar output.

## Before/after comparison

| Command | Before | After | Notes |
| --- | ---: | ---: | --- |
| `go test ./internal/build -count=1 -json` | 45.811s | 34.576s | Focused package runtime dropped; the former top `TestCompileAndRunNumericArrayLoweringM24` no longer performs five compiled sub-runs. |
| `go test ./cmd/oct -count=1 -json` | 351.471s | 365.339s | No `cmd/oct` tests were changed in W7d; the slower after run is environmental/cache noise in the unchanged CLI integration suite. |
| `go test ./internal/... ./cmd/oct -count=1 -json` | `cmd/oct` 346.489s; `internal/build` 52.450s | `cmd/oct` 357.884s; `internal/build` 48.770s | Full-suite runtime is still dominated by `cmd/oct` integration and wrapper tests; `internal/build` improved in the combined run despite noise. |

## Remaining slow paths

* `cmd/oct` remains dominated by broad CLI and wrapper integration tests (`IO`, utility wrappers, PDF/Image/Plot/Archive/Compression/Csv/Hash sidecar paths). These should stay as CLI tests unless replaced with one representative CLI test plus lower-level coverage for the mechanism under test.
* `internal/build` still has several compile-and-run tests that each pay a generated-Go build: Octagon round trips, batch execution, cross-package fallible/enum behavior, loop lowering parity, and named record arrays. Some may be future chimera candidates, but several have package-layout, artifact, deterministic repeated-run, or output-specific reasons to remain separate.
* Missing-sidecar diagnostics remain intentionally user-facing and expensive. They should not be removed without one representative CLI diagnostic per mechanism.
