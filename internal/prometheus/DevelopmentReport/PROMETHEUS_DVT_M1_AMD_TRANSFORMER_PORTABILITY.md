# PROMETHEUS DVT M1 — AMD transformer portability

Status: **COMPLETE**. Convergence: **SUCCESS**. DVT state: **COMPLETE**.

This record is deliberately an execution report, not an AMD feature claim based solely on extension bits.

## Direct hardware facts

The test machine is an ASUS Adolbook 14 M5406UA_ADOL14UA with an AMD Ryzen 9 8945H, 32 GiB RAM, Windows 11 Pro build 26200, and balanced power plan. The only Vulkan device is the integrated AMD Radeon 780M (vendor `0x1002`, device `0x1900`), AMD proprietary 26.6.4 (LLPC), Vulkan API 1.4.349 via loader 1.4.350. The Windows display driver is 32.0.31021.5001.

The direct Vulkan profile reports a compute queue with 64 timestamp-valid bits, a 10 ns timestamp period, 1024 maximum compute invocations, 32 KiB shared memory, 4 GiB storage-buffer range, and a 128-byte non-coherent atom. It exposes FP16, synchronization2, timeline semaphores, memory budget, and `VK_KHR_cooperative_matrix`. It is an integrated/unified-memory device; allocation/migration measurements remain pending.

The critical fact is a configurable subgroup range 32–64 with an effective subgroup size of **64**. This is not an NVIDIA-like warp32 device.

## Execution evidence

The Windows native reactor was rebuilt with Go 1.26.5 and MSVC 19.51. Before the full suite relink completed, the new hardware runner passed:

- native smoke;
- Vulkan reduction sum/max/stable-softmax correctness;
- M48 fixed-stack topology, CPU oracle, live fixed-stack, replay, and capacity tests;
- M49a matched FP32 A2x4 FFN suffix (L2 `3.57628e-07`, Linf `1.19209e-07`, GPU `243440 ns`, warm allocations `0`);
- M49a projection and RMSNorm hardware proofs.

The first failing path was cooperative matrix execution. Validation reported `VUID-VkPipelineShaderStageCreateInfo-module-08987`: the checked-in subgroup-scoped cooperative shader has `LocalSize.x = 32`, which is not a multiple of AMD's effective subgroup size 64. The extension and a useful tuple therefore do **not** make this shader executable.

`reactor_vulkan_sgemm.c` now classifies that exact contract as `COMPILER_ROUTE_UNAVAILABLE` while retaining independently useful FP16 features. The transformer uses its established fallback rather than attempting to make AMD imitate warp32. The matching hardware test now records this as a skip after checking the admission state.

## Current classification

| Path | Result |
| --- | --- |
| FP32 A2x4 / FFN / projection / RMSNorm | Supported |
| Row reduction and stable softmax | Supported |
| Fixed four-block stack | Supported by existing live-stack test |
| Cooperative FP16 shader | Unsupported by current LocalSize-32 shader contract |
| Conventional FP16 | Observed in M49a; full AMD qualification pending |

Controller authority is reset: **Unidentified**, rollout stage **0**, canary required, and RTX calibration has no AMD authority.

## Remaining gate

The amended Windows native build completed. Validation-enabled AMD hardware proofs now pass for M42 attention, M43 grouped attention, M44 projection, M45 residual, M46 RMSNorm, M47 complete block, and the M48 fixed four-block stack. M42 fault injection, M43 quarantine/replacement, M44 lifecycle faults, and M49b bounded-controller tests also pass. The unsupported cooperative proof skips with the explicit fixed-LocalSize-32 reason.

The machine-readable projection was generated twice with identical SHA-256: `5353196e2581b1c8c52fe13a227a66b93aa366ddb48fdd04fb68f80d5aeba3ca`.

AMD timestamps are valid at a 10 ns period. Very short individual stages may quantize to zero; aggregate attention and complete-block GPU intervals are nonzero, so tests require aggregate timing rather than incorrectly treating zero tick deltas as failed execution.

The broad SDSL-V lane still has deterministic-artifact checks that fail because installed DXC 1.10.5347 differs from checked-in provenance, plus one stale conformance hash. This is toolchain/provenance drift, not an AMD runtime failure, and was not regenerated during DVT.

Image-model readiness: **READY WITH REDUCED SHAPE**. The exercised fallback path is the existing bounded hardware corpus rather than the unexecuted 128×1024×4096 primary shape. DVT M2 should begin with the proven fallback topology and retain Stage-0 controller authority.

## Timing closeout attempt

The RTX-scale `PrometheusM43AttentionCorpus` retained approximately 700 MiB and was not tractable on Radeon 780M. AMD timing authority is therefore the already-proven bounded reduced-shape slice, not the RTX corpus wearing an AMD hat. The overnight duration from the attempted corpus is explicitly **not trusted** because the machine may have slept.

The DVT M2 design consequence is useful and direct: Radeon 780M work must use deliberately bounded workload and memory envelopes. This is a practical capacity/latency envelope, not a correctness limitation of the validated fallback transformer path.
