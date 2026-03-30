# P1c — Prometheus Dispatch / Fallback / Data-Movement Pressure Test

## 1) First dispatch model

- **Default route:** execute SGEMM on the existing CPU path.
- **Prometheus route is explicit:** use Prometheus only when the caller sets a runtime execution target (for example, `backend=prometheus`) and Prometheus runtime initialization succeeds.
- **No implicit auto-selection in v0:** there is no heuristic “best device” dispatch in the first contract.

## 2) First fallback model

- **Fallback trigger scope:** fallback is allowed only for **environmental unavailability** (missing device, init failure, unsupported runtime capability).
- **Fallback target:** CPU SGEMM with identical shape/type contract.
- **No silent correctness fallback on compute failure:** once a Prometheus dispatch has successfully started execution, any submission/runtime execution error is surfaced as failure, not silently retried on CPU.
- **Fallback visibility:** every fallback emits a structured reason code so benchmark/reporting can separate native Prometheus runs from fallback runs.

## 3) First data-movement model

- **Initial assumption:** API inputs/outputs are host-resident row-major `float32` buffers.
- **Prometheus execution data flow (v0):**
  1. host `A`/`B` copied to device buffers,
  2. SGEMM command submitted,
  3. result `C` copied back to host,
  4. API returns host-resident `C`.
- **Cost accounting rule:** report transfer-inclusive timing as the default claim path; kernel-only timing is supplemental and must be explicitly labeled.
- **No residency optimization in v0:** no persistent cross-call device-resident tensor contract yet.

## 4) First ownership/lifetime model

- **Caller owns host memory:** caller allocates/provides `A`, `B`, and destination `C` (or receives returned `C` per API shape), and remains owner of host buffers.
- **Prometheus runtime owns device resources:** device/context/queue/command resources are owned by a Prometheus runtime handle created/destroyed by Go runtime code.
- **Per-call transient ownership:** per-dispatch device buffers and submission artifacts are created and released within the call scope unless explicitly upgraded later.
- **Cleanup guarantee:** on any call exit path (success, fallback, error), transient native resources are released before returning to Go.

## 5) First failure propagation model

- **Native -> Go:** native/runtime failures are mapped to typed Go errors that include stage (`init`, `transfer_in`, `submit`, `transfer_out`, `cleanup`) and backend (`cpu` or `prometheus`).
- **Go -> Oct:** runtime errors surface as standard Oct runtime `err(...)` results/messages, preserving backend and stage summary.
- **Go -> benchmark output:** benchmark rows include terminal status: `ok`, `fallback(<reason>)`, or `error(<stage>,<code>)`; failed/error rows are never merged into Prometheus success aggregates.
- **Policy:** failure is data, not noise—every non-`ok` outcome must remain visible in reports.

## 6) Exact non-goals for v0/v1 Prometheus SGEMM

- Auto-tuning, dynamic tile selection, or heuristic backend arbitration.
- Persistent device-resident graph/tensor lifetimes across calls.
- Overlapped async transfer/compute pipelines and stream concurrency claims.
- Multi-device scheduling or distributed SGEMM.
- Mixed precision (FP16/BF16/TF32) or numerics policy expansion beyond initial `float32` contract.
- Kernel fusion, batched GEMM family expansion, or generalized BLAS surface.
- Claiming global CPU-vs-GPU superiority beyond protocol-scoped benchmark outputs.

## 7) Recommended next step

Implement a minimal **execution-status harness** before optimization work:

1. add one explicit dispatch selector (`cpu` | `prometheus`),
2. add structured fallback reason codes,
3. enforce transfer-inclusive timing as default benchmark metric,
4. emit per-run status records (`ok` / `fallback` / `error`) in machine-readable output,
5. gate any published performance summary on “no hidden fallback, no hidden errors” checks.

This establishes an honest runtime contract that is narrow, observable, and safe to iterate.
