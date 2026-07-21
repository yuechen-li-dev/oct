# Judge Q&A

**What was built during Build Week?** Complete transformer integration, bounded streaming/prefetch, performance optimization, production attention, OctMake production work, SDSL-V graphics compilation, Vulkan 1.4 admission, compiled Oct experiments, and M6A tensor feasibility.

**What existed before?** Oct, SDSL-V compute, Prometheus Vulkan/SGEMM foundations, and the owner’s review workflow.

**Complete app or transformer?** Prometheus runs the complete heavy transformer. Python remains for tokenizer/encoder, scheduler, VAE, and PNG.

**Is it quantized?** No. It uses the official full BF16 checkpoint, not INT8, INT4, or GGUF. Checkpoint-derived cache weights are FP16; computation is accurately described as FP32 where applicable.

**How can it run on 8 GiB?** Immutable weights remain in system memory and stream through bounded GPU windows. Model-owned GPU ceiling: about 1.005 GB Prefetch or 644 MB MinimumMemory.

**Is 165 seconds one image, nine images, or an average?** One measured deterministic 512x512 image generation, including all nine model evaluations; not an average.

**Why Vulkan?** It is this project’s explicit hardware-adaptive substrate. No CUDA-superiority claim is made.

**What did Oct/SDSL-V/OctMake contribute?** Oct supplied compiled semantic/numerical witnesses; SDSL-V compiles typed shaders to validated SPIR-V; OctMake gives typed, reproducible native build plans.

**Hardest problem?** Executing a full checkpoint larger than VRAM while preserving authority. Manifests, ownership, prefetch, profiling, and numerical tests solved it.

**How was correctness verified?** Payload hashes, route identities, numerical witnesses, relative-L2 thresholds, deterministic PNG hash, artifact validation, and regression tests.

**What did human versus AI do?** The owner set product, architecture, and acceptance. ChatGPT retained review context. Codex executed bounded implementation/testing/evidence tasks.

**Why M6A if not promoted?** It directly proved a 15.66% median layer-0 W1/W3 improvement with 19/20 wins and deterministic finite output. It is not a full-image claim, so FastMixedPrecision is explicit and CanonicalFP32 remains default.

**How much faster could it become?** The 140–146 second figure is only a projection and is not presented as measured.

**What comes next?** Reusable compiled reactors and separately authorized whole-transformer/final-image FastMixedPrecision work.

