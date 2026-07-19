# Prometheus EVT-2 Python image smoke

## Outcome

The EVT-2 shipping smoke succeeded through the real strangler path:

```text
fixed prompt
  -> ComfyUI Qwen tokenizer/encoder
  -> source-owned embedding and timestep modules
  -> Prometheus NoiseRefiner 0..1
  -> Prometheus ContextRefiner 0..1
  -> Prometheus MainTransformer layers 0..29
  -> source-owned final AdaLN/linear and unpatchify
  -> ComfyUI res_multistep/simple scheduler
  -> ComfyUI VAE
  -> RGB PNG
```

The output is a valid, non-constant, 512×512 RGB PNG showing a recognizable
illuminated lighthouse in fog. It is soft and has blocky/smudged detail, but it
has no corruption, invalid channel order, constant-output failure, or broken
PNG encoding.

| Result | Value |
|---|---|
| PNG | `C:\Users\yuech\AppData\Local\oct\evt2-z-image-turbo\shipping_smoke\zimage_turbo_prometheus_seed42.png` |
| PNG SHA-256 | `7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613` |
| PNG bytes | 262,126 |
| Wall time | 263.0914698 s |
| Denoising time | 237.1695135 s |
| Model evaluations | 9 |
| Model-owned allocation ceiling | 643,587,076 bytes |

The PNG remains a local payload artifact and is not committed. The deterministic
run record is
`internal/prometheus/DevelopmentReport/artifacts/Evt2Shipping/zimage_python_smoke.json`.

## Frozen request and authority

| Setting | Value |
|---|---|
| Prompt | `A lighthouse in fog at dawn` |
| Negative prompt | not evaluated; CFG=1 selects the positive branch |
| Seed | 42 |
| Size | 512×512 |
| Sampler | `res_multistep` |
| Scheduler | `simple` |
| Configured steps / actual evaluations | 9 / 9 |
| Sigmas | `1.0, 0.9600432515, 0.9131456017, 0.8573263884, 0.7897727489, 0.7063492537, 0.6007193923, 0.4626556337, 0.2745098174, 0.0` |
| AuraFlow shift | 3.0 |
| Model | `Tongyi-MAI/Z-Image-Turbo@f332072aa78be7aecdf3ee76d5c247082da564a6` |
| Source | `Tongyi-MAI/Z-Image@26f23eda626ffadda020b04ff79488e1d72004cd` |
| Checkpoint SHA-256 | `2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6` |
| Qwen SHA-256 | `6c671498573ac2f7a5501502ccce8d2b08ea6ca2f661c458e708f36b36edfc5a` |
| VAE SHA-256 | `afc8e28272cd15db3919bacdb6918ce9c1ed22e96cb12c4d5ed0fba823529e38` |
| Lock SHA-256 | `71ef202b4e34b562bd0d8526d1e0c674640cbba02fb7c484d8dadf981c8b226e` |

## Exact interception boundary

The pinned upstream forward path performs patch/caption/timestep embedding,
executes the two image refiners, two context refiners, joins the streams, runs
30 main layers, splits out the image region, applies `FinalLayer`, and
unpatchifies. The bridge replaces only both refiner families and the 30-layer
main body.

The installed ComfyUI source currently assembles context before image, while
the pinned source and accepted Prometheus compiled contract are image before
context. The adapter follows the pinned source/compiled lock and supplies the
two streams separately; Python never constructs the internal joint topology.
The returned stream contains image tokens only, so the source-owned final
AdaLN/linear consumes only the image region. This is equivalent to applying the
same tokenwise projection to the joint tensor and discarding projected context.

### Python to native

| Tensor | Exact contract |
|---|---|
| Patchified image | BF16, `[1,1024,3840]`, C-contiguous token-major, 7,864,320 bytes |
| Context | FP32, `[1,32,3840]`, C-contiguous token-major, 491,520 bytes |
| Timestep/AdaLN input | BF16, `[1,256]`, C-contiguous, 512 bytes |

At 512×512, the trusted Python patch embedder maps `[1,16,64,64]` latent input
with patch size 2 to 1,024 tokens. Qwen produces `[1,15,2560]` FP32 prompt
features. The source-owned caption embedder maps them to width 3,840 and the
learned pad token extends them to 32 tokens. The source-owned timestep embedder
uses `(1 - model_timestep) * 1000` and returns the 256-wide AdaLN input.

### Native to Python

Prometheus validates the full FP32 joint output `[1,1056,3840]` for finiteness,
then exposes only its leading image region as FP32 `[1,1024,3840]` (15,728,640
bytes). Python converts that region to the model boundary dtype and calls the
trusted final layer:

- LayerNorm, width 3,840, epsilon `1e-6`, no affine parameters;
- SiLU plus learned AdaLN scale projection `256 -> 3840`;
- learned linear projection `3840 -> 64`;
- patch-size-2 unpatchify to `[1,16,64,64]`;
- sign inversion used by the installed ComfyUI model wrapper.

Across all nine evaluations, the external final projection and unpatchify took
0.043722 s total. It is therefore source-faithful and negligible relative to
156.454331 s of recorded native model execution.

The trusted scheduler accepts one `[1,16,64,64]` model prediction for each
denoising evaluation and updates an FP32 latent of the same shape. The VAE input
contract is contiguous FP32 `[1,16,64,64]`; ComfyUI's trusted VAE produces a
`[1,512,512,3]` image tensor for PNG conversion.

## Closed C ABI

The stable declaration is
`tools/prometheus_zimage_bridge/prometheus_zimage_bridge.h`. It exposes only:

- ABI version;
- session create from reactor DLL, compiled-model lock, validated payload root,
  and `-1`/`0` device selection;
- one batch-one Z-Image model evaluation with the fixed tensors above;
- deterministic session destruction;
- sized error-text retrieval.

Session creation verifies the exact lock SHA-256, resolves all 34 packages from
the lock, validates every manifest, size, path containment, and payload SHA-256,
then creates one Vulkan runtime and one compiled-model session. Python cannot
provide layer IDs, block topology, shader IDs, cache identities, internal ABIs,
or transition order.

Each `evaluate()` serially enforces this native lifecycle:

1. reuse or prepare the 32-token context stream;
2. create the evaluation owner at `noise_refiner.0`, execute and rebind to 1;
3. capture PreparedImage and destroy the refiner owner;
4. compose lock-owned image-first JointWorking;
5. create the main owner at layer 0, execute, and immediately rebind/execute
   layers 1 through 29;
6. validate/read back the final boundary, destroy only the final owner, return
   image tokens.

The Vulkan runtime/device, compiled-model session, fixed device-local session
streams, validated payload index, and lock authority survive all scheduler
steps. Context is prepared once and reused for evaluations 2 through 9. The
accepted transition contract makes model-block pipelines and model-block
buffers owner-scoped; they are recreated with the per-evaluation owner rather
than retained illegally past layer 29. Measured evaluation wall times were
25.918–27.255 s (mean 26.290 s), so this lifecycle did not become a
catastrophic scheduler-loop regression for the smoke. A warm resource split is
the next performance action, not a correctness prerequisite for this image.

## Boundary and numerical evidence

The new smoke boundary records these first-evaluation identities:

| Boundary | SHA-256 | Check |
|---|---|---|
| Qwen prompt `[1,15,2560]` FP32 | `f6e4a2842dbbdfa7e983fb8260ab15a9e0ea1f763a6615fcaf170ec8cb8838bd` | bit-exact to frozen oracle |
| Image embedding `[1,1024,3840]` BF16 | `857cea75e69d665c43779c9bc860796e76ac8b78c5c70882e02a04940e78fded` | bit-exact to frozen oracle positive row |
| Context embedding `[1,32,3840]` FP32 | `d8d00f082d1b91a348e1f3b1e241acd71777e430f402757132115a7906f589d2` | finite, exact shape/layout |
| Full-run timestep `[1,256]` BF16 | `bc0ba90e94f5ae98779c6f7c44e7d1346f8aa6aa1cc048f62a748d96076823b2` | source check for sigma 1 / embedded time 0 |
| Prometheus image output `[1,1024,3840]` FP32 | `ed89f93b028a54e53e3ee902d05ce823d1ea6f165e628341201c4c0111b2fa22` | finite, exact shape/layout |

The older isolated `last_step=1` oracle capture has timestep-row SHA-256
`d88bf2be...99b`, not the full-run hash above. The smoke hash was independently
reproduced from the installed source-owned `TimestepEmbedder` for the first
full-run sigma. This partial-step invocation discrepancy is recorded in
`boundary_validation.json`; it did not produce an invalid image and is not
evidence against an accepted internal stage.

The previously accepted independent final CUDA boundary remains the native
numerical authority:

| Metric | Value |
|---|---:|
| Joint SHA-256 | `1015ca4b43537d5fdaa6720ca8bb7ec6882e0b260c08d4a33172266daabe2893` |
| Joint relative L2 | `1.02005e-5` |
| Image relative L2 | `1.19522e-5` |
| Context relative L2 | `4.2375e-6` |
| Threshold | `5e-5` |

Every smoke evaluation reported exactly 30 main layers and allocation evidence
`361,820,672 + 234,579,972 + 47,186,432 = 643,587,076` bytes.

## Python components retained

- installed ComfyUI tokenizer and Qwen3 text encoder;
- Z-Image `t_embedder`, `x_embedder`, `cap_embedder`, and learned pad tokens;
- Z-Image final LayerNorm/AdaLN/linear and unpatchify;
- ComfyUI `res_multistep` sampler, `simple` sigma scheduler, and AuraFlow shift;
- ComfyUI `ae.safetensors` VAE decoder;
- Pillow PNG writing and verification.

No tokenizer, Qwen, VAE, convolution reactor, generic Python ML framework, or
generic native tensor/operator ABI was added.

## Reproduction

From the repository root in PowerShell:

```powershell
cmd /c internal\prometheus\native\build_windows.cmd
go build -buildmode=c-shared -o out\prometheus\python_bridge\prometheus_zimage_bridge.dll ./tools/prometheus_zimage_bridge
& "$env:USERPROFILE\ComfyUI\.venv\Scripts\python.exe" tools\zimage_prometheus_smoke.py
```

The defaults use the installed ComfyUI source/data roots and
`%LOCALAPPDATA%\Oct\evt2-z-image-turbo`. Every path can be overridden by the
script's command-line options. Model payloads and the generated PNG remain
outside Git.

## Deferred work and next shipping action

The native 29-stage audit wiring and exhaustive 30-layer lifecycle matrix are
honestly deferred as M2D hardening. They were not blockers for this image and
were not resumed before the smoke succeeded.

The next shipping action is to preserve the accepted lifecycle while splitting
owner identity from safely reusable pipeline/allocation resources, then rerun
this exact smoke and demonstrate lower per-evaluation wall time without changing
the PNG or boundary identities. A minimal ComfyUI node remains optional and was
not added because the standalone path already provides the demo.
