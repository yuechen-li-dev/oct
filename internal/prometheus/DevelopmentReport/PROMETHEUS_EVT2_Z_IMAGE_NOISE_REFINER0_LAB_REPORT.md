# EVT-2 Z-Image `noise_refiner.0` laboratory report

## 1. Abstract

EVT-2 establishes a reproducible Oct-led laboratory reference for one
`noise_refiner.0` block. The accepted M1B policy is FP16 cached weights,
expanded to FP32 at arithmetic load, with FP32 activations and reductions.
It is ready for a conservative resident SDSL-V implementation.

## 2. Model and source authorities

Model: `Tongyi-MAI/Z-Image-Turbo@f332072aa78be7aecdf3ee76d5c247082da564a6`.
Source: `Tongyi-MAI/Z-Image@26f23eda626ffadda020b04ff79488e1d72004cd`.
Checkpoint SHA-256: `2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6`.

## 3. Experimental methodology

Oct owns structural proofs and typed selected witnesses. Conservative Go/Prometheus
equipment supplies fixed-order CPU contractions and payload scanning. Reduced
experiments prove layout/semantics; full captured replays settle ranges and
witness identities.

## 4. Oct experiment architecture

O0 bootstrapped with `oct new experiment ZImageTurboNoiseRefiner0`. Compiled
Oct proves AdaLN, RMSNorm, QKV/RoPE, softmax, full W2 dot, and adverse source/
cache rows. O19 reran M9 compiled: 9 pass, zero interpreted fallback.

## 5. Prometheus reference primitive policy

The diagnostic executor uses explicit FP32 products/sums and fixed logical
order. It has no production shader, adaptive numerical path, hidden
quantization, cross-stage mutation, or cross-stage fusion.

## 6. Tensor inventory

All 13 source BF16 tensors are finite and representable in FP16 without
overflow or saturation. The transformed package is 361,820,672 bytes,
aggregate SHA-256 `a1ba526898a2a7522b31167c6d5e1bc48c39a8708cf5c3ad88b193e536ca5d5e`.
The detailed inventory is `tensor_inventory.json`.

## 7. Exact block mathematics

`AdaLN -> RMSNorm1 -> attention scale -> QKV -> Q/K RMSNorm -> Q/K RoPE ->
scaled non-causal stable attention -> output projection -> RMSNorm2 -> gated
attention residual -> FFN RMSNorm1 -> MLP scale -> W1/W3 -> SiLU(W1)*W3 ->
W2 -> RMSNorm2 -> gated MLP residual`.

## 8. Tensor orientations and layouts

Source matrices are `[out,in]`; cache matrices are row-major `[in,out]`.
Fused QKV is contiguous `Q|K|V`, each `[1,1024,30,128]`; token/head/channel
index is `((token*30)+head)*128+channel`.

## 9. AdaLN findings

The 15,360 lane projection splits contiguously into attention scale, attention
gate, MLP scale, MLP gate. Scales use `1+raw`; gates use `tanh(raw)` and
broadcast over tokens. O3 compiled Oct proves this order.

## 10. RMSNorm findings

RMSNorm uses no mean subtraction and epsilon `1e-5` inside the inverse-root
calculation. Q/K learned scales are applied after their RMS reduction.

## 11. RoPE frame-33 derivation

Canonical coordinate is frame 33. Apply Q/K-only three-axis RoPE with scalar
axis widths `[32,48,48]`, even/odd pairs, theta 256, and the established
token-coordinate mapping. O4 proves a selected frame-33 pair and magnitude
preservation.

## 12. QKV findings

The physical fused projection is `[1024,11520]`; logical Q, K, V segments are
each width 3840. O4 proves the offsets and O9/O12 prove selected adverse rows
under BF16-source versus FP16-cache expansion.

## 13. Attention findings

Scores are FP32 dot products scaled by `1/sqrt(128)`. Softmax subtracts the
row maximum before exponentiation and probabilities/V aggregation remain FP32.
O6 proves a real 1,024-token row in compiled Oct.

## 14. FFN findings

W1 and W3 project the modulated FFN input to width 10,240; hidden is
`SiLU(W1)*W3`; W2 projects to 3,840 before its second RMSNorm.

## 15. W2 non-finite root cause

The historical non-finite was decisively decoder-caused: normal FP16 exponents
were decoded 15 exponent bits too large, scaling ordinary normals by 32768.
Corrected IEEE decoding makes all measured W2 products and partial sums finite.
The old all-FP16 rejection is withdrawn only as an inference from that defect.

## 16. Per-tensor precision policy

Every immutable weight: source BF16, package FP16, arithmetic FP32 expansion,
FP32 accumulation. Every activation: FP32. The complete table is
`precision_policy.json`; FP16 activation candidates from O14–O17 are excluded
from the accepted reference policy.

## 17. Accumulation and cast policy

Use fixed logical reduction order and FP32 multiply/add. Expand BF16 ABI input
and timestep to FP32 at ingress. Do not cast attention projection before its
RMSNorm; do not cast W2 before its RMSNorm or final gated residual.

## 18. Range and error analysis

Corrected W2 range is `[-62296.31640625,142581.390625]`; attention projection
is `[-7867.749,136264.64]`. Both exceed finite FP16 65,504. Independent
batch-two source summaries corroborate projection 136,192 and W2 142,336.

## 19. Canonical stage witnesses

There are 34 deterministic FP32 stage payloads. Two O19 generations match all
36 bundle files byte-for-byte. Projection artifact SHA-256 is
`f9350d37b46a26d132d4a1e6c80c984ebce87f6f3fe4fd9eb274ffbfd631f480`.
Final diagnostic F32 SHA-256 is
`4aff8bf19cfbfc9aebf2e8aa78ef91fb7bb5c117f98504080ed1bc3b206e0c43`.

## 20. Historical Comfy differential comparison

The captured BF16 compatibility output SHA-256 is
`6dae8d91b2118e7c425ee16d5db214ec0d8df1e988487e855aebd1fe81575873`.
Corrected diagnostic versus it is relative L2 `0.003709630779937986`, L-infinity
`0.41104888916015625`; it is compatibility evidence only, not the oracle.

## 21. Diagnostic Go executor comparison

The Go executor is diagnostic-only. Its corrected replays are deterministic
and provide the full local payload witnesses, but Oct structural contracts and
the pinned model/source remain semantic authority.

## 22. Limitations

Historical internal activation storage is not established. Optional post-norm
FP16 storage has only one ABI-compatible capture. No GPU numerical portability
claim is made; M1B must compare the reference shader to the supplied witnesses.

## 23. Production implementation contract

Use `evt2_m1b_production_handoff.json` plus the shader, memory, and execution
contracts. It preserves the M1a resident ABI unchanged.

## 24. Memory implications

Immutable FP16 weights cost 345.059082 MiB. The execution plan requires
115.0078125 MiB minimum FP32 scratch; reserve 128 MiB aligned. Do not allocate
a persistent 690.118164 MiB FP32 weight mirror.

## 25. Exact SDSL-V shader portfolio recommendation

Implement six reference boundaries: AdaLN; attention QKV/preprocess;
attention; attention projection/norm/residual; FFN W1/W3/gate; and W2/norm/
residual. See `production_shader_contract.json`.

## 26. Exact resident execution-plan recommendation

Use FP32 buffers A/B `[1024,3840]`, C `[1024,11520]`, D `[1024,10240]`, and
one-head FP32 logits/probabilities. Reuse C after attention for W1/gated hidden
and A after its input role for W2/final output.

## 27. Acceptance tolerances

Structural fields, layout, operation order, and finite state are exact. The
CPU canonical bundle is exact at all 34 stage hashes. The first GPU reference
shader must match selected canonical FP32 witness values or treat divergence as
a fault to localize; no unmeasured broad numerical tolerance is authorized.

## 28. One-shot GPU implementation plan

Implement the six reference shaders in stated order, preserve every witness
boundary, validate selected stage witnesses, then introduce only the listed
intra-boundary fusions. Do not add clamping, saturation, FP16 activations, or
cross-boundary fusion in M1B.

## 29. Reproduction instructions

Set `OCT_EVT2_CACHE` to the root containing `layers` and `OCT_EVT2_ORACLE` to
the pinned oracle revision root. Run compiled Oct M9, then:
`go run ./tools/zimage_canonical_reference -cache-root $env:OCT_EVT2_CACHE
-oracle-root $env:OCT_EVT2_ORACLE -out .tmp\\evt2-oct-oracle\\canonical
-capture -projections-out internal\\prometheus\\DevelopmentReport\\artifacts\\Evt2OctOracle\\canonical_stage_projections.json`.

## 30. Conclusion

**SUCCESS / COMPLETE / READY FOR PRODUCTION M1B–M1E / OCT LAB AUTHORITY
COMPLETE.** The initial production pass is mechanical under the conservative
FP32-activation policy; later activation-FP16 optimization requires a separate
ABI-compatible validation campaign.

Deterministic closeout identities: stage manifest
`0cab3d8fe179e70058cb22b37994413649f257268566b2c1dfb1254d2daeae65`;
stage projections `f9350d37b46a26d132d4a1e6c80c984ebce87f6f3fe4fd9eb274ffbfd631f480`;
shader contract `8a311ee26e42d3e00cb6df6e5ffb3466c03e4a08bcaba43ac753f8d102b6e109`;
memory contract `715ec6c201c0d88b0d6dd6ce3660345a97e7eaa85d21c9c9d82a28e94191e974`;
execution contract `85305f917a14f1485862f24724bf5b81850e8714ed4be1e6c6ba439b0366efa3`;
and production handoff `629be9339c87bd37a996ea1b2e169271cbc66c5971faa1c638de4ca6cbc4a590`.
