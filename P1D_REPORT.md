# P1d — Prometheus Minimal Native Slice Definition

## 1) Native-layer responsibilities (C/C++ Vulkan only)

The native slice should do **only** the irreducible Vulkan execution work for one SGEMM kernel invocation:

1. Create/destroy a Prometheus runtime handle that encapsulates Vulkan instance/device/queue/command-pool state.
2. Create/destroy GPU buffers required for one SGEMM call (`A`, `B`, `C`) and stage host-to-device / device-to-host copies.
3. Load/create one compute pipeline for SGEMM (shader module + pipeline layout + pipeline).
4. Record/submit/wait one command sequence: upload -> dispatch -> download.
5. Return explicit status codes and stage codes (init/upload/dispatch/download/teardown), with no policy decisions.

Everything else is out of scope for v0 native code.

## 2) Go-layer responsibilities

Go owns all orchestration, policy, and safety:

1. Public API and runtime lifecycle boundaries exposed to Oct-facing execution paths.
2. Shape/type validation for SGEMM contract (dimension compatibility, dtype gate, contiguous layout assumptions).
3. Dispatch decision logic (CPU vs Prometheus), including fallback policy and reason codes.
4. Correctness oracle integration and result verification thresholds.
5. Benchmark timing protocol, reporting, and aggregation semantics.
6. Error mapping from native status codes into structured Go errors.
7. Resource budgeting policy (timeouts, max matrix sizes, retries if any).

## 3) Oct-layer responsibilities

Oct should remain the user-space driver for workload intent:

1. Orchestration scripts selecting workloads, matrix-size sweeps, and benchmark scenarios.
2. Dispatch target selection input (for example, requesting Prometheus backend explicitly).
3. Benchmark suites and contract-level expectations for run labeling (`ok`, `fallback(...)`, `error(...)`).
4. Scenario composition across packages/programs; no native/runtime implementation logic.

## 4) Forbidden responsibilities in native code (v0)

The native slice must **not** absorb these in first implementation:

1. Backend selection/fallback policy.
2. Shape inference, user-facing validation, or semantic contract checks.
3. Benchmark orchestration, statistics, or report formatting.
4. Kernel auto-tuning, heuristic tiling search, or dynamic strategy selection.
5. Multi-device scheduling, stream graphs, or asynchronous execution frameworks.
6. Cross-operation fusion or generalized tensor runtime abstractions.
7. Oct package loading, test semantics, or language-level behavior decisions.

## 5) Minimal file/module shape recommendation

Keep the initial layout narrow and boundary-focused:

- `internal/prometheus/native/bridge.h`
  - C ABI surface (opaque handle + plain structs/enums + status codes).
- `internal/prometheus/native/bridge.cc`
  - Vulkan runtime/pipeline/buffer/dispatch implementation.
- `internal/prometheus/bridge.go`
  - cgo binding layer; converts Go structs/errors <-> C ABI.
- `internal/prometheus/runtime.go`
  - Go-owned lifecycle + dispatch entrypoint used by higher layers.
- `internal/prometheus/status.go`
  - canonical Go status/stage enums and reason mapping.

This keeps native code to one translation unit plus one header, with policy living in Go.

## 6) Recommended next step

Implement a **single end-to-end vertical slice** for fixed `float32` SGEMM with:

1. one native C ABI entrypoint for `sgemm(A,B,C,m,n,k)` plus init/shutdown,
2. Go-side validation + explicit backend selection + structured error mapping,
3. one Oct-driven benchmark scenario that emits `ok|fallback|error` labels.

Do not add tuning, async execution, or generalized tensor abstractions until this slice is correctness-verified against the existing CPU oracle path.
