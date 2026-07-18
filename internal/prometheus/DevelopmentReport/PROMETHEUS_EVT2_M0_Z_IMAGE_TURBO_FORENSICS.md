# Prometheus EVT-2 M0 — Z-Image-Turbo forensics and 8 GB execution plan

## Decision

Convergence outcome: **MEANINGFUL PROGRESSION**  
Milestone state: **IN PROGRESS**  
EVT-2 state: **READY WITH REQUIRED ADAPTATIONS**

The denoiser already on the desktop is an exact, verified Comfy single-file
conversion of Z-Image-Turbo. Its identity, tensor inventory, weight layout,
official topology, M1 block, memory envelope, asset gaps, and Prometheus gap
are now frozen. The only acceptance seam still open is an actual pinned
reference capture: the existing Comfy environment has the converted assets but
not Diffusers or the original Diffusers shards/tokenizer. M0 deliberately did
not download a second 24.6 GiB transformer merely to duplicate verified local
weights.

The required next action is narrow: establish the pinned reference environment,
then capture the requested fixed prompt/seed boundaries (including
`noise_refiner.0` input/output) before any Prometheus Z-Image operator work.

## Immutable identity and local assets

The inspected file is classified as **official Comfy-Org single-file
conversion**, not as a bare filename match. Its SHA-256 exactly matches the LFS
OID at pinned Comfy revision
[`9b4ad77b1aade74887689de16d9b95b959ce2d23`](https://huggingface.co/Comfy-Org/z_image_turbo/tree/9b4ad77b1aade74887689de16d9b95b959ce2d23):

| Asset role | committed/redacted location | bytes | SHA-256 | status |
|---|---:|---:|---|---|
| denoiser | `local-model-cache/z_image_turbo_bf16.safetensors` | 12,309,866,400 | `2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6` | exact Comfy conversion |
| Qwen3 text encoder | `local-model-cache/qwen_3_4b.safetensors` | 8,044,982,048 | `6c671498573ac2f7a5501502ccce8d2b08ea6ca2f661c458e708f36b36edfc5a` | exact Comfy companion |
| VAE | `local-model-cache/ae.safetensors` | 335,304,388 | `afc8e28272cd15db3919bacdb6918ce9c1ed22e96cb12c4d5ed0fba823529e38` | exact Comfy companion |
| optional LoRA | `local-model-cache/pixel_art_style_z_image_turbo.safetensors` | 170,128,328 | `09b1b45ceed0202929bca528e51b50208d0160f4e9d2ba0f42cb7e739a43577f` | present; excluded |
| optional ControlNet patch | `local-model-cache/Z-Image-Turbo-Fun-Controlnet-Union.safetensors` | 3,101,572,408 | `86c085c0d7853f12ce5183499934b54d08371c60f549c5a6b20615cd23989388` | present; excluded |

No absolute user path is committed. The local source path used at inspection is
the user-supplied ComfyUI diffusion-model location. Its observed modification
time was 2025-12-12T15:05:42Z. The saved local Comfy workflow selects the first
three assets above, a Lumina2/Qwen3 loader, `ModelSamplingAuraFlow(shift=3)`,
and no LoRA in the intended base path; its separate optional LoRA node is not
part of EVT-2.

The safetensors file contains 453 tensors, all BF16, with 12,309,817,472 data
bytes and a 48,920-byte header (`e36743d8e80c9cefedb334d77c51a3ad97c03743d9144b8c301868995e2385e5`).
The 30 main blocks are 361,820,672 bytes each; each of the two context-refiner
blocks is 353,925,632 bytes; each noise-refiner block is 361,820,672 bytes.
The largest tensors are the 34 packed `*.attention.qkv.weight` tensors,
each `[11520,3840]` / 88,473,600 bytes. The complete machine-readable record,
including offsets, ranks, per-tensor byte ranges, semantic ownership and
required transforms, is [z_image_turbo_forensics.json](artifacts/Evt2M0/z_image_turbo_forensics.json).

## Authority pin

The model authority is [Tongyi-MAI/Z-Image-Turbo at
`f332072aa78be7aecdf3ee76d5c247082da564a6`](https://huggingface.co/Tongyi-MAI/Z-Image-Turbo/tree/f332072aa78be7aecdf3ee76d5c247082da564a6),
Apache-2.0. Its `model_index.json` pins `ZImagePipeline`,
`ZImageTransformer2DModel`, Qwen3, Qwen2Tokenizer, `AutoencoderKL`, and
`FlowMatchEulerDiscreteScheduler`; its config declares 30 layers, two of each
refiner, width 3840, 30 Q/K/V heads, 16 latent channels, patch 2, and BF16
text-encoder storage. The official project source is
[Tongyi-MAI/Z-Image at `26f23eda626ffadda020b04ff79488e1d72004cd`](https://github.com/Tongyi-MAI/Z-Image/tree/26f23eda626ffadda020b04ff79488e1d72004cd),
also Apache-2.0.

The official model card calls for `diffusers` 0.36-era Z-Image support, BF16,
zero guidance, and `num_inference_steps=9` for eight DiT forwards. This is a
real authority discrepancy worth preserving: the pinned standalone source
configuration names default steps 8, while the pinned model card documents 9
to obtain eight evaluations. EVT-2 freezes the model-card invocation
(`steps=9`, expected eight evaluations), not an inferred default.

Official original assets not locally present are the model-index/config files,
tokenizer files and original three-shard Diffusers transformer. The original
transformer LFS shards total 24,619,690,888 bytes (9,973,693,184 +
9,973,714,824 + 4,672,282,880); they must not be downloaded alongside the
verified 12.31 GB Comfy conversion unless the reversible converter is first
audited. The local Qwen3/VAE conversion is enough for Comfy reference work but
not an asserted byte-for-byte replacement for those original Diffusers shards.

## Exact topology

At 512×512 the pipeline makes FP32 noise `[1,16,64,64]`, adds a frame axis,
and patchifies with `(F,H,W)=(1,64,64)`, patch `(1,2,2)`, producing 1024
image tokens of 64 channels before `x_embedder: 64 -> 3840`. Tokens are padded
to a multiple of 32 (none at 1024). The text encoder is Qwen3 (36 layers,
2560 hidden) and the denoiser consumes hidden state `-2`; the official source
normally truncates to 512 text tokens and removes masked positions before
the denoiser.

The execution graph is exact, not merely “a DiT”:

```text
FP32 timestep -> times 1000 -> sinusoidal 256 (FP32) -> 256x1024 SiLU -> 1024x256
image patch -> x_embedder -> image pad token -> 2× noise_refiner (modulated)
text hidden[-2] -> 2560 RMSNorm -> cap_embedder -> text pad token -> 2× context_refiner
concat(image tokens, then text tokens) -> 30× main modulated transformer blocks
-> retain image prefix -> final LayerNorm(eps=1e-6) * (1 + SiLU(t) linear) -> 3840x64
-> unpatchify -> model output `[16,1,64,64]` -> negate/squeeze -> Euler update
```

Every transformer block has 30 heads × 128 dimensions, non-causal attention,
Q/K RMSNorm (`eps=1e-5`), then three-axis RoPE. RoPE uses dimensions
`[32,48,48]`, lengths `[1536,512,512]`, theta 256; it computes complex FP32
pairs and casts the rotated result back to the input dtype. Attention is
scaled by the backend’s standard `1/sqrt(128)` SDPA scale. QKV is full
multi-head, not GQA (`n_kv_heads=30`).

The non-modulated context refiner does
`x += RMSNorm(attention(RMSNorm(x)))`, then
`x += RMSNorm(W2(SiLU(W1(RMSNorm(x))) * W3(...)))`. Noise and main blocks add
AdaLN: `Linear(256,4*3840)` is split into attention-scale, attention-gate,
MLP-scale and MLP-gate; gates are `tanh`, scales are `1+value`. The residual
order is exactly attention first, then FFN. FFN width is 10,240, no linear
biases; stored QKV is a Comfy fused `[11520,3840]` matrix, whereas the
official source exposes three bias-free projections. The final layer is the
only LayerNorm (mean subtraction) and has a biased `[64,3840]` projection.

## Reference and precision authority

The frozen intended oracle invocation is prompt `A lighthouse in fog at dawn`,
seed 42, 512×512, model-card `num_inference_steps=9`, guidance 0.0, and no
negative-prompt branch. The existing Comfy venv smoke found Python 3.12,
PyTorch 2.9.1+cu130, CUDA 13.0, Transformers 4.57.3, Safetensors 0.5.2 and
RTX 3070 BF16 capability; Diffusers was not installed. It did not run the
model. The exact blocker and required capture set are committed in
[reference_manifest.json](artifacts/Evt2M0/reference_manifest.json).

The first practical policy is **BF16 source -> FP16 transformed cache/GPU
storage, with FP32 accumulation for norms, RoPE, softmax and scheduler**. RTX
3070 reports BF16 support through the existing PyTorch environment, but this
is not enough to select BF16 Prometheus storage: current Prometheus paths are
F16/F32 and M49 has unresolved reduced-precision stage evidence. Do not
weaken tolerances or select a numerical tolerance before the same-input block
capture exists.

## Prometheus fit and M1

M40b/M42-M47 provide linear contraction, stable softmax, FP32 RMSNorm,
residual ownership and SiLU-gated FFN. They are not silently “Z-Image
support”: M43 is fixed to exactly eight heads and has no mask/RoPE/AdaLN
contract. The first mandatory addition is therefore a fixed 30-head-by-128
attention plan, plus its QKV split/layout contract; then small fixed maps for
three-axis RoPE and AdaLN/tanh. The full mapping is in
[operator_inventory.json](artifacts/Evt2M0/operator_inventory.json).

M1 is exactly `noise_refiner.0`, the first complete real-weight block executed
after the image patch embedding. It requires `[1,1024,3840]` image input,
`[1,256]` timestep conditioning, 13 tensors / 361,820,672 bytes, all named
and raw-hashed in [m1_block_contract.json](artifacts/Evt2M0/m1_block_contract.json).
It is not an isolated Q projection. Acceptance is real checkpoint weights plus
the captured reference block input producing its captured output; internal
stages must localize the first discrepancy.

## 8 GB and cache plan

The conservative planned fused M1 block peak is 874,512,384 bytes; even the
unfused score/probability accounting is 1,126,164,480 bytes. The product plan
reserves 2.5 GB driver/OS margin and remains roughly 3.87 GB below the 7.2 GiB
hard target. It keeps only the current transformed block, activations and one
reusable workset on the GPU; no next-block prefetch is needed for M1. Host RAM
soft ceiling is 24 GiB, no text encoder/denoiser/VAE residency overlaps, and
the observed 192.14 GB free disk accommodates the source (12.31 GB), one
transformed cache (at most 12.31 GB), bounded oracle cap (8 GiB) and small
conversion temporaries well below the 100 GiB project ceiling. See
[memory_budget.json](artifacts/Evt2M0/memory_budget.json).

Use `OCT_EVT2_CACHE` (for example `%LOCALAPPDATA%\\Oct\\evt2-z-image-turbo`) with:

```text
source/                 immutable user-obtained original/checkpoint references
layers/<source-hash>/   atomically written FP16 per-layer transforms + manifest
oracle/<model-rev>/     untracked reference captures and projections
tmp/                    per-tensor/block temp files only; cap 384 MiB
images/                 generated PNGs
reports/                local run reports
```

Each cache entry must record source file hash, source tensor range/hash,
transform version, destination layout/dtype/count and checksum; write `*.tmp`,
fsync/close, validate, then rename. Resume by validating those identities.
Cleanup is `Remove-Item -LiteralPath "$env:OCT_EVT2_CACHE\\tmp" -Recurse` only
after resolving that explicit cache path. Never create a second whole-checkpoint
temporary file. The tool invocation is:

```powershell
go run ./tools/zimage_forensics -checkpoint $env:OCT_ZIMAGE_CHECKPOINT -display-path 'local-model-cache/z_image_turbo_bf16.safetensors' -out internal/prometheus/DevelopmentReport/artifacts/Evt2M0/z_image_turbo_forensics.json
```

## Reader, licensing, and distribution

M0 adds a dependency-free Go safetensors inspector. It performs bounded header
read, JSON duplicate-key rejection, integer checked shape/range arithmetic,
unsupported-dtype rejection, file-range and overlap validation, deterministic
name ordering and path-redacted output. It does not load weights to GPU or
duplicate the payload. Eventual runtime loading should use the same validation
then direct per-layer reads/mapping on Windows/Linux.

Both upstream model and official code declare Apache-2.0. This records license
metadata, not legal advice: transformed-weight redistribution and whether a
cache is a redistributable derivative remain owner-review questions. Product
default is therefore importer/cache tooling plus immutable manifests only;
users download original weights from official authority. Do not bundle any
weights in a Prometheus installer.

## Baseline and validation

The current Prometheus baseline is the existing M42-M49 experimental
device-resident transformer sequence, including Stage 0/controller,
quarantine/reap, replay identities, warm-zero-allocation checks and shader
manifest authority. EVT-2 must preserve M49’s stage-local numerical audit
discipline: capture a reused buffer before the next block overwrites it and do
not turn a tolerance change into closure. M0 adds no GPU operator and no
runtime behavior change.

At repository revision `cb2b80c38d83a469dadbf16ced4509a6c780f09c`, the current
native executable was run: 429 tests, 394 pass, 32 skipped, 3 failed and 13
assertion failures. The failures are
`PrometheusM39bReductionRegistryIsProductionOwnedAndIsolated`,
`PrometheusReduction_PlannerCoversRequiredBoundariesDeterministically`, and
`PrometheusShaderRegistryIdsAreUnique` (the latter reported seven rather than
five M39b production reduction assets). `PrometheusVulkanRuntimePreflight`
skipped because Vulkan initialization failed. Therefore no current RTX Stage 0,
M42-M48, fault/quarantine/reap, warm allocation, shader-hash or replay refresh
is claimed; historical artifact identities are listed in
[prometheus_baseline.json](artifacts/Evt2M0/prometheus_baseline.json).

Permanent Go tests cover a valid deterministic header manifest, malformed
headers, duplicate tensor names, range overlap, out-of-range bytes,
unsupported dtypes, model-identity mismatch, display-path redaction and
Z-Image layer grouping/transform classification. The inspection command was
run twice with byte-identical JSON output (`F4F0B6771E2FE5B509DBB61B644DF91B71BB92040EFEABD02B96E7CC1D7D356C`).
The bounded cache identity fields are specified in the cache plan and must be
validated by the eventual importer; the repository-wide large-file scan was
clean.
# M0.5 continuation (2026-07-18)

The installed ComfyUI path has now supplied a bitwise-repeatable real
`noise_refiner.0` boundary oracle at the frozen 512x512 prompt/seed request.
M0 remains open only for the deterministic FP16 cache/drift evidence and the
monolithic validation-enabled RTX baseline contradiction documented in
`PROMETHEUS_EVT2_M05_Z_IMAGE_REFERENCE_AND_BASELINE.md`.
