# P16 M2 — Prometheus FFT ABI Skeleton + Diagnostics Defaults

## Scope
- Added public FFT ABI structs/enums/functions.
- Added default-off FFT impl stubs in `reactor_vulkan_fft.c`.
- Added FFT diagnostics API and sized variant.
- Added Marionette FFT API tests for default diagnostics and validation behavior.

## Non-goals
- No Vulkan FFT shader dispatch.
- No radix-2 implementation.
- No CPU FFT implementation in C.
- No production capability claim.
- No caps v2 changes.
- No SGEMM behavior changes.

## API additions
- `PrometheusComplex32`, `PrometheusFftRequest`, `PrometheusFftDiagnostics`.
- FFT flags for direction/inverse-normalize/benchmark opt-in.
- FFT path ids and path status enums.
- FFT detail codes for unavailable and request validation failures.
- Public API:
  - `prometheus_reactor_runtime_fft`
  - `prometheus_reactor_runtime_fft_benchmark_variant`
  - `prometheus_reactor_runtime_fft_diagnostics`
  - `prometheus_reactor_runtime_fft_diagnostics_sized`

## Diagnostics defaults
- `api_declared=1`
- `capability_reported=0`
- `production_enabled=0`
- `benchmark_enabled=0`
- default executed path unavailable.
- future plan/arena/cache fields stable zero defaults.

## Validation behavior
- Rejects: null request, wrong struct size, null input, null output, element_count=0, non-power-of-two N,
  invalid direction flags, zero batch_count, invalid stride (`stride!=0 && stride<element_count`).
- `stride_elements==0` records effective stride as contiguous `element_count`.
- Runtime execution APIs return unavailable detail (truthful non-implementation).

## Capability non-claim
- Probe behavior unchanged; FFT capability is not reported.
- FFT runtime calls remain explicitly unavailable.

## Tests added
- `internal/prometheus/native/Marionette/reactor_fft_api_tests.cpp`
  - default diagnostics state
  - unavailable execution behavior and diagnostics snapshot
  - invalid request validation detail coverage
  - benchmark variant unavailable/truthful diagnostics

## Deferred scope
- CPU/native oracle in C
- deterministic C plan builder
- Vulkan radix-2 shader
- batch arenas
- radix-4/radix-8
- adaptive radix
- twiddle strategy
- real-to-complex

## Validation commands/results
- Ran:
  - `go run ./cmd/oct test Experiments/PrometheusFftAlgorithmLab/M1`
  - `go run ./cmd/oct artifact Experiments/PrometheusFftAlgorithmLab/M1`
  - `bash internal/prometheus/native/build_stub.sh`
  - `out/prometheus/native/marionette_tests PrometheusReactor_Sgemm`
  - `out/prometheus/native/marionette_tests PrometheusReactor_Fft`
  - `go test ./internal/... ./cmd/oct`

