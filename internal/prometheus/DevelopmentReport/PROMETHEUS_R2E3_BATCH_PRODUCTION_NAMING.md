# Prometheus R2e3 — batch production naming

## Result

`reactor_sgemm_batch_m31.c/.h` are now `reactor_batch.c/.h`; private execution
symbol `prom_sgemm_batch_m31_execute` is now `prom_sgemm_batch_execute`.
The formal name is precise: this module owns production batch semantics, not
async token lifecycle, scheduling, queues, physical-ring ownership, or future
executors. Historical R2e1/M31 reports deliberately retain their milestone
names.

## Atlas and ownership

`reactor_batch.c` now begins with the durable execution atlas and numbered
navigation sections for planning, logical partitioning, runtime/failure state,
admission/refill, ownership drain, staging/publication, commit, diagnostics,
and test seams. It owns immutable plans, runtime state, deterministic failure
reduction, stop/skipped-tail behavior, refill coordination, staging, ordered
publication, and batch diagnostics preparation. It delegates M30 task lifecycle
and M30a reap/quarantine to their owners, and M29 slot/submission ownership to
the shared ring.

The dependency path remains public batch API -> immutable plan -> centralized
refill -> M30 -> M29 -> physical completion -> staged outputs -> atomic ordered
commit. No P11 route is restored.

## Documentation

Updated native architecture documents name `reactor_batch.c` as production
authority and link the P11 retrospective, R2e2 removal report, and new
`PROMETHEUS_CONCURRENCY_FUTURE_DIRECTIONS.md`. The new directions note
explicitly authorizes no implementation while recording conditional host
recording, real queue-owned executors, safe work stealing, deterministic versus
adaptive policy, proof gates, anti-patterns, and future TODOs. This closure
report is accompanied by `PROMETHEUS_R2_CLOSURE.md`.

## Manifest, ABI, and behavior parity

The canonical native manifest and generated Windows/Linux source lists use
`reactor_batch.c`. Public exports, function signatures, numeric enums/flags/
details, diagnostics layouts, public routing, and test-only routing are
unchanged. The private module name is the only build-graph change; M31 remains
the accepted implementation authority under its formal production identity.

## Validation and acceptance

All completed checks pass: manifest write/check, JSON validation, generated
list review, `git diff --check`, Windows launcher native build, and Linux shell
syntax checks. Both `go test ./internal/prometheus/... ./cmd/oct` and
`go test ./internal/... ./cmd/oct` pass. Focused Marionette coverage passes for
R2b planning, R2d authority, R2c failure/atomicity/drain/commit semantics,
M29, M30, M30a, resident, and EVT. The executed physical-ring lane records
correct hardware submissions and output correctness; no unexecuted hardware
claim is made.

The final source search finds no current nonhistorical occurrence of
`reactor_sgemm_batch_m31.c`, `reactor_sgemm_batch_m31.h`, or
`prom_sgemm_batch_m31_execute`. The old terms remain only in historical R2e1
material, as intended.

**Acceptance conclusion: R2e3 ACCEPTED. R2 is closed after R2e3 acceptance.**
