# P4a Report — Prometheus Bridge Dynamic Loading Scaffold (EVT Transition)

## What changed from proof-of-concept coupling

The old Prometheus bridge assumed native code was directly compiled into the Go package via cgo. That coupling has been replaced with a runtime bridge scaffold that treats the Reactor as an optional dynamic dependency discovered at runtime.

## What was reused

- Existing SGEMM request/result/correctness reporting and `.octagon` emission flow were kept.
- Existing backend-selection semantics (`cpu` vs `prometheus`) and explicit fallback visibility were retained.

## Dynamic loading model now

The Bridge now defines a minimal Reactor ABI contract and loading flow:

1. discover candidate library paths in deterministic order:
   - `OCT_PROMETHEUS_REACTOR` explicit path
   - repo-local dev path
   - executable-adjacent path
2. open library via bridge loader boundary
3. resolve required symbols:
   - `prometheus_reactor_abi_version`
   - `prometheus_reactor_runtime_create`
   - `prometheus_reactor_runtime_destroy`
   - `prometheus_reactor_runtime_probe`
4. verify ABI version
5. create runtime, then probe runtime readiness

## Optionality and explicit unavailable/error surfacing

Prometheus remains optional:

- If no Reactor is found, run status is explicit `fallback(prometheus_unavailable)` and CPU is used.
- If a Reactor is found but incompatible/broken, structured `ReactorIssue` errors are raised with specific issue codes:
  - `symbol_missing`
  - `abi_mismatch`
  - `runtime_create_failed`
  - `runtime_probe_unavailable`
  - `reactor_load_failed`

## Deferred to later milestones

This milestone intentionally does **not** include:

- production dynamic loader backend implementation for all target platforms
- full native Reactor implementation
- Vulkan SGEMM execution/performance claims
- broader Prometheus policy/CLI redesign

P4a establishes architecture boundaries and failure semantics first.
