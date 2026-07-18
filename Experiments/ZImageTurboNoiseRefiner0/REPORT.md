# Z-Image Turbo `noise_refiner.0` Oct experiment

This package is the EVT-2 laboratory vessel. Its scope is fixed: readable
reduced semantic proofs and deterministic witnesses. It is not a tensor
framework, a model importer, or a production GPU implementation.

## O0 — structural vessel (2026-07-18)

The compiled Oct contracts prove the padded-text frame-33 coordinate mapping,
the `[32,48,48]` complex-pair RoPE partitions, uncentered RMSNorm, and the
AdaLN order `attention scale, attention gate, MLP scale, MLP gate`. The Oct
artifact writes the deterministic structural witness under the development
report artifacts. A separately compiled Oct executable calls
`PrometheusMatMul` with a tiny `[1,2] x [2,1]` contraction. On this host it
reported `backend_used=cpu` and `fallback(prometheus_unavailable)`; that is a
recorded conservative CPU route, not GPU evidence.

## O1 — source-weight census (2026-07-18)

The exact contiguous 361,820,672-byte BF16 range containing all thirteen
source tensors was read from the pinned official Comfy conversion. No source
weight overflowed on BF16-to-FP16 conversion, including W2. The scan uncovered
a prior diagnostic-only FP16 decoder error: normal binary16 exponents had
binary32 bias applied without removing binary16 bias, inflating normal values
by `2^15`. The corrected decoder is covered by normal and subnormal tests.

## O2 — corrected diagnostic replay (2026-07-18)

The established local `OCT_EVT2_CACHE` and `OCT_EVT2_ORACLE` roots validated
the input, timestep, and cache identities. The corrected fixed-order FP32
diagnostic replay was deterministic across two runs: its final F32 SHA-256 is
`4aff8bf19cfbfc9aebf2e8aa78ef91fb7bb5c117f98504080ed1bc3b206e0c43`.
W2 and final output are finite, so the old W2 non-finite disappears completely.
This withdraws the historical all-FP16 rejection only as an inference from the
defective diagnostic; it does not establish a production precision policy.

## O3 — selected AdaLN Octagon witness (2026-07-18)

Compiled Oct loaded a typed selected witness from the O2 timestep projection
and proved the source split order and lane transforms. The emitted Octagon
witness SHA-256 is
`ef7d8580aa88205375b9d56cc4f523fc79788da3ac2e8b9027c97f93fa60d3e0`.
The selection is deliberately small: it proves the structural transform with
real values, but does not claim a full-width AdaLN projection or a storage
policy comparison.
