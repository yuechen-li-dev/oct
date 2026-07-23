# PROMETHEUS G4-E2B-M1 — text decoder layer 0 with per-layer embeddings

## State

**MEANINGFUL PROGRESSION — reference boundary, fail-closed checkpoint
selection, exact RMSNorm, and the first canonical Q/K/V projection boundary
are closed on the package-backed Windows RTX 3070 route.** The full
authoritative layer remains incomplete; the first remaining boundary is
authoritative Q/K normalization. M0 remains accepted and unchanged. No
checkpoint cache or checkpoint copy was added in this progression.

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

### Q/K/V projection repair and evidence

The defect was host-side dispatch metadata, not checkpoint authority, BF16
decode, shader arithmetic, synchronization, or a Gemma-specific convention.
The production baseline package kernel is an 8x8 scalar kernel: one invocation
writes exactly one `C[row,column]`. Its registry metadata accidentally claimed
eight output columns per invocation. Consequently the generic ABI dispatched
`ceil(N/64)` groups in Y although the shader needed `ceil(N/8)`:

- Q (`15x1536x2048`) issued `2x32` groups and wrote columns `0..255`; the
  zero-initialized unwritten region was columns `256..2047` (26,880 elements,
  1,792 per row; output tiles 32..255 at width eight).
- K/V (`15x1536x256`) issued `2x4` groups and wrote columns `0..31`; the
  zero-initialized unwritten region was columns `32..255` (3,360 elements,
  224 per row; output tiles 4..31).

This accounts for the former `~0.94` relative-L2 failures and the apparent GPU
zeros at authority-nonzero coordinates. The earliest wrong shared boundary was
therefore the logical output footprint used by `prom_sgemm_dispatch_geometry`,
not the shader. It now records one output in each dimension, so the canonical
topologies are Q `2x256x1` and K/V `2x32x1`. A direct-path diagnostic mode
initializes C to finite `-1234567`; the package-backed RTX test leaves zero
sentinels, proving that every coordinate was written rather than merely
computed to zero.

The host uploads row-major FP32 activations `[M,1536]` and a transposed,
row-major FP32 staging view of the BF16 checkpoint tensor `[out,1536]`, so the
SGEMM contract is `C[M,N] = A[M,K] * B[K,N]`. The Q/K/V tensor byte windows are
the validated BF16 ranges for the three exact `self_attn.*_proj.weight`
records; decoding preserves their full byte lengths and the runtime receives
`4*K*N` bytes of FP32 staging for B and `4*M*N` bytes for C. The selected
package implementation is `prometheus.core@1`, `kernel-1-default`, baseline
scalar, with push constants `(m,n,k)` and the topology above.

The independent CPU FP32-accumulation comparison now passes the repository's
near-zero policy for every element, and repeated Q/K/V calls are bit-stable:

| Boundary | Max abs | Max relative (policy) | Relative L2 | Worst index, reference / RTX | finite / zero / nonzero |
| --- | ---: | ---: | ---: | --- | --- |
| Q `[15,2048]` | `0.00012207031` | `0.0023254042` | `1.0076983e-7` | `2743`, `-650.5747 / -650.5748` | `30720 / 0 / 30720` |
| K `[15,256]` | `0.000061035156` | `0.0001151928` | `9.8288815e-8` | `856`, `-688.17847 / -688.1785` | `3840 / 0 / 3840` |
| V `[15,256]` | `0.000061035156` | `0.0002626255` | `9.8570522e-8` | `243`, `672.1707 / 672.17065` | `3840 / 0 / 3840` |

The reference's quantized-boundary hashes remain useful capture identities but
are not an FP32-bitwise SGEMM acceptance rule. The exact RMSNorm hash remains
`969264b7c7ed3fd03d364173143fcebe82fa7d1f7874b3c8ef908b6670bc9095`.

The focused package-backed corpus passed deterministic FP32 activations and
BF16-rounded weights for M=`1,2,7,15,16,17,31,32,33`, K=1536, N=256; canonical
Q N=2048; exhaustive `17x19x17`; and alternating `15,32,15`. All elements were
finite and written. The maximum corpus relative-L2 was `6.8640115e-7`.

The accepted M0 plan remains the governing limit: stream exact BF16 layer-0
and selected embedding/PLE rows; retain FP32 activations; do not make the full
checkpoint device-resident.

The next permitted boundary is authoritative Q/K normalization. Reusing the
existing Z-Image hard-coded 1024x3840 model block, SiLU, or `1e-5` RMSNorm
routes remains semantically wrong.

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
$env:OCT_PROMETHEUS_REACTOR='C:\Users\yuech\source\repos\oct\internal\prometheus\reactor\prometheus_reactor.dll'
go test -run TestGemma4E2BM1CanonicalQKVRTX -count=1 -v ./internal/prometheus

$env:OCT_PROMETHEUS_REACTOR='C:\Users\yuech\source\repos\oct\internal\prometheus\reactor\prometheus_reactor.dll'
go test -tags native -run TestWindowsRunSGEMMUsesRealReactorWhenDLLAvailable -count=1 -v ./internal/prometheus
go test -run TestGemma4E2BM1ProductionSGEMMSmallMCorpusRTX -count=1 -v ./internal/prometheus
go test -count=1 ./internal/prometheus/zimage

$env:PROMETHEUS_REQUIRE_VULKAN_HARDWARE='1'
$env:PROMETHEUS_VK_VALIDATION='1'
.\out\prometheus\native\marionette_tests.exe PrometheusBaselineScalar
go run ./tools/prometheus_native_manifest -check
$manifest = Get-Content -Raw out\prometheus\native\SerialCanonical\shaders\manifest.json | ConvertFrom-Json
$variant = $manifest.tables.variants | Where-Object { $_.id -eq 'kernel-1-default' }
$artifact = Join-Path out\prometheus\native\SerialCanonical\shaders\objects\sha256 ($variant.artifact_sha256 + '.spv')
& "$env:VULKAN_SDK\Bin\spirv-val.exe" $artifact
git diff --check
```

All commands above passed on July 23, 2026. `vulkaninfo --summary` identifies
the admitted RTX 3070 (Vulkan 1.4.329); the Windows native build completed in
133.5 seconds. Vulkan validation remained clean for the sentinel coverage lane,
the canonical shader package manifest check passed, and `spirv-val` accepted
the unchanged package kernel-1 artifact. The existing Z-Image contract
regression passed; the checkpoint-backed Q/K/V harness remains the relevant
real numerical path for this generic SGEMM repair. No shader source changed,
so DXC did not need regeneration. The Linux native build was started with `bash
internal/prometheus/native/build_linux.sh` and honestly exceeded the bounded
120-second window without completion; no Linux build pass is claimed. The
older Marionette default-SGEMM test was not used as package-backed evidence
because its no-config runtime still probes `out/prometheus/native/shaders`,
whereas the admitted canonical package lives under `SerialCanonical/shaders`.

## Recommended next slice

Finish M1 before beginning M2: implement one closed package-backed layer-0
Vulkan assembly and validate it on the RTX 3070 against these captured
boundaries.  Once that passes, M2 should be a short multi-layer prefix proving
layer-to-layer composition; persistent decode KV ownership should remain a
separate later slice.
