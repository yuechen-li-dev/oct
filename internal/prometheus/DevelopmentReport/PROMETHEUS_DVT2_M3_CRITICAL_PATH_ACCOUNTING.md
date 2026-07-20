# Prometheus DVT-2 M3 — Critical-path accounting

## Outcome

**Convergence outcome: SUCCESS**  
**Milestone state: COMPLETE**  
**DVT-2 state: READY FOR M4**

The canonical Prefetch run completed in **196.302 s** and preserved the accepted PNG SHA-256. **99.941%** of the measured wall is in named additive buckets; `Unexplained` is **0.115 s (0.059%)**.

The primary bottleneck is **GPU kernel compute**. MainTransformer GPU timestamps account for **149.746 s (76.3% of wall)** and cover **99.93%** of host-visible MainTransformer execution. The top secondary bottleneck is startup/model loading: session creation plus boundary-model load costs **25.832 s**.

## Reconciliation

The earlier `~386 ms` and `~974 ms` figures did not cover the same work. The accepted `~386 ms` samples were NoiseRefiner0. Under matched settings, isolated MainTransformer layers.0 is **544.003 ms host / 543.705 ms GPU**. The canonical retained loop averages **554.993 ms host / 554.614 ms GPU**. The old 2.5x discrepancy is therefore a measurement-boundary mismatch, not a retained-loop controller penalty.

Inter-layer GPU gaps total **0.154 s** across 261 transitions (mean **0.592 ms**, p95 **0.814 ms**, max **1.290 ms**). They are immaterial beside kernel time.

## Critical path

Python makes exactly **9 native calls**, one per evaluation, crossing **8,356,352 bytes in** and **15,728,640 bytes out** per call; there are no crossings inside the 30-layer chain. Total native-call/readback wall is **162.032 s**. Transfer work is **8.492 s**, overlap is **8.417 s**, and exposed transfer is **0.074 s**.

GPU busy compute is **157.880 s** (MainTransformer **149.746 s**, refiners **8.134 s**). Main ingress transfer is **0.024 s**, device-to-device joint composition is **0.022 s**, and final GPU readback is **0.006 s**.

Warm evaluations perform **508 queue submissions**, **539 fence waits**, **500 command resets**, and **34 descriptor updates**. Resource-creation/destruction/allocation/map counters are zero on the repeat delta. Despite the chatty controller, Main command recording is only **0.017 s**, queue-submit CPU time **0.006 s**, and descriptor update **0.001 s** across the full image. Fence wait (**149.822 s**) overlaps GPU execution and is not additive.

Validation enabled versus disabled changed a matched evaluation by **-0.076 s (-0.42%)**, within noise. Full timing probes versus the accepted minimized M2 repeat added **0.850 s (0.43%)**.

## Top GPU stages

| Rank | Stage | Full-image GPU time | Wall share |
|---:|---|---:|---:|
| 1 | ffn_w1_w3 | 42.024 s | 21.4% |
| 2 | qkv | 41.628 s | 21.2% |
| 3 | attention | 32.317 s | 16.5% |
| 4 | w2_residual | 19.610 s | 10.0% |
| 5 | projection_residual | 13.556 s | 6.9% |

Shader 42 (`main_transformer_ffn_w1_w3.sdslv`) assigns one output element per thread and serially accumulates 3,840 channels for both W1 and W3 without subgroup cooperation or shared input tiling. This is direct evidence for the single M4 target.

## M4 handoff

If M4 replaces shader 42's scalar per-output dot products with a tiled SGEMM/reactor implementation, then FFN W1/W3 GPU time should fall from 42.024 s to 21.012 s or less, reducing canonical full-image wall time from 196.302 s toward 175.290 s, while preserving the accepted PNG hash, FP32 activation/FP16 weight policy, two-window Prefetch ceiling, and zero immutable rereads.

No M3 optimization was applied. The only bounded repair was updating the isolated M2C fixture to select the now-required MinimumMemory execution profile.

## Validation and limitations

The Windows native build, default Marionette corpus (**440 tests: 405 passed, 35 hardware-gated skips, 0 failed**), matched isolated/retained lane, canonical Prefetch smoke, bridge build, Python syntax checks, payload check, lock check, JSON parsing, large-file scan, `git diff --check`, and accepted PNG hash are green. NVIDIA telemetry was available; vendor occupancy/register counters and transfer-engine utilization were not. The accidental dual-smoke contention trace is retained outside the committed artifact set as a labeled diagnostic and excluded from baseline accounting.
