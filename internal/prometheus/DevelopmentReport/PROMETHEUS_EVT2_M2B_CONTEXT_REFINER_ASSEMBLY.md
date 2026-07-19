# EVT-2 M2B: ContextRefiner family

Status: `IN PROGRESS` — the source, closed packages, and canonical two-block
laboratory are complete; the native compiled ContextRefiner assembly is not yet
implemented and therefore no RTX, lock, or four-block-prefix claim is made.

ContextRefiner is the source-pinned unmodulated `ZImageTransformerBlock` over
the text/context stream. The captured 15x2560 prompt embeddings are padded by
repeating the final row to 32 tokens, then transformed through `cap_embedder`
to `ContextEmbedding.FP32`. Both blocks use width 3840, 30 heads of width 128,
Q/K RMSNorm (epsilon `1e-5`), three-axis RoPE with theta 256 and axes
`[32,48,48]`, non-causal attention, and a bias-free 3840->10240->3840
SiLU(W1)*W3 FFN. They have no AdaLN, timestep input, or gates.

That is structurally distinct from NoiseRefiner even though the physical QKV,
attention, RMSNorm, and FFN primitives are reusable. NoiseRefiner is modulated
and gated over image `ModelEmbedding`; ContextRefiner is unmodulated and
ungated over `ContextEmbedding`. There is no legal direct NoiseRefiner output
to ContextRefiner input edge. Both ContextRefiner instances are identical
assembly with different immutable weights.

Each ContextRefiner package contains 11 tensors and 353,925,632 FP16 bytes.
Their aggregates are `c08b908a...e412ae` (block 0) and
`30268c3b...dfc95f` (block 1). All source tensors are finite and
FP16-representable; conversion found no overflow. The laboratory policy is
FP16 immutable weights expanded to FP32 for all arithmetic, reductions,
softmax, RoPE, and residuals. It has no activation FP16 or clamp path.

The deterministic canonical ContextRefiner0 input is
`e5cf72...511a9c`; its final is `d2b816...ae6df`. ContextRefiner1 consumes
that exact FP32 boundary and produces `08377e...220b0`. Each canonical bundle
contains 18 stage payloads and was generated twice with unchanged identities.

The next narrow implementation slice is a second lock-resolved native family:
an 11-weight, F32 `ContextEmbedding` ingress that bypasses the NoiseRefiner
BF16/timestep/AdaLN pipeline while reusing only contract-compatible physical
shaders. It needs a separate descriptor family, transactional 0->1 rebind,
static audit schedule, and clean-process RTX numerical closure before the
manifest/lock flow can be extended. MainTransformer remains excluded; the exact
M2C boundary is in `artifacts/Evt2M2b/evt2_m2c_handoff.json`.

Convergence outcome: `MEANINGFUL PROGRESSION`

Milestone state: `IN PROGRESS`

EVT-2 state: `READY WITH REQUIRED ADAPTATIONS`

Compiled-model status: `CONTEXT FAMILY PARTIAL`
