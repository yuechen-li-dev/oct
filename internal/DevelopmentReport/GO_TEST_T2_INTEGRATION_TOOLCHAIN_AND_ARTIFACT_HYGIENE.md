# Go test T2 integration/toolchain latency and artifact hygiene

## Outcome

T2 fixed the compiled-octest ownership defect and materially improved the
integration lane without removing compiler, executable, Git, Node, DXC, or
sidecar coverage. Normal compiled test, artifact, and benchmark runs leave no
runner sources, binaries, MIR files, or owned directories behind. Persistent
`oct build` output is unchanged.

| Measurement | Baseline | After | Change |
|---|---:|---:|---:|
| Integration wall | 60.401s | 47.637s | -12.764s (-21.1%) |
| Integration `cmd/oct` package | 57.745s | 44.550s | -13.195s (-22.9%) |
| Toolchain wall | 26.477s | 27.108s | +0.631s (+2.4%, run variance) |
| Toolchain `cmd/oct` package | 24.094s | 24.126s | +0.032s (flat) |
| Legacy runner artifacts after a normal lane | 3 new | 0 | fixed |
| Repository legacy runner pile | 27 files / 68,166,144 bytes | 0 | reclaimed |

The toolchain lane remains below the previously reported 29.9s target, but its
before/after change is not a material speedup. Its remaining cost is dominated
by real Git repositories, clones, commits, package sync, experiment execution,
and wrapper boundaries. Those operations were retained intentionally.

Raw event streams and summaries:

- `out/test-artifacts/go_test_integration_t2_baseline.json`
- `out/test-artifacts/go_test_toolchain_t2_baseline.json`
- `out/test-artifacts/go_test_t2_baseline.{json,md}`
- `out/test-artifacts/go_test_integration_t2_after.json`
- `out/test-artifacts/go_test_toolchain_t2_after.json`
- `out/test-artifacts/go_test_t2_after.{json,md}`
- `out/test-artifacts/go_test_t2_baseline_artifacts.json`

## Leak trace and root cause

The old path began in `internal/tester.writeCompiledTestRunner`. It generated
`zz_oct_test_runner_<pid>_<nanoseconds>.octest` directly in
`project.Package.Directory`. `build.CompileForTestWithSelectedFiles` derived the
binary path from that runner, producing the visible sibling
`zz_oct_test_runner_<pid>_<nanoseconds>.octest.octbin` in the package directory
or process working directory.

Ownership was split across two independent deferred callbacks:

1. `writeCompiledTestRunner` returned a callback that removed only the runner
   source.
2. `cleanupArtifact` attempted individual `os.Remove` calls for the binary and
   optional MIR file.

Both callbacks discarded cleanup errors. Therefore the success, runtime
failure, timeout, and partial-compile paths could return without reporting that
Windows had retained a generated executable. The historical Windows error is
not recoverable because it was deliberately ignored; the repository evidence
was deterministic—each baseline integration run added three approximately
2.5-MB binaries. `exec.CommandContext(...).CombinedOutput()` did wait for the
child, but no owned parent directory existed and no cleanup error reached the
caller. A nested Oct CLI subprocess did not change the creator: the tester in
that process still created the runner, but the parent Go test had no ownership
handle for its scattered child artifacts. Process crashes could bypass all
defers and leave indistinguishable files in the working tree.

## Lifecycle fix

`internal/tester/artifact_scope.go` is now the single ownership primitive.
Every compiled fact/theory, artifact, and benchmark case creates one uniquely
named directory under `OCT_TEST_TEMP_ROOT` or the OS temporary parent:

```text
octest-run-<unique>/
  runner.octest
  runner.octest.octbin
  runner.octest.octbin.mir (only when requested)
```

The project loader and compiler gained a narrow external-entry API:
`LoadForTestWithSelectedFilesInPackage` and
`CompileForTestWithSelectedFilesInPackage`. It resolves normal package sources
from the real package directory while loading the generated entry point from
the owned scope. The compiler output follows the temporary runner. Ordinary
`Compile` and the user-facing `oct build` path were not changed.

The creator defers one `RemoveAll` for the owned directory and joins cleanup
failures into the returned error. This runs after compilation and after
`CombinedOutput` has waited for successful, failing, cancelled, or timed-out
child processes, so Windows executable handles are closed before removal.
Partial compiler output is covered by the same directory cleanup. Panics still
run the defer when normal Go unwinding is possible.

`OCT_KEEP_TEST_ARTIFACTS=1` disables removal explicitly and prints
`Retained compiled test artifacts: <directory>`. Retention never writes into a
source tree. A hard process crash can still prevent in-process cleanup, but the
artifact is contained under the explicit temp root and can be removed on the
next run or by the cleanup utility.

## Regression coverage

Focused tests cover:

- success cleanup;
- compile failure and partial-file cleanup;
- runtime assertion failure;
- context timeout/cancellation;
- Windows process-handle closure before directory removal;
- explicit retention and path reporting;
- parallel scope uniqueness and parallel compiled execution;
- absence of generated files in source/fixture directories;
- persistence and location of user-requested `oct build` output;
- benchmark compile-failure cleanup.

The stale cleanup script has a standalone mixed-fixture test proving that its
dry run and delete modes match only
`zz_oct_test_runner_*.octest.octbin`, preserving `user-build.octbin` and
unrelated files.

## Stale artifact cleanup

`tools/Clean-OctTestArtifacts.ps1` is dry-run by default, accepts a `-Root` and
`-OlderThan` threshold, prints every candidate, count, and reclaimed bytes, and
requires `-Delete` before mutation. `-IncludeTempDirectories` optionally adds
only the known owned directory names `octest-run-*`, `oct-artifact-run-*`, and
`oct-benchmark-run-*`. It never uses a broad `*.octbin` match.

The T2 cleanup removed 27 known legacy files totaling 68,166,144 bytes. No
arbitrary `.octbin`, build output, or user-authored file was removed.

## Latency refactors and deliberate serialization

- Isolated compiled CLI and corpus families now overlap behind a four-slot
  package semaphore. Each case retains its own source, output, and artifact
  directory. Tests using `os.Chdir`, `t.Setenv`, fixed output paths, mutable
  runtime state, or generated committed files remain serial.
- The Oct executable remains built once per `cmd/oct` test process through the
  existing `sync.Once` harness. T2 did not introduce a second build path.
- Experiment tests now share one immutable Base dependency Git repository;
  each test still creates and clones its own experiment repository and cache.
- Git discovery in `cmd/oct` and `internal/pkgmgr`, and Node discovery in
  `internal/interpret`, now run once per package process. DXC discovery already
  had only one real smoke call in its package.
- The real sidecar lane continues to share binary discovery/build outputs but
  not mutable wrapper/runtime instances. All 13 sidecars are built once before
  the lane.

The source audit moved external-tool discovery sites from 22 to 13 and shared
setup sites from 3 to 7. Static subprocess construction sites remain about 70
because real process-boundary coverage was preserved. Go's `-json` stream does
not expose child-process creation, so these are source-site counts rather than
invented dynamic invocation totals. There are still zero test-side
`go run ./cmd/oct` sites and zero arbitrary sleeps.

## Validation

All requested available lanes passed on Windows:

- `go test -count=1 ./...`
- `go test -count=1 -tags=integration ./...`
- `go test -count=1 -tags=toolchain ./...`
- `go test -count=1 -tags='integration toolchain' ./...`
- `go run ./tools/build_sidecars --out dist/sidecars` (13 sidecars)
- documented slow wrapper command with `OCT_SLOW_TESTS=1` and
  `OCT_WRAPPER_PATH=dist/sidecars` (`cmd/oct` package 72.332s)
- focused lifecycle and repeated compiled-octest runs
- cleanup utility mixed-fixture test and repository dry-run

The combined lane reported `cmd/oct` at 66.830s. Every full, combined, focused,
and real-sidecar check ended with zero owned-temp entries and zero
`zz_oct_test_runner_*.octest.octbin` files. The native Vulkan/reactor lane was
audited but not executed because this task did not provide
`OCT_RUN_PROMETHEUS_INTEGRATION=1` and a reactor DLL; its compiled benchmarks use
the same lifecycle-scoped benchmark path exercised elsewhere.

CI now supplies one explicit `${{ runner.temp }}/oct-test-artifacts` parent,
uploads retained/crash artifacts only after failure, and removes the parent in
an `always()` step. The local PowerShell lane runner similarly supplies a unique
repo-local root and cleans it in `finally`, unless debug retention is explicit.

## Remaining bottlenecks and T3 recommendations

1. A compiled octest command still performs one Go compilation per fact/theory
   row. The next meaningful reduction is a package/file-level compiled harness
   that preserves exact per-case failure attribution, timeout isolation, and
   selected-file reachability. Do not batch until those properties are explicit.
2. `internal/build/compiler_test.go` still pays for many real generated-program
   builds. Split pure lowering checks from a smaller table of executable
   boundaries, migrating any duplicated language semantics to `Language/`.
3. Package and experiment tests remain serial where process-global working
   directory or cache environment variables are mutated. Add explicit working
   directory and cache-root options below `cli.Execute` before parallelizing
   them.
4. If exact dynamic subprocess/tool counts become a release gate, add an
   opt-in process-local metrics sink at the command-runner seams. Go test JSON
   alone cannot count grandchildren reliably on Windows.

T2 therefore converges in the **success** state: the visible ownership bug is
fixed across normal and failure paths, stale files are safely removable,
integration latency improved materially, and the remaining toolchain cost is
isolated to deliberate real boundaries.
