# PX16 M17 - Remove CPU Tolerance Scan From SGEMM Dispatch

## Root cause

M16/H2 localized the remaining large SGEMM wall-clock gap to `pre_dispatch_ms`. The culprit was the FP16 tolerance evaluator in production layout/precision fact preparation.

`prom_fp16_evaluate_tolerance(...)` recomputed both FP32 reference products and quantized FP16-storage products for every output element and every K term. That made the production GPU dispatch path perform O(M*N*K) CPU work before submitting the GPU work.

## Call-site audit

| Former call site | Production dispatch? | Benchmark/reporting? | Correctness validation? | Actual A/B data? | Complexity | Debug-gated? | M17 action |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `prom_reactor_runtime_sgemm_impl_with_variant(...)` | Yes | Yes, via EVT production performance lane | No | Yes | O(M*N*K) | No | Replaced with cheap production FP16 facts. |
| `prom_reactor_runtime_sgemm_batch_impl(...)` | Yes, batch planning/dispatch | No direct EVT row, but production API | No | Yes | O(M*N*K) | No | Replaced with cheap production FP16 facts. |

After M17, there are no remaining calls to `prom_fp16_evaluate_tolerance(...)`; the helper was removed.

Related CPU work:

- `prom_apply_debug_row_major_oracle(...)` remains gated by `PROM_TESTCFG_PACKED4_DEBUG_ORACLE_CHECK` and is not part of default EVT performance timing.
- Packed4 and FP16 upload transforms remain O(M*K + K*N) and are upload/pre-dispatch preparation work, not cubic tolerance analysis.
- EVT correctness validation still uses its CPU oracle outside the production benchmark mode.

## What tolerance was used for

The tolerance evaluator populated FP16 diagnostics and the layout/precision selector facts:

- `fp16_tolerance_known`
- `fp16_tolerance_pass`
- FP16 error/cancellation diagnostic fields
- FP16 utility score
- special-value rejection

It did not retune occupancy selection, change kernel code, alter SDSL-V behavior, or feed P14/P15/FFT authority. It was a pre-dispatch diagnostic/policy fact producer that had drifted into the runtime hot path.

## Fix strategy

Production SGEMM and batch planning now call `prom_fp16_prepare_production_tolerance_facts(...)`.

That helper:

- sets production FP16 tolerance facts to known/pass without running a dense CPU tolerance oracle;
- preserves special-value rejection by scanning A/B inputs directly in O(M*K + K*N);
- resets FP16 error diagnostic fields to zero because production dispatch no longer computes dense FP16 error diagnostics;
- keeps FP16 utility below the existing selection threshold unless tests explicitly use `PROM_TESTCFG_FORCE_FP16_UTILITY_WIN`;
- leaves judgment-engine gates, selector scoring, kernels, dispatch authority, SDSL-V, P14/P15, and FFT untouched.

New diagnostics:

- `px16_m17_last_tolerance_eval_wall_ns`
- `px16_m17_last_tolerance_eval_in_dispatch`
- `px16_m17_last_tolerance_eval_source`

For production dispatch, `px16_m17_last_tolerance_eval_in_dispatch=false` and `px16_m17_last_tolerance_eval_wall_ns=0`.

## Validation behavior

Correctness validation remains separate from benchmark timing. The EVT correctness lane still computes CPU-oracle output validation through `PrometheusSgemmPx16Evt_CorrectnessValidationLane`; the production performance lane does not run dense oracle/tolerance work.

## Regression diagnostics

Added:

```bat
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16M17_ToleranceEvalNotInDispatch
```

The test runs:

- `128x128x128`
- `256x256x256`
- `512x512x512`

It asserts that tolerance evaluation is not reported inside production dispatch and writes:

- `out/test-artifacts/prometheus_sgemm_px16_m17_tolerance_dispatch.json`

The existing H2 coarse diagnostic now also writes `tolerance_eval_ms`, `tolerance_eval_in_dispatch`, `tolerance_eval_source`, and pre-dispatch cube ratios.

## Before/after timing

Before M17, M16/H2 recorded an example `square_512x512x512` total around `1158.98 ms`, with `pre_dispatch_ms` around `1149.63 ms`.

After M17, the production runtime reports `tolerance_eval_in_dispatch=false` and `tolerance_eval_ms=0` for the performance path. Fresh hardware timing should be regenerated with:

```bat
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16Deep_CoarsePhaseLocalization
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16M17_ToleranceEvalNotInDispatch
out\prometheus\native\marionette_benchmarks.exe PrometheusSgemmPx16Evt
```

## Required verification

```bat
go test ./internal/prometheus/... ./cmd/oct
go test ./internal/... ./cmd/oct
cmd /c "call \"C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat\" -arch=x64 -host_arch=x64 >nul && internal\prometheus\native\build_windows.cmd"
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16Resident
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16M15aSdslScalarPlusLowKRepro
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16Deep_CoarsePhaseLocalization
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16M17_ToleranceEvalNotInDispatch
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16Evt_CorrectnessValidationLane
out\prometheus\native\marionette_benchmarks.exe PrometheusSgemmPx16Evt
```
