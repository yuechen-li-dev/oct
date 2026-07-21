# Judge-facing submission copy

## Title and tagline

**Prometheus: compiling a 12.31 GB transformer to run on an 8 GiB GPU**

**Tagline:** A compiled Vulkan transformer system that streams an official 12.31 GB BF16 model through a bounded GPU window and generated a deterministic 512x512 image in 165.051 seconds.

## Approximately 50 words

During Build Week, Prometheus became a compiled Vulkan execution system for the complete Z-Image-Turbo transformer. Its 12.31 GB BF16 checkpoint exceeds an RTX 3070’s 8 GiB VRAM, so Prometheus streams manifest-authorized weights while keeping FP32 activations resident. It generated one deterministic 512x512 image in 165.051 seconds, including all nine model evaluations.

## Approximately 150 words

Prometheus treats a trained model as a compiled program. During Build Week, we turned an experimental Vulkan runtime into a full Z-Image-Turbo transformer execution system: two noise refiners, two context refiners, 30 main layers, and nine evaluations within one image generation. The official checkpoint has 453 BF16 tensors totaling 12,309,817,472 bytes—larger than the entire 8 GiB VRAM of our RTX 3070.

The system compiles manifest authority into native execution descriptors and SDSL-V shaders. Immutable weights stay in system memory; persistent FP32 activations stay on the GPU; one active and one prefetched weight window move only the next legal package. Model-owned GPU memory peaks at 1,005,407,748 bytes with prefetch, or 643,587,076 bytes in MinimumMemory. Measured complete-image time improved from 263.091 to 165.051 seconds while preserving the canonical lighthouse PNG hash.

## Full description

The constraint was simple: the official Z-Image-Turbo transformer checkpoint is 12.31 GB of BF16 weights, but the NVIDIA RTX 3070 has 8 GiB of VRAM. Prometheus compiles model-manifest authority into native descriptors and SDSL-V shader routes. Immutable weights stay in system memory, FP32 activations remain on the GPU, and bounded layer packages stream through an active window while Prefetch fills the next legal package.

The complete proof is a deterministic 512x512 RGB PNG for “A lighthouse in fog at dawn,” seed 42. One generation contains nine scheduler/model evaluations: two noise-refiner blocks, two context-refiner blocks, and all 30 main layers on each evaluation—270 main-layer and 306 transformer-block executions. The production Auto route generated one such image in 165.051 seconds, not nine images and not an average. The initial measured complete-image run was 263.091 seconds: an observed 98.040-second (37.3%) progression.

Build Week added complete transformer integration; bounded streaming and prefetch; critical-path measurement; tiled SGEMM reuse; production attention; OctMake typed NativePlans; canonical SDSL-V compute/vertex/pixel compilation through typed semantics, VD-MIR, HLSL, DXC, and validated SPIR-V; Vulkan 1.4 admission; and compiled Oct O0–O19 model investigation. M6A proved a real RTX 3070 tensor-hardware route at a layer-0 W1/W3 boundary: 15.66% median improvement, preserved as an explicit FastMixedPrecision experimental profile while CanonicalFP32 remains the authoritative default.

Python remains the bootstrap shell for tokenizer/encoder, scheduling, VAE, and PNG work. Prometheus executes the complete heavy transformer body.

## What changed during Build Week

- Complete transformer integration, bounded streaming, host caching, persistent ownership, and double-buffered Prefetch.
- 99.941% wall-time accounting; GPU kernels established as the main bottleneck.
- Tiled SGEMM reuse for W1/W3, QKV, and projection; M4 reached 165.439 seconds.
- Production Auto attention at shader identity 49; integration run 165.051 seconds.
- OctMake typed NativePlans, C11/C++23 targets, variants, dependencies/staleness, structured execution, diagnostics, traces, and reproducible action identities.
- SDSL-V compute/vertex/pixel typed parsing/semantics, VD-MIR, generated HLSL, DXC, validated SPIR-V, deterministic artifacts, and interface/binding provenance.
- Vulkan 1.4 production policy; DXC’s highest spelling is `vulkan1.3`, with validated SPIR-V 1.6 admitted under Vulkan 1.4 semantics.
- Compiled Oct O0–O19 facts with zero interpreted fallback.

## Human / ChatGPT / Codex collaboration

The owner was product owner, TPM, architect, acceptance authority, and engineering orchestrator: selecting the problem, priorities, constraints, milestones, production policy, and stop/go decisions. Persistent ChatGPT maintained cross-milestone context, reviewed handoffs, challenged claims, and shaped bounded tasks. Fresh Codex author turns implemented, built, tested, inspected failures, and left auditable reports and JSON evidence. Claude and Gemini are credited only where repository evidence records their contribution. This was human-directed, evidence-gated agentic engineering.

## Challenges, results, future direction

The hard problem was preserving correctness while moving a checkpoint larger than VRAM through a GPU at useful speed. The response was explicit ownership, payload identities, deterministic output, numerical authority, profiling, and reversible optimizations. Result: the observed 263.091-to-165.051-second progression with the canonical image preserved. Next: reusable compiled compute reactors and separately authorized whole-transformer/final-image FastMixedPrecision investigation.

Repository: <https://github.com/yuechen-li-dev/oct>. Evidence: [CLAIMS_EVIDENCE.md](CLAIMS_EVIDENCE.md).
