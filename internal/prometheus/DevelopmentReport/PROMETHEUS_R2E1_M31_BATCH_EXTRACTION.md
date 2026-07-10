# Prometheus R2e1 — M31 batch extraction

## Executive summary

R2e1 moves the accepted M31 immutable planning, per-entry runtime state,
failure reducer, refill/drain coordination, staging, ordered commit, and M31
diagnostics preparation from `reactor_vulkan_sgemm.c` into the single coherent
module `reactor_sgemm_batch_m31.c`.  The new narrow header exposes only
`prom_sgemm_batch_m31_execute`.  The retained P11 legacy entry and executor
remain in the monolith unchanged in behavior.

This is an extraction only. No public ABI, production flags, diagnostics
schema, sync path, registry, or P11 implementation behavior was redesigned.

## Boundary and API

`reactor_sgemm_batch_m31.c` owns:

- immutable `prom_sgemm_batch_plan` / per-entry planning and deterministic
  requested-width, cap, round-robin, and contiguous formulas;
- `prom_sgemm_batch_entry_runtime` and its complete state vocabulary;
- the phase-ranked `(entry ID, phase, observation sequence)` reducer;
- admission stop, skipped-tail marking, real shared-ring refill, staged
  outputs, drain/reap/quarantine coordination, and entry-order commit;
- M31-only test failure decoding and M31 v1 diagnostic preparation.

`reactor_sgemm_batch_m31.h` contains only the M31 execute declaration. The
transitional private aggregate remains in
`reactor_vulkan_sgemm_internal.h`; this permits direct, explicit access to the
existing M29/M30/M30a state without inventing accessors, managers, or a job
framework. M29 ring submission and M30/M30a task, poll, feedback, and reap
helpers remain in their current owner because they are also used by async.

The exact symbol inventory is
`out/test-artifacts/prometheus_r2e1_symbol_boundary.json`.

## Routing

Before and after extraction, the production route is:

```text
prometheus_reactor_runtime_sgemm_batch
  -> prom_reactor_runtime_sgemm_batch_impl
  -> supported production-mask validation
  -> prom_sgemm_batch_m31_execute (new module)
  -> M30 task lifecycle -> M29 shared physical ring -> Vulkan SGEMM
```

The retained compatibility route is separately named:

```text
prometheus_reactor_runtime_sgemm_batch_legacy_test
  -> prom_reactor_runtime_sgemm_batch_legacy_test_impl
  -> retained P11 executor
```

There is no production fallback to P11. The authority proof is recorded in
`out/test-artifacts/prometheus_r2e1_authority_callgraph.json`.

## Dependency and diagnostics treatment

The new M31 source has no P11 executor, CPU reference result authority,
worker-thread, worker-local Vulkan-resource, fake queue-topology, empty-submit,
or P11 event-ring dependency. It uses only the existing shared M29/M30/M30a
helpers and the private runtime aggregate. Existing diagnostic fields are
populated from plan facts, runtime records, ring evidence, lifecycle evidence,
and commit results; final public diagnostic export stays cross-cutting in the
monolith.

## Residual deletion island

The remaining P11-only planner, workers, CPU reference path, thread bridge,
empty-submit resource path, fake topology, event rings, slot state, injections,
diagnostic branches, legacy entry, and slow suite remain identifiable in
`reactor_vulkan_sgemm.c` and `reactor_p11_m6_batch_tests.cpp`. The concrete
R2e2 deletion inventory is
`out/test-artifacts/prometheus_r2e1_residual_p11_cluster.json`. No P11 code
was deleted in R2e1.

## Build, ABI, and validation

The monolith decreases from 11,430 to 10,089 lines. The authoritative M31
implementation is 463 lines in its new source; the 95-line M31 header owns the
plan/runtime declarations, while the 918-line private aggregate header is a
transitional declaration-only boundary for pre-existing runtime state.

The canonical native manifest now includes `reactor_sgemm_batch_m31.c`; the
generated Windows and Linux lists were regenerated with
`go run ./tools/prometheus_native_manifest -write` and checked.

- Windows launcher native build: pass (138.279 seconds).
- Linux shell syntax: `build_linux.sh` and `native_sources_linux.sh` pass
  `bash -n`.
- Go: `go test ./internal/prometheus/... ./cmd/oct` and
  `go test ./internal/... ./cmd/oct` pass.
- RTX 3070: NVIDIA GeForce RTX 3070, vendor `0x10de`, device `0x2488`,
  Vulkan 1.4.329, driver 596.36; public M31 authority and R2b plan tests pass.
- M29, M30, M30a, and M31 transfer lanes pass. M30a is run in its required
  isolated process; its artifact proves quarantine/reap, physical completion,
  slot reuse, ordered skipped feedback, and unsafe device-loss classification.
- Public widths 1, 2, and 4, width capped at entry count, contiguous planning,
  production-to-M31 routing, real submissions, and no legacy reachability pass.
- The R2c failure-sensitive seven-test subset passes 7/7 in three consecutive
  fresh processes. The fatal unsafe test passes in its own fresh process.
- Retained P11 slow suite: 62 pass, 1 expected skip, 0 fail.

`prometheus_sgemm_batch_refill_ring.json` proves real submissions and ring
evidence: depth 1/2/4 observed maximum in-flight 1/2/4, eight submissions per
case, query harvests, ordered commit, and atomic failure drain. The ABI result
remains in `out/test-artifacts/prometheus_r2e1_abi_parity.json`: no public
header or API wrapper changed.

## Acceptance conclusion

R2e1 ACCEPTED — safe to delete residual P11 cluster

## R2e2 readiness

The accepted M31 engine is now a distinct source module and P11 is a visible
legacy island. R2e2 can delete that island only after its explicit test-only
compatibility decision; this extraction does not begin deletion.
