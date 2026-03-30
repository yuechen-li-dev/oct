# P1b — Prometheus Correctness Oracle / Benchmark Honesty Pressure Test

## 1) Reference correctness method

- **Reference algorithm:** deterministic CPU SGEMM in Go using `float32` inputs/outputs and **`float64` accumulation** (`sum += float64(a)*float64(b)`), then cast to `float32` for store.
- **Memory/layout contract:** row-major `A[M,K]`, `B[K,N]`, `C[M,N]`; explicit leading dimensions in harness to avoid hidden transpose assumptions.
- **Comparison policy:**
  - Validate full output tensor element-wise (`allclose` style), plus:
    - max absolute error
    - max relative error
    - count of elements outside tolerance
    - first failing index/value pair for debugging
- **Tolerance policy (initial):**
  - `abs_err <= 1e-4 OR rel_err <= 1e-3` for `float32` SGEMM
  - additional guard: no `NaN` or `Inf` unless explicitly expected by test case
- **Corpus data generation:** fixed seeds for reproducibility + a few hand-constructed adversarial patterns (zeros, ones, alternating sign, high-dynamic-range values).

---

## 2) First shape corpus

Use one compact set that still exercises regime changes:

- **Tiny sanity:**
  - `(1,1,1)`, `(2,2,2)`, `(4,4,4)`
- **Small awkward:**
  - `(3,5,7)`, `(7,3,5)`, `(5,7,3)`
- **Medium square-ish:**
  - `(64,64,64)`, `(128,128,128)`, `(256,256,256)`
- **Rectangular (tall/skinny & wide/short):**
  - `(1024,64,256)`, `(64,1024,256)`, `(256,64,1024)`, `(256,1024,64)`
- **Large throughput probes (still practical early):**
  - `(512,512,512)`, `(1024,1024,1024)`
- **Edge-ish alignment/padding stress:**
  - `(33,33,33)`, `(127,129,131)`, `(255,257,259)`

(Shape tuple is `(M,N,K)`.)

---

## 3) Benchmark protocol

- **Two reporting modes (must both be shown):**
  1. **End-to-end (transfer-inclusive):** host->device copy + kernel + device->host copy
  2. **Kernel-only (compute-focused):** steady-state kernel timing after buffers are resident
- **Cold vs warm:**
  - **Cold:** first invocation after process start/context init (report separately; never mixed into warm average)
  - **Warm:** discard 3-5 warmup iterations, then measure steady-state repetitions
- **Repetitions/statistics:**
  - At least 30 timed warm repetitions per shape
  - Report median, p90, and best; avoid only-best-number claims
- **Setup costs:**
  - Context creation/JIT/allocator startup reported as one-time setup section, not amortized into per-call unless explicitly stated
- **CPU comparison fairness:**
  - Compare against a strong CPU baseline (single-thread and multithread modes both reported)
  - Same data type (`float32`) and same mathematical operation (`C=A×B`, no fused extra ops)
  - CPU and GPU both use preallocated buffers in compute-only mode; both include allocation/transfer/setup in end-to-end mode when relevant
- **Performance metric:**
  - Report wall time and computed GFLOP/s (`2*M*N*K / time_s / 1e9`) with exact formula included

---

## 4) Dishonest benchmark patterns to reject

Reject any result if it does one or more of:

- reports only kernel time while claiming end-to-end speedup
- includes warm cache/path for one device and cold path for the other
- cherry-picks only best-of-N without median/p90
- compares GPU against intentionally weak CPU baseline (e.g., scalar naive only) and presents as “CPU”
- changes precision or algorithmic work between competitors
- excludes correctness checks from benchmark runs (fast wrong answer)
- mixes one-time compile/init costs into one side only
- omits shape details and reports a single “X faster” headline

---

## 5) Early success/failure criteria

- **Correctness pass criteria:**
  - 100% shapes in first corpus pass tolerance policy
  - zero unexpected `NaN/Inf`
  - deterministic pass across at least 3 repeated runs with same seed
- **Correctness failure meaning:**
  - Any out-of-tolerance element is a correctness failure; performance claims for that config are invalid until fixed
- **Benchmark pass criteria:**
  - Protocol-compliant measurements collected for all corpus shapes in both reporting modes
  - Variability disclosed (median/p90/best)
- **Benchmark failure meaning:**
  - Missing protocol fields or unfair comparison invalidates speedup claims; results are exploratory only, not publishable claims
- **Allowed conclusion at P1b:**
  - “Prometheus runs SGEMM correctly on this corpus and achieves these measured timings under this explicit protocol.”
- **Disallowed conclusion at P1b:**
  - “Prometheus is generally faster than CPU/other stacks” (too broad for early corpus/protocol scope)

---

## 6) Recommended next step

Implement a minimal SGEMM harness that:

1. runs the full corpus,
2. executes CPU-reference correctness checks first,
3. records benchmark metrics in both timing modes,
4. emits a machine-readable report (JSON + markdown summary), and
5. hard-fails CI if correctness fails or benchmark protocol fields are missing.

This keeps correctness gating ahead of performance storytelling from day one.
