# P5c — Enable `PROMETHEUS { ... }` in Compiled Benchmark Authoring Surface

Date executed: **2026-04-21 (UTC)**

## 1) Blocker discovered in P5b

`PROMETHEUS { ... }` was not parseable/lowerable in `.octest [Benchmark]` functions, so compiled benchmark authoring only had the CPU path.

## 2) Minimal pipeline stages fixed

1. **Lex/parse/AST surface**
   - Added `PROMETHEUS` as a language keyword.
   - Added `ast.PrometheusStmt` and parser support for `PROMETHEUS { ... }` blocks.

2. **Typecheck legality (kept narrow)**
   - `PROMETHEUS { ... }` is accepted only inside `[Benchmark]` functions.

3. **MIR lowering + compiled backend plumbing**
   - Lowering now tracks when expressions are inside a Prometheus block.
   - `Matrix<Float> @ Matrix<Float>` inside `PROMETHEUS { ... }` lowers to a dedicated compiled builtin (`PrometheusMatMulMM`).
   - Generated compiled Go helper emits explicit backend truth (`backend_requested=prometheus`, `backend_used=cpu`, `status=fallback(prometheus_unavailable)`) and executes the existing compiled matrix multiply path, avoiding silent substitution.

4. **Benchmark runner observability**
   - Compiled benchmark stdout is now surfaced by `oct bench`, so backend requested/used/status lines emitted by Prometheus-path execution are visible in benchmark runs.

## 3) Official M0 corpus status

M0 now contains both benchmark variants:

- `M0/matrix_mul_cpu_reference.octest`
- `M0/matrix_mul_prometheus.octest`

Both compile and run through the compiled benchmark path.

## 4) Reporting/fallback truth in current cloud environment

In this Codex/cloud snapshot, Prometheus runtime remains environment-constrained and reports explicit fallback:

- `backend_requested=prometheus`
- `backend_used=cpu`
- `status=fallback(prometheus_unavailable)`

No silent CPU substitution is reported as success.

## 5) What remains for local validation later

- Re-run the same M0 corpus on local hardware with a valid Prometheus reactor.
- Verify runs where `backend_used=prometheus` and status stays explicit.
- Capture hardware-specific timing behavior after backend-truth validation.

## Convergence state

**Success**: the benchmark authoring gap isolated by P5b is closed for compiled benchmark path symmetry (CPU + Prometheus-authored blocks in `.octest [Benchmark]`).
