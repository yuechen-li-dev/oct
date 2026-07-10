# Go test T3 — compiled harnesses and native artifact names

## Outcome

T3 completes the T1–T3 test-infrastructure refactor. Compiled Octests now pay
one native compilation per package-qualified declared Suite, or per unsuited
`.octest` file. Theory rows are harness data/cases rather than compilation
units. Native executables use target-platform names; `.octbin` is reserved for
a future Oct-owned portable format.

On the measured Windows workstation, the integration wall improved from the
T2 baseline of 47.637s to 43.048s (-4.589s, -9.6%). The toolchain wall was
27.472s versus 27.108s in T2 (+0.364s, normal variance). The compiler package
proof-layer refactor reduced its tagged package time from 9.311s to 7.264s
(-22.0%). The default, integration, toolchain, combined, and real-sidecar lanes
all passed.

## `.octbin` audit and artifact decision

Before T3, `go build -o` wrote native executables with an `.octbin` suffix.
Representative Windows output began with `MZ` and launched directly as a PE
executable. It was not serialized IR, bytecode, a portable container, or a
wrapper. Producers were ordinary `oct build`, compiled Octest/benchmark/
artifact runners, and tests/helpers invoking compiler output. Consumers were
direct executable launches, sibling sidecar discovery, CLI tests, cleanup
patterns, CI checks, and historical documentation.

`build.ArtifactKind`, `Target`, and `OutputPath` now own naming:

| Kind | Windows | Linux | macOS |
|---|---|---|---|
| executable/test executable | `.exe` | no suffix | no suffix |
| shared library | `.dll` | `.so` | `.dylib` |
| static/import library | `.lib` | `.a` | `.a` |
| portable Oct artifact | `.octbin` | `.octbin` | `.octbin` |

No portable emitter exists, so `.octbin` is reserved and native builds do not
emit it. Historical cleanup remains narrowly limited to the old generated
runner pattern; it never deletes arbitrary executables or portable artifacts.

## Suite/file harness contract and isolation

- `[Suite("Numerics")]` groups selected cases by package identity plus Suite
  identity. The same display name in another package is a different harness.
- A case with no Suite belongs to its source-file harness. There is no package-
  wide fallback batch.
- Multiple files may contribute to one declared Suite. Selected-file loading
  includes only the selected runner/test files while preserving ordinary
  package sources and imports.
- Stable case IDs include package, normalized source path, and display identity;
  theory identities include their deterministic row index.

Each generated harness contains zero-argument case helpers and accepts
`--case <stable-id>`. Successful dispatch emits a compact JSON record. The Go
parent launches the already-compiled harness once per case. This keeps the
current strong isolation semantics: assertions and runtime failures map to the
selected case, a timeout kills only that invocation, and a fatal crash cannot
erase previously completed attribution. `--all` is not the normal path because
compiled assertions currently terminate the process with `os.Exit(1)`.

Harness files live in one T2-owned scope per group. Cleanup occurs after all
case processes close. `OCT_KEEP_TEST_ARTIFACTS=1` retains and reports the owned
directory; persistent `oct build` output remains outside test scopes.

## Compile-count proof

The focused proof fixture contains two facts in one Suite across two files and
three theory rows in one unsuited file:

```text
cases=5
suite_groups=1
file_groups=1
harness_groups=2
native_compilations=2
process_launches=5
single_case_reruns=0
artifact_scopes_created=2
artifact_scopes_cleaned=2
```

The pre-T3 strategy required five native compilations for the same five rows;
T3 requires two, a 60% reduction. Adding theory rows changes case/process counts
but not the file-group compilation count. Separate focused contracts cover a
multi-file Suite, identical Suite names in different packages, mixed suited and
unsuited files, multiple theory rows, and selected-file filtering.

`OCT_TEST_METRICS=1` is process-local and reports all fields above. It adds no
persistent cache or cross-run state.

## Compiler proof layers

Every `internal/build/compiler_test.go` test was classified as follows.

Pure lowering/IR/type diagnostic:

- `TestLowerProgramBuildsMIRShape`
- `TestMIRDumpShowsExplicitOctagonRuntimeCalls`
- `TestMIRDumpShowsExplicitBatchLowering`
- `TestCompileFlowDecisionDoesNotUseSpecialCaseShimPath`
- `TestCompileFlowRememberResumeMIRDump`
- `TestCompileModeUnsupportedSurfaceDiagnostics`
- `TestCompileVectorsMatricesBoundaryErrorsM93`
- `TestCompileFailsWhenImportedPackageMissingSymbol`

Generated source/artifact structure:

- `TestCompileWritesMIRDumpWhenRequested`
- `TestCompileUsesTemporaryForDiscardForLoopVariable`

Real native executable boundary:

- `TestCompileAndRunSubsetProgram`
- `TestCompileForTestLowersBenchmarkFunctionIntoMIRAndRuns`
- `TestCompileForTestLowersBenchmarkPrometheusBlockIntoMIRAndRuns`
- `TestCompileAndRunCrossPackageFallibleAndEnum`
- `TestCompileAndRunNamedRecordArraySurface`
- `TestCompileAndRunLoopLoweringParity`
- `TestCompileResolvesManifestDependencyFromPackageCache`
- `TestCompileAndRunBatchParameterSweepAndOrder`
- `TestCompileAndRunLogicalOperatorsShortCircuitIndexExpressions`
- `TestCompileAndRunBatchDeterministicRepeatedRuns`
- `TestCompileAndRunBatchEmptyAndSingleElement`
- `TestCompileAndRunBatchFailWholeBatch`
- `TestCompileAndRunFalliblePropagationAndMatch`
- `TestCompileAndRunFallibleMatchErrBranch`
- `TestCompileAndRunFallibleUnwrap`
- `TestCompileFallibleUnwrapFailureIsFatal`
- `TestCompileAndRunWriteOctagonSuccess`
- `TestCompileAndRunLoadOctagonSuccessAndFallibleIntegration`
- `TestCompileAndRunOctagonRoundTrip`
- `TestCompileAndRunLoadOctagonFailureReturnsErr`
- `TestCompileAndRunFlowCoreRuntimeBuiltins`
- `TestCompileAndRunComplexBuiltinsAndArithmetic`
- `TestCompileAndRunFloatBuiltinConvertsInt`
- `TestCompileKeepsIntArgumentAfterFloatUse`
- `TestCompileCoercesIntArrayLiteralsInFloatArrayContexts`
- `TestCompileFlowResultBeforeCompletionFails`
- `TestCompileAndRunFlowRememberResumeAndResumeTarget`
- `TestCompileFlowResumeEmptySlotFailsDeterministically`
- `TestCompileFlowBoardAndWhenActionBlock`
- `TestCompileFlowUtilityWhenUsesMIRAndRuntimeState`
- `TestCompileAndRunStandaloneUtilityWhenParity`
- `TestCompileAndRunVectorsMatricesM93`
- `TestCompileRunParityVectorsMatricesM93`
- `TestCompileAndRunIfExpressionConditionSwitchSurface`
- `TestCompileForTestLowersPrometheusMatMulBuiltinOutsidePrometheusBlock`
- `TestCompileAndRunNumericShapeLoweringChimera`
- `TestCompileFirstClassRangeExpressionsOutsideForLoop`

The pure/structure tests use `inspectProgram` to typecheck, lower, and emit Go
without invoking `go build` or executing an artifact. Runtime families remain
end-to-end because generated Go compilation or process behavior is part of
their proof.

Coverage mapping:

| Reduced heavyweight proof | Cheaper replacement | Retained connection proof |
|---|---|---|
| Octagon MIR calls | direct `lowerProgram` + `dumpMIR` | Octagon write/load/round-trip executable tests |
| batch MIR worker shape | direct MIR inspection | batch order/empty/failure executable tests |
| flow decision and remember/resume MIR | direct MIR inspection | flow core, board, utility, resume executable tests |
| discard-loop generated Go safety | direct `emitGo` inspection | loop-lowering executable parity |
| unsupported/vector boundary diagnostics | typecheck/lower/emit inspection | vectors/matrices executable and parity tests |

## Explicit CLI execution context

`cli.ExecutionContext` carries `WorkingDir`, `CacheRoot`, `Stdin`, `Stdout`, and
`Stderr`. `main` and existing callers retain environment/default behavior via
`cli.Execute`; in-process tests pass explicit values through
`cli.ExecuteWithContext`.

The shared `executeCLIInDir` helper no longer calls `os.Chdir`. New/init/pkg,
ordinary path commands, and registry/wrapper output paths resolve from the
explicit working directory. Package manager and experiment Git tests can use
an explicit cache root through `NewManagerWithCacheDir` and
`RunFromGitWithCacheDir`; the experiment family no longer mutates
`OCT_PKG_CACHE_DIR`. Independent context tests run in parallel and prove the
process cwd is unchanged.

Remaining serial families intentionally mutate other environment variables,
committed generated files, or shared sidecar/native resources. The one direct
`os.Chdir` site left in `cmd/oct` is a fixed-output Prometheus artifact test and
remains serial pending an artifact-output-root API; it is not part of ordinary
CLI package execution.

## Timing and validation evidence

| Lane | Wall | `cmd/oct` package | Result |
|---|---:|---:|---|
| default | 6.287s | 0.257s | pass |
| integration | 43.048s | 40.124s | pass |
| toolchain | 27.472s | 24.423s | pass |
| integration + toolchain | 66.507s | 63.426s | pass |
| 13-sidecar build | 10.412s | n/a | pass |
| real sidecar validation | 25.331s | 24.284s | pass |

The slowest integration families are still genuine compiler/process boundaries:
Suite selection (5.59s), representative generated-Go hardening (5.50s),
compiled Markdown without a sidecar (5.31s), selected-file compiled linkage
(5.22s), and String ergonomics (5.00s). The under-30-second stretch target was
not met because those retained boundaries now dominate; semantics were not
weakened to chase it.

Final audit after all lanes: zero repository harness files, zero owned temp
roots, zero `.octbin` native outputs, and zero generated `.octbuild/gen-*`
directories. Persistent user-build behavior and Windows PE `MZ` coverage pass.

Evidence:

- `out/test-artifacts/go_test_t3_baseline.{json,md}`
- `out/test-artifacts/go_test_t3_after.{json,md}`
- per-lane `go_test_t3_*_after_events.json`
- `out/test-artifacts/go_test_t3_compile_count_proof.json`
- `out/test-artifacts/compiler_t3_{pre,post}_split.json`

## Stable T1–T3 architecture

T1 established fast default feedback and explicit integration/toolchain/native
lanes. T2 established owned test artifacts, unconditional cleanup, retention,
and bounded external work. T3 establishes Suite/file compilation boundaries,
stable case replay, platform-native artifact naming, proof-layered compiler
tests, explicit CLI execution context, and measurable runner seams.

The T1–T3 test-infrastructure refactor is complete. Prometheus M31 is not part
of this work.
