# Prometheus DVT-2 M5: attention and shader cleanup

Date: 2026-07-20

## Convergence outcome

- Convergence outcome: **MEANINGFUL PROGRESSION**
- Milestone state: **IN PROGRESS**
- DVT-2 state: **READY WITH REQUIRED ADAPTATIONS**

The production sources now remove the three obvious single-lane attention
passes without changing the FP32 activation contract.  A real RTX 3070
representative MainTransformer/30-layer authority run passes.  This is not a
complete M5 closeout because the required fixed Prefetch full-image PNG/timing
lane was not rerun after the change, so no new canonical wall or PNG identity
is asserted.

## Implemented route

`CanonicalFp32` remains the only accepted route:

- activations, accumulation, and outputs are FP32;
- immutable weights remain packed FP16;
- the denominator remains one lane-0 addition in logical key order;
- maximum, exponential, and normalization work are cooperative;
- scores/probabilities remain workgroup-local and no global attention matrix is
  materialized.

All three production families were audited:

| Family | Tokens | Workgroup | M5 cleanup |
|---|---:|---:|---|
| NoiseRefiner | 1024 | 256 | four keys/lane, parallel max/exp/normalize |
| ContextRefiner | 32 | 256 | active first 32 lanes, parallel max/exp/normalize |
| MainTransformer joint | 1056 | 256 | up to five keys/lane, parallel max/exp/normalize |

The fixed lane-0 merge of 256 per-lane maxima preserves the old first-key
left-biased NaN behavior.  Only the denominator uses floating-point addition
in canonical key order.  A fixed parallel denominator tree was intentionally
not promoted or tested: it would be a separate deterministic route with
different arithmetic order.

## Packed FP16 cleanup

The audit finding was real in the M4 tiled paths.  Each 16-column output tile
is even aligned, but its loader unpacked each `F16x2` word separately for the
two adjacent columns and selected a half via runtime parity.  M5 changes the
load topology in QKV, NoiseRefiner W1/W3, MainTransformer W1/W3, and attention
projection to load each word once and directly store `.x`/`.y` into adjacent
shared-tile columns.  W1/W3 still shares its input tile and all arithmetic is
FP32 after weight expansion.

## Audit and generated code

M4 had already removed the sample-key ladder: literal keys and the bounded
`WriteAuditSample`/`WriteAuditProbability` helpers are present, while the
persistent summary uses a closed payload enum and exhaustive `match`.  M5 did
not add a redundant serializer abstraction.

All changed SDSL-V sources compile; generated HLSL/SPIR-V was regenerated and
validated with Vulkan 1.0 `spirv-val`.  HLSL shows `Maximums[256]`, cooperative
exp/normalization loops, and pair-based FP16 loads without a runtime `% 2`
selection.  No cooperative-matrix capability or activation narrowing was
introduced.

## Real validation and performance

The local payload checker passed.  The representative real MainTransformer
lane passed:

| Witness | Relative L2 | L-infinity | Result |
|---|---:|---:|---|
| representative MainTransformer | 8.38066e-7 | 6.40869e-4 | pass |
| 30-layer authority | 1.02005e-5 | 1.17188e-2 | pass, limit 5e-5 |

Its GPU median was 430.046 ms (CPU median 430.665 ms), versus the accepted M4
representative reference of 436.342 ms.  That is encouraging but is not a
substitute for the required full-image stage and Prefetch timing campaign.

The focused M1B real lane reached QKV and retained its prior QKV metrics, then
failed on an unrelated existing assertion that expects 26 parameter sets after
this single-block test uploaded 13.  The failure is recorded rather than used
as a numerical failure.

The accepted prior PNG SHA-256 remains
`7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613`; it has
not yet been re-smoked after M5 and must not be claimed as a fresh result.

## Required closeout adaptation

Run the fixed canonical Prefetch full-image lane against this generated native
build, emit stage totals and wall timing, and confirm the PNG identity before
changing the milestone state to COMPLETE.  Measure packed-pair A/Bs one kernel
at a time and retain only wins.  If the serial canonical denominator prevents
the attention target, record that result and leave a separate
`ParallelDeterministic` reduction experiment unpromoted unless its complete
authority campaign is accepted.

## M6 handoff

M6 may investigate the RTX 3070 16x16x16 `F16 x F16 -> F32` cooperative matrix
route for W1/W3, QKV, W2, and projection.  It requires an explicit FP32
activation-to-FP16 fragment boundary, memory accounting, a distinct execution
profile, and a complete numerical campaign.  It must never silently replace
`CanonicalFp32`.

Machine-readable evidence is under `artifacts/Dvt2M5/`.
