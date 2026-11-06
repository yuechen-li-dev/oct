# Concept resident model block — M5

Date: 2026-11-05

Status: **MILESTONE VERIFIED — success (A) for the bounded resident-block and
pre-attention port.** Full-project and complete O19 qualifications are not claimed.

## Ownership and execution

`internal/prometheus/reactors/PrometheusModelBlock.concept` owns the resident
block allocations, replacement candidates, six pre-attention pipelines (or the
closed identity pipeline), and its pending submission. The existing shared
`VkDevice` and queue remain owned by the runtime. Concept Vulkan supplies the
borrowed attachment, allocation/drop, push descriptors, recording, synchronization,
submission and completion machinery. The common queue-submit callback remains
the single host synchronization authority.

The C state contains borrowed resource views for the remaining attention/FFN
and shared model portfolios. The existing admission, immutable package identity,
generation bookkeeping and transaction policy remain in C. Replacement allocation
and release route through the Concept owner, including stack-held old-weight
views after a successful commit. Session stream allocations and standalone
ContextRefiner/MainTransformer owners remain later migration slices.

Pre-attention uses a fixed schedule of named `ProjectionStage` cases after BF16
ingress. `CaptureStage` maps exhaustively to a typed `CapturePlan` describing
producer boundary, source, offset and contiguous/segmented layout. `BlockProgram`
selects closed execution, pre-attention or the ingress probe. These are closed
domain choices, with no heuristic selection or additional job scheduler.

The old C closed executor, pre-attention recorder and separate BF16 recorder have
been removed. Individual prefix invocations from the legacy static-audit path
borrow its command buffer and call the same Concept stage implementation. The
legacy caller owns synchronization, submission and retirement of that borrowed
recording. Its remaining C attention/FFN dispatches are still live.

## Retirement and diagnostics

- Completion uncertainty retains the owned submission and all dependencies.
  Retry drains it before overwriting mapped input; root teardown drains it before
  freeing any view. Combined injected faults are consumed only when execution
  reaches their actual phase.
- A reset legacy fence carries no submission authority. The C remainder now
  tracks a successful pending submission separately, preventing a later Concept
  quarantine from waiting forever on an unsignaled, never-submitted legacy fence.
- Concept pushes the six prefix interfaces. A standalone NoiseRefiner owner
  therefore creates twenty persistent legacy descriptor sets instead of twenty-six;
  all twenty-six model/audit pipelines remain cold-created and reused.
- Warm replay reuses buffers and pipelines. The current Vulkan library creates
  recording/fence/query objects per submission; this milestone does not claim
  persistent command-object reuse or an end-to-end speedup.

## Foundation and package fixes

Concept Vulkan now has measured `BufferRange` ABI evidence and checked
`CopyBytes`/`ConceptVkCmdCopyRegion`. Bounds checks use subtraction to avoid
overflow, reject same-buffer overlap, and make zero-length copies no-ops. Byte
offsets and lengths preserve ordinary [Vulkan buffer-copy semantics](https://docs.vulkan.org/refpages/latest/refpages/source/vkCmdCopyBuffer.html),
including partial packed BF16 input words. Library admission failure is `-7601`,
separate from driver allocation errors. Runtime facts cover actual offset copies,
untouched sentinels, invalid ranges, overflow and overlap in Normal and Verify.

Several Z-Image rows in the native selector intentionally omit interface sizes.
Both the new adapter and the retained model pipeline factory now validate
bindings/push bytes against the immutable selected shader-package variant.
Tests reject wrong bindings, push extents, absent owners and missing variants
without opening SPIR-V artifacts. No omitted selector value is treated as a
complete interface declaration.

## GPU evidence

Windows, Vulkan SDK 1.4.350.0, RTX 3070, NVIDIA driver 610.88;
`PROMETHEUS_VK_VALIDATION=1` and `VK_LAYER_VALIDATE_SYNC=1`. The latter is the
[validation layer's synchronization setting](https://github.com/KhronosGroup/Vulkan-ValidationLayers/blob/main/docs/updating_from_VK_EXT_validation_features.md).

- All 65,536 BF16 bit patterns widen exactly. Counts 1, 3 and 65,535 also pass,
  preserving partial packed-word handling.
- An independent dyadic-weight witness runs every production prefix shader.
  It samples token boundaries and all three RoPE axes, with largest observed
  absolute error **3.93391e-6** against the host closed-form oracle.
- The legacy attention/projection/residual consumer reads Concept-owned QKV,
  weights and gates directly. The independently known full residual is exact.
  The complete C tail, thirteen-weight candidate rebind, and static audit replay
  execute; all Concept allocation/pipeline counts return to their initial values.
- Record/submit/observation failures, combined phase injections, retry, SGEMM
  coexistence and root destruction are checked through public runtime paths.
- Real checkpoint weights plus the pinned timestep reproduce all six existing
  AdaLN canonical coordinates with **zero observed error**, across three runs
  without additional buffer allocations or weight uploads.

Native logs live under ignored
`out/test-artifacts/prometheus_concept_model_block_m5/`. The final selected lane
passes **29 tests, zero skips**, with no VUID, synchronization-hazard or validation
error messages in its logs. Ingress timestamps begin after upload copies, so
those transfers are excluded from its compute interval.

## Payload authority and limits

The checkpoint is present at
`C:/Users/yuech/ComfyUI/models/diffusion_models/z_image_turbo_bf16.safetensors`.
The existing cache builder verified source SHA-256
`2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6` and rebuilt
the thirteen-weight cache with aggregate
`a1ba526898a2a7522b31167c6d5e1bc48c39a8708cf5c3ad88b193e536ca5d5e`.
This local recovery cache is ignored at `.tmp/m5-payload`.

The retained O19 `timestep_input.f32.bin` stage yields the exact BF16 timestep,
SHA-256 `bc0ba90e94f5ae98779c6f7c44e7d1346f8aa6aa1cc048f62a748d96076823b2`.
The real-weight AdaLN witness does not consume the image activation: its test
uses an explicitly zero image operand and qualifies only that canonical stage,
alongside the separate independent full-prefix witness.

The documented durable bundle is absent. A readable-files search of the user
tree and C: found no `noise_refiner_0_input.bin` or `capture.json`. Candidate
inverse reconstructions did not match the required input hash and were not
used. The unchanged full O19 replay still requires the original captured input
and complete bundle described in `docs/EVT2_LOCAL_PAYLOADS.md`. This milestone
does not substitute another activation, weaken a hash, or claim that full replay.

Linux GPU execution, experimental Vulkan-1.0 controls and actual physical
device-loss recovery are unqualified here. The frozen Zig compiler is unchanged;
both Zig suites were skipped.
Full shared-portfolio/prefetch execution remains part of later model/session
qualification; this lane exercises the standalone owner, legacy tails, rebind,
static replay and the adjacent session's cold lifecycle.

## Reproduction

```powershell
cmd /d /c internal\prometheus\native\build_windows_launcher.cmd
go run ./tools/prometheus_concept -check
go run ./tools/prometheus_native_manifest
$env:PROMETHEUS_VK_VALIDATION = '1'
$env:VK_LAYER_VALIDATE_SYNC = '1'
$env:OCT_EVT2_CACHE = '<validated cache root>'
$env:OCT_EVT2_ORACLE = '<root containing run_02/noise_refiner_0_timestep.bin>'
$env:OCT_PROMETHEUS_M5_ADALN_REAL = '1'
out/prometheus/native/marionette_tests.exe PrometheusConceptModelBlock
out/prometheus/native/marionette_tests.exe PrometheusResidentModelBlock
out/prometheus/native/marionette_tests.exe PrometheusM1BBf16IngressWidensEveryBitPatternExactlyOnGpu
out/prometheus/native/marionette_tests.exe PrometheusShaderPackage
```

From Concept, run `go test ./internal/concept -run '^TestVulkanLibraryNormalAndVerify$' -count=1`,
`go run ./cmd/concept test libraries/Vulkan --verbose`, and the same CLI command
with `--verify`. Generated C, headers, provenance and native archives remain
ignored build products. The consumer manifest and source inventory declare the
new module and adapters; no generated C or payload binary belongs in Git.

## Final closeout

- The canonical Windows launcher passes (MSVC 14.51.36231 for the common native
  target; Clang C11/C++23 for the manifest archives). Its final build artifact is
  `out/test-artifacts/prometheus_native_windows_build.log`.
- All 29 selected native tests pass with synchronization validation and no skips.
  The repaired M2C fixture supplies the current explicit `AUTO` route and also
  verifies rejection of the previously omitted zero policy (FB-116).
- Concept Vulkan's 16 facts pass in both Normal and Verify, through its package
  CLI and `TestVulkanLibraryNormalAndVerify` (strict C11/native companion and
  measured ABI included). Logs: `.tmp/m5-concept-vulkan-final.log`,
  `.tmp/m5-concept-vulkan-cli.log`, `.tmp/m5-concept-vulkan-verify.log`.
- `go test` passes for Oct `internal/prometheus`, `tools/prometheus_concept`,
  `tools/prometheus_native_manifest`, `tools/sdslv_workspace_check`, and
  `internal/prometheus/shaderpackage`. Both Concept-wide vet and the relevant
  Oct vet lane pass. The SDSL-V module corpus passes all three cases.
- Three fresh `prometheus_concept -check` generations match; native manifest
  parity, shader workspace ownership checks and both repositories' diff checks
  pass. Generated C/provenance and local recovery payloads are ignored.
- Concept's broad `go test -timeout 75m ./...` is **not green**: its native
  mixed-call Normal host hit the two-minute process deadline under concurrent
  qualification. The unchanged failed test subsequently passed alone in Normal
  and Verify (245.41 s total including build/setup). The broad failure remains
  in `.tmp/m5-concept-full.log`; its isolated rerun is
  `.tmp/m5-concept-mixed-rerun.log`. FB-117 records the remaining qualification
  scheduling/budget issue and the secondary misleading parity diagnostic.
- The unchanged full O19 preflight fails on the missing captured input:
  expected 7,864,320 bytes, found zero. Its evidence is
  `.tmp/m5-original-o19-preflight.log`. The real-weight AdaLN and independent
  full-prefix tests above are separate, explicitly bounded evidence.

FB-115 (package-owned interface admission) and FB-116 are resolved. FEEDBACK now
contains 8 open, 11 deferred, 97 resolved and 1 superseded entry, with unique IDs
through FB-117. Pre-existing uncommitted Concept compiler/scoped-cleanup work
from M3 was retained. M5 changes only the Vulkan library in that repository.
The fetched Concept `origin/main` contains the current checkout's base through
its merge commit; there are no incoming source differences in the affected
library/compiler paths.

The next implementation slice is the remaining attention/FFN execution and its
model/session ownership bookkeeping. The current borrowed C views and shared
Concept stage entry points form the handoff; another independent scheduler or
shader implementation is unnecessary.
