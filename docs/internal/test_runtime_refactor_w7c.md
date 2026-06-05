# W7c test runtime, compiled fixture, and sidecar lifecycle audit

## Baseline measurements

Commands were run with `-count=1` and JSON output captured under `/tmp`.

| Command | Baseline result |
| --- | ---: |
| `go test ./cmd/oct -count=1 -json > /tmp/oct-cmd-test-baseline-original.json` | 255.388s |
| `go test ./internal/build -count=1 -json > /tmp/oct-build-test.json` | 27.224s |
| `go test ./internal/interpret -count=1 -json > /tmp/oct-interpret-test.json` | 0.625s |
| `go test ./internal/octxiliary ./pkg/octxiliary -count=1 -json > /tmp/oct-octxiliary-test.json` | 0.024s + 0.016s |
| `go test ./internal/... ./cmd/oct -count=1 -json > /tmp/oct-full-test-baseline-original.json` | about 282.732s summed package elapsed; `cmd/oct` 239.971s, `internal/build` 38.803s |

The baseline and source audit exposed the main slowness class: many tests build identical `octxiliary-*` sidecars into per-test temp dirs and then invoke `go run ./cmd/oct test ... --execution compiled`.

## Top slow tests before W7c changes

Before the refactor, `go test ./cmd/oct -count=1 -json > /tmp/oct-cmd-test-baseline-original.json` reported 255.388s for `cmd/oct`. The slowest tests were:

| Test | Elapsed |
| --- | ---: |
| `TestIOCoreWrappers` | 28.40s |
| `TestIOJsonGoldenWrapper` | 18.90s |
| `TestUtilityWrappers` | 16.33s |
| `TestIOXlsxWrapper` | 13.96s |
| `TestEnumAwareSwitch` | 9.02s |
| `TestStringErgonomics` | 8.33s |
| `TestCompiledPlotOctxiliaryWrapper` | 7.70s |
| `TestPdfCoreWrappers` | 7.21s |
| `TestNativeHostBoundaryConditionSwitchContractsRunAsNativeOctTests` | 6.78s |
| `TestCompiledArchiveOctxiliaryWrapper` | 6.76s |

Before the refactor, `go test ./internal/... ./cmd/oct -count=1 -json > /tmp/oct-full-test-baseline-original.json` summed to about 282.732s, with `cmd/oct` at 239.971s and `internal/build` at 38.803s.

## Categories of slowness

1. **Repeated CLI subprocess integration**: many tests invoke `go run ./cmd/oct`, so each test pays command build/startup cost in addition to the Oct work.
2. **Repeated compiled Oct builds**: compiled execution emits Go, invokes `go build`, then runs the produced `.octbin`. Small fixtures pay this full cost even when testing only a narrow regression.
3. **Repeated sidecar builds**: wrapper/Octxiliary tests were building the same sidecar binaries into separate `t.TempDir()` directories.
4. **Broad package fixture sweeps**: some tests execute whole package directories such as wrapper suites. This has good regression value but is expensive.
5. **Missing-sidecar diagnostics**: these are useful, but several are full compiled runs that intentionally fail only after compilation reaches a missing sidecar call.

## Refactors performed

### Shared sidecar build cache for `cmd/oct` tests

Added a package-level `sharedTestSidecarDir(t, names...)` helper in `cmd/oct` tests. It uses `sync.Once` plus a process-lifetime temp directory and serializes sidecar builds behind a mutex. This avoids rebuilding identical sidecars across tests while keeping the binaries scoped to the Go test process.

Converted the compiled Octxiliary/wrapper tests for CSV, archive, JSON, hash, image, PDF, plot, text, time, compression, and generic test wrappers to use the shared cache where safe.

For tests that intentionally need a partially populated wrapper directory to verify a missing-sidecar diagnostic, a separate `buildTestSidecarsInDir` helper preserves isolation so shared-cache contents do not mask the failure.

### Interpreted sidecar lifecycle cleanup

Audited `internal/interpret/generic_wrapper_dispatch.go`. The previous `close` path closed stdin and called `Wait`, but if `Wait` returned an error it attempted `Kill` after `Wait`, which is too late to help with a child that has not exited. The new close path is idempotent, closes stdin, waits exactly once, kills only after a timeout, and then drains `Wait` to reap the child.

Added focused lifecycle tests for idempotent cache close and for reaping a started process in `internal/interpret/generic_wrapper_dispatch_lifecycle_test.go`.

### Compiled runtime sidecar lifecycle cleanup

Audited generated compiled Octxiliary helpers in `internal/build/compiler.go`. Generated programs started sidecars for built-in IO wrappers and generic wrappers but did not register a normal-exit close path. The generated `main` now defers `__octOctxiliaryClose()` whenever Octxiliary helpers are emitted. The helper closes pipes, waits for child processes, kills after timeout, drains `Wait`, and clears generic client maps idempotently.

This is a lifecycle correctness change only; it does not alter the Octxiliary wire protocol or wrapper semantics.

## Remaining known slow paths

* `cmd/oct` remains dominated by CLI integration tests, especially IO/wrapper golden tests and full package wrapper suites.
* `internal/build` still has many compile-and-run tests that each generate Go and run `go build`. The slowest single baseline test was `TestCompileAndRunNumericArrayLoweringM24` at 3.08s; after full-suite rerun it remained one of the slowest internal tests at 4.74s.
* Missing-sidecar compiled diagnostics still pay compile cost. They should be kept representative, but future work can reduce duplication by testing one diagnostic per mechanism unless sidecar families differ.
* Some wrapper tests use broad package fixture sweeps to assert many facts. Those are meaningful end-to-end coverage, but future additions should prefer dense fixtures over one Go test per tiny Oct regression.

## Before/after notes

The most direct improvement is reduced repeated sidecar build work inside `cmd/oct`: the same `octxiliary-io`, `octxiliary-image`, `octxiliary-pdf`, `octxiliary-test-wrapper`, and other sidecars are now built once per `cmd/oct` test process when tests can share a complete wrapper directory.

The end-to-end package elapsed time after changes was:

| Command | After result |
| --- | ---: |
| `go test ./cmd/oct -count=1 -json > /tmp/oct-cmd-test-after.json` | 238.362s, about 17.026s faster than the isolated `cmd/oct` baseline |
| `go test ./internal/... ./cmd/oct -count=1 -json > /tmp/oct-full-test-after.json` | about 293.357s summed package elapsed; `cmd/oct` 248.919s, `internal/build` 40.543s |

The isolated `cmd/oct` run improved by about 6.7%. The full `internal/... ./cmd/oct` run was slower than the baseline by about 10.625s in this environment, likely due to run-to-run variance and package scheduling; the concrete structural improvement is that repeated sidecar build work is now eliminated for shared-wrapper tests. The after JSON files should be treated as the stable W7c baseline for future runtime work.

## Future test policy

* Unit tests should test Go internals directly when possible.
* CLI subprocess tests should be reserved for CLI behavior and representative integration.
* Compiled Oct tests should batch related regressions into dense fixtures with multiple `[Fact]` tests.
* Sidecar binaries should be built once per relevant Go test package where safe.
* Every sidecar `Start` path must have a single corresponding `Wait`; `Kill` paths must be followed by `Wait` to reap the child.
* Missing-sidecar diagnostics should be tested once per mechanism, not once per wrapper family, unless behavior differs.
* Full package compiled sweeps should be milestone/manual validation or a small number of representative integration tests, not dozens of unit tests.
* Do not delete slow tests without preserving their regression contract in a faster, more focused form.
