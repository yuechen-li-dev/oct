# Devpost fields

## Project name
Prometheus: compiling a 12.31 GB transformer to run on an 8 GiB GPU

## Tagline
A compiled Vulkan transformer system that streams an official 12.31 GB BF16 model through a bounded GPU window and generated one deterministic 512x512 image in 165.051 seconds.

## Category
Developer Tools

## Short description
Prometheus compiles and executes the complete Z-Image-Turbo transformer on an 8 GiB RTX 3070 even though the official 12.31 GB BF16 checkpoint exceeds VRAM. It streams manifest-authorized weights through bounded GPU windows, keeps FP32 activations resident, and generated one deterministic 512x512 image in 165.051 seconds, including all nine model evaluations.

## Long description
Use the approximately 300-word Full description in [SUBMISSION.md](SUBMISSION.md).

## Inspiration
We wanted an execution substrate that treats a trained model as a compiled program: explicit manifests, payload identities, shader routes, numerical authority, and reproducible hardware evidence instead of an opaque load-and-run call.

## What it does
Prometheus executes the complete heavy Z-Image-Turbo transformer—two noise refiners, two context refiners, and 30 main layers—across all nine evaluations of one image workflow. Python remains the bootstrap shell for the other pipeline components.

## How it was built
Model manifests compile into native descriptors and SDSL-V shader routes. Immutable BF16-source weights live in system memory; persistent FP32 activations remain on the GPU; active and prefetched windows stream only the next authorized package. SDSL-V runs through typed semantics, VD-MIR, HLSL, DXC, and validated SPIR-V; Vulkan 1.4 is production policy.

## How ChatGPT/Codex was used
The owner directed product, architecture, priorities, constraints, and acceptance. Persistent ChatGPT retained context and reviewed handoffs. Fresh Codex tasks implemented bounded work, built, tested, inspected failures, and left reports and JSON evidence. This was human-directed, evidence-gated engineering.

## Challenges
The central challenge was executing a 12.31 GB checkpoint on an 8 GiB GPU while preserving numerical and route authority. Explicit ownership, bounded prefetch, profiling, deterministic output, and regression evidence made optimization safe.

## Accomplishments
Complete transformer integration; bounded streaming/prefetch; 263.091-to-165.051-second observed full-image progression; OctMake productionization; SDSL-V graphics compilation; Vulkan 1.4 admission; compiled Oct O0–O19 investigation; and M6A direct tensor-hardware feasibility.

## What is next
Build reusable compiled reactors and separately authorize whole-transformer and final-image FastMixedPrecision work. The FastMixedPrecision projected full-image figure is not presented as measured.

## Repository URL
https://github.com/yuechen-li-dev/oct

## Testing instructions
See [CLAIMS_EVIDENCE.md](CLAIMS_EVIDENCE.md) for committed reports and machine-readable evidence. Live GPU reproduction requires the documented Windows RTX 3070 environment; this submission does not ask judges to rerun it.

## Supported platforms
Authoritative live Prometheus evidence is Windows x86-64 on one NVIDIA RTX 3070. Vulkan/toolchain portability claims remain bounded to recorded evidence.

## Known limitations
No CUDA comparison, cross-vendor DVT, graphics runtime, hosted service, or full Python-free application claim. FastMixedPrecision is experimental and explicit; CanonicalFP32 remains default.

## Video URL
[VIDEO_URL_OWNER_REQUIRED]

## Feedback Session ID
019f6cb4-b438-70e2-b91c-487d7ad45bbd
