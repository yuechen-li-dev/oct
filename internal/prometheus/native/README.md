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
  - M5/M6 Complex32 radix-2 Vulkan execution family; production qualification
    remains unclaimed until its required RTX 3070 evidence is recorded.
- `../reactors/PrometheusReduction.concept` and `PrometheusExecution.concept`
  - production row-wise FP32 sum/max/stable-softmax planning and execution,
    reusable slot buffers, package-created pipelines, completion, quarantine,
    and retirement. The executor uses DragonGod's FIFO ready queue and Standard
    spans/math over a borrowed Concept Vulkan device attachment.
- `reactor_concept_reduction_abi.c` and `reactor_concept_reduction.cpp`
  - an opaque C11 facade and C++23 public/package adapters. Generated private
    class layouts and C11 atomics do not cross into C++.
- `../reactors/PrometheusRmsNorm.concept`
  - RMSNorm weights, reusable buffers, FP32/BF16 normalization, and the resident
    head-normalization → RoPE → attention scores chain. Three producer slots
    retain ordinary output, Q, and K; a lazy score slot owns its output and
    pending submission. Independent standalone RoPE storage preserves resident
    Q/K. Reuse and destruction drain submitted work before releasing buffers.
- `reactor_concept_rmsnorm_abi.c` and `reactor_concept_rmsnorm.cpp`
  - C11 packet projection and C++23 boundary/package adaptation. The existing
    pure RMSNorm planner supplies strategy and admission policy. The whole
    score chain uses the same Concept attachment without a host detour or
    legacy model-state allocation.
- `reactor_vulkan_queue.cpp`
  - the common host queue API synchronization boundary for all native families.
- `reactor_vulkan_fused_reduction.c`
  - public plan serialization, ABI validation, CPU oracles, and still-live model
    resource support. Its standalone C reduction executor has been removed.
- `reactor_vulkan_gemma4e2b.c`
  - pure RMSNorm planning/oracles; no GPU execution or resource ownership. The C RMSNorm
    executor and weight ownership were removed in
    [Concept M2](../../../docs/internal/prometheus_concept_rmsnorm_m2.md); the C
    score executor was removed in
    [Concept M3](../../../docs/internal/prometheus_concept_attention_m3.md), and standalone
    RoPE in [Concept M4](../../../docs/internal/prometheus_concept_rope_m4.md). The M42-M49b
    transformer executors, the numerical-research shadow, and the
    transformer control module were removed in Phase 0, and the file was
    renamed from `reactor_vulkan_transformer.c` (see
    `docs/design/PROMETHEUS-AUDIT-2026-09.md` in the Concept repository);
    their reports remain under `../DevelopmentReport/`.

Pre-DVT M0 removed live transformer implementation from the fused-reduction
translation unit. `reactor_vulkan_runtime_internal.h` is the intentionally
temporary private migration seam for the historical mixed slot/state record;
its callable reduction contract does not transfer slot, descriptor, or buffer
ownership to the transformer runtime.

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
- FFT executes its bounded Complex32 radix-2 Vulkan route through the generic
  runtime-services seam. It has no host FFT fallback; hardware qualification is
  tracked separately from route availability.
- Fused reduction is row-wise and bounded; it does not own arbitrary tensor
  axes, a graph executor, normalization policy, or SGEMM selection.

## Native build entrypoints

Supported native build helpers:

- Linux: `bash internal/prometheus/native/build_linux.sh`
- Compatibility: `build_stub.sh` prints a deprecation warning and forwards to `build_linux.sh`.
- Windows: `internal\prometheus\native\build_windows_launcher.cmd` (authoritative launcher; locates Visual Studio, initializes x64 MSVC, and writes `out\test-artifacts\prometheus_native_windows_build.log`).
- Low-level Windows build body: `internal\prometheus\native\build_windows.cmd` (called by the launcher; do not duplicate its source lists or build logic).

The consumer package is [`../manifest.concept`](../manifest.concept). The
helpers build its C11 and C++23 archives through `concept package build`, then
link them with the remaining legacy operations. Clang/LLVM (including
`llvm-ar`), a C++23 standard library with `std::expected`, and the Vulkan SDK
are required alongside the platform tools. Set `CONCEPT_LIBRARY_ROOT` to the
Concept distribution's `libraries`; the development default is the sibling
`Concept/libraries`. Set `CONCEPT_BIN` to a built driver, install `concept` on
PATH, or keep the sibling Concept compiler checkout for the Go bootstrap.
The foundational Vulkan implementation is compiled from that dependency.

The native helpers regenerate Concept C, headers, and provenance into ignored
`internal/prometheus/.native-build/generated/` before compiling any consumer.
For a clean package build, run `go run ./tools/prometheus_concept -build-package`;
this emits source, validates source/output hashes, then invokes the manifest
builder. Direct `concept package build internal/prometheus` consumes those
materialized inputs and therefore follows emission. Standalone emission is
`go run ./tools/prometheus_concept -concept-root ../Concept`; `-check` compares
fresh generation. These generated C/JSON files are build products and do not
belong in Git. Provenance records the actual compiler binary and, when built
from a checkout, its revision and source/library hashes. See
[shared-device migration evidence](../../../docs/internal/prometheus_concept_shared_device_m1.md).

Expected Windows outputs:

- `out\prometheus\native\marionette_tests.exe`
- `out\prometheus\native\marionette_benchmarks.exe`

Benchmark-lane note:

- `marionette_benchmarks.exe` is the supported P13 benchmark smoke lane in this repository.
- `marionette_tests.exe --bench` uses Marionette's registered `BENCHMARK(...)` registry.
- The current Prometheus DVT smoke path is FACT-driven, so `--bench` may legitimately report `0 benchmark(s)` until explicit benchmark registrations are added.
