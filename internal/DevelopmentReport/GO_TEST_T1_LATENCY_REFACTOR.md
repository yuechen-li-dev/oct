# Go test T1 latency refactor

## Outcome

T1 reached the fast-lane targets on the measured Windows workstation without changing production behavior. `cmd/oct` package time fell from 74.217s to 0.113s (99.8%); its uncached command wall time fell from 75.250s to 1.107s. The `internal/...` uncached command wall time fell from 12.532s to 4.514s. No remaining default package takes ten seconds.

| Measurement | Before | After | Change |
|---|---:|---:|---:|
| `cmd/oct` uncached package | 74.217s | 0.113s | -99.8% |
| `cmd/oct` uncached wall | 75.250s | 1.107s | -98.5% |
| `cmd/oct` warm package | 73.426s | 0.121s | -99.8% |
| `internal/...` uncached wall | 12.532s | 4.514s | -64.0% |
| `cmd/oct` build/init-only package | 0.065s | 0.067s | unchanged |
| `internal/...` build/init-only wall | 3.065s | 3.632s | normal run variance |

The baseline warm `cmd/oct` command had an anomalous 121.972s wall time while Go reported 73.426s in-package. The uncached run and package event stream are therefore the primary comparison. The post-change warm command completed in 1.312s wall time.

## Autopsy

The bottleneck was test execution, not package initialization. The baseline emitted 525 `cmd/oct` test/subtest pass events totaling 98.29 aggregate test-seconds. Expensive families repeatedly crossed compiler and process boundaries:

| Baseline item/family | Time | Expensive operation | Purpose | Treatment |
|---|---:|---|---|---|
| `TestEnumAwareSwitch` | 3.83s | compiled artifact generation/execution | enum switch parity | integration lane; retain |
| `TestCompiledMarkdownHelpersNoSidecar` | 3.37s | build shared CLI, compiled test runner | executable/compiled smoke | integration lane; retain boundary smoke |
| `TestStringErgonomics` | 3.33s | repeated run/build variants | string parity | integration lane; retain |
| Native condition-switch contracts | 2.38s | language corpus execution | native-host contract | integration lane; retain |
| Experiment execution root | 2.27s | milestone discovery and test execution | experiment orchestration | integration lane; retain |
| `ExpRun` family | 10.98s per source-file aggregate | Git repositories, cache, compiler runs | package/experiment integration | toolchain lane; retain |
| `internal/build` package | 10.99s package event | repeated Go compilation and artifact execution | compiler boundary | integration lane; retain |
| `internal/pkgmgr` package | 5.35s package event | Git repositories and filesystem copies | package manager integration | toolchain lane for external cases |

Static baseline inspection of `cmd/oct` found 25 subprocess construction sites, 13 literal `go run ./cmd/oct` sites, four sleeps, repeated temp project/corpus construction, and one package-wide shared binary/sidecar harness. The subprocesses were concentrated in compiled artifact and external-tool boundaries; the direct CLI seam already existed as `internal/cli.Execute`.

## Changes

- Kept the existing in-process CLI handler. `main` was already thin, so no production refactor was justified. Ordinary CLI tests use `cli.Execute`; executable coverage stays in tagged smoke/integration tests.
- Added 45 `integration`, 36 `toolchain`, and three `native` test files across the repository. Tags classify dependency boundaries; tests were moved, not deleted.
- Replaced every literal test-side `go run ./cmd/oct` call (13 to zero) with the existing `sync.Once` shared Oct test binary. This preserves the executable boundary while avoiding repeated Go driver work.
- Moved compiler artifact/executable tests in `internal/build` and generated-runner integration to `integration`.
- Moved Node, Git, Go wrapper-build, and sidecar tests to `toolchain`. Real tool discovery and explicit skips remain visible only in that lane.
- Moved Windows Prometheus reactor tests to `native`; pure Go bridge/runtime tests remain default.
- Replaced three 20ms cache sleeps with fixed directory modification times. Replaced the 1.1s makefile sleep with an explicit future timestamp. Default and tagged unit code contains no arbitrary sleep.
- Centralized cross-lane test helpers in `helpers_test.go`. Shared setup remains immutable (`sync.Once` binary/sidecar discovery); temp outputs remain per test.
- Kept working-directory tests serial and documented why. No new parallelism was added around environment mutation, `os.Chdir`, shared sidecars, or generated outputs.
- Parallelized the pure help, version, and formatter command-handler tests; their buffers and temporary files are isolated per test.
- Added transparent PowerShell lane runners and a JSON-event latency summarizer.

The existing tests already use coherent tables for CLI usage, formatter modes, compiler diagnostic families, and SDSL-V emission variants. T1 did not combine unrelated semantic families into a mega-table. No meaningful test or golden assertion was removed, and no integration failure was converted into a default-lane skip.

## Coverage preservation

Default `go test ./...` retains pure implementation packages and in-process CLI help/version/format/SDSL-V tests. The representative core run/build boundary in `cmd/oct/main_test.go`, exhaustive command, corpus, artifact, experiment, benchmark, generated-runner, and compiler executable coverage remain under `integration`. Git/Go/Node/DXC/sidecar coverage remains under `toolchain`. Reactor/Vulkan coverage remains under `native`.

CI now runs default, integration, and external-tool lanes as separately named steps before the existing real-sidecar sub-lane. Generated-file and golden checks remain in their owning lane and are not rewritten unless their existing explicit update path is used.

## Remaining bottlenecks and T2

The slowest default family after T1 is `internal/tester.TestLanePartitionMixedFile`; `internal/tester` is the slowest default package at about 2.1s. `cmd/oct` has no remaining default test above 0.1s. The compiled boundary tests in `cmd/oct/main_test.go` remain deliberate integration coverage, not a greater-than-ten-second default blocker.

Recommended T2 work:

1. Split `internal/build/compiler_test.go` into focused host-lowering unit tests and a smaller executable integration table, migrating any remaining duplicated language semantics to `Language/` contracts.
2. Add an explicit working-directory option below `cli.Execute` so package-command tests can stop mutating `os.Chdir` and can then be parallelized safely.
3. Batch `internal/tester` compiled lane-partition cases behind one compiler fixture where failure attribution remains exact.
4. Split mixed pure/external functions in `internal/pkgmgr` test files so pure manifest/plan coverage can return to the default lane without Git.

## Evidence

- `out/test-artifacts/go_test_latency_baseline.{json,md}`
- `out/test-artifacts/go_test_latency_after.{json,md}`
- Raw before/after Go JSON streams in `out/test-artifacts/`
- `docs/GO_TEST_LANES.md`
