# Claims-to-evidence matrix

| Claim | Status | Authority | Safe wording / note |
| --- | --- | --- | --- |
| 453 BF16 tensors; 12,309,817,472 bytes | Architectural | [EVT2 forensics](../../internal/prometheus/DevelopmentReport/PROMETHEUS_EVT2_M0_Z_IMAGE_TURBO_FORENSICS.md) | “12.31 GB official BF16 transformer”; not quantized. |
| RTX 3070, 8 GiB | Architectural | [shipping JSON](../../internal/prometheus/DevelopmentReport/artifacts/Evt2Shipping/zimage_python_smoke.json) | One recorded Windows platform. |
| 2+2 refiners, 30 layers, 9 evaluations, 270/306 executions | Production | [shipping JSON](../../internal/prometheus/DevelopmentReport/artifacts/Evt2Shipping/zimage_python_smoke.json) | “Complete transformer across nine evaluations.” |
| 512x512 lighthouse, seed 42, canonical hash | Production | [shipping JSON](../../internal/prometheus/DevelopmentReport/artifacts/Evt2Shipping/zimage_python_smoke.json) | Local authoritative asset; recorded hash. |
| 643,587,076 / 1,005,407,748 model-owned bytes | Production | [M2 memory JSON](../../internal/prometheus/DevelopmentReport/artifacts/Dvt2M2/dvt2_m2_memory_profiles.json) | Excludes driver bookkeeping. |
| 263.091 -> 165.051 seconds | Production | [shipping JSON](../../internal/prometheus/DevelopmentReport/artifacts/Evt2Shipping/zimage_python_smoke.json), [M5B JSON](../../internal/prometheus/DevelopmentReport/artifacts/Dvt2M5bBuiltinTopology/dvt2_m5b_builtin_promotion_evidence.json) | Observed complete-image runs; not average. |
| 8.488 transfer; 8.434 overlap; ~0.051 exposed | Production | [M2 report](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_M2_DOUBLE_BUFFERED_PREFETCH.md) | Repeat trace. |
| 99.941% accounted; GPU bottleneck | Production | [M3 JSON](../../internal/prometheus/DevelopmentReport/artifacts/Dvt2M3/dvt2_m3_wall_time_accounting.json) | M3 critical-path accounting. |
| M4 165.439 and tiled contraction gains | Production | [M4](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_M4_OBVIOUS_SHADER_OPTIMIZATION.md) | Major contractions, not a universal claim. |
| Vulkan 1.4 and SPIR-V 1.6 | Production | [Mx5](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_MX5_VULKAN14_MIGRATION.md) | DXC highest spelling remains vulkan1.3. |
| SDSL-V graphics | Production | [M41](../../internal/prometheus/DevelopmentReport/SDSL_V_M41_CANONICAL_FULL_LANGUAGE_IMPLEMENTATION.md) | Compiler/toolchain; no graphics runtime. |
| M6A 440.529 vs 371.525 ms; 15.66%; 19/20 | Experimental | [M6A JSON](../../internal/prometheus/DevelopmentReport/artifacts/Dvt2M6a/m6a_layer0_evaluation_rtx3070.json) | Real layer-0 boundary, not full image. |
| M6A L2 4.65441e-6; 2,359,296,000 weight elements | Experimental | [M6A report](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_M6A_COOPERATIVE_MATRIX_FEASIBILITY.md) | FastMixedPrecision explicit; CanonicalFP32 remains default. |

