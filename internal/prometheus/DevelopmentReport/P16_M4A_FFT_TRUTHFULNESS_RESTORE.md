# P16 M4a — Restore FFT Benchmark Truthfulness and Diagnostics Isolation

## Summary
P16 M4a corrects the M4 blocker: host-side C FFT execution has been removed from the benchmark variant API path. Radix-2 benchmark variant remains declared and requestable, but execution is truthfully reported as unavailable until a real Vulkan radix-2 dispatch path exists.

## Changes made
1. Removed reachable host-side FFT execution from benchmark path
- Deleted host execution helpers (`prom_fft_reverse_bits`, `prom_fft_execute_forward_radix2`) from `reactor_vulkan_fft.c`.
- `prom_reactor_runtime_fft_benchmark_variant_impl(...)` now only validates/records request and plan metadata for supported benchmark shape constraints; it does not compute FFT output and does not return benchmark success.

2. Restored truthful diagnostics for benchmark variant requests
- For `PROM_FFT_BENCHMARK_VARIANT_RADIX2` requests in benchmark-eligible shape subset:
  - requested path recorded as radix-2 benchmark reserved path
  - executed path remains unavailable
  - validation status remains registered (not benchmark-enabled)
  - failure detail remains unavailable (not wired)
- Production remains disabled and capability remains unclaimed.

3. Fixed FFT diagnostics isolation
- Added explicit FFT diagnostics slot cleanup on runtime destroy (`prom_fft_diag_forget_handle(handle)`), invoked from `prom_reactor_runtime_destroy_impl`.
- This prevents stale FFT diagnostics state from leaking when a handle address is recycled by allocator across test cases.

4. Kept benchmark variant constants
- Retained public constants in `reactor_api.h`:
  - `PROM_FFT_BENCHMARK_VARIANT_NONE`
  - `PROM_FFT_BENCHMARK_VARIANT_RADIX2`

5. Updated Marionette FFT tests
- Removed host-execution correctness FACT that asserted FFT numeric output from benchmark variant path.
- Updated benchmark-variant FACT to assert truthful unavailable/not-wired behavior while still recording requested variant/path.
- Preserved plan/diagnostics/default/freshness/invalid-handle/production-unavailable coverage.

## Scope truth after M4a
- Production FFT: unavailable.
- Capability claim: unchanged (no FFT claim).
- Benchmark radix-2 variant: declared, requestable, not wired to Vulkan execution yet.
- No host-side C FFT fallback remains reachable via benchmark API.

## Validation
- Build + native test filters + native benchmark filter executed.
- Oct FFT M1 lab test/artifact executed.
- Full Go test sweep executed.

## Next step
Implement actual Vulkan-backed radix-2 benchmark execution path (pipeline/shader/dispatch/device buffers) and only then promote benchmark path status from unavailable/not wired to benchmark-enabled.
