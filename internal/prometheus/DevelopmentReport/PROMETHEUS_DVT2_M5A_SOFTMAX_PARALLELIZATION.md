# Prometheus DVT-2 M5a: focused attention softmax parallelization

Date: 2026-07-20

## Outcome

**MEANINGFUL PROGRESSION.** The three production attention shaders were audited,
then a bounded canonical-order-preserving parallel softmax route was built,
generated, rebuilt into the native Vulkan reactor, numerically validated, and
measured in the canonical nine-evaluation Prefetch image path. It is rejected:
the canonical image remained bit-identical, but wall time regressed from
**165.439 s** to **175.232 s**. The source and generated headers were restored
to the validated M4 baseline; no negative route remains in production.

## Source audit and candidate

`nr0_attention_streaming` (1024 keys),
`context_refiner_attention_streaming` (32 keys), and
`main_transformer_joint_attention_streaming` (1056 keys) each use a 256-thread
workgroup and workgroup-local FP32 scores/probabilities. Each had lane 0 do the
max pass, `exp` plus denominator pass, and normalization pass while the other
255 lanes were idle. Probability-times-V is already distributed over the 128
value channels. Audit writes are limited to selected rows.

The tested route used per-lane maxima and a fixed 256-leaf binary tree,
distributed exponentiation/normalization by the existing score stripes, and a
single lane-0 denominator addition in ascending logical-key order. It used no
atomics, no global attention matrix, no precision change, and no packed-FP16 or
cooperative-matrix change. Starting every max leaf at key zero preserves the
prior ordered non-finite behavior: key-zero NaN poisons the result, later NaNs
do not win ordered comparisons, and infinities remain ordered.

## Generated code and validation

All three candidate sources passed `oct sdslv check`; DXC generated HLSL/SPIR-V
and the Windows native build completed. Generated code contained the fixed tree
and lane-distributed exp/normalization rather than a lane-0 serial max/exp/
normalization loop. The candidate raised synchronization from 2 barriers to
13 and workgroup storage to 9,476 bytes for the largest 1056-key row.

`go run ./tools/evt2_payload_check` passed. The real retained-stream
MainTransformer replay passed representative relative L2 `8.38066e-7` and final
30-layer relative L2 `1.02005e-5` (limit `5e-5`). The canonical Prefetch smoke
also produced the accepted PNG SHA-256
`7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613`.

## Performance decision

| Route | Canonical Prefetch wall | Decision |
| --- | ---: | --- |
| accepted M4 | 165.439 s | baseline |
| M5a canonical-order parallel route | 175.232 s | reject (+9.794 s, +5.92%) |

The candidate is numerically correct but full-image-negative. Its extra fixed
tree barriers and shared-memory traffic are a concrete bounded explanation to
investigate; no aspirational attention reduction is claimed. The optional
parallel denominator tree was not run because the prerequisite canonical-order
route was not green.

## Artifacts and M5b handoff

Machine-readable audit and rejection evidence are in
`artifacts/Dvt2M5a/`. The full smoke output is local at
`out/prometheus/dvt2_m5a_prefetch/` and is intentionally not committed.

M5b should profile barrier/shared-memory cost against the original serial
softmax and test only a barrier-conscious attention reduction topology that
keeps the lane-0 canonical denominator. It must first pass the same real
MainTransformer, 30-layer, PNG, and full-Prefetch timing gates before any
production promotion.
