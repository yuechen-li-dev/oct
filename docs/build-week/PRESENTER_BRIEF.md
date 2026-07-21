# Presenter technical briefing

A transformer evaluation is one pass of the denoising model at one scheduler timestep. One image takes nine timesteps, so it has nine evaluations—not nine images. Each evaluation runs two noise refiners, two context refiners, and 30 main layers: 270 main-layer and 306 transformer-block executions.

The 12.31 GB figure is immutable BF16 weights in system RAM. They do not all need to be in model-owned VRAM. Prometheus streams a bounded current package through the GPU; Prefetch also fills the next package while the current package computes. FP32 activations persist on the GPU. Think of a library: the collection stays in the building; the desk holds the book being read and the next book arriving. Prefetch peaks at 1,005,407,748 model-owned GPU bytes; MinimumMemory uses 643,587,076.

The repeat trace had 8.488 seconds of transfer work, 8.434 seconds overlapped, and about 0.051 seconds exposed. Profiling accounted for 99.941% of wall time and showed GPU kernels dominated. SGEMM is the matrix multiplication at the heart of transformer projections; tiled SGEMM improved W1/W3, QKV, and projection by reusing data in shared memory and registers. Production attention Auto selects shader identity 49; identity 41 is the serial fallback.

BF16 and FP16 are compact formats; FP32 is more precise. The official checkpoint is not quantized. BF16-to-FP16 occurs when checkpoint-derived cache payloads are made; no claim says every instruction operates on BF16. Production activations and accumulation are FP32. M6A packs an activation to FP16 and uses cooperative matrices—tensor-style GPU matrix hardware—for W1/W3, then returns to FP32. It is an explicit FastMixedPrecision profile, never the default. It delivered a real faster layer-0 boundary, not a full-image result.

SDSL-V is the typed shader language: typed parsing/semantics -> VD-MIR -> generated HLSL -> DXC -> validated SPIR-V -> Vulkan. Vulkan 1.4 is production policy. OctMake is a typed build system for NativePlans, C11/C++23 targets, variants, dependencies, staleness, diagnostics, traces, and reproducible action identities. Oct O0–O19 was a compiled lab for exact model semantics and numerical witnesses. Manifest and route authority bind payloads and shader identities so experiments cannot silently become production.

