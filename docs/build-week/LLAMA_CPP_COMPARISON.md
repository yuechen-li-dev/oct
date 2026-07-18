# llama.cpp CUDA, Vulkan, and SDSL-V Vulkan comparison

Status: **MEANINGFUL PROGRESSION** (2026-07-18 UTC). This is a benchmark-only
shader-injection evidence packet, not a claim that Prometheus is a complete
llama.cpp backend.

## Identity and environment

| Item | Value |
| --- | --- |
| llama.cpp | 86a9c79f866799eb0e7e89c03578ccfbcc5d808e |
| Oct | dff95f309f7949b024f6416b41380596df733e15 |
| GPU | NVIDIA GeForce RTX 3070, device 0, driver 596.36 |
| CUDA / Vulkan | CUDA 13.2.51, Vulkan SDK/runtime 1.4.350.0 |
| DXC / glslc | DXC 1.10 / shaderc 2026.2 |
| CPU / plan | AMD Ryzen 7 7700X, Balanced power plan |
| model | Qwen/Qwen2.5-1.5B-Instruct-GGUF at 91cad51170dc346986eccefdc2dd33a9da36ead9 |
| model SHA-256 | fc89e330deb3fd8fa560f1c0f35a1e2b8da96d59e13445559ed190307a6f5649 |
| SDSL-V shader SHA-256 | 1d50d664966549d69d6b4e1a70e29e4abb67af6b31bf6b306ab8911f806f3170 |

Three Release builds use GGML_NATIVE=OFF, full device-0 offload, F16 KV,
mmap, 8 threads, 512 batch/ubatch, and flash attention off: build-cuda,
build-vulkan-stock, and build-vulkan-sdslv. The experimental worktree is
shortened only to avoid Windows/MSBuild path length in llama.cpp's nested
shader generator.

The first candidate, Unsloth Llama 3.2 1B F16, was SHA-verified but rejected by
this pinned upstream because its GGUF lacks tokenizer merges. It was not
benchmarked. Qwen 2.5 is a public dense nominal-1.5B F16 GGUF with attention,
RMSNorm, and SwiGLU; however, llama-bench reports 1.777B parameters for this
file. It therefore does not satisfy the user's strict 1.0–1.5B final-model
constraint and is retained only as an integration diagnostic, not final model
selection.

## Feasibility and injection

| GGML candidate | Decision |
| --- | --- |
| SOFT_MAX | Rejected: production SDSL-V ABI and semantics omit GGML mask/sink/ALiBi behavior. |
| RMS_NORM ordinary FP32 | Implemented: direct pipeline replacement. |
| F16 MUL_MAT | Rejected: existing SDSL-V cooperative GEMM uses incompatible packed layout and descriptors. |
| attention/block | Rejected: no complete interface-compatible path. |

The adapter replaces only pipeline_rms_norm_f32. It uses the exact 116-byte
GGML binary push-constant layout, bindings 0 input and 2 output from llama.cpp's
global layout, one 512-lane group per row, FP32 accumulation, no temporary
buffer, and a compile-time-unrolled reduction. Fused and partial RMSNorm
families remain explicitly logged stock fallbacks.

The opt-in patch is
tools/llama_cpp_benchmark/patches/llama_cpp_86a9c79f_sdslv_rmsnorm.patch.
It logs the shader identity, every eligible fallback family, and graph-node
coverage. It applies cleanly to a fresh pinned checkout.

## Correctness

test-backend-ops test -o RMS_NORM -b Vulkan0 passed all 21 upstream FP32
cases on the SDSL-V build: widths 64 and 1025, contiguous and view layouts,
five epsilons, and in-place output. The test uses a CPU reference. Its passing
CSV mode does not emit L2, Linf, or relative-error values, so no invented
numerical metrics appear here. Model logits are not yet collected.

## Operator timing

The identical benchmark harness adds decode [2048,1,1,1], prompt
[2048,128,1,1], awkward [2053,7,3,1], and primary [2048,512,1,1] RMSNorm
cases. One densified-graph host-duration observation at decode is:

| backend | us/run | effective GB/s |
| --- | ---: | ---: |
| CUDA | 6.04 | 2.53 |
| stock Vulkan | 4.77 | 3.20 |
| SDSL-V Vulkan | 5.19 | 2.94 |

SDSL-V decode has ten samples after three warmups:
[5.32, 5.39, 5.32, 5.32, 5.32, 5.32, 5.32, 5.37, 5.32, 5.32] us.
Mean 5.332, median 5.32, sample standard deviation 0.026, minimum 5.32,
and p95 5.39 us. GPU timestamps, all three remaining distributions, and
temporary-byte dispatch accounting are still pending.

## llama-bench pp128 integration reference

Ten repetitions, Qwen F16, pp128, full GPU offload, F16 KV, batch/ubatch 512,
and flash attention off:

| build | prompt tok/s | standard deviation |
| --- | ---: | ---: |
| CUDA | 6800.73 | 1311.98 |
| stock Vulkan | 3585.03 | 49.94 |
| SDSL-V Vulkan build | 3545.69 | 51.43 |

The CUDA first sample was slow and is retained. llama-bench has its default
warmup, but three persistent process-level warmups were not achieved.

The Qwen graph produced no SDSL-V substitution or coverage log. Therefore its
SDSL-V GPU-time coverage is unmeasured and the pp128 values are integration
references only, not evidence of end-to-end SDSL-V acceleration.

## Claims and next seam

Safe claim: “A benchmark-only SDSL-V FP32 RMSNorm SPIR-V module directly
replaced llama.cpp's ordinary Vulkan RMSNorm pipeline and passed the upstream
CPU-reference operator suite; end-to-end model coverage is not yet
demonstrated.”

Do not update the video or Devpost packet. Do not claim CUDA superiority,
general Vulkan speedup, a quantized path, or a complete Prometheus backend.
The next coherent seam is identifying the actual Qwen RMSNorm pipeline family,
then collecting model logits and the full llama-bench matrix with non-zero
coverage.
