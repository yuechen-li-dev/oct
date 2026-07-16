# Prometheus M42 device-resident scaled dot-product attention operator

## Outcome

Convergence outcome: **SUCCESS**  
Milestone state: **COMPLETE**  
Production classification: **2 — experimental operator candidate**

M42 implements one bounded, one-head forward attention operator:

```text
Q = X x Wq
K = X x Wk
V = X x Wv
Scores = Q x transpose(K) x scale
P = stable_softmax(Scores)
Output = P x V
```

Q, K, and V are produced by three real Vulkan SGEMM dispatches. Q, K, V,
Scores, P, and Output remain device-resident until their last consumers. The
normal application path uses one command buffer, one queue submit, and one
final compact Output readback outside the central attention GPU interval.

No model importer, arbitrary batching/head layout, mask system, normalization,
rotary embedding, backward pass, flash-attention fusion, graph executor,
Direct3D, CUDA/PTX interoperability, or global SGEMM selector change was added.

## Tensor convention and bounded request

M42 is one attention head per request. All logical tensors are contiguous
row-major:

| Tensor | Logical shape |
|---|---|
| X | `[Tokens, ModelWidth]` |
| Wq, Wk, Wv | `[ModelWidth, HeadDim]` |
| Q, K, V | `[Tokens, HeadDim]` |
| Scores, P | `[Tokens, Tokens]` |
| Output | `[Tokens, HeadDim]` |

`ValueDim` remains explicit in the internal request but must equal `HeadDim`.
The first product workload is therefore honestly `Tokens=128`,
`ModelWidth=1024`, `HeadDim=128`, not M40b's historical MxNxK shorthand.

The internal `prom_m42_attention_request` accepts finite FP32 X, dimensions,
scale policy, precision policy, preferred execution path, fallback permission,
host-X or resident-X mode, required generations, bounded fault injection, and
optional audit destinations. Scale defaults to the FP32 value
`1/sqrt(HeadDim)` unless explicitly supplied. Dimensions are nonzero and
bounded to Tokens <= 1024, ModelWidth <= 4096, HeadDim <= 1024, and 16,777,216
elements for each significant logical matrix. Checked 16-element rounding and
byte arithmetic reject overflow before resource work.

The API is internal and experimental. It exposes no public raw Vulkan handle
and does not define a tensor graph or general operator dispatcher.

## Precision routes and selection

The cooperative and conventional-FP16 routes use:

```text
FP32 caller data
  -> F16x2 packed inputs
  -> FP32 SGEMM accumulation/output
  -> GPU F16x2 packing at every inter-SGEMM boundary
  -> FP32 final Output
```

Thus X and Wq/Wk/Wv are rounded to f16 for the projections; Q and K are rounded
again before QK-transpose; P and V are rounded before P-by-V. The CPU oracle
matches every boundary. A2x4 remains a separate all-FP32 product comparison and
is not described as bit-equivalent.

Selection is deterministic. An eligible reduced-precision request selects the
cooperative path. Missing cooperative capability falls back to conventional
FP16 while preserving precision. An explicit FP32 policy or active cooperative
rollback selects A2x4. The plan reports the preferred and selected paths,
fallback flag, and exact reason. Existing production selectors remain
unchanged.

## Persistent weights and resident X

Wq, Wk, and Wv are one family-owned generation. Preparation:

1. validates and hashes each finite FP32 matrix;
2. uploads each FP32 matrix once into retained device-local storage;
3. GPU-packs each once into retained padded F16x2 storage;
4. records the three content hashes and three stable generation fields;
5. retains both representations so cooperative, conventional FP16, and A2x4
   comparison paths do no per-invocation weight work.

Replacement requires a strictly newer generation and first waits/reaps every
shared slot. Stale Wq, Wk, or Wv generations reject before acquisition.
Preparation time, GPU upload/pack time, retention, replacement, and reuse are
reported separately.

Host-X mode packs the selected representation per invocation and uploads it
inside the command buffer before the central timed interval. Resident-X
preparation uploads FP32 X once, GPU-packs its reduced representation once, and
retains both with a stable generation/hash. Resident execution rejects stale or
shape-incompatible X and has zero per-operation host conversion.

## Q/K/V production and K layout

The first three compute dispatches are genuine SGEMMs using the selected path:

```text
X x Wq -> Q[Tokens, HeadDim]
X x Wk -> K[Tokens, HeadDim]
X x Wv -> V[Tokens, HeadDim]
```

Each output has a distinct device-local FP32 buffer and an explicit
compute-write producer state. No Q/K/V host reconstruction or readback occurs
in the application path.

K has one canonical logical shape, `[Tokens, HeadDim]`. The reduced path uses
the attention-specific pack/layout shader to produce physical row-major
`KTranspose[HeadDim, padded Tokens]` in F16x2 storage. The exact mapping is:

```text
K[token, head] -> KTranspose[head * paddedTokens + token]
```

The FP32 baseline uses a separate bounded FP32 transpose with the same logical
mapping. There is no host transpose, hidden reinterpretation, or arbitrary
language/runtime transpose feature. V stays ordinary row-major for P-by-V.

## Score, softmax, and P-by-V path

Cooperative QK-transpose consumes packed Q and packed K-transpose, zero-pads
M/N/K to 16, accumulates to FP32, and writes one padded FP32 score matrix.
A2x4 and conventional FP16 are proven comparison/fallback paths.

Score scaling uses one attention-only in-place FP32 shader over the logical
Tokens-by-Tokens region. This preserves the M40a cooperative shader identity
and the production M39b softmax identities. An explicit compute-write to
compute-read/write barrier precedes scaling, and another compute-write to
compute-read barrier precedes softmax.

M39b consumes Scores directly with rows=Tokens, logical width=Tokens, and
physical input stride=padded Tokens. Widths through 1024 use its production
fused stable-softmax shader; its staged plan remains available above that
threshold inside the established reduction envelope. P is written as compact
FP32 `[Tokens, Tokens]`.

The reduced path then GPU-packs P into padded F16x2 storage and consumes it with
packed row-major V in P-by-V. A2x4 consumes FP32 P and V directly. Every route
accumulates and writes final FP32 Output. The final padded Output is compacted
by bounded per-row transfer regions into one logical readback buffer, so awkward
HeadDim tails never escape.

## Command, barriers, and timestamps

The normal trace is one owner, one command buffer, and one submit:

1. optional host-X upload;
2. Q projection;
3. K projection;
4. V projection;
5. Q pack;
6. K transpose/layout;
7. V pack;
8. QK-transpose;
9. in-place scale;
10. M39b softmax plan;
11. P pack;
12. P-by-V;
13. final Output transfer/readback.

Only stages required by the selected precision route dispatch. Every producer
to consumer transition uses the existing compute-write to compute-read pattern
on the exact buffer. Host upload uses host-write to transfer-read, then
transfer-write to compute-read. Final Output uses compute-write to
transfer-read and transfer-write to host-read. Queue-family indices remain
ignored because one compute queue owns the plan.

The slot-owned 32-query region records 26 exact timestamps: upload, each Q/K/V
projection, each Q/K/V pack/layout stage, QK-transpose, scale, softmax, P pack,
P-by-V, and final transfer. The central GPU number begins immediately before Q
projection and ends immediately after P-by-V. It excludes final readback.

Optional Q/K/V/Scores/P audit readbacks occur only after the application submit
and timestamp harvest in a second bounded audit submission. They are never
counted as intermediate copies in the normal plan.

## Ownership, retained storage, and failure

M42 extends the existing M39b/M40b physical ring rather than adding a scheduler.
The family owns persistent weights, resident X, descriptor/pipeline layouts,
three attention pipelines, the three existing comparison SGEMM pipelines, and
the production reduction pipelines. Each slot owns X upload/device storage,
Q/K/V, packed Q/K-transpose/V, Scores, P, packed P, Output, final/audit
readback, reduction temporaries, descriptor sets, command buffers, fence,
generation, and request identity.

Buffers grow and are retained; no Vulkan buffer allocation, descriptor-layout
creation, or pipeline creation occurs during warm execution. The slot is not
recycled until P-by-V, final readback, and any explicit audit finish.

Fault injection after Q, QK-transpose, and softmax submits a bounded partial
plan, waits for physical completion, returns logical failure, and keeps the slot
recyclable. Injection after P-by-V submit creates completion uncertainty,
quarantines the whole slot, and reports it non-recyclable. A newer weight
generation forces physical reap before replacement; recovery then executes with
the fresh generation. Final diagnostics show no quarantined slots, stale use,
or leak.

## SDSL-V source and identity

Three new selector-ineligible experimental SDSL-V sources live under the M39a
workspace policy:

| Identity | Role | SPIR-V SHA-256 |
|---|---|---|
| `m42-attention-pack-f32-to-f16` | padded row-major pack and canonical K transpose | `d5c3fd6a2af8965f95bb01775be42e859f9d79af34451596b54ae9d77d930a35` |
| `m42-attention-k-transpose-f32` | bounded FP32 K transpose | `fb812063431bc816e2b7471a908492b4631f2aa7899f6cdd0f43cc6747d47ca3` |
| `m42-attention-scale-scores-f32` | logical in-place score scale | `083d00ecc944435ff57d85a1ed003a6cb25a6f1f31b74c52ff8d1feb509b3578` |

DXC 1.9.0.5347 / 1.10.5347-fe261573 compiled all three for Vulkan;
`spirv-val` accepted them, inspection proves their entry/local-size/binding
contracts, and deterministic regeneration reproduced the same bytes. The M40a
cooperative source/SPIR-V hash, production reduction shaders 16-20, production
SGEMM IDs, registry ownership, and selectors did not change.

Operator replay identity includes dimensions, resolved FP32 scale, precision,
selected path, K layout, probability strategy, weight hashes/generations, all
attention/cooperative shader hashes, M39b reduction replay identity, and the
deterministic stage/command plan. The workspace checker validates the committed
18-record artifact.

For the primary 128/1024/128 workload, the exact operator replay IDs are
`13592800400211002722` (cooperative FP16), `17883492772666172286` (A2x4 FP32),
and `4884010612153911321` (conventional FP16). All three compose the same M39b
reduction replay ID, `6731912303627754029`.

## Correctness and permanent tests

The CPU oracle implements the exact selected contract, including input and
inter-SGEMM f16 rounding, FP32 accumulation, stable softmax, probability row
sums, and P rounding before P-by-V. It rejects nonfinite inputs and reports the
first mismatch with stage, row, column, expected/actual, absolute/relative
error, logical/padded shape, operator replay ID, and reduction replay ID.

Permanent non-hardware coverage proves shape/overflow validation, default
scale, one-head restriction, padding, K mapping, producer stages, buffer
lifetime plan, barriers, absence of intermediate copies, final-readback order,
fallback reasons, probability conversion, replay stability, finite reference,
precision distinction, row sums, and first-mismatch localization.

Hardware facts prove final-only and audit runs, persistent weights, host X,
resident X, stale generation rejection, capability fallback, all four fault
points, quarantine/reap, and validation cleanliness. The final M42 fact sweep
passed 5/5. The six-workload, three-path benchmark corpus passed 18/18 outputs;
validation warnings/errors and device loss were zero.

The full MSVC native build and the Linux GCC build both compile/link the shared
reactor and Marionette binaries; the Linux smoke fact passes. This WSL2 host
does not expose initialized Vulkan services, so its M42 filter passes the two
portable contracts and explicitly skips the three hardware facts (zero
failures). The Windows RTX 3070 filter executes all five facts with no skips.

## RTX 3070 performance

The committed artifact is
`DevelopmentReport/artifacts/M42/device_resident_attention_rtx3070.json`.
Times below are medians. `GPU` is Q projection through P-by-V and excludes final
readback. Host-fed is persistent weights plus new host X plus final Output
readback. Resident-X includes final Output readback.

| Workload T/M/H | Cooperative GPU | vs A2x4 | vs conventional FP16 | Host-fed | host vs A2x4 | Resident-X | resident vs A2x4 | Readback |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 16/64/16 | 72.1 us | 1.86x | 0.98x | 0.624 ms | 0.58x | 0.612 ms | 0.56x | 0.008 ms |
| 128/1024/128 | 484.8 us | 3.05x | 1.21x | 1.682 ms | 1.46x | 1.260 ms | 1.70x | 0.222 ms |
| 128/1024/256 | 440.2 us | 4.68x | 2.38x | 1.566 ms | 1.98x | 1.188 ms | 2.41x | 0.426 ms |
| 256/1024/128 | 570.3 us | 3.75x | 1.98x | 2.603 ms | 1.37x | 1.691 ms | 1.90x | 0.447 ms |
| 127/1001/127 | 776.3 us | 9.81x | 1.11x | 2.732 ms | 3.81x | 1.718 ms | 5.12x | 0.205 ms |
| 1024/128/64 | 1.444 ms | 2.26x | 2.42x | 4.228 ms | 1.37x | 4.070 ms | 1.41x | 0.844 ms |

The primary cooperative stage medians are Q/K/V 141.8/140.3/135.7 us,
Q/K-layout/V packing 4.1/5.1/4.1 us, QK-transpose 17.4 us, scale 3.1 us,
softmax 8.2 us, P pack 4.1 us, and P-by-V 17.4 us. Real upstream Vulkan
producers therefore preserve the cooperative advantage; the three projections,
not softmax or layout, dominate the primary operator.

At 1024 tokens, P-by-V becomes the dominant cooperative stage at 507.8 us;
softmax is 203.8 us and P packing is 176.1 us. P conversion is material there
but does not erase the 2.26x A2x4 or 2.42x conventional-FP16 full-operator win.
K layout is never dominant. The awkward padded case still wins A2x4 by 9.81x,
although it only narrowly beats conventional FP16 by 1.11x. The tiny case is a
clear same-precision rollback: conventional FP16 is 1.02x faster than
cooperative.

The primary resident-X 10-operation and 100-operation median end-to-end times
are 1.247 ms and 1.170 ms. One-submit remains the owned plan; M40b already
established its advantage over the bounded two-submit comparison, and M42 adds
no scheduler. Host X validation/packing is the main non-readback host cost on
larger input matrices; resident X removes it.

## Performance questions answered

1. **Real producer preservation:** yes; primary full GPU attention is 3.05x
   A2x4 and QK-transpose itself is only 17.4 us after real Q/K producers.
2. **Full attention win:** yes versus A2x4 on all six workloads, 1.86x-9.81x.
3. **Dominant stage:** Q/K/V projections for five workloads; P-by-V at 1024
   tokens.
4. **Softmax cost:** minor through 256 tokens; material but not dominant at
   the 1024 boundary.
5. **P conversion:** 4-20 us through the normal corpus and 176 us at 1024;
   it does not erase the product win.
6. **K layout:** 4-15 us and never dominant.
7. **Padding/small rollback:** awkward padding remains favorable; tiny 16/64/16
   rolls back to conventional FP16 by 0.98x.
8. **Submit topology:** M42 keeps the M40b-proven one-submit topology.
9. **Host preparation:** yes for host-fed larger matrices; resident X removes
   per-operation conversion.
10. **Eligibility stability:** stable enough for this bounded operator against
    A2x4, but not yet for global or cross-device production policy.

## Classification and next workload

Classification is **experimental operator candidate**. The complete bounded
operator is correct, lifecycle-safe, validation-clean, faster than A2x4 across
the corpus, and has explicit fallback. Promotion is withheld because evidence
is one RTX 3070/driver and one exact cooperative tuple, the operator requires
reduced-precision permission, conventional FP16 wins the tiny case, masks and
multiple heads remain outside the contract, and no second hardware family has
validated the eligibility boundary. No global SGEMM selector changes.

The exact next model-facing workload is **eight independent one-head requests
with Tokens=128, ModelWidth=1024, HeadDim=128 sharing one resident X generation
and distinct persistent Wq/Wk/Wv generations**, measured as a bounded fixed
head group without adding arbitrary batching or a graph runtime. That tests
real multi-head weight ownership and output concatenation pressure while
retaining M42's one-head operator as the execution primitive.
