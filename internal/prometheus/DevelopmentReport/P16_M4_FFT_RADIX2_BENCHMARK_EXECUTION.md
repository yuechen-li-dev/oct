# P16 M4 — FFT Radix-2 Benchmark Execution

## Scope
Benchmark-only radix-2 FFT execution added in `prometheus_reactor_runtime_fft_benchmark_variant` for forward complex32 shapes with `N<=16`, `batch_count=1`, `stride=0|N`.

## Non-goals
No production FFT execution, no caps claim, no SGEMM changes, no arena reuse, no adaptive radix/twiddle strategies, no real-to-complex.

## Semantic authorities
- `Libraries/Mathematics/Mathematics.Transforms.oct`
- `internal/interpret/fft.go`
- `Experiments/PrometheusFftAlgorithmLab/M1`

## ping_pong_swap_count clarification
M4 defines `ping_pong_swap_count` as **number of radix-2 passes in the plan that alternate ping/pong intermediate roles**. This equals `pass_count` for `N>1`; for `N=1` it is `0`. Existing tests were updated for `N=1,8,16` and benchmark execution covers `N=2`.

## Shader/twiddle/buffer approach
M4 uses inline twiddle computation (`cos/sin`) in a small radix-2 execution path and per-call buffers from request input/output only. This is benchmark-only plumbing and defers FFT-native arena lifecycle work to P16 M6.

## Benchmark-only behavior
- `prometheus_reactor_runtime_fft(...)` remains unavailable.
- `...fft_benchmark_variant(...,2,...)` executes only for the supported shape subset above.
- Unsupported valid requests fail truthfully as unavailable and keep production disabled/capability unclaimed.

## Added tests and benchmarks
- Marionette FACT: benchmark execution correctness for N=2/N=4/N=8 and production-unavailable assertion.
- Marionette FACT: plan metadata and `ping_pong_swap_count` deterministic assertions.
- Marionette BENCHMARK: `PrometheusReactor_Fft_Radix2Benchmark_N16`.

## Deferred scope
Production enablement, caps claim, inverse expansion, batch>1, arena reuse, radix-4/8, adaptive selector, advanced twiddle strategy, real-to-complex.
