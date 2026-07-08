# P13 DVT-2 - RTX 3070 Full Validation Run + Assumption Audit

## 1. Environment fingerprint

- Date: 2026-04-29
- OS: Windows 10.0.26200.8246
- Compiler/toolchain:
  - `internal\prometheus\native\build_windows.cmd`
  - MSVC toolset path: `C:\Program Files\Microsoft Visual Studio\18\Community\VC\Tools\MSVC\14.50.35717`
  - `cl.exe /Bv`: compiler `19.50.35729.0`, linker `14.50.35729.0`
- Vulkan SDK: `C:\VulkanSDK\1.4.341.1`
- Vulkan loader instance version from `vulkaninfo --summary`: `1.4.341`
- Validation GPU: `NVIDIA GeForce RTX 3070`
- Vendor ID / device ID: `0x10de / 0x2488`
- NVIDIA Windows display driver: `32.0.15.9636`
- Vulkan driver version: `596.36`
- Vulkan API version on RTX 3070: `1.4.329`
- Queue families from `vulkaninfo --json=0`:
  - family `0`: `16` queues, `GRAPHICS + COMPUTE + TRANSFER + SPARSE`, timestamp valid bits `64`
  - family `1`: `2` queues, `TRANSFER + SPARSE`, timestamp valid bits `64`
  - family `2`: `8` queues, `COMPUTE + TRANSFER + SPARSE`, timestamp valid bits `64`
  - family `3`: `1` queue, `TRANSFER + SPARSE + VIDEO_DECODE`, timestamp valid bits `32`
  - family `4`: `1` queue, `TRANSFER + SPARSE + VIDEO_ENCODE`, timestamp valid bits `32`
  - family `5`: `1` queue, `TRANSFER + SPARSE + OPTICAL_FLOW`, timestamp valid bits `64`
- Runtime-selected queue families from DVT artifact:
  - compute queue family: `0`
  - transfer queue family: `1`
- Timestamp support:
  - runtime `timestamp_available = 1`
  - runtime `timestamp_valid_bits = 64`
  - runtime `timestamp_period_ns = 1`
- FP16 / subgroup / storage capability exposure from `vulkaninfo --json=0`:
  - `storageBuffer16BitAccess = true`
  - `uniformAndStorageBuffer16BitAccess = true`
  - `shaderFloat16 = true`
  - subgroup size `32`
  - subgroup quad operations in all stages `true`
- Loader observations:
  - `vulkaninfo` still reports third-party overlay-layer warnings (`GalaxyOverlayVkLayer*`, `VK_LAYER_OBS_HOOK`)
  - these warnings did not block build or validation runs

Evidence files generated during DVT-2:

- `VP_VULKANINFO_NVIDIA_GeForce_RTX_3070_596_36_0_0.json`
- `out/test-artifacts/P13_M5_DVT2_Rtx3070ValidationArtifact/p13_dvt2_rtx3070_validation.txt`

## 2. Sanity baseline results

Required commands run from repo root:

```bat
internal\prometheus\native\build_windows.cmd
out\prometheus\native\marionette_tests.exe
out\prometheus\native\marionette_slow_tests.exe
out\prometheus\native\marionette_benchmarks.exe
out\prometheus\native\marionette_tests.exe PrometheusReactor_Sgemm
```

Observed results on the final DVT-2 tree:

- `build_windows.cmd`: success
- `marionette_tests.exe`: `166` tests, `162` passed, `4` skipped, `0` failed
- `marionette_slow_tests.exe`: `3` tests, `3` passed, `0` failed
- `marionette_benchmarks.exe`: `15` tests, `15` passed, `0` failed
- `marionette_tests.exe PrometheusReactor_Sgemm`: `9` tests, `9` passed, `0` failed

Default-suite skips remained the expected FP16-selection-dependent checks plus the harness example skip:

- `PrometheusReactor_BufferReuseSafety_BaselineThenFP16SameShape`
- `PrometheusReactor_BufferReuseSafety_FP16ThenBaselineSameShape`
- `PrometheusReactor_BufferReuseSafety_FP16ThenPacked4SameShape`
- `SmokeFactCanBeSkipped`

Blocker classification: none. DVT-2 was not blocked by build or environment breakage.

## 3. Occupancy variant correctness matrix

Source: `out/test-artifacts/P13_M5_DVT2_Rtx3070ValidationArtifact/p13_dvt2_rtx3070_validation.txt`

- All `60/60` variant-shape observations passed CPU-oracle correctness on real RTX 3070 hardware.
- Aggregate worst-case error across the full matrix:
  - `max_abs_error = 0`
  - `max_rel_error = 0`
- Concrete shape set used:
  - `1x1x1`
  - `1xN-small = 1x17x9`
  - `Mx1-small = 19x1x7`
  - `3x7x5`
  - `15x17x11`
  - `8x8x9`
  - `16x16x17`
  - `64x64x65`
  - `wide-short-small = 4x19x7`
  - `tall-skinny-small = 21x5x11`
  - `K-heavy-small = 7x9x33`
  - `ml-ffn-like-small = 128x344x128`

| Variant | Shape | Correct | Max abs | Max rel | Executed | Path | Fallback | Timestamp | Confidence | DVT candidate | Notes |
| --- | --- | --- | ---: | ---: | --- | --- | --- | --- | --- | --- | --- |
| baseline-scalar | 1x1x1 | pass | 0 | 0 | baseline-scalar | baseline | none | 1/1 | high | observed-pass | selector reported small-register-tile |
| baseline-scalar | 1xN-small | pass | 0 | 0 | baseline-scalar | baseline | none | 1/1 | high | observed-pass | selector reported small-register-tile |
| baseline-scalar | Mx1-small | pass | 0 | 0 | baseline-scalar | baseline | none | 1/1 | high | observed-pass | selector reported small-register-tile |
| baseline-scalar | 3x7x5 | pass | 0 | 0 | baseline-scalar | baseline | none | 1/1 | high | observed-pass | selector reported small-register-tile |
| baseline-scalar | 15x17x11 | pass | 0 | 0 | baseline-scalar | baseline | none | 1/1 | high | observed-pass | selector reported small-register-tile |
| baseline-scalar | 8x8x9 | pass | 0 | 0 | baseline-scalar | baseline | none | 1/1 | high | observed-pass | selector reported small-register-tile |
| baseline-scalar | 16x16x17 | pass | 0 | 0 | baseline-scalar | baseline | none | 1/1 | high | observed-pass | selector reported small-register-tile |
| baseline-scalar | 64x64x65 | pass | 0 | 0 | baseline-scalar | baseline | none | 1/1 | high | observed-pass | selector reported small-register-tile |
| baseline-scalar | wide-short-small | pass | 0 | 0 | baseline-scalar | baseline | none | 1/1 | high | observed-pass | selector reported small-register-tile |
| baseline-scalar | tall-skinny-small | pass | 0 | 0 | baseline-scalar | baseline | none | 1/1 | high | observed-pass | selector reported small-register-tile |
| baseline-scalar | K-heavy-small | pass | 0 | 0 | baseline-scalar | baseline | none | 1/1 | high | observed-pass | selector reported small-register-tile |
| baseline-scalar | ml-ffn-like-small | pass | 0 | 0 | baseline-scalar | baseline | none | 1/1 | high | observed-pass | selector reported small-register-tile |
| memory-conservative | 1x1x1 | pass | 0 | 0 | memory-conservative | baseline | mc_baseline_strict_alias | 1/1 | high | observed-pass | MC alias on baseline path |
| memory-conservative | 1xN-small | pass | 0 | 0 | memory-conservative | baseline | mc_baseline_strict_alias | 1/1 | high | observed-pass | MC alias on baseline path |
| memory-conservative | Mx1-small | pass | 0 | 0 | memory-conservative | baseline | mc_baseline_strict_alias | 1/1 | high | observed-pass | MC alias on baseline path |
| memory-conservative | 3x7x5 | pass | 0 | 0 | memory-conservative | baseline | mc_baseline_strict_alias | 1/1 | high | observed-pass | MC alias on baseline path |
| memory-conservative | 15x17x11 | pass | 0 | 0 | memory-conservative | baseline | mc_baseline_strict_alias | 1/1 | high | observed-pass | MC alias on baseline path |
| memory-conservative | 8x8x9 | pass | 0 | 0 | memory-conservative | baseline | mc_baseline_strict_alias | 1/1 | high | observed-pass | MC alias on baseline path |
| memory-conservative | 16x16x17 | pass | 0 | 0 | memory-conservative | baseline | mc_baseline_strict_alias | 1/1 | high | observed-pass | MC alias on baseline path |
| memory-conservative | 64x64x65 | pass | 0 | 0 | memory-conservative | baseline | mc_baseline_strict_alias | 1/1 | high | observed-pass | MC alias on baseline path |
| memory-conservative | wide-short-small | pass | 0 | 0 | memory-conservative | baseline | mc_baseline_strict_alias | 1/1 | high | observed-pass | MC alias on baseline path |
| memory-conservative | tall-skinny-small | pass | 0 | 0 | memory-conservative | baseline | mc_baseline_strict_alias | 1/1 | high | observed-pass | MC alias on baseline path |
| memory-conservative | K-heavy-small | pass | 0 | 0 | memory-conservative | baseline | mc_baseline_strict_alias | 1/1 | high | observed-pass | MC alias on baseline path |
| memory-conservative | ml-ffn-like-small | pass | 0 | 0 | memory-conservative | baseline | mc_baseline_strict_alias | 1/1 | high | observed-pass | MC alias on baseline path |
| small-register-tile | 1x1x1 | pass | 0 | 0 | small-register-tile | srt_2accum_k | none | 1/1 | high | observed-pass | real benchmark seam path |
| small-register-tile | 1xN-small | pass | 0 | 0 | small-register-tile | srt_2accum_k | none | 1/1 | high | observed-pass | real benchmark seam path |
| small-register-tile | Mx1-small | pass | 0 | 0 | small-register-tile | srt_2accum_k | none | 1/1 | high | observed-pass | real benchmark seam path |
| small-register-tile | 3x7x5 | pass | 0 | 0 | small-register-tile | srt_2accum_k | none | 1/1 | high | observed-pass | real benchmark seam path |
| small-register-tile | 15x17x11 | pass | 0 | 0 | small-register-tile | srt_2accum_k | none | 1/1 | high | observed-pass | real benchmark seam path |
| small-register-tile | 8x8x9 | pass | 0 | 0 | small-register-tile | srt_2accum_k | none | 1/1 | high | observed-pass | real benchmark seam path |
| small-register-tile | 16x16x17 | pass | 0 | 0 | small-register-tile | srt_2accum_k | none | 1/1 | high | observed-pass | real benchmark seam path |
| small-register-tile | 64x64x65 | pass | 0 | 0 | small-register-tile | srt_2accum_k | none | 1/1 | high | observed-pass | real benchmark seam path |
| small-register-tile | wide-short-small | pass | 0 | 0 | small-register-tile | srt_2accum_k | none | 1/1 | high | observed-pass | real benchmark seam path |
| small-register-tile | tall-skinny-small | pass | 0 | 0 | small-register-tile | srt_2accum_k | none | 1/1 | high | observed-pass | real benchmark seam path |
| small-register-tile | K-heavy-small | pass | 0 | 0 | small-register-tile | srt_2accum_k | none | 1/1 | high | observed-pass | real benchmark seam path |
| small-register-tile | ml-ffn-like-small | pass | 0 | 0 | small-register-tile | srt_2accum_k | none | 1/1 | high | observed-pass | real benchmark seam path |
| balanced-2x2-accum4 | 1x1x1 | pass | 0 | 0 | balanced-2x2-accum4 | b2x2_row_major_biased | none | 1/1 | high | observed-pass | real benchmark seam path |
| balanced-2x2-accum4 | 1xN-small | pass | 0 | 0 | balanced-2x2-accum4 | b2x2_row_major_biased | none | 1/1 | high | observed-pass | real benchmark seam path |
| balanced-2x2-accum4 | Mx1-small | pass | 0 | 0 | balanced-2x2-accum4 | b2x2_row_major_biased | none | 1/1 | high | observed-pass | real benchmark seam path |
| balanced-2x2-accum4 | 3x7x5 | pass | 0 | 0 | balanced-2x2-accum4 | b2x2_row_major_biased | none | 1/1 | high | observed-pass | real benchmark seam path |
| balanced-2x2-accum4 | 15x17x11 | pass | 0 | 0 | balanced-2x2-accum4 | b2x2_row_major_biased | none | 1/1 | high | observed-pass | real benchmark seam path |
| balanced-2x2-accum4 | 8x8x9 | pass | 0 | 0 | balanced-2x2-accum4 | b2x2_row_major_biased | none | 1/1 | high | observed-pass | real benchmark seam path |
| balanced-2x2-accum4 | 16x16x17 | pass | 0 | 0 | balanced-2x2-accum4 | b2x2_row_major_biased | none | 1/1 | high | observed-pass | real benchmark seam path |
| balanced-2x2-accum4 | 64x64x65 | pass | 0 | 0 | balanced-2x2-accum4 | b2x2_row_major_biased | none | 1/1 | high | observed-pass | real benchmark seam path |
| balanced-2x2-accum4 | wide-short-small | pass | 0 | 0 | balanced-2x2-accum4 | b2x2_row_major_biased | none | 1/1 | high | observed-pass | real benchmark seam path |
| balanced-2x2-accum4 | tall-skinny-small | pass | 0 | 0 | balanced-2x2-accum4 | b2x2_row_major_biased | none | 1/1 | high | observed-pass | real benchmark seam path |
| balanced-2x2-accum4 | K-heavy-small | pass | 0 | 0 | balanced-2x2-accum4 | b2x2_row_major_biased | none | 1/1 | high | observed-pass | real benchmark seam path |
| balanced-2x2-accum4 | ml-ffn-like-small | pass | 0 | 0 | balanced-2x2-accum4 | b2x2_row_major_biased | none | 1/1 | high | observed-pass | real benchmark seam path |
| aggressive-4x4-accum8 | 1x1x1 | pass | 0 | 0 | aggressive-4x4-accum8 | a2x4_row_biased_accum8 | none | 1/1 | high | observed-pass | real benchmark seam path |
| aggressive-4x4-accum8 | 1xN-small | pass | 0 | 0 | aggressive-4x4-accum8 | a2x4_row_biased_accum8 | none | 1/1 | high | observed-pass | real benchmark seam path |
| aggressive-4x4-accum8 | Mx1-small | pass | 0 | 0 | aggressive-4x4-accum8 | a2x4_row_biased_accum8 | none | 1/1 | high | observed-pass | real benchmark seam path |
| aggressive-4x4-accum8 | 3x7x5 | pass | 0 | 0 | aggressive-4x4-accum8 | a2x4_row_biased_accum8 | none | 1/1 | high | observed-pass | real benchmark seam path |
| aggressive-4x4-accum8 | 15x17x11 | pass | 0 | 0 | aggressive-4x4-accum8 | a2x4_row_biased_accum8 | none | 1/1 | high | observed-pass | real benchmark seam path |
| aggressive-4x4-accum8 | 8x8x9 | pass | 0 | 0 | aggressive-4x4-accum8 | a2x4_row_biased_accum8 | none | 1/1 | high | observed-pass | real benchmark seam path |
| aggressive-4x4-accum8 | 16x16x17 | pass | 0 | 0 | aggressive-4x4-accum8 | a2x4_row_biased_accum8 | none | 1/1 | high | observed-pass | real benchmark seam path |
| aggressive-4x4-accum8 | 64x64x65 | pass | 0 | 0 | aggressive-4x4-accum8 | a2x4_row_biased_accum8 | none | 1/1 | high | observed-pass | real benchmark seam path |
| aggressive-4x4-accum8 | wide-short-small | pass | 0 | 0 | aggressive-4x4-accum8 | a2x4_row_biased_accum8 | none | 1/1 | high | observed-pass | real benchmark seam path |
| aggressive-4x4-accum8 | tall-skinny-small | pass | 0 | 0 | aggressive-4x4-accum8 | a2x4_row_biased_accum8 | none | 1/1 | high | observed-pass | real benchmark seam path |
| aggressive-4x4-accum8 | K-heavy-small | pass | 0 | 0 | aggressive-4x4-accum8 | a2x4_row_biased_accum8 | none | 1/1 | high | observed-pass | real benchmark seam path |
| aggressive-4x4-accum8 | ml-ffn-like-small | pass | 0 | 0 | aggressive-4x4-accum8 | a2x4_row_biased_accum8 | none | 1/1 | high | observed-pass | real benchmark seam path |

## 4. Diagnostics truth matrix

Lifecycle truth was consistent across every observation for each variant family.

| Variant | Cases | Executed variant | Path id | Fallback | benchmark_enabled | dvt_validated | pvt_validated | production_eligible | dispatch_enabled |
| --- | ---: | --- | --- | --- | ---: | ---: | ---: | ---: | ---: |
| aggressive-4x4-accum8 | 12 | aggressive-4x4-accum8 | a2x4_row_biased_accum8 | none | 1 | 0 | 0 | 0 | 0 |
| balanced-2x2-accum4 | 12 | balanced-2x2-accum4 | b2x2_row_major_biased | none | 1 | 0 | 0 | 0 | 0 |
| baseline-scalar | 12 | baseline-scalar | baseline | none | 1 | 1 | 1 | 1 | 1 |
| memory-conservative | 12 | memory-conservative | baseline | mc_baseline_strict_alias | 1 | 0 | 0 | 0 | 0 |
| small-register-tile | 12 | small-register-tile | srt_2accum_k | none | 1 | 0 | 0 | 0 | 0 |

Additional truth observed:

- `requested_variant` identity remained preserved in diagnostics for all `60/60` observations.
- `executed_variant` identity remained preserved in diagnostics for all `60/60` observations.
- `memory-conservative` correctly reported:
  - executed variant `memory-conservative`
  - path id `baseline`
  - fallback `mc_baseline_strict_alias`
- SRT / B2x2 / A2x4 all reported real wired non-baseline path IDs with `fallback_reason = none`.
- Production dispatch remained unchanged and disabled for all non-baseline variants.

## 5. Timestamp / timing plumbing sanity

This section is observational only. No tuning conclusions were drawn.

Observed across the full `60`-observation DVT artifact:

- `timestamp_available = 1` for all observations
- `timestamp_valid = 1` for all observations
- `timestamp_failure_reason = none` for all observations
- `timing_source = vulkan_timestamp_query` for all observations
- `timing_confidence = high` for all observations
- `gpu_duration_ns_mean > 0` for all observations
- runtime timestamp metadata stayed constant:
  - valid bits `64`
  - period `1 ns`

Representative GPU mean-duration ranges by variant family:

- `baseline-scalar`: `13184 .. 135440 ns`
- `memory-conservative`: `13616 .. 135216 ns`
- `small-register-tile`: `13504 .. 192544 ns`
- `balanced-2x2-accum4`: `13760 .. 273632 ns`
- `aggressive-4x4-accum8`: `13296 .. 337504 ns`

Important DVT-2 conclusion:

- Real RTX 3070 timing plumbing is truthful and usable.
- CPU wall-clock fallback was not needed on the DVT occupancy matrix.
- This is a correctness-of-instrumentation result, not a performance-ranking result.

## 6. Async / transfer / queue validation

Hardware-sensitive coverage stayed green after the DVT artifact addition.

Default lane pass coverage included:

- `PrometheusReactor_AsyncDeferredCompletionIsExplicitlyObservable`
- `PrometheusReactor_AsyncFailureRemainsVisibleUntilExplicitAbandon`
- `PrometheusReactor_AsyncInFlightOwnershipAndAbandonmentAreSafe`
- `PrometheusReactor_AsyncUseBeforeCompleteAndDoubleConsumeAreRejected`
- `PrometheusReactor_M31_TransferQueueFallback_NoDedicatedQueue`
- `PrometheusReactor_M31_TransferQueueFallback_PseudoSharedQueue`
- `PrometheusReactor_M31_TransferQueuePath_EnabledWhenDedicatedAndLarge`
- `PrometheusReactor_M31_TransferQueueAsyncReadinessWaitsForTransferAndCompute`
- `PrometheusReactor_M31_TransferSubmitFailureMarksSlotFailure`
- packed4 correctness / fallback / tail coverage tests
- `PrometheusReactor_SgemmReuseHandlesShapeAndBufferChangesCorrectly`

Extended slow-lane queue and churn run:

```bat
out\prometheus\native\marionette_slow_tests.exe PrometheusReactor_P13_M10_,PrometheusReactor_P13_M11_,PrometheusReactor_P11_M16_,PrometheusReactor_P11_M17_,PrometheusReactor_P11_M20_
```

Observed result:

- `28` tests run
- `27` passed
- `1` skipped
- `0` failed

The single skip remained explicit and non-fatal:

- `PrometheusReactor_P11_M20_LaneSlotLifecycleAdvancesToInFlightOrComplete`
  - reason: available-path requirement not met in current backend configuration

Queue-specific conclusions:

- Real RTX 3070 runtime sees a dedicated transfer queue (`compute family 0`, `transfer family 1`).
- The explicit M31 / M16 / M17 / M20 transfer and multi-queue tests still pass on hardware.
- The custom occupancy DVT matrix itself never used the transfer queue:
  - `transfer_queue_used = 0` across all `60` occupancy observations
  - `dedicated_transfer_available = 1` across all `60` occupancy observations
  - `transfer_fallback_reason = 5` (`PROM_TRANSFER_FALLBACK_REQUIRED`) across all `60` occupancy observations
- Interpretation:
  - dedicated transfer availability on RTX 3070 is real
  - but the small DVT occupancy characterization shapes do not force transfer-queue use
  - queue/transfer correctness therefore comes from the explicit hardware-facing queue tests, not from the occupancy matrix alone

## 7. Lease / controller invariant validation

Lease and controller invariants remained stable on hardware.

Direct coverage:

- `PrometheusReactor_P13_M10_ResourceLease_SingleSgemmGrantYieldSmoke`
- `PrometheusReactor_P13_M10_ResourceLease_BatchGrantYieldSmoke`
- `PrometheusReactor_P13_M10_ResourceLease_BatchFailedSlotDenied`
- `PrometheusReactor_P13_M10_ResourceLease_BatchInvalidatedSlotDenied`
- `PrometheusReactor_P13_M10_ResourceLease_BatchUnsafeRuntimeDenied`
- `PrometheusReactor_P13_M10_ResourceLease_BatchOutstandingCapBlocksLookahead`
- `PrometheusReactor_P13_M11_ResourceLease_DiagnosticsCoherentOnDeny`
- `PrometheusReactor_P13_M11_ResourceLease_RepeatRunCountersStable`
- `PrometheusJudgmentEngine_P13_M14_SingleCallModeSkipsContentionBackpressureButKeepsHardGates`
- `PrometheusJudgmentEngine_P13_M14_ResourceLeaseDecisionIsPureFromFacts`

Observed hardware truth:

- no lease-deny regressions were surfaced in the single-call occupancy DVT matrix
- DVT artifact per-observation snapshot ranges:
  - lease request count: `3 .. 180`
  - lease grant count: `3 .. 180`
  - lease yield count: `3 .. 180`
  - lease deny count: `0 .. 0`
  - outstanding depth snapshot: `1 .. 2`

Interpretation:

- request / grant / yield counters remained coherent
- no deny without explicit deny diagnostics was observed
- no double-yield regression surfaced
- no stale invalidation / churn regression was surfaced in the buffer-reuse and batch-churn lanes
- outstanding depth return-to-safe behavior is validated by the dedicated lease/batch tests; the per-observation DVT artifact captures active-call snapshots, not final post-run drain state

M14 / bug-class conclusion:

- No new evidence of the M14 stale-invalidation / singleton-backpressure bug class appeared on the RTX 3070 path.

## 8. DVT artifact summary

Primary DVT artifact:

- `out/test-artifacts/P13_M5_DVT2_Rtx3070ValidationArtifact/p13_dvt2_rtx3070_validation.txt`

Artifact summary:

- schema: `prometheus.sgemm.occupancy_dvt2.rtx3070.v1`
- observation count: `60`
- variants covered: `5`
- shapes covered: `12`
- correctness failures: `0`
- diagnostics truth mismatches: `0`
- timestamp-invalid observations: `0`
- CPU fallback timing observations: `0`

Artifact-level takeaways:

- all requested benchmark variants are materially working on real NVIDIA silicon
- lifecycle diagnostics are truthful enough to support DVT closeout evidence
- timing fields on hardware are not simulated in this matrix
- selector output and executed benchmark path are distinct truths and both remain visible

## 9. EVT assumption audit

### 9.1 Which EVT assumptions held on RTX 3070?

- The Windows native build path remains healthy.
- The benchmark dispatch seam works on real NVIDIA hardware.
- Baseline, MC alias, SRT, B2x2, and A2x4 all execute correctly against the CPU oracle on hardware.
- Lifecycle diagnostics (`benchmark_enabled`, `dvt_validated`, `pvt_validated`, `production_eligible`, `dispatch_enabled`) are truthful on hardware.
- Timestamp plumbing is real and valid on RTX 3070, not just a simulated software-lane concept.
- Dedicated transfer queue presence detected in earlier work remains true on the final DVT-2 tree.
- Async / cleanup / batch / lease tests fixed in DVT-1 remained stable in DVT-2.

### 9.2 Which EVT assumptions were false or incomplete?

- `vulkaninfo --summary` alone was incomplete for DVT closeout.
  - Full fingerprinting needed `vulkaninfo --json=0` plus runtime diagnostics to capture queue-family details, timestamp valid bits, subgroup shape, and FP16 exposure.
- "Dedicated transfer queue available" did not imply "transfer queue will be used by these occupancy DVT shapes."
  - All `60` occupancy artifact observations stayed on compute-only execution despite transfer availability.
- "Selector recommendation" and "executed benchmark variant" are separate truths.
  - On these custom DVT shapes, the selector consistently reported `small-register-tile`, while the benchmark seam still executed the explicitly requested variant correctly.
- FP16 device support and FP16 runtime selection are also separate truths.
  - The RTX 3070 exposes FP16-related Vulkan features, but the default FP16 transition tests still skip because the runtime did not select FP16 on this validation path.

### 9.3 Which software Vulkan / llvmpipe behaviors differed from RTX 3070?

- Real RTX 3070 produced valid GPU timestamps with high confidence across the whole DVT matrix.
- Real RTX 3070 exposed a real dedicated transfer queue topology (`0 -> compute`, `1 -> transfer`).
- Real RTX 3070 validated execution of the real non-baseline shader paths rather than just software-lane correctness/fallback behavior.
- Real RTX 3070 made the selector behavior visible under a real NVIDIA profile:
  - for the tested small shapes, selector output favored `small-register-tile`
  - software-only lanes did not settle the "what does the selector recommend on real 3070 hardware?" question

### 9.4 Which skips / fallbacks changed?

- Occupancy DVT matrix:
  - no correctness skips
  - no timestamp fallbacks
  - no diagnostics mismatches
  - only the intentional MC alias fallback remained
- Default suite:
  - FP16-transition-dependent skips remain
- Extended slow suite:
  - one existing available-path skip remains explicit

### 9.5 Any driver / device-specific observations?

- NVIDIA proprietary driver `596.36` and Windows display driver `32.0.15.9636` behaved cleanly for this DVT scope.
- Queue-family and timestamp metadata on this device were stable and truthful.
- Overlay-layer warnings are still present in the environment but were operationally benign for DVT-2.

### 9.6 Any issues that should be deferred to PVT?

- Larger-shape and broader-shape characterization beyond the bounded DVT matrix.
- Cross-device / cloud / multi-GPU validation of the same variant family.
- FP16-selection-specific transition coverage once the runtime actually selects FP16 on a representative hardware path.
- Any production policy or performance ranking decisions.

### 9.7 Any blockers before DVT closeout?

- No DVT-closeout blocker remains.

## 10. Issues found and fixes applied

Runtime correctness issues found in DVT-2:

- none

Diagnostics mismatches found in DVT-2:

- none

Timing plumbing issues found in DVT-2:

- none

Validation-harness issue found in DVT-2:

- Before this run, the repo had smoke-level occupancy artifacts but no single artifact that captured the full RTX 3070 DVT matrix requested by this milestone.

Fix applied:

- added `P13_M5_DVT2_Rtx3070ValidationArtifact` to `internal/prometheus/native/Marionette/reactor_p13_m4_occupancy_benchmark_tests.cpp`
- scope of the addition:
  - no runtime shader, controller, recipe, or production dispatch changes
  - no lifecycle-flag promotion changes
  - only bounded evidence collection for DVT-2

Failure-triage classification for this issue:

- test expectation / evidence-collection gap

## 11. Deferred PVT items

- Do not enable non-baseline production dispatch yet.
- Do not mark non-baseline variants `pvt_validated`.
- Do not tune recipes or ranking based on the GPU duration values collected here.
- Carry forward:
  - larger-shape occupancy characterization
  - broader transfer-queue usage characterization
  - FP16-selection transition coverage when the runtime actually selects FP16
  - multi-device / cloud validation

## 12. Readiness recommendation

State: **Success**

Recommendation:

- Close P13 DVT-2 as successful on the local Windows RTX 3070 machine.
- Historical DVT-2 recommendation at the time was to keep the then-current promotion seam unchanged:
  - baseline remained production-eligible and dispatch-enabled
  - non-baseline occupancy variants remained benchmark-only
- Px16 M3 later superseded that lifecycle gate:
  - wired occupancy variants are EVT-dispatchable once they have a real path/pipeline
  - DVT/PVT lifecycle fields remain telemetry rather than dispatch permission
  - `occupancy_apply_safety_clamp` remains the real engineering safety gate
- Proceed to PVT planning with the following explicit truths in hand:
  - real RTX 3070 hardware correctness is validated for the occupancy variant family
  - lifecycle diagnostics are truthful on hardware
  - timestamp plumbing is truthful on hardware
  - queue / async / lease fixes from DVT-1 remained stable in DVT-2
  - selector recommendation, benchmark override, and transfer-queue usage must be treated as separate concerns in later phases
