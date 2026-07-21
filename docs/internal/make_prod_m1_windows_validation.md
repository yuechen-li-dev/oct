# MAKE-PROD-M1 Windows validation

Status: complete (2026-07-20, Windows).

MAKE-PROD-M0 is the direct Windows native-build vertical slice. MAKE-PROD-M1
adds Windows transitive-header correctness: a successful native compile now
commits its command identity, declared inputs, typed discovery identity, and
canonical discovered inputs together in the action state. GCC/Clang discovery
is explicitly deferred backend work.

## Real MSVC collection

The isolated fixture at
`internal/makecmd/testdata/msvc_source_dependencies_fixture` was compiled with
Microsoft C/C++ Optimizing Compiler 19.51.36248 for x64. Its source path and
headers deliberately contain a space. The actual `/sourceDependencies` output
reported schema `Version: "1.2"`, `Data.Source`, and `Data.Includes`; it
identified the expected source and both its direct and transitive headers. The
sanitized version-1.2 shape is retained as
`internal/makecmd/testdata/msvc_source_dependencies_1.2.json`.

The collector owns that JSON shape. State stores only normalized canonical
paths, collector provenance, discovery kind `msvc.sourceDependencies`, semantic
schema `v1`, and expected source/output identities. Attempt-local JSON paths
are deliberately absent from the command hash and committed state.

## Transaction evidence

The real fixture established all of the following:

- First build compiled and atomically committed discovery state; the next
  unchanged build was `UpToDate`.
- Direct-header and transitive-header changes each rebuilt the consumer with
  `DiscoveredDependencyNewerThanOutput`; an unrelated-header change was a no-op.
- Removing the direct header produced `DiscoveredDependencyMissing` in
  `oct make explain`.
- A deliberate compiler error left prior successful state byte-for-byte
  unchanged. After source repair, `PreviousFailure` retried only the failed
  compile and its required phony downstream action.
- Discovery and state-persistence failure paths write phase-specific artifacts
  and a pending action marker. A marker prevents old state plus a newly written
  object from being cacheable; no pending collector file is committed state.
- Real Windows replacement briefly encountered `Access is denied` while
  replacing a state file during the Prometheus run. The atomic rename path now
  retries transient sharing violations for two seconds while retaining
  rename-based atomic replacement. The action recovered via `PreviousFailure`.

Unit coverage also verifies old state without discovery, unsupported identity,
source/output/action mismatch, equal basenames and variants, persistence
failure, all three failure artifacts, stable hashes across attempt paths, and
prior valid state surviving process, discovery, and persistence failure.

## Prometheus revalidation

`BuildNative` enabled real MSVC discovery for the Windows direct backend. The
first M1 invocation conservatively rebuilt native compile actions whose old
state lacked discovery (`DiscoveryStateAbsent`); the next unchanged invocation
was a no-op apart from phony targets. SerialCanonical, normal Marionette, and
the benchmark build artifact built normally. `TestNative` ran only
`PrometheusNativeHarness_Smoke` and reported 1/1 pass. The isolated M5b build
artifact also built without execution.

A controlled comment-only change to
`internal/prometheus/native/reactor_judgment_engine.h` rebuilt actual
MSVC-discovered consumers including the judgment engine, Dominatus adapter,
Vulkan common/SGEMM, batch, fused-reduction, model-block, and transformer
actions, then linked the required Reactor and Marionette outputs. Restoring the
exact header contents rebuilt those required consumers once more; production
source and generated tracked state/trace contents were restored before
completion. No GPU workload, M5b dispatch, benchmark execution, legacy script,
CMake, or Ninja invocation was used.

## Final checks

```text
go test ./internal/makecmd
go test ./cmd/oct
go test -tags toolchain ./cmd/oct
go run ./cmd/oct test Libraries/Make --execution interpreted
go run ./tools/prometheus_native_manifest -check
git diff --check
```

The Make corpus passed 5/5 and manifest parity passed. The Go test lanes and
diff check passed after the M1 changes.
