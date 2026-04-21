# P5b — Prometheus Benchmark Harness Experiment (M0)

Date executed: **2026-04-21 (UTC)**

## 1) Experiment folder structure

- `Experiments/PrometheusBenchmarkHarness/manifest.oct`
- `Experiments/PrometheusBenchmarkHarness/REPORT.md`
- `Experiments/PrometheusBenchmarkHarness/M0/main.oct`
- `Experiments/PrometheusBenchmarkHarness/M0/matrix_mul_cpu_reference.octest`

This follows the existing experiment convention (`manifest.oct` + `REPORT.md` + milestone folder).

## 2) Benchmark corpus cases added

### Added

- **CPU reference benchmark**
  - File: `M0/matrix_mul_cpu_reference.octest`
  - Case: `[Benchmark] fn MatrixMulCPUReferenceM0() -> Void`
  - Workload shape: matrix-vector multiply via `@`.

### Requested-but-blocked in current snapshot

- **Prometheus-forced `.octest [Benchmark]` case using `PROMETHEUS { ... }`**

Blocker evidence:

1. `internal/lex/lex.go` does not define a `PROMETHEUS` keyword/token.
2. There is no Oct-surface benchmark routing construct to force Prometheus backend from `.octest`.
3. Prometheus execution currently surfaces through `oct prometheus-sgemm <cpu|prometheus>` (CLI path), not Oct benchmark syntax.

## 3) Benchmark compile/lowering status

### CPU `.octest [Benchmark]`

Command:

- `go run ./cmd/oct bench Experiments/PrometheusBenchmarkHarness --octagon-out /tmp/p5b-bench.octagon`

Observed:

- Benchmark discovered and run under milestone mode.
- Compiled benchmark execution path succeeded.
- Milestone-prefixed benchmark `.octagon` output emitted at `/tmp/M0.p5b-bench.octagon`.

## 4) Benchmark execution status

### CPU benchmark case

- `Main.MatrixMulCPUReferenceM0` passed end to end.

### Prometheus harness status in this environment

Command:

- `go run ./cmd/oct prometheus-sgemm prometheus --octagon-out /tmp/p5b-prometheus.octagon`

Observed:

- Prometheus request was accepted at CLI level.
- Runtime status was explicit fallback for all starter corpus shapes:
  - `backend_requested=prometheus`
  - `backend_used=cpu`
  - `status=fallback(prometheus_unavailable)`
  - `vulkan_env=unavailable`

This is environment-honest for Codex/cloud: no hardware-GPU claim is made.

## 5) Artifacts/reports emitted

- CPU benchmark run summary:
  - `/tmp/M0.p5b-bench.octagon`
- Prometheus SGEMM harness report:
  - `/tmp/p5b-prometheus.octagon`

Both artifacts were emitted and loadable as plain `.octagon` output.

## 6) CPU vs Prometheus symmetry status (M0)

- **Partial:** CPU path is validated through `.octest [Benchmark]` compiled benchmark harness.
- **Blocked:** Prometheus-forced path is currently only available through `oct prometheus-sgemm`, not `.octest [Benchmark]` + `PROMETHEUS { ... }` authoring.

## 7) What remains for local/hardware validation

1. Add Oct-surface Prometheus routing in benchmark authoring (`PROMETHEUS { ... }` or equivalent) so CPU and Prometheus both run through the same `.octest [Benchmark]` mechanism.
2. Re-run M0 on local hardware with reactor available and verify:
   - `backend_used=prometheus`
   - non-software Vulkan environment where applicable.
3. Only after (1) and (2), start performance-focused corpus expansion.

## Convergence state

**Meaningful progression**: official experiment scaffold exists and end-to-end benchmark/report harness is proven for CPU plus CLI Prometheus harness; the next blocker is isolated with concrete evidence (missing Oct-language Prometheus routing for `.octest [Benchmark]`).
