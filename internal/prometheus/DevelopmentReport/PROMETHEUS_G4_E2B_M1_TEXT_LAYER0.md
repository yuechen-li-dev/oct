# PROMETHEUS G4-E2B-M1 — text decoder layer 0 with per-layer embeddings

## State

**MEANINGFUL PROGRESSION — the owner-selected portable BF16 projection contract
is now the M1 authority.** The production route BF16-rounds the resident layer
RMSNorm activation before Q/K/V projection, uses the existing BF16 checkpoint
weights with FP32 SGEMM accumulation, then stores each projection as BF16 before
re-expansion. The historical oneDNN capture remains an important diagnostic,
not a cross-backend bitwise contract. Q/K normalization is compared to the new
portable strict reference, not accidentally to oneDNN reduction crossings.
RoPE now has an authority fixture and validated package artifact, but its native
device invocation seam remains unimplemented. M0 remains accepted and
unchanged. No checkpoint cache or checkpoint copy was added in this progression.

The configured local checkpoint root (`G4E2B_CHECKPOINT_ROOT`) passed the unchanged
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

That resident BF16 projection-to-normalization boundary is now closed under the
portable contract. Reusing the existing Z-Image hard-coded 1024x3840 model
block, SiLU, or `1e-5` RMSNorm routes remains semantically wrong.

### Q/K normalization semantics

The captured source establishes the following exact layer-0 facts; this
supersedes any generic-Gemma shorthand in earlier report language:

- Q has eight heads and K/V have one KV head, with `head_dim=256`; every query
  head maps to KV head zero (`query_head / 8`).
- Both `q_norm` and `k_norm` are learned-weight RMSNorm over exactly one
  `[256]` head vector, with epsilon `1e-6`; they are not the layer-input
  `[1536]` RMSNorm. The scale is the corresponding checkpoint vector
  `self_attn.{q_norm,k_norm}.weight`.
- Layer 0 is `sliding_attention` with causal local window 512. The canonical
  15-token witness is consequently causal-only in practice, but must not be
  used to infer that later rows are globally visible.
- RoPE is local/default one-dimensional RoPE: positions originate at zero,
  theta is `10000`, inverse frequencies are
  `1 / (10000 ** (arange(0,256,2) / 256))`, and the reference forms the
  duplicated-frequency cosine/sine vector before `apply_rotary_pos_emb`.
  The implementation must preserve both source components before updating a
  pair; the prior in-place even/odd hazard remains a required focused
  regression.
- Attention applies no additional `1/sqrt(head_dim)` factor: the captured
  eager call uses scaling `1.0` before stable softmax.

To prove the layout and reduction boundary without widening the public API, a
model-private `prometheus_reactor_runtime_gemma4e2b_m1_head_rmsnorm` symbol
was added. It uses the established M46 FP32 reduction/application machinery
but admits flattened head rows only: Q is `[15,8,256] -> [120,256]`, and K is
`[15,1,256] -> [15,256]`. The reference harness now emits complete comparison
fixtures `layer0_q_normalized.bf16le.bin` and
`layer0_k_normalized.bf16le.bin` in the pre-RoPE `[token,head,channel]`
layout; they are authority inputs only and are written outside the repository.

On the rebuilt package-backed Windows RTX 3070 route, both calls had one final
readback, no intermediate host replacement, all expected coordinates written,
no zero/NaN/infinity values, and stable repeat results. Against an independent
FP32 CPU reconstruction of the exact dispatched input they were:

| Boundary | Max abs | Relative L2 | Coverage |
| --- | ---: | ---: | --- |
| Q head RMSNorm | `0.000002861023` | `1.2711686e-7` | `30720/30720` finite nonzero values |
| K head RMSNorm | `0.00000023841858` | `1.1853089e-7` | `3840/3840` finite nonzero values |

This was the pre-repair localization, not a current failure: PyTorch's layer
executes a BF16 activation before the Q/K linear, while corrected package SGEMM
had been supplied with the uncast FP32 layer-RMSNorm value. Passing that older
projection into the otherwise-correct head reduction caused Q max absolute
error `0.05006504` and K `0.0029935837`. It established a missing precision
boundary—not a Q/K layout or scale error—and is now repaired by the
model-private resident stage described below. No host conversion, CPU result
replacement, or Gemma-specific SGEMM/GEMV workaround was introduced.

### Resident BF16 transition implementation and exact localization

The requested model-private stage is now package-backed as
`kernel-67-default`, `Gemma4E2BM1Bf16Roundtrip_CS`, sourced from
`internal/prometheus/shaders/sdslv/production/gemma4e2b/bf16_roundtrip.sdslv`.
It has four storage-buffer bindings, a 32-byte push record, and 256 threads per
workgroup. It writes an FP32 destination holding the exact value of
FP32 -> BF16 -> FP32, rather than introducing a public quantization reactor or
a host numerical conversion. Finite values use round-to-nearest-even at BF16
bit 16; infinities are preserved and NaNs retain their high payload bits with
the BF16 quiet bit set. The private Q/K entrypoint accepts only flattened
256-channel rows, at most 120 rows, checkpoint scale length 256, and epsilon
`1e-6`; malformed geometry is rejected before dispatch.

The stage runs in the M46 command buffer after its resident upload and before
the first head reduction. A second in-place device stage runs after M46 apply
and before its only result readback, matching the captured BF16 normalized
output storage. Both Q `[120,256]` and K `[15,256]` reported complete output,
were finite, and were repeat-stable. This is device numerical work, but it is not yet a direct
device-buffer handoff from generic SGEMM: the current public projection ABI
performs its established host readback before the private head route uploads.
No host-side numerical conversion or replacement occurs.

The reference harness now writes bounded BF16 storage witnesses for
`layer0_q_linear`, `layer0_k_linear`, and `layer0_v_linear`, in addition to
the normalized fixtures. They demonstrate the relevant precision graph:

```text
input RMSNorm: BF16 source -> FP32 resident arithmetic/result
Q/K/V projection: CPU-BF16 authority stores BF16; later consumers re-expand FP32
Q/K head normalization: BF16 input -> FP32 reduction/application -> BF16 output storage
RoPE inputs/constants/output: not implemented on Vulkan; captured source uses BF16 cos/sin
attention scores/aggregation: not implemented; V's authority input is BF16-expanded
```

The following is the historical pre-activation-repair RTX witness. It records
why post-projection BF16 rounding alone was insufficient and is retained as a
diagnostic, not as the current portable comparison:

| BF16 storage witness | Max abs | Failing elements | First differing authority / RTX value |
| --- | ---: | ---: | --- |
| Q `[15,2048]` | `4.0` | `10,418/30,720` | index `7`: `11.5 / 11.5625` |
| K `[15,256]` | `1.0` | `1,327/3,840` | index `0`: `2.578125 / 2.59375` |
| V `[15,256]` | `1.0` | `1,734/3,840` | index `0`: `1.8125 / 1.8671875` |

Before the activation repair, complete normalized comparison consequently
failed. The normalization implementation was not implicated: before BF16
authority was introduced, its direct-FP32 operational comparisons were Q
`2.861023e-6` (relative L2 `1.2711686e-7`) and K `2.3841858e-7` (relative L2
`1.1853089e-7`). The only supported next repair investigation is the shared
projection contraction's BF16 semantics; do not alter RMSNorm, add GEMV, or
start RoPE to compensate downstream.

### Projection-contraction operand and portability evidence

The direct operand investigation closes the activation/weight ambiguity without
changing production arithmetic. A new external fixture
`layer0_input_rmsnorm.bf16le.bin` proves that the BF16 image of the RTX input
RMSNorm output matches the authority element-for-element before all three
projections. The Q/K/V weights are the same validated BF16 checkpoint byte
windows consumed by both sides (raw SHA-256 respectively
`d8edcc96...`, `58fb897c...`, and `69a32bbb...`); no transpose or operand
layout discrepancy remains.

The authoritative expression is `torch.nn.Linear(BF16 activation, BF16
checkpoint weight)`. `Gemma4RMSNorm` computes its norm and learned scale in
FP32, then casts to the BF16 activation presented to the linear. The uncast
RMSNorm value differs from its BF16 stored value in 23,037 of 23,040 elements
(max `0.7925415`), even though its BF16 image is exact. That missing activation
storage boundary explains the earlier large Q/K/V mismatch:

| Reconstruction against CPU-BF16 storage | Q exact / differing | K exact / differing | V exact / differing |
| --- | ---: | ---: | ---: |
| A: uncast FP32 RMSNorm x BF16 weight, FP32 accumulation, BF16 output | `20,304 / 10,416`, max `4.0` | `2,513 / 1,327`, max `1.0` | `2,106 / 1,734`, max `1.0` |
| B: BF16 activation x BF16 weight, ordinary FP32 `F.linear`, BF16 output | `30,709 / 11`, max `0.25` | `3,838 / 2`, max `0.0078125` | `3,838 / 2`, max `0.25` |
| C: captured `nn.Linear` BF16 operation | `30,720 / 0` | `3,840 / 0` | `3,840 / 0` |

The package RTX operand witness is exact. Before this continuation the scalar
package SGEMM still received the uncast FP32 activation. A controlled strict scalar FP32
reconstruction with the accepted BF16 activation image passes K and V
completely, while Q has exactly three remaining BF16 crossings (max `0.0625`;
first flat index `5832`, authority `-0.022705078` / `0xbcba`, scalar
`-0.022827148` / `0xbcbb`). This verifies that a resident activation BF16
transition is necessary, but cannot yet close Q storage by itself.

The remaining Q crossings are backend-sensitive rather than a stable model
precision contract. The capture ran on PyTorch `2.9.1+cpu`, MKL 2025.3 and
oneDNN v3.7.1, AVX512, eight compute/inter-op threads, with unset
`OMP_NUM_THREADS`, `MKL_NUM_THREADS`, and `ONEDNN_MAX_CPU_ISA`. The captured
oneDNN-enabled `nn.Linear` is bit-stable across repeats and across one versus
eight threads. Disabling oneDNN is also repeat-stable and thread-independent,
but changes `9` Q, `2` K, and `2` V BF16 values, matching the ordinary FP32
linear reconstruction rather than the captured output. A compact order witness
`[16777216, -8388608, -8388608, -0.5]` produces scalar FP32/BF16 `-0.5` /
`0xbf00` and pairwise-tree `0.0` / `0x0000`; reduction topology can therefore
change a final BF16 result without any operand error.

### Owner decision: portable projection authority

The owner selected the portable Gemma projection contract: BF16 activation
operands, BF16 checkpoint-weight operands, multiplication after FP32
re-expansion, FP32 accumulation, and BF16 projection storage. Execution must
remain deterministic and bit-stable on each admitted backend, but FP32
reduction order is implementation-defined within the stated BF16 acceptance
policy. Exact oneDNN-enabled `torch.nn.Linear` parity is therefore diagnostic
only, not a production target.

There are now five deliberately distinct evidence streams:

1. portable semantic authority: the operand and storage contract above;
2. deterministic strict reference: increasing-K scalar FP32 accumulation with
   the exact BF16 casts;
3. historical oneDNN-enabled capture: the original complete fixture;
4. oneDNN-disabled diagnostic capture: stable but changed at Q `9`, K `2`, and
   V `2` BF16 values;
5. Vulkan production: package `prometheus.core@1`, existing scalar
   `kernel-1-default`, compared first to the strict reference.

The resident repair is model-private
`prometheus_reactor_runtime_gemma4e2b_m1_projection_activation_rmsnorm`. It
uses `kernel-67-default` only after the layer RMSNorm apply stage, yielding the
FP32 re-expansion of the authoritative BF16 activation before Q/K/V upload.
The existing shared SGEMM reduction topology is unchanged. Q/K head RMSNorm
continues to use its independently accepted BF16 input/output route.

### Portable projection and Q/K-normalization acceptance — July 23, 2026

The repaired canonical package-backed RTX 3070 route first compares the
resident BF16 activation image directly with
`layer0_input_rmsnorm.bf16le.bin`: all `23,040` BF16 values match exactly.
Each checkpoint weight is decoded from the validated BF16 window and is passed
as an FP32 re-expansion of those same bits. The canonical production sequence
is therefore:

```text
FP32 layer RMSNorm apply
  -> kernel-67-default: FP32 -> BF16 -> FP32 activation image
  -> kernel-1-default: BF16-reexpanded activation x BF16-reexpanded weight,
     FP32 accumulation
  -> model-private Q/K per-head RMSNorm, whose resident input stage supplies
     the required BF16 projection-storage image before the head reduction
```

The GPU does no host numerical conversion or result replacement. The existing
generic SGEMM ABI still makes its normal bounded host transport between public
calls; this is not a new resident generic-GEMM chaining API. Q/K enter the
next private GPU stage where their BF16 storage image is made before head
normalization. V has no downstream stage in this continuation; its BF16 image
is formed only in the host-side *comparison authority*, never as a production
replacement. The precision transition itself is resident, package-backed Vulkan
work. Its FP32 buffers retain four bytes per logical element; physical BF16
semantics are represented by the exact FP32 re-expansion, so element count,
descriptor range, and capacity remain `4 * elements` bytes and no byte-packed
BF16 alignment claim is made.

Against the deterministic increasing-K strict reference, all three canonical
Vulkan projection storage tensors are exact BF16 matches. No sentinel survived,
all values were finite, and repeated Q/K/V calls were bit-stable:

| Tensor | Shape | Exact / differing | Max BF16 ULP | Max abs | Relative L2 | finite / zero / NaN / infinity |
| --- | --- | ---: | ---: | ---: | ---: | --- |
| Q | `[15,2048]` | `30720 / 0` | `0` | `0` | `0` | `30720 / 0 / 0 / 0` |
| K | `[15,256]` | `3840 / 0` | `0` | `0` | `0` | `3840 / 0 / 0 / 0` |
| V | `[15,256]` | `3840 / 0` | `0` | `0` | `0` | `3840 / 0 / 0 / 0` |

The historical oneDNN-enabled capture differs from that strict portable Q
reference at three BF16 values (maximum `0.0625`; first flat index `5832`,
`0xbcba` versus `0xbcbb`) and matches K/V. This is retained as backend evidence
only. The oneDNN-disabled capture's Q `9`, K `2`, V `2` changes confirm that
neither capture is the portable arithmetic target.

Q/K normalization now consumes the accepted resident BF16 projection images.
It preserves `[token,head,component]` layout—Q `[15,8,256]`, K `[15,1,256]`—
with one 256-component learned-scale RMS reduction per head and epsilon
`1e-6`. Complete results are finite, fully written, repeat-stable, and fresh
across the alternating Q/K invocations:

| Tensor | Exact / differing vs independent CPU head reconstruction | Max BF16 ULP | Max abs | Relative L2 | Worst coordinate (flat) |
| --- | ---: | ---: | ---: | ---: | --- |
| Q normalized | `30714 / 6` | `1` | `0.00390625` | `4.802524913e-5` | `5664`: `0.63671875 / 0.6328125` (`0x3f23 / 0x3f22`) |
| K normalized | `3840 / 0` | `0` | `0` | `0` | none |

The six Q differences are final BF16 rounding crossings caused by the accepted
head-reduction order; every one is exactly one ordered BF16 step, with no
non-finite result. The BF16-stage operational acceptance is therefore ordered
ULP `<= 1` and relative L2 `<= 1e-4`; it is deliberately separate from the
projection policy (relative L2 `<= 1e-5`) and does not relax the projection
contract. Representative factors continue to use the captured learned Q/K
weights and epsilon; the existing direct-FP32 head operational witnesses remain
Q max abs `2.861023e-6`, relative L2 `1.2711686e-7`, and K max abs
`2.3841858e-7`, relative L2 `1.1853089e-7`.

The next layer-0 boundary is precise positional transformation (RoPE): no RoPE,
attention score, softmax, aggregation, output projection, FFN, or PLE work
began in this continuation.

## Validation run

```powershell
go run ./tools/gemma4e2b_forensics -root $env:G4E2B_CHECKPOINT_ROOT `
  -authority internal\prometheus\DevelopmentReport\artifacts\G4E2BM0\checkpoint_authority.json

$env:PYTHONPATH = "$env:TEMP\g4e2b-transformers-5.6.2"
<validated-python> tools\gemma4e2b_m1_reference.py ...

<validated-python> -m py_compile tools\gemma4e2b_m1_reference.py
go test ./tools/gemma4e2b_forensics ./internal/prometheus/zimage
$env:G4E2B_CHECKPOINT_ROOT='<validated checkpoint root>'
go test -count=1 -v ./internal/prometheus/gemma4e2b
$env:OCT_RUN_PROMETHEUS_INTEGRATION='1'
$env:G4E2B_CHECKPOINT_ROOT='<validated checkpoint root>'
$env:OCT_PROMETHEUS_REACTOR=(Join-Path (Get-Location) 'internal\prometheus\reactor\prometheus_reactor.dll')
go test -run TestGemma4E2BM1CanonicalQKVRTX -count=1 -v ./internal/prometheus

$env:OCT_PROMETHEUS_REACTOR=(Join-Path (Get-Location) 'internal\prometheus\reactor\prometheus_reactor.dll')
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

The checkpoint authority check, reference Python syntax check, package manifest
check, native package check, targeted SDSL-V/DXC compile, `spirv-val` for the
new kernel-67 artifact, targeted Go tests, Windows native build, canonical RTX
test, small-M RTX corpus, real-DLL bridge test, Z-Image regression, and
`git diff --check` passed on July 23, 2026. The canonical Q/K/V route passed
under the portable contract and remains bit-stable. `vulkaninfo --summary`
identifies the admitted RTX 3070 (Vulkan 1.4.329); Vulkan validation ran with
zero errors for the baseline scalar diagnostic-sentinel coverage test. The
workspace-wide `tools/sdslv_workspace_check` is not a passing gate in this
state: it reports that the pre-existing scalar asset source is absent as a
literal from `reactor_shader_registry.c`, even though the manifest/package
checks and the actual kernel-67 SDSL-V/DXC/validation route pass. This
continuation did not paper over that unrelated registry-check limitation. The
Linux native build was previously started with `bash
internal/prometheus/native/build_linux.sh` and honestly exceeded the bounded
120-second window; no Linux build pass is claimed.

## First remaining layer-0 boundary

### Positional-transformation staging — July 23, 2026

The accepted reference makes the Gemma layer-0 RoPE precision graph explicit:
Q/K head RMSNorm produces BF16-stored values re-expanded to FP32; positions are
integer indices beginning at zero; inverse frequencies are FP32
`1 / 10000^(2i/256)` for `i=0..127`; FP32 angles are evaluated with FP32
transcendentals; cosine and sine are separately BF16-rounded; paired FP32
multiply/add results are BF16-stored before downstream re-expansion. The
logical production layout is `[token,head,component]`: Q `[15,8,256]`, K
`[15,1,256]`. All 256 components transform. Pairing is **half-split**:
component `i` pairs with `i+128`, not with an adjacent component. Q and K use
the same positions, signs, and frequencies.

The reference harness now emits bounded comparison fixtures for BF16 cosine,
sine, Q RoPE, and K RoPE. A model-private SDSL-V artifact
`rope_half_split.sdslv` was added and built as package
`prometheus.core@1` / `kernel-68-default` / `Gemma4E2BM1RopeHalfSplit_CS`.
It reads both half-pair source values into locals before writing a distinct
output buffer, which directly excludes the in-place overwrite hazard. SDSL-V
validation, DXC, package validation, and `spirv-val` pass for the artifact.

This is deliberately not yet counted as a positional RTX pass: the production
native invocation seam that allocates/binds the separate RoPE output buffer and
feeds it from the accepted Q/K head-normalization device buffers remains to be
implemented. Consequently no complete Q/K RoPE tensor comparison, lifecycle
test, or in-place-pairing GPU regression is claimed. The first remaining
boundary is that narrow model-private device invocation seam; attention scores
remain out of scope until it passes.

Do not begin M2, scores, softmax, aggregation, output projection, FFN, or PLE.
The first remaining M1 boundary is the model-private RoPE invocation seam
described above, followed by complete transformed Q/K tensor comparison before
attention begins.
The accepted small-M SGEMM dispatch-footprint repair is closed and must not be
revisited.

## Positional native-dispatch continuation — July 23, 2026

This continuation established a narrow package-backed native Vulkan RoPE
dispatch, but does **not** yet close the requested end-to-end resident handoff.
The status is therefore meaningful progression rather than a complete
positional closure.

### Accepted positional authority and package ABI

The authority remains fixed: Q is logical `[15,8,256]`, K is logical
`[15,1,256]`, each pair is `i <-> i+128`, positions are zero-origin, and the
precomputed `[15,256]` cosine and sine tables are BF16 storage values. The
shader is `prometheus.core@1`, package variant `kernel-68-default`, entry point
`Gemma4E2BM1RopeHalfSplit_CS`, with exactly four storage-buffer bindings and a
32-byte push block:

| Binding | Role | Required logical elements |
| --- | --- | --- |
| 0 | normalized Q or K source | `tokens * heads * 256` |
| 1 | BF16-reexpanded cosine table | `15 * 256` |
| 2 | BF16-reexpanded sine table | `15 * 256` |
| 3 | distinct BF16-reexpanded transformed output | `tokens * heads * 256` |

The shader reads both members of every half-split pair before it writes the
separate destination. It consumes the table data directly, so no production
transcendental operation remains and RTX `sin`/`cos` variation cannot explain a
result. The observed fixture evidence further fixed the accepted arithmetic
storage graph: each BF16-decoded product is BF16-rounded before the final
BF16-rounded sum/difference. This was a precision-boundary correction only;
the accepted geometry, signs, theta, origin, and pairing were unchanged.

### Native seam and lifecycle evidence

`prometheus_reactor_runtime_gemma4e2b_m1_rope` is model-private. It rejects
nulls, unsupported token/head/head-width geometry, mismatched element counts,
arithmetic-size overflow, nonfinite inputs, identical cosine/sine roles, and
source/table-to-output aliasing before it acquires a slot or records a command.
It resolves `kernel-68-default` through the immutable package manifest; no
embedded substitute SPIR-V is used. A rejection of head count `2` was followed
by valid Q then K dispatches in the same runtime.

The first Q dispatch created the pipeline and eight bounded buffers; the
following K dispatch created no buffers and reused all eight. Its descriptor
update count advanced from one to two across the differing Q/K shapes, proving
the four binding ranges are refreshed rather than inherited from the Q
allocation. Q/K, K/Q, repeated Q, repeated K, and post-rejection Q/K calls are
bit-stable. The direct shader witness computes each output member from the
original pair and is not an in-place sequential update.

### RTX tensor evidence

The live Windows RTX 3070 Vulkan lane passed with validation enabled. Q had
30,720 elements: 30,718 exact BF16 matches, 2 differing values, maximum ordered
BF16 distance 1, maximum absolute error `0.0009765625`, and relative L2
`6.329223424217383e-06`; all values were finite, with 11 zeros and no NaNs or
infinities. K had 3,840/3,840 exact BF16 matches, zero difference, zero maximum
error, and no nonfinite values. The two Q differences are fully accounted for
by the already-accepted Q head-RMSNorm one-BF16-step policy (the normalizer has
six such differences); the package shader matched exactly when compared against
the BF16 storage graph over its actual resident source image. No aggregate
tolerance is being used to hide a layout, position, pairing, table-addressing,
or binding error.

### Validation

Passed: checkpoint authority checker; Python reference syntax check; targeted
Gemma checkpoint tests; package build/check; SDSL-V validation; DXC;
`spirv-val`; Windows native build (129.113 seconds); canonical RTX Q/K/V
projection and Q/K normalization test; live package-backed Q/K RoPE test;
small-M SGEMM RTX corpus; baseline scalar diagnostic-sentinel Vulkan test;
Z-Image regression; native-manifest check; and `git diff --check`.

The workspace-wide SDSL-V checker remains separately blocked by its known
pre-existing scalar-SGEMM registry source-literal failure; this continuation
did not weaken or mask it. Linux native build was not rerun in this continuation
and no Linux pass is claimed.

### First remaining layer-0 boundary

The remaining implementation boundary is precise: the new RoPE API currently
uploads its accepted normalized source image into its bounded device slot before
dispatch. It must instead consume the already-resident output view from the
accepted Q/K head-RMSNorm continuation, preserving the same distinct RoPE
destination buffer and avoiding the current host staging seam. Projection,
normalization, RoPE arithmetic, package identity, descriptor ABI, and all
attention work remain closed or out of scope as stated above; do not begin
scores, masking, softmax, aggregation, output projection, residual, FFN, PLE,
or M2.

## Resident normalization-to-RoPE closure — July 23, 2026

The upload-based source route is now superseded in the model-private production
chain. `prometheus_reactor_runtime_gemma4e2b_m1_head_rmsnorm_rope` accepts only
the fixed M1 Q/K geometry and records M46 head RMSNorm plus package-backed
`kernel-68-default` in one serialized submission. It does not expose an
arbitrary-device-buffer API. M46's `continuation.n` (`slot->m46_output`) is
the exact binding-0 source for RoPE; no normalized tensor is copied to host,
reconstructed, uploaded, or copied into an upload-oriented device slot.

Binding 0 is that resident normalized FP32 BF16-reexpansion, bindings 1 and 2
are the authoritative `[15,256]` BF16 cosine and sine tables uploaded to their
existing bounded table buffers, and binding 3 is the distinct device-local
`slot->gemma4e2b_m1_rope_output`. Q uses 122,880 bytes and K uses 15,360 bytes;
both source and destination offsets are zero and their ranges are exact. The
only host transfer after the chain is the final transformed destination
readback used by the caller/test. The normalizer's source range is never
read back in this route (`normalized_readback_count=0`).

The dependency is an in-command-buffer barrier on `continuation.n` from
`COMPUTE_SHADER/SHADER_WRITE` to `COMPUTE_SHADER/SHADER_READ`; the distinct
destination is made visible through `COMPUTE_SHADER/SHADER_WRITE` to
`TRANSFER/TRANSFER_READ`, followed by the final host-visible readback barrier.
The single submission fence gates every caller-visible output. Package tables
are separately ordered `HOST_WRITE -> TRANSFER_READ -> SHADER_READ`. Thus Q/K
alternation cannot depend on queue idleness or stale descriptor state.

Live RTX evidence: the direct chain passed Q then K, K then Q, repeated Q,
repeated K, alternating shapes, and a rejected invalid-head call followed by
valid Q then K. Descriptor updates advanced across the Q/K range change; the
pipeline remained one settled package pipeline. The first direct Q reported
`resident_source_bound=true`, source/destination `122880` bytes, and zero
normalized readbacks; K reported the same proof with `15360` bytes. New RoPE
destination buffers are bounded slot allocations and are reused after warm-up.

Numerically, the direct resident chain is stage-local exact against the
authoritative BF16 storage graph over the accepted normalized values. K is
3,840/3,840 exact against pristine authority. Q is 30,718/30,720 exact;
the two one-ordered-BF16-ULP final values remain solely the accepted Q
normalization-source deviations and each propagates through its half-split
partner pair. There is no RoPE-originated divergence and no shader
transcendental operation. Q retains 11 zeros and all 30,720 finite values;
K retains 3,840 finite values, with no NaNs, infinities, or sentinels.

Focused validation passed: Windows native build (129.6 s), real RTX Vulkan
canonical Q/K/V projection, accepted Q/K normalization, resident Q/K RoPE
comparison, descriptor alternation/repetition/recovery corpus, package-backed
ABI resolution, and targeted Go test. Vulkan validation was clean in the live
lane. The earlier accepted SDSL-V/DXC/`spirv-val`, package, small-M SGEMM, and
Z-Image checks remain unchanged. Linux native build was not rerun here, so no
Linux pass is claimed. The known workspace-wide scalar-SGEMM registry
source-literal SDSL-V failure remains pre-existing and separate.

This closes the positional continuation only. The first remaining layer-0
boundary is attention-score construction from the separately resident
transformed Q and K outputs; it remains explicitly out of scope for this work.

## Attention-score authority and package prefix — July 23, 2026

The pinned `google/gemma-4-E2B-it` revision `3e22461f...` establishes the
pre-mask layer-0 score contract: Q is logical `[15,8,256]`, K is
`[15,1,256]`, both are flattened token-major BF16-reexpanded FP32 resident
storage, and scores are logical/physical `[8,15,15]` flattened as
`[queryHead,queryPosition,keyPosition]`. The grouped-query map is
`kvHead = queryHead / (8 / 1) = 0`. Coordinates are accumulated in increasing
`d=0..255` order as FP32 products and additions, then scaled once after the
dot product by `1/sqrt(256) = 1/16`; score storage is FP32. The configuration
contains no attention-logit soft-cap or query-pre-attention scale. Its
`final_logit_softcapping=30` applies to final vocabulary logits and is not
part of this pre-mask attention boundary. Masking and softmax remain excluded.

`kernel-69-default`, package identity `prometheus.core@1`, has been added as
the narrow raw-score shader package artifact. Its ABI is binding 0 resident
RoPE Q, binding 1 resident RoPE K, binding 3 distinct FP32 score output, and a
32-byte push block carrying geometry and the explicit scale. SDSL-V validation
and package build/check pass (69 kernels). It has no masking, exponentiation,
or softmax operation.

This is meaningful progression, not score-stage closure: the first remaining
implementation boundary is the model-private native lifetime seam that retains
both separately produced kernel-68 destination buffers simultaneously and
binds those exact slot buffers to kernel 69 without a host round-trip. No RTX
score tensor, numerical acceptance, or score-stage Vulkan dispatch is claimed
yet. Attention masking, softmax, V aggregation, and all downstream stages have
not been started.

### Raw-score resident seam implementation (pending live RTX acceptance)

The native seam is now implemented as a closed M1 API rather than as a public
Vulkan-buffer surface. `PrometheusGemma4E2BM1AttentionScoresRequest` accepts
only the accepted Q/K projection boundaries, their head RMSNorm weights, the
shared RoPE tables, and the final FP32 score host destination. It runs the
existing RMSNorm-to-kernel-68 continuation for Q and K with their individual
RoPE host outputs absent. The two slot-owned kernel-68 destinations therefore
remain resident together, with independent Q/K slot ids and generations; the
existing bounded slot allocator skips either pin until score completion.

Kernel 69 resolves only through `prometheus.core@1` / `kernel-69-default`.
Its direct descriptor map is binding 0 = retained positional Q, binding 1 =
retained positional K, binding 3 = distinct FP32 score output. The shared
four-binding layout also receives K at unused binding 2, which kernel 69 does
not declare or read. Its 32-byte push block carries exactly token count 15,
query heads 8, key heads 1, head width 256, and scale 1/16.

The recorder uses compute-write to compute-read barriers for both retained
RoPE sources before kernel 69, then compute-write to transfer-read for the
score output, and transfer-write to host-read for the sole final readback. The
resident-only positional path has no Q/K readback, reconstruction, upload,
staging transfer, or device copy between kernels 68 and 69. The pins release
only after score completion. Runtime cleanup owns kernel-69 pipeline teardown.

The added bounded warm-up allocation is one reusable score device/readback pair:
`8*15*15*4 = 7,200` bytes each (14,400 bytes total). The accepted positional
sources remain BF16-reexpanded FP32 storage (Q 122,880 bytes, K 15,360 bytes),
and no checkpoint-wide device residency or allocation framework was added.

The authoritative Windows native build completed on 2026-07-23. The broad
Marionette executable is not a pass signal here: hardware cases report Vulkan
runtime unavailable, and the run later reaches the known pre-existing
`PrometheusM34bA2x4UsesCanonicalDispatchFootprint` scalar-SGEMM registry
source-literal failure before crashing. This did not exercise kernel 69.

Accordingly, no live RTX score tensor, complete 1,800-value stage-local
comparison, pristine-source provenance, descriptor-freshness corpus, or clean
Vulkan-validation claim is made yet. The first remaining boundary is bridge and
harness exposure of this closed native entrypoint followed by the full live RTX
acceptance corpus. Masking and softmax remain explicitly out of scope.

### Go RTX bridge and focused authority harness (pending checkpoint-backed run)

The closed native score-chain symbol is now resolved by both Windows and Linux
dynamic bridge shims and exposed only through
`nativeRuntime.Gemma4E2BM1AttentionScores`. The Go call accepts the two fixed
projection images, the fixed Q/K head-normalization weights, shared RoPE tables,
and fixed geometry/generation identity; it does not expose a buffer handle,
Vulkan object, arbitrary descriptor, or general graph API. It allocates only
the final `[8,15,15]` FP32 result slice returned by the native final readback.

`runGemma4e2bCanonicalQKVRTX` now invokes that real bridge operation after the
accepted Q/K continuation. The accompanying stage-local checker evaluates the
same scalar sequential graph over the accepted positional Q/K values: token
major Q `[15,8,256]`, K `[15,1,256]`, KV head 0 for every Q head, coordinates
0 through 255 in order, then one post-sum multiply by `1/16`. It contains no
mask, softmax, query prescale, or attention-logit cap. The focused test asserts
the final-only readback count, zero host detours, two positional dispatches,
one score dispatch, distinct Q/K/score physical slots, exact source/destination
ranges (122,880 / 15,360 / 7,200 bytes), 1,800 outputs, and exact stage-local
bit agreement.

Focused non-live checks on 2026-07-23 passed:

- `go test ./internal/prometheus -count=1`;
- SDSL-V check for `attention_scores.sdslv`;
- DXC compilation of kernel 69; and
- `spirv-val` under the accepted Vulkan 1.4 platform contract.

The live focused invocation was attempted with
`OCT_RUN_PROMETHEUS_INTEGRATION=1`, but this workspace has no
`G4E2B_CHECKPOINT_ROOT`; the test correctly skips before it can construct the
validated external checkpoint-backed inputs. Therefore no live RTX tensor,
pristine comparison/provenance, alternating preparation corpus, transactional
rejection/recovery, allocation-reuse measurement, or zero-error Vulkan
validation claim is made. The next concrete step is to rerun this focused test
with the validated checkpoint root and a Vulkan-capable runtime. Linux native
build was attempted through `build_linux.sh` but exceeded a practical
64-second window without completing; its remaining process was terminated. No
Linux pass is claimed.

### Checkpoint-backed live score dispatch — July 23, 2026

Bounded discovery found the configured local checkpoint root. It was first
checked with
`go run ./tools/gemma4e2b_forensics -root $env:G4E2B_CHECKPOINT_ROOT -authority internal\\prometheus\\DevelopmentReport\\artifacts\\G4E2BM0\\checkpoint_authority.json`;
the authority report is valid and includes the expected
`model.safetensors` SHA-256
`2db5482b20d746879bb3ef79b5203e9075a2e2b98f54ec7c2f281c1477ddc550`
(10,246,621,918 bytes). No checkpoint payload was copied or downloaded.

The live focused Go test then passed with that root, the real bridge DLL,
hardware required, and Vulkan validation requested. It invokes the packaged
kernel-69 operation; the native result proves two positional dispatches, one
kernel-69 dispatch, one final score readback, zero Q/K host detours, and
three distinct reusable slot identities. The final tensor has 1,800 FP32
values and is bit-exact against the accepted sequential stage-local graph.
The direct source/destination ranges are Q 122,880 bytes, K 15,360 bytes,
and score 7,200 bytes. The score runtime begins in a fresh model-private
session so that the accepted positional comparison lane's retained evidence
cannot become an untracked input to the score chain.

Live execution exposed and corrected two lifecycle defects before this pass:
the default depth-two reusable ring could not retain Q and K while acquiring
a distinct score slot, so the internal bridge now requests the minimum depth
three; and resident-only RoPE had incorrectly made allocation of its device
destination conditional on a host readback request. The destination is now
always allocated for direct RoPE while its host readback remains conditional.
The Windows native build passed after that correction (139.5 s), targeted Go
tests passed, and `git diff --check` passed. Generated import-library build
outputs were restored and remain unmodified.

This is not yet a complete raw-score closure. The focused live test proves
the real resident Q-first chain and all 1,800 stage-local score bits, but it
does not yet emit the required pristine-source causal-provenance report,
structural/future-key witnesses, K-first complete-score chain, malformed
score-invocation recovery corpus, or repeated score allocation/freshness
measurements. Those are the first remaining layer-0 boundary. Masking and
softmax remain out of scope. The broad Marionette result remains separate:
Vulkan is unavailable in that lane and it subsequently reaches the known
pre-existing scalar-SGEMM registry source-literal failure before crashing.

### Lifecycle/provenance continuation — July 23, 2026

The closed score request now has an explicit model-private preparation-order
scalar: `0` retains Q then K and `1` retains K then Q. It does not expose a
Vulkan buffer, descriptor, or ownership handle. Fresh-session live execution
passes for either selected order, and the original Q-first score proof remains
bit-exact across all 1,800 FP32 scores. The Go harness also evaluates the
complete pristine positional fixture score graph beside the resident graph so
that the pending provenance table has both authorities available.

The same-session lifecycle corpus exposed the first remaining native defect.
After one successful complete score chain releases its Q/K pins, the first
positional preparation of the next chain is rejected before dispatch with
`PROM_M46_DETAIL_STALE_WEIGHT_GENERATION` (`-7406`), even when the second
chain supplies strictly larger generations (the reproduction was repeated
with generations 20/21 and 100/101). The native result records zero completed
positional dispatches for the rejected second chain. This localizes the issue
to M46 same-session weight-generation reset/reacquisition after score
completion; it is neither score arithmetic nor Q/K role ordering. The live
test intentionally preserves this reproducer. Consequently, repeated/
alternating freshness, same-session allocation settling, transactional
recovery, and the final causal-provenance closure cannot yet be claimed.

Follow-up instrumentation narrows the transition further: the second chain's
M46 weight preparation succeeds, but its immediately following M49 execution
returns `-7406` at required-weight validation before the outer positional
dispatch counter can advance. The closed diagnostic fields are therefore zero
on the outer score result: no M46 preparation rejection occurred. The first
divergence is the handoff from a successfully prepared M46 weight authority to
M49's required-weight-generation/hash validation after score completion.

### Stabilization checkpoint — July 23, 2026

This checkpoint freezes the live raw-score prefix without attempting the
remaining same-session repair. The authority-verified checkpoint-backed RTX
path has two numerically accepted fresh-session orders: Q-first and K-first.
Each dispatches the package-backed kernel 69 against the exact resident
kernel-68 positional destinations and returns all `1,800 / 1,800` FP32 scores
bit-exact against the accepted sequential stage-local graph. Q is retained in
role/slot 0, K in role/slot 1, and the distinct FP32 score destination is slot
3; direct ranges are 122,880, 15,360, and 7,200 bytes respectively. The
ring depth is bounded at three reusable resident roles, and resident-only
RoPE allocates its destination without a host Q/K readback or host detour.

The open defect is deliberately preserved and reproducible in one reusable
native session: the second chain's M46 preparation succeeds, then the
immediately following M49 required-weight validation rejects with
`PROM_M46_DETAIL_STALE_WEIGHT_GENERATION` (`-7406`) before any second-chain
positional dispatch. `observed_weight_generation` and
`requested_weight_generation` are the closed M46 diagnostic fields. They are
zero in this particular outer failure because the current result propagation
returns them for an M46 preparation failure or completed M46/M49 execution;
M49's early required-weight rejection has no separate propagated observation
yet. The exact handoff is M46 `weight_result.generation` /
`weight_result.hash` to M49 `required_weight_generation` /
`required_weight_hash`. This is evidence of that M46-to-M49 generation/hash
boundary, not evidence that M46 admission itself failed. No cause is asserted.

The Windows native build passed in 143.7 seconds. Targeted Prometheus Go tests,
kernel 68/69 SDSL-V and package checks, DXC/`spirv-val` validation, and
`git diff --check` passed for this checkpoint. Focused live first-chain
Q-first/K-first tests remain the numerical pass signals. The narrow
same-session diagnostic must continue to fail at the M46-success-to-M49-`-7406`
boundary until its lifecycle state is repaired. Linux has not completed within
the bounded build window; no Linux pass is claimed. The broad Marionette
Vulkan-unavailable run and its pre-existing scalar-SGEMM registry source-literal
failure remain unrelated.

Still open before raw pre-mask score construction can be closed are
same-session lifecycle reuse, pristine-score causal provenance, malformed-call
rejection/recovery, and allocation/freshness corpus evidence. Masking,
softmax, V aggregation, output projection, residual work, FFN, PLE, M2, and
later layer-0 work have not begun. Resume with architectural regrouping, not
by automatically implementing the `-7406` repair. The first focused command
is `go test -run TestGemma4E2BM1CanonicalQKVRTX -count=1 -v ./internal/prometheus`
with the validated checkpoint root, reactor DLL, hardware-required, and
Vulkan-validation environment variables set as above.
