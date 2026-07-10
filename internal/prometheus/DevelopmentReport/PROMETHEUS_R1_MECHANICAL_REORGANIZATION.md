# Prometheus R1 mechanical reorganization

Status: R1a ACCEPTED — safe to begin R1b. No execution-authority extraction has begun.

## Checkpoint R1a

- Added `native_manifest.json` as the canonical production and Marionette source/test list.
- Added a deterministic standard-library-only renderer/validator at `tools/prometheus_native_manifest`.
- Windows consumes `native_sources_windows.cmd`; Linux consumes `native_sources_linux.sh`. Both drivers run the same drift check first.
- The canonical normal test list includes the previously omitted FFT and P14 test sources on Windows.
- Renamed the authoritative Linux driver to `build_linux.sh`; `build_stub.sh` is a warning-only compatibility forwarder.
- No public header, ABI layout, enum, selector, routing, kernel, diagnostics, or runtime C implementation changed in this checkpoint.

### R1a validation record

| Check | Result |
|---|---|
| Windows build through VS DevCmd | PASS; exit code 0; approximately 135 seconds wall time |
| Manifest validation | PASS before the Windows build and again after it |
| Outputs | `prometheus_reactor.dll`, `marionette_tests.exe`, `marionette_slow_tests.exe`, and `marionette_benchmarks.exe` |
| Native smoke | PASS: `PrometheusNativeHarness_Smoke` (1/1) against the newly built `marionette_tests.exe` |
| Go bridge lane | PASS: `go test ./internal/prometheus/... ./cmd/oct` |

The build emitted no stderr warnings or errors. The full stdout/stderr capture is retained at
`out/test-artifacts/prometheus_r1a_windows_build.stdout.log` and
`out/test-artifacts/prometheus_r1a_windows_build.stderr.log`.

## Baseline artifacts

- `out/test-artifacts/prometheus_r1_authority_baseline.json`
- `out/test-artifacts/prometheus_r1_symbol_map.json`
- `out/test-artifacts/prometheus_r1_extraction_ledger.json`

## Pending checkpoints

R1b was authorized after R1a validation. R1c remained intentionally unstarted.

## R1b extraction audit — stopped before movement

R1b did not move code. The requested independent C translation units cannot be
introduced mechanically from the current source without first completing a
much broader private-runtime decomposition.

The retained P11 range directly calls more than thirty file-local helpers for
planning, selector facts, worker resource ownership, thread bridging, event
drain, CPU reference production, failure injection, and output cleanup. It
also reads and writes the monolithic `prometheus_runtime` aggregate directly.
That aggregate contains P11, ring, M30/M30a, synchronous Vulkan, pipeline,
arena, selector, control, and diagnostics state in one declaration.

The physical-ring helpers are similarly not independent: M29 uses them
directly; M30/M30a couples them to task-buffer ownership, quarantine/reap,
and ordered completion feedback; M31 couples them to batch admission and
atomic output staging. Moving only `prom_sgemm_ring_*` would require exporting
static async recording/harvesting helpers or creating a new job interface.
Either would exceed the requested minimum private-state seam and risks changing
the authority/lifetime contract.

No accessor layer, callback abstraction, routing change, or legacy deletion
was introduced. R1b is therefore **BLOCKED before extraction**.

## Final R1 judgment — PARTIALLY ACCEPTED

### Completed work

R1a is accepted. The repository now has a deterministic canonical native
production/test manifest, generated Windows and Linux build fragments with
drift validation, equivalent platform source/test sets, the authoritative
`build_linux.sh` driver, and a warning-only `build_stub.sh` compatibility
wrapper. Baseline authority and symbol artifacts, behavior-parity evidence,
and the extraction ledger are present. The Windows native build, native public
ABI smoke, manifest validation, shell syntax checks, and requested Go bridge
tests passed.

### R1b blocker evidence

The retained P11 executor is not a self-contained unit. Its code directly
uses file-local planning, selector, resource, thread, event, CPU-reference,
failure, cleanup, and diagnostics helpers while mutating the same aggregate as
the synchronous path, M29 ring, M30/M30a lifecycle, and M31 refill path. The
persistent-ring operations have the same problem in the opposite direction:
their slot lifecycle is coupled to M30/M30a task-buffer ownership and ordered
feedback, plus M31 admission and atomic output staging.

### Why extraction was not forced

Exporting the static helper surface or wrapping every aggregate field in
accessors would create a distributed monolith: the same mutable authority
would be spread across files while callers still depend on its full shape and
lifetime. A generic job layer, callbacks, or an opaque manager would add
indirection and risk changing the proven M29/M30/M31/P11 ownership contracts.
That is worse than retaining the visible monolith for this milestone and is
explicitly outside R1's mechanical scope.

### R1c decision

R1c was not attempted. Scheduler, SGEMM, Vulkan, and diagnostics extraction
depends on the same unresolved ownership boundary; proceeding would require
semantic architectural consolidation rather than behavior-preserving movement.

### R2 handoff

R2 should first establish owned subsystem state and explicit internal
operation records for the physical ring, async lifecycle, batch staging, and
legacy P11 contract. It can then consolidate authority deliberately, retain
P11 compatibility as an explicitly non-hardware result path, and introduce a
kernel registry/diagnostics changes only under their separately approved
semantic contracts. The R1 manifest and baseline artifacts provide the parity
guardrails for that work.
