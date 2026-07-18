# Direct GGML GLSL and SDSL-V SPIR-V comparison

Status: **MEANINGFUL PROGRESSION** (RTX 3070, controlled reduction rerun).

This is a direct Vulkan shader comparison, not CUDA, llama.cpp routing, or model throughput. [Phase-one evidence](artifacts/ggml_sdslv_spirv_comparison_phase1.json) is preserved; the current [raw artifact](artifacts/ggml_sdslv_spirv_comparison.json) records source/SPIR-V hashes, commands, modules, raw samples, per-run statistics, and deterministic projections.

## Two-round feedback loop

Phase one established that the existing wide-row SDSL-V path was strong, while
also exposing a physical short-row failure. It was not discarded or overwritten:

| Phase-one shape | GGML ns | original SDSL-V ns | SDSL-V delta |
| --- | ---: | ---: | ---: |
| stable softmax 1024x8, wide row | 17,424 | 8,624 | **-50.5%** |
| row sum 64x512, short row | 6,192 | 12,624 | +103.9% |
| stable softmax 64x512, short row | 9,376 | 20,288 | +116.4% |

The human read that mixed evidence as a missing physical regime, not a reason
to add another semantic operator or reactor. The bounded packed-row plan below
is the resulting implementation change; the controlled rerun is its second
round of evidence. This is the same evidence-driven loop that produced the
earlier wide-row strategy: observe a real boundary, choose one bounded change,
then keep both the win and the loss visible.

## Provenance and controls

GGML is pinned to `9be313313c8ecb9488911bd64550190e3ed80f38`. Its public `vulkan-shaders-gen.cpp` used Vulkan SDK `glslc` with `-fshader-stage=compute --target-env=vulkan1.2 -O`; SDSL-V used canonical `oct sdslv compile-spv --validate --require-spirv-val`. Kaiju owns the same Vulkan 1.2 instance/device/queue/buffers/descriptors/timestamps for every direct dispatch.

The rerun used Windows **Balanced**, recorded RTX 3070 driver 596.36, 220 W power limit, and the visible compute-process list, then used 10 warmups, 50 GPU timestamp samples, and three alternating-family performance passes. Clocks were observed, not locked; desktop processes were present and permission prevented a complete process-memory audit. This is controlled local evidence, not a clock-locked lab claim.

## Physical finding and bounded plan

Old SDSL-V assigns one 256-lane workgroup per row. At width 64 only 64 lanes load useful values before an eight-round shared-memory sum tree; fused softmax repeats the tree for max and denominator. The old modules contain 2 row-sum and 4 softmax barrier instructions.

`RowSumPackedShort_CS` and `SoftmaxPackedShort_CS` map eight logical rows to one 256-lane workgroup as eight 32-lane partitions. Each partition has two useful elements per lane at width 64 and uses five rounds per tree, with no cross-row arithmetic or temporary allocation. `spirv-val` passes the 2,864-byte row-sum and 4,200-byte softmax modules.

| Module | GGML GLSL bytes | old SDSL-V bytes | packed SDSL-V bytes |
| --- | ---: | ---: | ---: |
| FP32 RMSNorm | 67,852 | 5,744 | n/a |
| row sum | 3,884 | 3,204 | 2,864 |
| stable softmax | 78,704 | 4,628 | 4,200 |

Size is recorded as structure/provenance, not as a performance proxy: the
packed modules are smaller and have fewer inspected opcodes, but the timing
tables—not binary size—establish the measured crossover.

One reduction semantic authority now selects a fixed physical plan before recording:

| Operation | Packed selection | Otherwise |
| --- | --- | --- |
| row sum | width <=96 and rows >=512, or width <=128 and rows >=1024 | existing wide/staged plan |
| stable unmasked softmax | width <=128 | existing wide/staged plan |

The match has stable replay identity and uses the normal production registry/pipeline owner; it is not a scheduler or a divergent runtime shader branch.

## Controlled direct results

All 228 validation runs and 684 performance runs were finite with zero Vulkan validation errors and worst CPU-oracle Linf `5.96046447753906e-8`.

| Operation and shape | GGML ns | old SDSL-V ns | packed SDSL-V ns | Packed vs GGML |
| --- | ---: | ---: | ---: | ---: |
| row sum 64x1 | 3,093 | 3,413 | 3,232 | +4.5% |
| row sum 64x512 | 3,445 | 6,400 | 3,445 | **0.0%** |
| row sum 64x1024 | 4,427 | 9,451 | 3,541 | **-20.0%** |
| stable softmax 64x1 | 4,309 | 4,181 | 3,904 | **-9.4%** |
| stable softmax 64x512 | 4,917 | 9,643 | 4,203 | **-14.5%** |
| stable softmax 64x1024 | 6,347 | 15,797 | 4,619 | **-27.2%** |
| stable softmax 1024x8, wide plan | 8,363 | 4,437 | n/a | **-46.9%** |

Packed sum still loses GGML at 64x128 (+8.8%), reaches parity at 64x512, and wins at 64x1024. Packed softmax wins every measured short-width row. The strongest packed result is softmax 16x1024 at 29.0% lower median GPU time; its largest packed loss is row-sum 64x128 at 8.8% higher. The artifact contains minimum, median, mean, sample standard deviation, p95, and raw samples for every pass.

At the key feedback shape, row-sum 64x512 improved from **12,624 ns** in phase
one to **3,445 ns** in the controlled rerun: 72.7% lower than the original
SDSL-V plan and equal to the 3,445 ns GGML median in round two. Softmax 64x512
improved from 20,288 ns to 4,203 ns, 14.5% below GGML in the rerun.

## Matmul audit: genuine ABI seam

GGML publishes FP16 `mul_mm.comp` normal and cooperative families selected by layout/type/feature/accumulation configuration. SDSL-V's available cooperative proof is an experimental KHR path with packed F16x2 `u32` inputs, FP32 output, 16x16x16 subgroup tiles, and multiples-of-16 admission. Kaiju does not negotiate `VK_KHR_cooperative_matrix` or model GGML's selected push-constant/layout ABI and preparation conversions. Forcing either side into the current harness would omit asymmetric setup, so no matmul timing is published. This is a genuine independent ABI seam, not a win/loss result.

## Safe claim and limits

> In the same Vulkan harness on RTX 3070, SDSL-V-generated SPIR-V ran plain FP32 softmax 46.9% faster than GGML's production GLSL shader at 1024x8. A direct short-row loss led to one compiler-selected packed-row plan, which reduced row-sum 64x512 from 86% slower than GGML to parity while passing the same CPU-reference and Vulkan-validation checks.

Do not claim universal GGML/Vulkan/CUDA superiority, llama.cpp replacement, model throughput, clock locking, or a fair matmul result. The prior [llama.cpp injection report](LLAMA_CPP_COMPARISON.md) remains substitution history with zero graph coverage, not performance authority.
