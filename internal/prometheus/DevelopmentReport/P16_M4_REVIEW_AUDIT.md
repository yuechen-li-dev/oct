# P16 M4 Reviewer Audit — FFT Benchmark-only Radix-2 Execution

## Verdict
**Outcome C — blocker**

M4 does **not** implement a Vulkan FFT execution path. The benchmark variant path currently executes a host-side C radix-2 FFT directly in `reactor_vulkan_fft.c`, then marks diagnostics as if the Vulkan radix-2 path executed.

## 1) Is benchmark execution path actually Vulkan?
**No.**

### Evidence
- `prom_reactor_runtime_fft_benchmark_variant_impl(...)` conditionally enters the M4 benchmark lane and then calls `prom_fft_execute_forward_radix2(request->input, request->output, &plan);` directly in C. There is no Vulkan pipeline creation, no shader dispatch, no command buffer recording, and no device buffer upload/download sequence in this file.
- `prom_fft_execute_forward_radix2(...)` performs full FFT math on host arrays and writes directly to `request->output`.
- The file has no Vulkan compute objects/functions for FFT (no shader module/pipeline handles, no `vkCmdDispatch`, no FFT-specific `VkBuffer` transfer choreography).

### Additional inconsistency
- Diagnostics set `requested_path_id`/`executed_path_id` to `PROM_FFT_PATH_VULKAN_RADIX2_RESERVED` on success even though execution is host-side in the current implementation.

## 2) Did any C-side CPU FFT execution slip in?
**Yes (blocker).**

### Host-side FFT math found
In `prom_fft_execute_forward_radix2(...)`:
- bit-reversal reorder loop over host `output[...]`
- iterative radix-2 butterfly loops over host arrays
- twiddle math using `cosf` and `sinf`
- direct writeback to host `request->output`

This is exactly the class of C-side FFT execution the P16 doctrine forbids for milestone intent.

## 3) Is production still unavailable?
**Yes.**

- `prometheus_reactor_runtime_fft(...)` remains unavailable via `prom_reactor_runtime_fft_impl(...)` returning `PROM_ERROR` with `PROM_FFT_DETAIL_UNAVAILABLE` for valid requests.
- `production_enabled` remains `0` by default.
- No FFT capability bit/claim was added in this audit pass.

## 4) Is `requested_variant == 2` cleanly defined?
**Partially, then hardened in this audit.**

### Before audit
- Runtime C file used internal `#define PROM_FFT_BENCHMARK_VARIANT_RADIX2 2u`.
- Marionette tests/bench used raw `2u` literals.

### Hardening fix made
- Added public named API constant enum in `reactor_api.h`:
  - `PROM_FFT_BENCHMARK_VARIANT_NONE = 0`
  - `PROM_FFT_BENCHMARK_VARIANT_RADIX2 = 2`
- Replaced magic-number benchmark calls in FFT Marionette FACT/BENCHMARK files with `PROM_FFT_BENCHMARK_VARIANT_RADIX2`.
- Runtime now uses the shared API constant and removed duplicate local macro.

## 5) Are correctness and benchmark lanes separated?
**Yes.**

- Correctness/diagnostics are in Marionette `FACT` tests (`reactor_fft_api_tests.cpp`).
- Performance measurement is in Marionette `BENCHMARK_WITH_ITERATIONS` (`reactor_fft_benchmarks.cpp`).
- No `FACT` timing loop was repurposed as benchmark lane.

## 6) Are small-shape correctness tests meaningful?
**Yes (within current host-execution limitation).**

FACT coverage includes:
- N=2 numeric check
- N=4 impulse check
- N=8 alternating check
- production-disabled and capability-unclaimed assertions
- unsupported shape (`N=32`) failure path assertion
- requested/executed path diagnostics assertions

## 7) Is `ping_pong_swap_count` unambiguous?
**Yes.**

Observed plan diagnostics and assertions align with:
- `ping_pong_swap_count == plan_pass_count`
- N=1 => 0
- N=2 => 1
- N=8 => 3
- N=16 => 4

## 8) Is M4 scope constrained and truthful?
**Constraints are implemented, but execution-path truthfulness is violated by path labeling.**

Implemented benchmark constraints checked in code:
- `batch_count == 1`
- `stride_elements == 0 || stride_elements == element_count`
- `element_count <= 16`
- forward only (`INVERSE` rejected)

Deferred scope remains deferred (inverse/radix-4/radix-8/adaptive/real-complex not enabled), but success diagnostics currently imply Vulkan path execution when host path actually runs.

## 9) Is SGEMM untouched?
**Yes (no FFT-driven SGEMM behavior changes found in audited files).**

SGEMM Marionette filter run remains pass/skip as environment-dependent.

## Validation runs
- `bash internal/prometheus/native/build_stub.sh` — pass
- `out/prometheus/native/marionette_tests PrometheusReactor_Fft` — **fails 1 FACT** (default diagnostics expectation stale after benchmark test mutates static FFT diag slot state)
- `out/prometheus/native/marionette_tests PrometheusReactor_Sgemm` — pass with expected Vulkan-unavailable skips
- `out/prometheus/native/marionette_tests --bench PrometheusReactor_Fft` — benchmark executes
- `go run ./cmd/oct test Experiments/PrometheusFftAlgorithmLab/M1` — pass
- `go run ./cmd/oct artifact Experiments/PrometheusFftAlgorithmLab/M1` — pass
- `go test ./internal/... ./cmd/oct` — pass

## Why this is a blocker
M4 was expected to be benchmark-only **Vulkan-backed** execution plumbing. The current implementation is benchmark-only **host-side C FFT execution** while reporting Vulkan path ids. This is a milestone-truthfulness and architecture-boundary violation for P16 FFT doctrine.

## Recommended next step
- Do **not** bless M4 as Vulkan benchmark execution.
- Either:
  1. re-scope wording/reporting to explicitly say host-side benchmark surrogate (and avoid Vulkan executed-path claim), or
  2. implement actual Vulkan compute FFT execution path and only then report/diagnose it as executed Vulkan radix-2.
