# Prometheus Native Vulkan Reactor Topology

SDSL-V production-source, generated-artifact, benchmark, and audit ownership
is documented in `docs/SDSL_V_WORKSPACE.md`. The native shader manifest remains
the production registry's declarative asset inventory.

Current Vulkan reactor file topology (P12 M5 baseline):

- `reactor_vulkan.h`
  - shared internal Vulkan reactor declarations and stable `prom_reactor_runtime_*_impl` declarations used by `reactor_api.c`.
- `reactor_vulkan_common.c`
  - shared Vulkan plumbing helpers used by reactor families.
- `reactor_vulkan_sgemm.c`
  - synchronous/async SGEMM runtime support and shared M29/M30 lifecycle helpers.
- `reactor_batch.c`
  - the sole production batch engine: immutable logical plans, centralized
    shared-ring refill, deterministic failure reduction, staged atomic commit,
    and truthful batch evidence. It delegates task lifecycle and physical ring
    ownership to M30/M30a and M29; it does not own async tokens, queues, or
    future executors.
- `reactor_vulkan_fft.c`
  - inert future FFT reactor family stub; no capability/API/runtime behavior claims.
- `reactor_vulkan_fused_reduction.c`
  - production M39b row-wise FP32 sum/max/stable-softmax family: deterministic
    plans, one-workgroup and staged dispatch, persistent family ring,
    device-local reusable temporaries, timestamps, validation, and CPU oracle.
  - Current pre-DVT debt: this unit also retains live M42-M49b transformer
    runtime ownership through a shared private slot/state type.  See
    `../DevelopmentReport/PROMETHEUS_PRE_DVT_M0_TRANSFORMER_RUNTIME_HARDENING.md`;
    this is a migration target, not intended final ownership.

Topology rule:

> Split by compute primitive family, not by internal SGEMM subsystem.

## Batch execution

Public SGEMM batch execution is M31-only. The low eight flag bits are a
requested logical planning width and bit `0x00000100` selects contiguous
logical partitioning; neither creates a thread, queue, or lane-owned Vulkan
resource. All other legacy/test bits are rejected before admission with
`PROM_DETAIL_BATCH_UNSUPPORTED_OPTION` (`-6613`).

There is no native CPU batch fallback, empty-submit batch path, worker-thread
bridge, worker-local batch command resource, or fake multi-queue batch mode.
The v1 diagnostic fields retained from that history are ABI tombstones or
logical-plan aggregates; M31 ring, submission, completion, commit, feedback,
and quarantine evidence is authoritative. Historical P11 reports are retained
under `internal/prometheus/DevelopmentReport/`; see
`PROMETHEUS_P11_ARCHITECTURE_RETROSPECTIVE.md` for the deleted design.
R2e2 deletion evidence is in `PROMETHEUS_R2E_P11_REMOVAL.md`; design-only
future concurrency directions are in
`PROMETHEUS_CONCURRENCY_FUTURE_DIRECTIONS.md`. Neither documents an
implemented concurrency expansion.

Scope guardrails for this topology:

- `reactor_vulkan_common.c` is for genuinely shared Vulkan plumbing only.
- SGEMM policy/runtime behavior stays localized in `reactor_vulkan_sgemm.c` for auditability.
- FFT remains inert until an implementation milestone explicitly adds behavior.
- Fused reduction is row-wise and bounded; it does not own arbitrary tensor
  axes, a graph executor, normalization policy, or SGEMM selection.

## Native build entrypoints

Supported native build helpers:

- Linux: `bash internal/prometheus/native/build_linux.sh`
- Compatibility: `build_stub.sh` prints a deprecation warning and forwards to `build_linux.sh`.
- Windows: `internal\prometheus\native\build_windows_launcher.cmd` (authoritative launcher; locates Visual Studio, initializes x64 MSVC, and writes `out\test-artifacts\prometheus_native_windows_build.log`).
- Low-level Windows build body: `internal\prometheus\native\build_windows.cmd` (called by the launcher; do not duplicate its source lists or build logic).

Expected Windows outputs:

- `out\prometheus\native\marionette_tests.exe`
- `out\prometheus\native\marionette_benchmarks.exe`

Benchmark-lane note:

- `marionette_benchmarks.exe` is the supported P13 benchmark smoke lane in this repository.
- `marionette_tests.exe --bench` uses Marionette's registered `BENCHMARK(...)` registry.
- The current Prometheus DVT smoke path is FACT-driven, so `--bench` may legitimately report `0 benchmark(s)` until explicit benchmark registrations are added.
