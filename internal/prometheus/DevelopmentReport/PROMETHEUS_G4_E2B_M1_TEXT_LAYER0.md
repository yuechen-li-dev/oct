# PROMETHEUS G4-E2B-M1 — text decoder layer 0 with per-layer embeddings

## State

**MEANINGFUL PROGRESSION — reference boundary and fail-closed production
checkpoint selection are closed; the package-backed Windows RTX 3070 route now
executes one authoritative layer-0 boundary exactly, and the first remaining
Vulkan blocker is localized to Q/K/V projection execution on the canonical
Gemma shapes.** M0 remains accepted and unchanged. No public API, checkpoint
cache, or checkpoint copy was added in this progression.

The owner-local checkpoint at `C:\Models\gemma-4-E2B-it` passed the unchanged
full authority check: exact repository/revision, all nine required objects,
the 10,246,621,918-byte safetensors SHA-256, header, and all 2,011 tensor
records.  The repair-stage and owner-backup siblings were not touched.

## Authoritative layer-0 graph

The graph below is from the pinned `transformers==5.6.2` Gemma4 source and the
pinned configuration, not a generic Gemma reconstruction.  Layer 0 is
`sliding_attention`: eight query heads, one KV head, head width 256, causal
window 512, attention scale 1.0, default one-dimensional RoPE theta 10,000,
and RMSNorm epsilon `1e-6`.

```text
ids -> embed_tokens[id] * sqrt(1536)                         [B,S,1536]
     -> input RMSNorm -> Q/K/V linear
     -> Q and K RMSNorm -> local RoPE; V RMSNorm (no scale)
     -> causal local 8Q:1KV attention -> O linear
     -> post-attention RMSNorm -> add initial embedding
     -> pre-FFN RMSNorm -> gate/up linear -> tanh-GELU(gate) * up
     -> down linear -> post-FFN RMSNorm -> add FFN residual
     -> PLE gated residual -> layer scalar (one) -> layer-0 output
```

The exact M1 tensor identities are:

- `model.language_model.embed_tokens.weight`
- `model.language_model.embed_tokens_per_layer.weight`
- `model.language_model.per_layer_model_projection.weight`
- `model.language_model.per_layer_projection_norm.weight`
- `model.language_model.layers.0.{input_layernorm,post_attention_layernorm,pre_feedforward_layernorm,post_feedforward_layernorm}.weight`
- `model.language_model.layers.0.self_attn.{q_proj,k_proj,v_proj,o_proj,q_norm,k_norm}.weight`
- `model.language_model.layers.0.mlp.{gate_proj,up_proj,down_proj}.weight`
- `model.language_model.layers.0.{per_layer_input_gate,per_layer_projection,post_per_layer_input_norm}.weight`
- `model.language_model.layers.0.layer_scalar`

All are BF16 and are already included in the 2,011-tensor authority.
`v_norm` is RMSNorm-without-scale and consequently has no checkpoint weight.

## PLE finding

M0's shorthand “layer input” must not be interpreted as an addition to the
ordinary initial embedding.  The pinned source proves this precise behavior:

```text
token = embed_tokens_per_layer[id] * sqrt(256)               [B,S,35,256]
context = RMSNorm((base_embedding @ model_projection^T) / sqrt(1536))
ple = (token + context) / sqrt(2)

after the ordinary FFN residual in layer 0:
  delta = RMSNorm(per_layer_projection(tanh-GELU(gate(residual)) * ple[:,:,0,:]))
  output = residual + delta
```

Thus PLE is an explicit layer-0 numerical input but enters at the final gated
residual boundary, not the initial token-embedding boundary.  Repeated IDs use
the same token lookup rows; different positions can still differ through their
base embeddings and the residual stream.  The harness materializes the packed
lookup, normalized context projection, and selected layer-0 PLE separately.

For the accepted 15-token oracle, suppressing only the PLE residual leaves the
initial layer hidden state unchanged (Linf delta 0) and changes the final
layer-0 output by Linf **0.625**.  This is a single-layer characterization, not
a general interpretation of PLE.

## Independent CPU-BF16 reference capture

`tools/gemma4e2b_m1_reference.py` is a bounded reference-only harness.  It
uses the official Gemma4 layer modules from Transformers 5.6.2, reads only
selected embedding rows plus exact layer-0 tensors from the validated external
checkpoint, and never writes weights into the repository.  It produces:

- compact FP32 summaries/hashes for base embedding, packed and effective PLE,
  first norm, Q/K/V, normalized/RoPE Q/K, V norm, pre-softmax, probabilities,
  attention before/after O projection, residuals, FFN inputs/projections/gated
  product/down projection, PLE residual, and final output;
- deterministic selected attention rows (heads 0 and 7, first/final rows);
- small external fixtures: token IDs, BF16 base embedding, BF16 layer-0 PLE,
  and BF16 final output.

The source-faithful reconstruction exactly reproduced every overlapping M0
summary hash, including the final layer-0 hash:

```text
base embedding              40910ad962bdcdea...
input RMSNorm               969264b7c7ed3fd0...
Q / K / V                   3999e585... / abd74262... / 4116fcda...
attention probabilities     961c999202e30cd0...
attention after O projection a3a2af11e7fc5957...
FFN down projection         3d11b3f8d7228bbc...
layer-0 output              86020e3d878d566d...
```

This establishes the missing M0 pre-softmax and PLE boundaries without
repeating checkpoint forensics.

The 15-token local-attention witness is finite, assigns exactly zero
probability to every future position, and has BF16 probability-row sums in
`[0.9976959228515625, 1.0020205974578857]` (maximum error from one
`0.0023040771484375`).  It is intentionally shorter than the 512-token window,
so a later production corpus must add a compact >512-token official-reference
case to distinguish causal-only from causal-local masking.

## Production Vulkan implementation and current blocker

`internal/prometheus/gemma4e2b` now owns the closed host-side checkpoint seam.
`OpenLayer0Checkpoint` validates all nine pinned objects, complete checkpoint
SHA-256, safetensors header/physical identity, and all 2,011 tensor records
before returning only the 21 exact layer-0 ranges. `ReadRows` is bounded to the
two BF16 embedding tables and rejects invalid token IDs, overflows, renamed
tensors, shape/dtype differences, and adjacent-range reads. It holds an
external file handle only; it copies no weights into the repository and has no
fallback source. The real owner checkpoint integration test passed, including
repeated row identity.

The Go/native bridge now passes a shader-package root into
`prometheus_reactor_runtime_create`, resolves a narrow model-private entrypoint
`prometheus_reactor_runtime_gemma4e2b_m1_input_rmsnorm`, and discovers the
canonical package manifest from both supported repo-local DLL locations:

- `out/prometheus/native/prometheus_reactor.dll`
- `internal/prometheus/reactor/prometheus_reactor.dll`

That second discovery path mattered on **July 23, 2026** because the existing
Windows native integration helper prefers the repo-local `internal/...` DLL
path, while the canonical shader package manifest lives under
`out/prometheus/native/SerialCanonical/shaders/manifest.json`. Before this
fix, the generic real-DLL SGEMM integration lane truthfully fell back to CPU
even though the rebuilt DLL bytes were identical in both locations.

The current package-backed canonical harness is
`internal/prometheus/gemma4e2b_m1_rtx.go`. It consumes the accepted BF16 base
embedding fixture, the closed checkpoint rows/ranges, and the accepted
reference hashes at these first four boundaries:

- `layer0_input_rmsnorm`
- `layer0_q_linear`
- `layer0_k_linear`
- `layer0_v_linear`

### Exact admitted RTX boundary reached

On the admitted Windows RTX 3070 route, the model-private RMSNorm path reached
the first authoritative boundary exactly:

| Boundary | Reference hash | RTX quantized hash | Result |
| --- | --- | --- | --- |
| `layer0_input_rmsnorm` | `969264b7c7ed3fd03d364173143fcebe82fa7d1f7874b3c8ef908b6670bc9095` | `969264b7c7ed3fd03d364173143fcebe82fa7d1f7874b3c8ef908b6670bc9095` | exact match |

Native telemetry for that successful RMSNorm call:

- backend: `windows_native_vulkan`
- submit count: `1`
- final readback count: `1`
- output written: `true`
- matched input: `true`
- retained bytes: `196,788`
- buffer allocations: `6`
- descriptor updates: `3`
- pipeline creations: `2`
- command-buffer reuse count: `0`
- end-to-end GPU path time: about `0.87–1.51 ms` across repeated runs on July 23, 2026

This is real package-backed RTX numerical execution, not compilation-only or a
CPU fallback.

### First remaining blocker: Q/K/V projection execution

After the exact RMSNorm boundary, the same accepted canonical case still fails
at the next graph step. The admitted RTX projection outputs are finite but do
not match either the accepted authority hashes or a CPU row-major contract
evaluation over the same decoded checkpoint weights:

| Boundary | Authority hash | RTX quantized hash | CPU row-major contract hash |
| --- | --- | --- | --- |
| `layer0_q_linear` | `3999e585b50bbb44f05b716c2eff1fdfce141a98208af2f72092dc4029ea8ed9` | `5e45c0a677536f473e370cf9c5e567c4c58156a7cbc8c2b3bee2179b45617a25` | `213463d159b2b9c96e730e96e6e58817d71c509312cc663117919bdbd2147bb8` |
| `layer0_k_linear` | `abd74262322068e44de11950ebc0d09ce34a1f0f3093771abf02c83f5539b477` | `72d7e23d55511ff77e42a6169632666607140a19887c9e4eae6c48ab564c518a` | `2699befd0a59d6ddbb77dcdce9894bd20977b128fa99dc99b27d48f130f1d4a8` |
| `layer0_v_linear` | `4116fcda3d74748f837b6a7850f17a7c5710d4e4cf3b269fe7643e664634aaae` | `7d354aa5b0111eb22645737777e48c7a51ec12887954b9ce32f73e310ecfa340` | `bb55a0251d6a40dec70edd11d32c0ff91f049d67208a330150c6cb0b228cab24` |

GPU-versus-CPU-contract error is large rather than roundoff-sized:

- Q: max-abs `1323.3868`, relative-L2 `0.9415936985608241`, worst index `6857`
  (`actual 0`, `reference 1323.3868`)
- K: max-abs `688.17847`, relative-L2 `0.9398494543686065`, worst index `856`
  (`actual 0`, `reference -688.17847`)
- V: max-abs `887.7394`, relative-L2 `0.9354558580861995`, worst index `1011`
  (`actual 0`, `reference 887.7394`)

This divergence persisted after reopening a fresh runtime for the Q/K/V calls,
so it is not explained by sharing one runtime instance between the new
Gemma-private RMSNorm entrypoint and the generic SGEMM entrypoint.

Separately, the generic real-DLL Windows SGEMM integration lane now passes
again after the package-root discovery fix, so the next blocker is narrowed to
the admitted projection route for the canonical Gemma shapes/weights rather
than a blanket “real SGEMM is unavailable” claim.

The accepted M0 plan remains the governing limit: stream exact BF16 layer-0
and selected embedding/PLE rows; retain FP32 activations; do not make the full
checkpoint device-resident.

The next isolated blocker is now concrete: identify why admitted projection
execution for shapes `15x1536x2048` and `15x1536x256` diverges on the RTX path,
then carry the canonical execution forward into Q/K normalization, positional
transform, and attention. Reusing the existing Z-Image hard-coded 1024x3840
model block, SiLU, or `1e-5` RMSNorm routes would still be semantically wrong.

## Validation run

```powershell
go run ./tools/gemma4e2b_forensics -root C:\Models\gemma-4-E2B-it `
  -authority internal\prometheus\DevelopmentReport\artifacts\G4E2BM0\checkpoint_authority.json

$env:PYTHONPATH = "$env:TEMP\g4e2b-transformers-5.6.2"
C:\ComfyUI Files\.venv\Scripts\python.exe tools\gemma4e2b_m1_reference.py ...

C:\ComfyUI Files\.venv\Scripts\python.exe -m py_compile tools\gemma4e2b_m1_reference.py
go test ./tools/gemma4e2b_forensics ./internal/prometheus/zimage
G4E2B_CHECKPOINT_ROOT=C:\Models\gemma-4-E2B-it go test -count=1 -v ./internal/prometheus/gemma4e2b
$env:OCT_RUN_PROMETHEUS_INTEGRATION='1'
$env:G4E2B_CHECKPOINT_ROOT='C:\Models\gemma-4-E2B-it'
$env:OCT_PROMETHEUS_REACTOR='C:\Users\yuech\source\repos\oct\out\prometheus\native\prometheus_reactor.dll'
go test -run TestGemma4E2BM1CanonicalQKVRTX -count=1 -v ./internal/prometheus

$env:OCT_PROMETHEUS_REACTOR='C:\Users\yuech\source\repos\oct\internal\prometheus\reactor\prometheus_reactor.dll'
go test -tags native -run TestWindowsRunSGEMMUsesRealReactorWhenDLLAvailable -count=1 -v ./internal/prometheus
git diff --check
```

All commands above passed. `vulkaninfo --summary` identifies the admitted RTX
3070 (Vulkan 1.4.329); the Windows native build also completed successfully in
129.848 seconds (its launcher log records exit code 0). The canonical Gemma
RTX run reached one exact numerical boundary and then localized the next
projection blocker exactly as recorded above. DXC, Gemma SDSL-V, and SPIR-V
validation still are not represented as Gemma M1 closure evidence because the
complete layer-0 package-backed assembly does not exist yet. The existing Linux
native build was started but exceeded this session's 120-second command window
before it emitted a completion result, so no Linux build pass is claimed.

## Recommended next slice

Finish M1 before beginning M2: implement one closed package-backed layer-0
Vulkan assembly and validate it on the RTX 3070 against these captured
boundaries.  Once that passes, M2 should be a short multi-layer prefix proving
layer-to-layer composition; persistent decode KV ownership should remain a
separate later slice.
