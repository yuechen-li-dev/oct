# EVT-2 M2B: ContextRefiner family

Status: `COMPLETE`

ContextRefiner is the source-pinned unmodulated `ZImageTransformerBlock` over
the text/context stream. Captured 15x2560 prompt embeddings are padded by
repeating the final row to 32 tokens, then transformed through `cap_embedder`
to `ContextEmbedding.FP32`. Both blocks use width 3840, 30 heads of width 128,
Q/K RMSNorm (epsilon `1e-5`), three-axis RoPE with theta 256 and axes
`[32,48,48]`, non-causal attention, and a bias-free 3840->10240->3840
SiLU(W1)*W3 FFN. There is no AdaLN, timestep input, learned gate, BF16 ingress,
or activation FP16 path.

This is structurally distinct from NoiseRefiner even though compatible physical
FP32 primitives are reused. NoiseRefiner consumes the image `ModelEmbedding`
stream; ContextRefiner consumes `ContextEmbedding`. The generated lock records
two independent resident streams:

    NoiseRefiner0 -> NoiseRefiner1
    ContextEmbedding -> ContextRefiner0 -> ContextRefiner1

There is no legal direct NoiseRefiner output to ContextRefiner input edge. The
two ContextRefiner blocks share one compiled assembly but use distinct immutable
11-tensor FP16 packages (353,925,632 bytes each):
`c08b908a...e412ae` for block 0 and `30268c3b...dfc95f` for block 1. Weight
bytes expand to FP32 only at arithmetic use; reductions, softmax, RoPE,
projections, FFN, and residuals remain FP32.

The lock expanded from `b3660657...d55ce67` to
`4b4aeb04...7a6970`. Its new semantic, production, and audit identities are
separately recorded in `artifacts/Evt2M2b/m2b_lock_identity.json`; descriptors,
static audit schedule, and native resolvers are generated from that lock rather
than accepting a caller family, cache aggregate, ABI, or shader list.

## Compiled result

The resident ContextRefiner transition snapshots the completed block-0 FP32
boundary into a device-local immutable predecessor buffer at the first block-1
execution. Subsequent block-1 warm and static-audit executions consume that
same buffer, preventing an accidental ContextRefiner1-to-ContextRefiner1
feedback path. The rebind validates the complete package, stages uploads,
updates descriptors, commits atomically, and invalidates stale output/replay
acceptance.

On the real native Vulkan lane, block 0’s 16-stage audit had worst projection
Linf `3.8147e-5`; its final matched its canonical authority at relative L2
`3.19051e-7`, Linf `1.83105e-4`. Block 1’s corresponding values were
`6.86646e-5`, `4.64152e-7`, and `6.10352e-4`. Both are inside the declared
relative-L2 `1e-5` and Linf `1e-3` bound. Each block also completed ten warm
runs with zero buffer allocation, weight upload, pipeline creation, descriptor
growth, payload reads, or host tensor work.

The clean-process multi-family command ran the complete NoiseRefiner and
ContextRefiner resident chains under the same generated lock. NoiseRefiner1
remained within relative L2 `1.27829e-6`; the independent ContextRefiner
results above also passed. This is the valid four-block prefix proof for the
pinned parallel-stream dependency graph, not a fabricated cross-family tensor
handoff. The exact command and final metrics are in
`artifacts/Evt2M2b/m2b_four_block_prefix.json`.

The ContextRefiner memory contract is a 512 MiB resolved steady-state ceiling,
with a 353,925,632-byte immutable package and a 1 MiB audit arena. The shared
window remains sized to the larger already-proven NoiseRefiner package. The
resident owner does not expose a device-wide peak-VRAM counter, so the report
does not mislabel the allocation ceiling as telemetry.

## Evidence and next handoff

`artifacts/Evt2M2b/` contains the source audit, both inventories and cache
identities, canonical authorities, shader split, ABI, lock identities,
audit profiles/results, resident-chain and four-block evidence, memory/timing,
fault/replay contracts, and the M2C handoff. Large payloads are not committed.

M2C remains intentionally limited to a representative MainTransformer family
after its concatenated image/context FP32 boundary, modulation, gates, and
parameter inventory are frozen. It must not be inferred from ContextRefiner.
See `artifacts/Evt2M2b/evt2_m2c_handoff.json`.

Convergence outcome: `SUCCESS`

Milestone state: `COMPLETE`

EVT-2 state: `READY FOR M2C`

Compiled-model status: `CONTEXT REFINER ASSEMBLY COMPLETE`
