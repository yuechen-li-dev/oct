# Prometheus Native Vulkan Reactor Topology

SDSL-V production-source, generated-artifact, benchmark, and audit ownership
is documented in `docs/SDSL_V_WORKSPACE.md`. The native shader manifest remains
the production registry's declarative asset inventory.

The Concept migration is a maintenance decision. Handwritten Vulkan C became
difficult to extend and keep correct even with the tooling around it. Concept
owns typed GPU operations and resource lifetimes; Concept-shaped C++23 remains
useful for host integration, package adaptation, and boundaries that benefit
from its existing ecosystem. Each slice replaces an implementation when the
replacement is simpler and demonstrably works. Language purity and source-line
ratios are not acceptance criteria.

Current Vulkan reactor file topology:

- `reactor_vulkan.h`
  - shared internal Vulkan reactor declarations and stable `prom_reactor_runtime_*_impl` declarations used by `reactor_api.c`.
- `reactor_vulkan_common.c`
  - shared Vulkan plumbing helpers used by reactor families.
- `../reactors/PrometheusSgemm.concept`, `reactor_concept_sgemm.cpp`, and `reactor_sgemm.cpp`
  - one Concept recording/submission owner for sync, resident, async, batch and
    placement-audit work. C++23 retains package instances, mapped allocation
    metadata, the shared runtime host, and existing judgment/diagnostic policy.
    C11 transports the generated job layout. The old C executor is deleted;
    ordinary Oct `@` keeps its existing dispatch pending M14. See
    [M13 qualification](../../../docs/internal/prometheus_concept_sgemm_m13.md).
- `reactor_batch.c`
  - the sole production batch engine: immutable logical plans, centralized
    shared-ring refill, deterministic failure reduction, staged atomic commit,
    and truthful batch evidence. It delegates task lifecycle and physical ring
    ownership to M30/M30a and M29; it does not own async tokens, queues, or
    future executors.
- `reactor_vulkan_fft.c`
  - Legacy Complex32 request validation and plan diagnostics. Concept owns its
    Vulkan executor alongside the binary64 Oct FFT executor; see the
    [M12 qualification](../../../docs/internal/prometheus_concept_fft_m12.md).
- `../reactors/PrometheusFft.concept` and `reactor_concept_fft.cpp`
  - Reusable FFT buffers, package pipelines, submission/completion and retirement
    in Concept. The C++23 adapter supplies FP64 host twiddles and per-runtime
    attachment; C11 only transports generated layouts and spans.
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
- `../reactors/PrometheusModelBlock.concept`
  - owns the resident block's buffers, replacement candidates, package pipelines,
    and pending work. Its named pre-attention stages implement BF16 ingress,
    AdaLN, normalization/modulation, QKV projection, and Q/K normalization/RoPE;
    its attention and FFN stages complete standalone NoiseRefiner execution.
    Typed capture plans include the split W3 intermediate. Static C audit replay
    borrows the same Concept stage implementations and resources. ContextRefiner
    reuses those stages with its own text-position head contract, unit modulation
    and eleven logical weights. MainTransformer adds mixed image/text positioning,
    timestep preparation and joint publication to the shared stage vocabulary.
    Three separate pipeline banks retain all production portfolios in one owner.
    Its session owner also owns PreparedImage, PreparedContext and JointWorking,
    capture/composition recordings, pending work, stream versions and provenance.
    Main chains borrow the joint through checked begin/commit/abort transitions.
    One typed weight binding implements synchronous initial upload, replacement
    and minimum-memory retarget. It owns candidate allocation, GPU copy work,
    validity, commit generations and failed-candidate retirement. Its separate
    prefetch object borrows the two Concept-owned shared windows, owns its transfer
    context/staging/submissions, and uses an owned semaphore handoff before commit.
- `reactor_concept_model_block_abi.c` and `reactor_concept_model_block.cpp`
  - C11 packet/view transport and C++23 shader-package adaptation. All thirteen
    NoiseRefiner, eleven ContextRefiner and thirteen MainTransformer production
    pipelines belong to Concept. Session packets project borrowed buffers and
    Concept-owned versions; C does not independently mutate stream validity.
    C stages caller bytes and projects binding packets; Concept authorizes
    publication. Admission, model-output bookkeeping and public packet transport
    remain migration seams. Async transfer and the compute handoff are Concept
    operations; native callbacks retain the common host queue lock. See
    [Concept M11](../../../docs/internal/prometheus_concept_prefetch_m11.md),
    [Concept M10](../../../docs/internal/prometheus_concept_weights_m10.md) for
    synchronous weight binding and recovery, and
    [M9](../../../docs/internal/prometheus_concept_session_m9.md) for
    stream ownership, cross-owner retirement and failure qualification, and
    [M8](../../../docs/internal/prometheus_concept_main_m8.md) for Main numerics,
    timing, attention routes and remaining research boundaries.
- `reactor_vulkan_model_block.c`
  - the remaining model/session policy and execution paths. Closed-block and
    production NoiseRefiner, ContextRefiner and MainTransformer execution delegate
    to Concept, including session stream capture/composition. C retains
    model/session admission, descriptor packet adaptation and explicit audit
    transport. Both synchronous and prefetch uploads delegate to Concept. The three standalone rebind entry points and
    the closed retarget sequence share the Concept synchronous binding protocol;
    the old C synchronous upload recorder and candidate owner have been removed.
    Pipeline interfaces are checked against the selected shader-package variant,
    while the small native shader registry supplies selector metadata.
- `reactor_main_experiments.inc`
  - the former Main C factory/recorder, isolated behind explicit M5b/M6a research
    build flags. It is not compiled into the default production path and cannot
    act as a runtime fallback. These experiments retain separate qualification.
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
- SGEMM policy and allocation metadata stay in `reactor_sgemm.cpp`; physical
  recording/submission ownership lives in `PrometheusSgemm.concept`.
- Both native FFT precisions require GPU completion. The bounded binary64
  forward path requires enabled `shaderFloat64`; Oct's ordinary builtin owns
  measured CPU/GPU selection. Strict `PROMETHEUS` calls never use CPU fallback.
- Fused reduction is row-wise and bounded; it does not own arbitrary tensor
  axes, a graph executor, normalization policy, or SGEMM selection.

## Native build entrypoints

Stage the source-controlled shader portfolio as a runtime package alongside
the native binaries (including M12's FP64 FFT shader):

```powershell
go run ./cmd/oct sdslv package build --manifest internal/prometheus/native/shaders/manifest.json --repo . --out out/prometheus/native/shaders --ids internal/prometheus/native/reactor_shader_ids.generated.h
```

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
