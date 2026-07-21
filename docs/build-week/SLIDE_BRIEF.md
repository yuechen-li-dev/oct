# Slide-deck source brief

Use the existing local lighthouse PNG only: C:\Users\yuech\AppData\Local\oct\evt2-z-image-turbo\shipping_smoke\zimage_turbo_prometheus_seed42.png; SHA-256 7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613.

## 1. Prometheus
**On slide:** “12.31 GB transformer. 8 GiB GPU. One 512x512 image in 165.051 s.” Headline left, lighthouse right.
**Notes:** Prometheus is a compiled transformer execution system, not merely a shader experiment. This is one deterministic image generation, including all nine evaluations.
**Evidence:** [M5B promotion JSON](../../internal/prometheus/DevelopmentReport/artifacts/Dvt2M5bBuiltinTopology/dvt2_m5b_builtin_promotion_evidence.json).

## 2. From experiment to complete execution
**On slide:** Before: runtime + prototype SGEMM. Now: complete Z-Image-Turbo transformer, manifests, native descriptors, SDSL-V routes, evidence.
**Notes:** Credit the runtime foundation as pre-existing; Build Week integrated the complete heavy transformer and production evidence path.
**Evidence:** [scope](BUILD_WEEK_SCOPE.md), [shipping JSON](../../internal/prometheus/DevelopmentReport/artifacts/Evt2Shipping/zimage_python_smoke.json).

## 3. The memory constraint
**On slide:** “453 BF16 tensors / 12,309,817,472 bytes” over “RTX 3070 / 8 GiB VRAM.”
**Arrangement:** two boxes and an arrow labelled “cannot all reside together.”
**Notes:** Official full BF16 checkpoint, not INT8/INT4/GGUF.
**Evidence:** [forensics](../../internal/prometheus/DevelopmentReport/PROMETHEUS_EVT2_M0_Z_IMAGE_TURBO_FORENSICS.md).

## 4. Compiled bounded-streaming architecture
**On slide:** Manifest -> native descriptors + SDSL-V shaders -> active GPU window / prefetched next window. Footer: FP32 activations persist; immutable weights remain in system memory.
**Notes:** Prefetch: 1,005,407,748 model-owned GPU bytes. MinimumMemory: 643,587,076 bytes. These are VRAM ceilings, not total RAM.
**Evidence:** [M2](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_M2_DOUBLE_BUFFERED_PREFETCH.md).

## 5. Complete execution proof
**On slide:** 2 noise + 2 context refiners; 30 main layers x 9 evaluations = 270 main-layer executions. Include prompt, seed 42, PNG hash.
**Notes:** One deterministic 512x512 RGB lighthouse; 306 transformer-block executions. Python surrounds the pipeline; Prometheus owns the complete heavy transformer.
**Evidence:** [shipping JSON](../../internal/prometheus/DevelopmentReport/artifacts/Evt2Shipping/zimage_python_smoke.json).

## 6. Measured progression
**On slide:** 263.091 s -> 165.051 s; 98.040 seconds removed / 37.3% observed improvement. Add “one complete image each; not an average.”
**Notes:** M4 reached 165.439 seconds. Prefetch overlapped 8.434 of 8.488 transfer seconds.
**Evidence:** [M4](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_M4_OBVIOUS_SHADER_OPTIMIZATION.md), [M5B JSON](../../internal/prometheus/DevelopmentReport/artifacts/Dvt2M5bBuiltinTopology/dvt2_m5b_builtin_promotion_evidence.json).

## 7. Build Week systems work
**On slide:** Four tiles: OctMake; SDSL-V graphics; Vulkan 1.4; Oct O0–O19 laboratory.
**Notes:** SDSL-V adds typed compute/vertex/pixel source through semantics, VD-MIR, HLSL, DXC, and validated SPIR-V. Vulkan 1.4 is production policy; DXC highest spelling is vulkan1.3.
**Evidence:** [graphics](../../internal/prometheus/DevelopmentReport/SDSL_V_M41_CANONICAL_FULL_LANGUAGE_IMPLEMENTATION.md), [Mx5](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_MX5_VULKAN14_MIGRATION.md).

## 8. Direct tensor-hardware feasibility
**On slide:** Layer-0 W1/W3: 440.529 ms -> 371.525 ms; 15.66% median; 19/20 wins.
**Notes:** Real layer boundary, not full image. Deterministic, finite, completed-layer relative L2 4.65441e-6. Explicit FastMixedPrecision; CanonicalFP32 remains default.
**Evidence:** [M6A](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_M6A_COOPERATIVE_MATRIX_FEASIBILITY.md).

## 9. Human-directed agentic engineering
**On slide:** Owner -> persistent ChatGPT -> fresh Codex -> tests/reports/artifacts -> owner acceptance.
**Notes:** Owner made architecture and acceptance decisions; ChatGPT held review context; Codex implemented bounded work and evidence.

## 10. Why it matters / next
**On slide:** “Treat the model as a compiled program—not as a blob that must fit in VRAM.”
**Notes:** Next is reusable compiled reactors and separately authorized FastMixedPrecision work. Do not show unmeasured 140–146 s projections.

