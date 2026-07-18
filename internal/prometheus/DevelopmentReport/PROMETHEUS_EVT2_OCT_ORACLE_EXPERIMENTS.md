# PROMETHEUS EVT-2 — Oct Oracle Experiments

## Campaign state

Convergence outcome: **SUCCESS**
Campaign state: **COMPLETE**
EVT-2 state: **READY FOR PRODUCTION M1B–M1E**
Oracle status: **OCT LAB AUTHORITY COMPLETE**

This living report supersedes no source authority. It records the Oct campaign
after M1b0-R was rescaled from a Go diagnostic implementation to an Oct-led
laboratory program.

## O0 — Oct vessel and structural proof (2026-07-18)

Question: can Oct express and verify the source-derived semantic seams before
any production shader exists?

Method: bootstrap `Experiments/ZImageTurboNoiseRefiner0` with
`go run ./cmd/oct new experiment ZImageTurboNoiseRefiner0`; implement compiled
Oct contracts for frame coordinates, pairwise RoPE, RMSNorm, and AdaLN; emit an
Oct JSON witness; compile a separate tiny `PrometheusMatMul` vessel.

Result: all three Oct contracts passed in compiled execution with no fallback.
The witness hash is
`6292c9e2efbd9f89e010b345f20b9e5bfbc1b38e33a273c4005867226ce21bac`.
The tiny matrix vessel produced 11 and explicitly reported the Prometheus CPU
fallback (`prometheus_unavailable`).

Interpretation: Oct is established as the readable structural authority; the
Prometheus seam is available only as conservative CPU equipment on this host.
This is not GPU evidence.

Confidence: high for frame-33 / operation-order structure; none for hardware
performance or GPU numerical portability.

Next question: are any real source weights outside FP16 range?

## O1 — tensor census and W2 diagnostic audit (2026-07-18)

Question: does source-weight conversion explain the historical W2 non-finite?

Method: download exactly the contiguous official 13-tensor range, hash it,
scan each BF16 value deterministically, and compare IEEE BF16 semantic values
with nearest-even FP16. Repeat the generated inventory byte-for-byte.

Result: all thirteen tensors are finite and have zero FP16 conversion overflow
or saturation. W2 is in `[-1.796875, 1.7421875]`; it has only three nonzero
values that underflow to FP16 zero and maximum conversion error
`2.9802322387695312e-08`. The repeated inventory SHA-256 is
`815298da249f2b817fabda59ff72a7ce4873aba80f597c57dfca3d13fc04cbd2`.

The scan exposed a fault in the diagnostic-only Go FP16 decoder: its normal
exponents were 15 bits too large, turning FP16 1.0 into 32768. The fix and
normal/subnormal tests are local to `internal/prometheus/zimage`.

Interpretation: source-weight overflow is eliminated as the W2 cause. The
historical non-finite occurrence is preserved as a diagnostic observation, but
the old final hash is more clearly non-authoritative: its execution was
confounded by the decoder defect. No storage policy is accepted from it.

Confidence: high for the source range and the decoder fault; incomplete for
the full corrected activation path.

Next question: replay the exact captured input and timestep through the
corrected decoder under BF16, FP16, and mixed policies.

## O2 — corrected full-path diagnostic replay (2026-07-18)

Question: after recovering the established M0.5/M0.75/M1a payload convention,
does corrected FP16 decode retain a W2 non-finite or move the first discrepancy?

Method: set `OCT_EVT2_CACHE` to the recovered local EVT-2 root and
`OCT_EVT2_ORACLE` to its pinned revision root; validate the cache aggregate and
the captured input/timestep identities through `LoadNoiseRefiner0PayloadBundle`.
Run the fixed-order FP32 diagnostic executor twice using cached FP16 weights
expanded through the corrected IEEE decoder. Scan W2 and final output, then
compare final F32 to the captured BF16 compatibility output.

Result: all decoder scalar checks, all 63,488 finite binary16 bit patterns, and
all real cached values passed. The permanent cache regression scans every
cached tensor and rejects the historical 32768 normal-value scaling. Both full
replays produced final SHA-256
`4aff8bf19cfbfc9aebf2e8aa78ef91fb7bb5c117f98504080ed1bc3b206e0c43`.
W2 is finite (`[-62296.31640625, 142581.390625]`) and final output is finite
(`[-6.843756198883057, 40.04807662963867]`). The historical W2 non-finite
disappears completely. Relative L2 difference against the historical BF16
compatibility capture is `0.003709630779937986`; its L-infinity difference is
`0.41104888916015625`.

Interpretation: the old all-FP16 rejection is withdrawn **as a conclusion from
the defective Go diagnostic**: the observed W2 failure was caused by decoding
ordinary FP16 normals 32768 times too large. This does not accept FP16 as the
production storage policy; Go is still diagnostic-only and the Oct-led BF16,
FP16, and mixed-policy experiments remain necessary.

Confidence: high that the historical non-finite was a decoder artifact; medium
for this one corrected full-path diagnostic replay; none yet for canonical
production tolerances or per-tensor storage selection.

Next question: establish the source-derived pre-attention, QKV, RMSNorm, and
AdaLN witnesses in Oct, then compare BF16-expanded and FP16-expanded policies
at those auditable boundaries.

## O3 — canonical Oct AdaLN selected witness (2026-07-18)

Question: can compiled Oct prove the four-way AdaLN split and transforms using
values selected from the validated O2 captured-path replay?

Method: store eight selected finite lane values and expected outputs in a typed
Octagon fixture; load it with `LoadOctagon<AdaLnSelectedWitness>` in compiled
Oct; apply the lane rule directly; and emit the resulting typed witness.
The fixture covers attention-scale, attention-gate, MLP-scale, and MLP-gate
values from the O2 timestep projection. It is a selected witness, not a claim
that Oct has loaded all 15,360 values or performed the projection itself.

Result: the compiled test passed with zero fallback. It confirms contiguous
lane order `[attention_scale, attention_gate, mlp_scale, mlp_gate]`, `1 + x`
only for scale lanes, and `tanh(x)` only for gate lanes. The typed witness
SHA-256 is `ef7d8580aa88205375b9d56cc4f523fc79788da3ac2e8b9027c97f93fa60d3e0`.

Interpretation: Oct now owns the executable AdaLN split/transform contract and
can consume deterministic typed selected payloads without a new dynamic tensor
feature. O3 also exposed and repaired a narrow interpreted `LoadOctagon`
inconsistency: signed numeric data literals are now materialized as data, with
a corpus regression. The new signed-data test passes in both execution modes;
an unrelated pre-existing compiled loader gap for dimensioned/record/enum
fixtures remains outside this experiment's narrow repair.

Confidence: high for four-way split and transform semantics; medium for the
selected replay values; no claim yet for full-width precision sensitivity.

Next question: use the same typed-witness route to prove RMSNorm and QKV
reshape/RoPE boundary behavior against selected full-path stage values.

## O4 — selected QKV and frame-33 RoPE witness (2026-07-18)

Question: do the token-local fused QKV offsets and Oct's frame-33 RoPE formula
match selected values from the corrected full-path replay?

Method: load a typed Octagon fixture containing token-zero representatives at
the three fused segment starts and one positive Q-norm complex pair. In compiled
Oct, split `[Q | K | V]` at reduced width one and rotate the pair at coordinate
33, first-axis scalar width 32, pair index 4. Compare the computed pair to the
captured Q-RoPE stage and check magnitude preservation.

Result: the compiled test passed with zero fallback. The selected fused values
confirm Q at offset 0, K at offset 3840, and V at offset 7680 for token zero.
The selected Q-norm pair rotates to `[-0.4407204, -0.1587258]` under the
frame-33 formula. The emitted witness SHA-256 is
`8b4c9658399770b1333426af13981eed31e939d434f4ce31632b2ee9217b33d9`.

Interpretation: this is a structural proof of QKV local split and a numerical
selected-stage confirmation of the canonical RoPE sign/frequency convention.
It generalizes across tokens because the fused split is a fixed per-token
contiguous layout; it is not a full tensor comparison or a proof of all three
axis coordinates.

Confidence: high for split/rotation semantics; medium for selected stage
agreement; full attention and Q/K norm precision comparisons remain open.

Next question: perform selected RMSNorm reduction-order and BF16-versus-FP16
comparisons, then use reduced softmax/attention witnesses before detailed FFN
product and partial-sum instrumentation.

## O5 — complete selected real Q RMSNorm row (2026-07-18)

Question: does the captured Q-normalization boundary use uncentered RMSNorm,
`epsilon = 1e-5`, learned scale, and fixed left-to-right reduction?

Method: deterministically export the complete first Q row/head (128 values),
the validated cached Q-norm scale, and the captured Q-norm output as typed
Octagon. Compiled Oct recomputes all 128 outputs using the explicit scalar
reduction, then checks every channel. The O1 source census independently shows
that this 128-value Q scale is exactly representable in FP16.

Result: the compiled test passed with zero fallback. The Oct witness SHA-256 is
`d5449a720ba48de7ad38fd0b0a899502e4563d9096e0a80bbea1948a57763b34`.

Interpretation: the Q/K normalization formula, epsilon placement, no-mean
property, and learned-scale multiplication are now proven against a complete
real head-width row. This proves the boundary formula, not that QKV projection
itself is insensitive to FP16 weight storage.

Confidence: high structural and selected-row numerical agreement.

Next question: does an entire attention probability row use stable softmax and
the documented probability normalization?

## O6 — complete selected real attention softmax row (2026-07-18)

Question: is attention softmax stable and numerically consistent at a full
1,024-token row without evaluating the full 30-head attention matrix on CPU?

Method: export token-0/head-0 logits and probabilities from O2 as typed
Octagon. Compiled Oct subtracts the row maximum, exponentiates, uses a fixed
left-to-right denominator, and checks all 1,024 probabilities plus their sum.

Result: the compiled test passed with zero fallback; the row sum is one within
`1e-10`. The witness SHA-256 is
`89ab12ebbe8b6a7a20b421ad50ec728f2dd9d1c92f6b6e5b79ac203ada298714`.

Interpretation: a selected full-length row confirms stable-softmax order and
normalization. It generalizes structurally because every attention row uses the
same reduction program; it is not a full matrix range audit.

Confidence: high for formula and selected-row evidence; attention projection
precision policy remains open.

Next question: instrument selected FFN/W2 products and partial sums under
BF16-expanded, cached-FP16-expanded, and mixed representations.

## O7 — complete selected real W2 product and partial-sum witness (2026-07-18)

Question: does the corrected cached-FP16 path contain a hidden W2 product or
accumulation non-finite at the previously implicated stage?

Method: export the complete token-0/channel-0 W2 reduction: 10,240 real gated
hidden values and their column-0 cached FP16 weights. Compiled Oct performs the
same fixed left-to-right dot product. Conservative Go equipment separately
records FP32 product and partial-sum ranges for the same operands.

Result: the compiled test passed with zero fallback. The Oct dot differs from
the FP32 diagnostic witness by `5.38e-4`, within the predeclared `1e-3`
tolerance caused by Oct's Float arithmetic versus the diagnostic's FP32
accumulation. The explicit FP32 replay has no non-finite product or partial
sum; products range from `-748.3084` to `409.52567`, partial sums from
`-4047.3066` to `549.47736`, and both its fixed-order result and captured W2
output are `-4012.8704`. Witness SHA-256:
`d91a9076956b5aeaeb1e69a00ea351939b911501f75d1e9515199d01d3bf469d`.

Interpretation: the mandatory historical W2 failure is not an activation,
multiplication, or FP32 fixed-order accumulation overflow at this complete real
reduction. The selected result generalizes structurally to W2's reduction
order, but it does not settle BF16-vs-FP16 error across all W2 columns.

Confidence: high for corrected cached-FP16 selected W2 finiteness and range;
cross-policy error analysis is still required before a production policy.

Next question: compare selected complete reductions under original BF16 weights
expanded to FP32, cached FP16 expanded to FP32, and justified mixed candidates.

## O8 — source-BF16 versus cached-FP16 W2 policy comparison (2026-07-18)

Question: did the cache's BF16-to-FP16 W2 conversion change the complete
selected W2 reduction that was historically blamed for non-finiteness?

Method: recover W2 output-channel zero's contiguous 10,240 BF16 source row
from the pinned source range and compare it term-for-term to cache column zero,
whose physical layout is the documented transpose. Apply both weight vectors to
the same O7 gated-hidden row in compiled Oct and conservative fixed-order FP32.

Result: all 10,240 source BF16 W2 values are exactly equal to their cached FP16
values after expansion. Maximum per-weight difference and final dot difference
are both zero; both reductions equal `-4012.8704`. The typed witness SHA-256 is
`4e47bd929ccb66c6411c37445b272ee1622e500ecc9f11e9cc3c6c7dfc5aa3f2`.

Interpretation: source-weight conversion is excluded for this complete W2
reduction, complementing the O1 whole-tensor conversion census. The historical
failure was the decoder defect, not W2 source storage or multiplication range.
This is strong evidence for FP16-safe W2 *weight storage*; it does not by
itself certify every other tensor or a full-block production precision policy.

Confidence: high for W2 source/cache equivalence and selected complete dot.

Next question: extend cross-policy comparisons to QKV/FFN W1/W3 and use those
measurements to propose a per-tensor policy rather than a single-dtype rule.

## O9 — QKV/W1/W3 source-policy complete selected projections (2026-07-18)

Question: do the largest remaining projections preserve their source BF16
weights in the transformed FP16 cache at a complete 3840-term real dot?

Method: for token zero/output channel zero, compare each contiguous source row
with its documented transposed cache column for QKV, W1, and W3. Use the real
modulated input for each path and prove source-BF16 versus cached-FP16 dots in
compiled Oct.

Result: QKV and W1 are exact across all 3840 selected weights. W3 has one
weight delta of `2.9802322e-8`, but its source-BF16 and cached-FP16 FP32 dot
outputs remain exactly equal (`20.99694`). QKV and W1 outputs are respectively
`0.11094941` and `4.170632`, with zero policy delta. All three compiled Oct
theory cases passed with zero fallback.

Interpretation: selected complete projections materially support FP16 storage
for QKV/W1/W3, while retaining the broader rule that all reductions expand to
FP32. These samples are not a whole-tensor error bound or final all-block
policy.

Confidence: high selected complete-dot evidence; representative rows and
remaining tensors still need coverage.

Next question: select adversarial/high-range rows and complete per-tensor
conversion/range evidence before proposing package and runtime dtypes.

## O10 — adverse numerical census and activation-range boundary (2026-07-18)

Question: where are the adverse source/cache rows and captured activation
extrema, and which values cross finite FP16 range?

Method: scan every source/cache tensor row for maximum magnitude, conversion
error, relative conversion drift, and positive/negative cancellation. Scan all
34 corrected diagnostic stage payloads for extrema and their flat indices.

Result: all 13 tensors and 34 stages were scanned. The largest source weight is
W3 row 3120 (`2.609375`); the largest source/cache per-weight conversion error
is `2.9802322387695312e-08`. W2 reaches
`[-62296.31640625, 142581.390625]`, and attention projection reaches
`[-7867.749, 136264.64]`: both exceed finite FP16 range. Gated hidden remains
within range (`[-4194.241,3668.6272]`). Selected logits remain modest
(absolute maximum `4.8397255`), while Q/K RoPE absolute maxima are `6.576` and
`6.615` respectively.

Interpretation: FP16 package storage remains a supported candidate for all
weights, with mandatory FP32 expansion and reductions. W2 and attention
projection must remain FP32 through their following RMSNorm operations. W2 is
then normalized before the MLP gate and residual: no FP16 cast may occur before
that normalization. The historical internal activation-storage dtype remains
unresolved; captured inputs/output are BF16 but the corrected stage witness is
FP32.

Confidence: high for this capture's extrema and conversion census; activation
cast error bounds and adversarial projection rows remain open.

Next question: execute high-range/error/cancellation projection rows and
explicit activation-cast experiments before finalizing per-stage storage.

## O11 — adverse maximum-output complete projections (2026-07-18)

Question: do maximum observed QKV/W1/W3 outputs expose source-BF16 versus
cached-FP16 divergence that token-zero representative rows missed?

Method: use O10 extrema coordinates: QKV token 537/channel 10546, W1 token
555/channel 8078, and W3 token 501/channel 3120. Each is a complete 3840-term
source/cache projection and is executed in compiled Oct.

Result: all three compiled theory cases passed with zero fallback. Source BF16
and cached FP16 outputs match exactly at the adverse maxima: QKV `432.27505`,
W1 `103.68054`, and W3 `333.02914`.

Interpretation: the prior selected projection conclusion survives high-output
adversarial rows. This strengthens FP16 weight-storage evidence, but says
nothing about safe activation casts after those projections.

Confidence: high for high-output source/cache rows; conversion-error and
cancellation rows plus activation-cast error experiments remain open.

Next question: test the highest conversion-error and strongest-cancellation
rows, then quantify proposed FP16 activation casts against retained FP32.

## O12 — adverse conversion-error and cancellation projections (2026-07-18)

Question: do the O10 maximum-conversion-error or strongest-cancellation rows
amplify source-BF16 versus cached-FP16 error?

Method: execute complete 3840-term token-zero projections for QKV/W1/W3
maximum-error rows (31, 5, 0) and strongest-cancellation rows (2736, 609,
7625) in compiled Oct.

Result: all nine O11/O12 theory cases pass with zero fallback. Error/cancel
rows contain 3838–3840 exact weights, maximum weight delta `2.9802322e-8`, and
every compared BF16-expanded versus FP16-expanded FP32 output is exactly equal.

Interpretation: the adverse source/cache evidence now covers maximum output,
conversion-error, and cancellation selections. It materially supports FP16
weight packaging with FP32 expansion, while leaving activation cast errors as
the next independent question.

Confidence: high for these adversarial source/cache reductions.

Next question: prove the explicit FP16 activation-overflow fault condition for
attention projection and W2, then test lower-range activation cast candidates.

## O13 — FP16 activation round-trip census (2026-07-18)

Question: which captured activation stages are outright impossible to store in
FP16, and which merely need downstream-error testing?

Method: apply deterministic IEEE FP32→FP16 nearest-even→FP32 diagnostic round
trips to all 34 captured stage payloads; record overflows, L-infinity error,
relative L2 error, and worst coordinate.

Result: exactly 1,024 values overflow in attention projection and exactly
1,021 in W2. Their L-infinity round-trip errors are `70728.64` and `77045.39`.
No other captured stage overflows by range. The largest non-overflow cast error
is gated hidden (`1.76`), followed by QKV/V/Q/K/attention aggregation/W3
(`0.12` scale), so range-only safety does not establish their storage policy.

Interpretation: attention projection and W2 are hard FP32 requirements through
their following RMSNorm boundaries. Lower-range stages are candidates only;
they require a downstream replay/error experiment before any FP16 storage
acceptance.

Confidence: high for this captured-path round-trip census.

Next question: execute controlled downstream replay with candidate activation
casts at auditable boundaries, beginning after normalization rather than before
it.

## O14 — controlled post-FFN-normalization activation cast replay (2026-07-18)

Question: what final-output error propagates from an FP16 activation round trip
at a lower-range, post-normalization boundary while W2 and projection remain
FP32?

Method: run the corrected full diagnostic program with a single IEEE FP16
round trip inserted after `ffn_norm`; retain all projection and W2 arithmetic in
FP32. Compare its full final F32 output against O2 baseline.

Result: replay is finite with final SHA-256
`5405ff9f0da41f1060154be5365c31c079d05cf51fddb3a296312e84010d6307`.
Baseline-versus-candidate L-infinity is `0.0006103515625`; relative L2 is
`9.103202131045231e-06`.

Interpretation: post-FFN-normalization FP16 storage is a promising candidate
under this one captured path, but this remains diagnostic-only evidence. It
does not approve FP16 storage for other stage boundaries or set production
tolerances.

Next question: repeat controlled casts at other post-normalization boundaries
and compare their downstream errors before selecting a minimal FP32 scratch
plan.

## O15 — controlled post-attention-normalization activation cast replay (2026-07-18)

Question: what final-output error propagates from FP16 storage after
`attention_norm`, before modulation/QKV, while hard FP32 stages remain FP32?

Method: run the corrected full diagnostic with a single FP16 round trip after
`attention_norm`; compare full final F32 output to O2 baseline.

Result: replay is finite with final SHA-256
`bf31335a415f3a962547be9d979637788d703ac6796fcd528462445ec6d96886`.
L-infinity is `0.00006103515625`; relative L2 is
`6.82871884669764e-07`.

Interpretation: this is a stronger one-path candidate than O14. It remains
diagnostic evidence only; combined casts are required before selecting the
scratch/storage plan.

Next question: replay with both O14 and O15 candidate casts enabled to quantify
their compounded downstream error.

## O16 — combined post-normalization activation cast replay (2026-07-18)

Question: do the two lower-range candidate casts compound into an unacceptable
full-block error while hard FP32 boundaries remain protected?

Result: the combined replay is finite, SHA-256
`b549fc4ecd75284623c99527265ad3f07e26fc88b4a40b5b42643d3ea55b2491`.
Against baseline, L-infinity is `0.00060272216796875` and relative L2 is
`9.129747823427625e-06`, essentially dominated by the O14 FFN-normalization
cast.

Interpretation: combined normalized-activation FP16 storage is a viable
diagnostic candidate for this capture, but final production acceptance requires
stage-specific tolerance selection and repeat/adversarial inputs.

Next question: establish acceptance tolerances and repeat the selected policy
on additional deterministic captures before production handoff.

## O17 — deterministic repeat of combined normalized-activation candidate (2026-07-18)

Question: is the O16 candidate result deterministic at every captured witness
boundary, rather than merely producing the same final-output hash?

Method: rerun the same full [1,1024,3840] diagnostic path with the identical
payloads, fixed reduction order, and FP16 round trips only after
`attention_norm` and `ffn_norm`. Compare every generated payload SHA-256 to
the O16 bundle.

Result: all 36 captured files, including all 34 stage payloads, match O16
byte-for-byte. The final SHA-256 is again
`b549fc4ecd75284623c99527265ad3f07e26fc88b4a40b5b42643d3ea55b2491`;
the baseline comparison remains L-infinity `0.00060272216796875` and relative
L2 `9.129747823427625e-06`.

Interpretation: the selected candidate policy has a deterministic witness for
the pinned [1,1024,3840] capture. This proves repeatability of the diagnostic
equipment, not an independent-input production tolerance.

Next question: census the established independent batch-two capture to test
whether the range-based FP32 prohibitions survive a materially different
captured input, while preserving the one-item resident ABI contract.

## O18 — batch-two independent-capture range census (2026-07-18)

Question: do the FP16 overflow prohibitions hold on an established capture
outside the pinned O2 batch-one block input?

Method: validate the `run_01` capture identities and inspect its source
stage-summary ranges. This is deliberately a range census, not a replay:
`run_01` is batch two while the accepted resident ABI and O2 contract are
batch one.

Result: `run_01` has BF16 input shape `[2,1024,3840]`, timestep `[2,256]`,
and output `[2,1024,3840]`. Its attention projection absolute maximum is
`136192`; W2 absolute maximum is `142336`; both exceed finite FP16
(`65504`). Gated hidden is `4192`, W1 `103.5`, W3 `356`, QKV `432`, and
attention aggregation `274`.

Interpretation: the hard rule is now corroborated across a separate captured
batch: do not cast attention projection before `attention_norm2`, and do not
cast W2 before `ffn_norm2`. The batch-two source capture is historical
compatibility/range evidence, not a new canonical resident ABI or a substitute
for an ABI-compatible replay.

Next question: turn the confirmed hard boundaries and O14–O17 deterministic
candidate errors into a conservative stage-tolerance proposal and the
canonical witness manifest.

## O19 — canonical witness and tolerance calibration (2026-07-18)

Question: can the fixed-order FP32 laboratory baseline generate repeatable,
full-stage witness identities suitable for a reference shader implementation?

Method: rerun the full captured block twice with corrected FP16 decoding,
cached FP16 weights expanded to FP32, and fixed-order FP32 arithmetic. Emit all
34 stage payloads and deterministic projections. Re-run the compiled Oct M9
adverse-row contract.

Result: Oct M9 passes 9/9 compiled tests with no fallback. The two canonical
bundles match all 36 files byte-for-byte. Both projection artifacts hash to
`f9350d37b46a26d132d4a1e6c80c984ebce87f6f3fe4fd9eb274ffbfd631f480`;
the complete stage manifest hash is
`0cab3d8fe179e70058cb22b37994413649f257268566b2c1dfb1254d2daeae65`.

Interpretation: the laboratory reference has reproducible full-stage
witnesses. The acceptance posture is deliberately strict: layouts, operation
order, finite state, and canonical CPU stage identities are exact; a GPU
reference shader must localize any selected-witness disagreement before any
optimized tolerance is introduced.

Next question: package the accepted conservative FP32-activation policy and
the shader/memory/execution boundary plan for M1B.

## O20 — production handoff and final laboratory report (2026-07-18)

Question: is there now one conservative, evidence-backed contract from which
the resident SDSL-V block can be implemented without guessing?

Method: select the conservative policy rather than adopting the one-capture
activation-FP16 candidates: cached FP16 weights expand to FP32 per arithmetic
load; every activation and reduction remains FP32. Bind that choice to the
full stage manifest, exact layouts, hard overflow boundaries, a six-shader
reference portfolio, and a bounded resident scratch plan.

Result: the production handoff, shader contract, memory contract, execution
contract, and final lab report now exist. The hard FP32 conditions remain
attention projection through `attention_norm2`, and W2 through `ffn_norm2`
and the final residual. No clamping or saturation is allowed.

Interpretation: EVT-2 is complete as an Oct laboratory authority for the
conservative M1B reference implementation. The historical Comfy BF16 result
and corrected Go executor remain comparison evidence, not replacement
oracles. Optional FP16 activation storage is deferred until an ABI-compatible
independent-capture experiment validates it.

Next question: M1B implementation of the six SDSL-V reference shader
boundaries, retaining the specified witness boundaries before optimization.
