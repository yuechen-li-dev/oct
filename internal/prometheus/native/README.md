# Prometheus Native Vulkan Reactor Topology

Current Vulkan reactor file topology (P12 M5 baseline):

- `reactor_vulkan.h`
  - shared internal Vulkan reactor declarations and stable `prom_reactor_runtime_*_impl` declarations used by `reactor_api.c`.
- `reactor_vulkan_common.c`
  - shared Vulkan plumbing helpers used by reactor families.
- `reactor_vulkan_sgemm.c`
  - complete SGEMM reactor family implementation (authoritative SGEMM runtime path).
- `reactor_vulkan_fft.c`
  - inert future FFT reactor family stub; no capability/API/runtime behavior claims.
- `reactor_vulkan_fused_reduction.c`
  - inert future fused-reduction reactor family stub; no capability/API/runtime behavior claims.

Topology rule:

> Split by compute primitive family, not by internal SGEMM subsystem.

Scope guardrails for this topology:

- `reactor_vulkan_common.c` is for genuinely shared Vulkan plumbing only.
- SGEMM policy/runtime behavior stays localized in `reactor_vulkan_sgemm.c` for auditability.
- FFT/fused-reduction files remain inert until implementation milestones explicitly add behavior.

## Native build entrypoints

Supported native build helpers:

- Linux: `bash internal/prometheus/native/build_linux.sh`
- Compatibility: `build_stub.sh` prints a deprecation warning and forwards to `build_linux.sh`.
- Windows: `internal\prometheus\native\build_windows.cmd`

Expected Windows outputs:

- `out\prometheus\native\marionette_tests.exe`
- `out\prometheus\native\marionette_slow_tests.exe`
- `out\prometheus\native\marionette_benchmarks.exe`

Benchmark-lane note:

- `marionette_benchmarks.exe` is the supported P13 benchmark smoke lane in this repository.
- `marionette_tests.exe --bench` uses Marionette's registered `BENCHMARK(...)` registry.
- The current Prometheus DVT smoke path is FACT-driven, so `--bench` may legitimately report `0 benchmark(s)` until explicit benchmark registrations are added.
