# EVT-2 M0.5 — Z-Image-Turbo Reference and Baseline

## Result

Convergence outcome: **MEANINGFUL PROGRESSION**  
Milestone state: **IN PROGRESS**  
EVT-2 state: **READY WITH REQUIRED ADAPTATIONS**

The local ComfyUI path is a working, pinned numerical witness.  It produced a
bitwise-repeatable full boundary oracle for the actual `noise_refiner.0` block;
it did not require Diffusers shards or altered packages.  M0.5 remains open for
only two bounded gates: a dependency-free deterministic M1 FP16 cache and a
clean monolithic validation-enabled native hardware pass.

## Reference authority

The selected path is the installed ComfyUI Lumina/Z-Image implementation,
executed with the existing Qwen3 encoder, verified BF16 Comfy checkpoint, and
`ae.safetensors` VAE.  Its reference-only environment is Python 3.12.9,
PyTorch 2.9.1+cu130, CUDA 13.0, cuDNN 91200, Transformers 4.57.3,
Safetensors 0.5.2, NVIDIA driver 596.36, and RTX 3070.

The frozen request uses prompt `A lighthouse in fog at dawn`, seed 42, 512x512,
9 scheduler steps / 8 evaluations, `res_multistep`, `simple`, AuraFlow shift
3, no LoRA and no ControlNet.  In Comfy this is `cfg=1`: the single positive
branch fast path corresponding to the frozen no-guidance Turbo contract.  A
first exploratory `cfg=0` capture was rejected because it evaluated positive
and zeroed-negative branches as batch two; it is not the M1 authority.

Selected run `run_02` produced PNG SHA-256
`f0556937c803a1706d4c0a0b13158f37c697fa0f47d57379cab81f40c5a76dc5`.
Run `run_03` is byte-identical for the PNG and every retained full tensor.

## M1 block oracle

The full BF16 input, timestep, and output are cache-relative under the
configured `OCT_EVT2_CACHE` root and are identified in
[`noise_refiner_0_oracle.json`](artifacts/Evt2M05/noise_refiner_0_oracle.json).
Their shapes are exactly `[1,1024,3840]`, `[1,256]`, and `[1,1024,3840]`.
The capture also retains bounded deterministic authorities for AdaLN, QKV,
Q/K RMSNorm, RoPE, selected attention logits/probabilities, attention output,
both residual boundaries, and all FFN stages.  The layout contract freezes
QKV order, reshape order, RoPE axes, no-mask behavior, and residual order.

No numerical tolerance has been selected: repeat variation is zero, but the
necessary FP16-transformed-weight drift experiment has not yet been executed.

## Native baseline restoration

The three recorded registry failures were stale expectations: the current
production registry owns seven reduction assets/implementations, not five.
The test now names the exact seven IDs, including `RowSumPackedShort` and
`SoftmaxPackedShort`.  Fresh native rebuild plus targeted registry, planner,
and validation-enabled required-hardware Vulkan preflight all pass.  The direct
preflight reports `caps_available=1`, `vk_result=0`, and RTX 3070 vendor 4318.

The full rebuilt suite has 429 tests: 397 pass, 32 skip, and 0 fail.  It is not
yet the requested clean hardware baseline because its late monolithic preflight
skipped Vulkan even while the same filtered required-hardware test passed
immediately afterward.  This process-order contradiction is recorded rather
than masked; no skip classification was changed.

## Next exact work

1. Add the bounded Go BF16-to-FP16 per-tensor transformer, generate the 13
   `noise_refiner.0` cache entries twice, and measure substituted-weight drift.
2. Isolate the monolithic native process-order Vulkan loss, then rerun the M42
   through M49b hardware baseline with validation and warm-allocation evidence.
3. Freeze the resulting tolerance envelope and begin M1 on the captured block.

Reference Python remains oracle-only.  Prometheus product execution continues
to own its reader, transformed cache, SDSL-V shaders, and native runtime.
