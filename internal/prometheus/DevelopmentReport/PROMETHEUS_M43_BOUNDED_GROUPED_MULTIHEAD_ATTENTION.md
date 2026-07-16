# Prometheus M43 bounded grouped multi-head attention

## Outcome

Convergence outcome: **SUCCESS**  
Milestone state: **COMPLETE**  
Production classification: **2 — experimental grouped operator candidate**

M43 implements one fixed eight-head attention group over one shared X:

```text
X[Tokens, ModelWidth]
  x {Wq[h], Wk[h], Wv[h]} for h=0..7
  -> {Q[h], K[h], V[h]}
  -> Q[h] x transpose(K[h]) x scale
  -> stable_softmax
  -> P[h] x V[h]
  -> {O[h]}
```

The preferred resident-X route records all 24 Q/K/V projections first, uses
one exact 24-buffer visibility barrier, completes all eight M42 attention
tails, copies only the final eight outputs, and submits one command buffer
behind one group fence. Every intermediate remains device-resident.

No graph scheduler, arbitrary HeadCount, heterogeneous heads, batch dimension,
mask system, output projection, residual, normalization, rotary embedding,
model importer, training path, Direct3D, PTX, CUDA interop, or distributed
execution was added. The global SGEMM selector, production shader IDs,
manifests, module hashes, and ordinary M42 behavior are unchanged.

## Tensor convention and physical output

The logical contract is fixed:

| Tensor | Shape |
|---|---|
| X | `[Tokens, ModelWidth]` |
| Wq/Wk/Wv per head | `[ModelWidth, HeadDim]` |
| Q/K/V per head | `[Tokens, HeadDim]` |
| Scores/P per head | `[Tokens, Tokens]` |
| logical O | `[Tokens, 8, HeadDim]` |

The physical output is the bounded head-major equivalent:

```text
eight O[h][Tokens, HeadDim] device buffers
aggregate index = ((h * Tokens) + token) * HeadDim + column
```

Each head has one explicit `prom_device_buffer_view`. The eight views form one
bounded aggregate; each range is non-overlapping and remains alive under the
group slot. The single correctness readback is compact head-major
`[8, Tokens, HeadDim]`.

Token-major would be immediately convenient for a conventional output
projection, but the existing P-by-V SGEMM naturally writes one contiguous
per-head row-major result. Choosing head-major avoids a device-side concatenate
or a new layout shader solely for presentation. A later output-projection
consumer must explicitly resolve whether to interleave once or consume the
eight views directly; M43 does not hide that cost.

## Bounded internal request

`prom_m43_attention_group_request` contains:

- exactly `HeadCount=8`;
- Tokens, ModelWidth, HeadDim, scale, and one precision policy;
- host-X or resident-X mode and one nonzero shared-X generation;
- exact caller-supplied X and grouped-output element counts;
- eight preferred paths and eight bounded rollback facts;
- a required generation matrix `[8][3]` for Wq/Wk/Wv;
- complete-head, projection-grouped, or eight-sequential-M42 audit strategy;
- one bounded fault point and head index;
- one compact final output destination.

`prom_m43_attention_plan` records every head's selected M42 path and plan,
per-head and aggregate replay identities, exact stages, descriptors,
timestamps, dispatches, barrier calls/ranges, copies, submits, memory, output
layout, and eligibility reason. It is an internal Vulkan-reactor contract and
does not expose a public tensor graph.

Scale defaults to the FP32 value `1/sqrt(HeadDim)`. Inputs use contiguous
row-major logical tensors. Cooperative and conventional-FP16 heads consume
f16-rounded packed inputs and accumulate/output FP32. A2x4 consumes FP32.
M42's inter-SGEMM Q/K/V/P precision boundaries and M39b's logical-width versus
padded-stride softmax contract are reused without duplication.

## Shared X ownership

Host-X execution:

1. validates and hashes finite FP32 X once;
2. writes one retained host-visible upload buffer;
3. copies X once to one device-local FP32 buffer;
4. invokes the existing M42 pack shader once to create one padded F16x2 buffer;
5. exposes the FP32 or packed view to every selected head path.

The primary host-X medians were 386.3 us validation/hash, 26.2 us GPU upload,
and 5.1 us GPU pack. Counters are exactly one conversion, one upload, and eight
consumers.

Resident-X preparation owns one family resource with upload, FP32, and packed
buffers plus generation, shape, and content hash. Primary preparation was
361.8 us wall / 33.3 us GPU. Warm resident execution performs no conversion or
upload. Stale generation or shape rejects before slot acquisition. Replacement
waits/reaps all grouped slots before changing the family resource, so the
shared immutable view survives its final consumer.

## Persistent weight ownership

M43 owns 24 independent family resources:

```text
weight[head 0..7][Q, K, V]
```

Each resource separately retains host upload, FP32 device, and padded F16x2
device buffers plus model width, head dimension, content hash, and strictly
increasing generation. Preparation requires the exact weight element count and
validates, uploads, and GPU-packs only the
named resource. Replacing head 3's Wk does not modify any other generation.

Primary one-time preparation for all 24 weights was 8.851 ms wall and
801.3 us GPU. No weight allocation, conversion, upload, pipeline creation, or
layout creation occurs in a warm grouped operation. Replacement conservatively
waits all group slots; this is broader than a per-weight use list but remains
bounded and lifecycle-correct.

## Execution strategies

Three deterministic plans use the same persistent inputs and M42 stages:

1. `complete_heads`: complete head 0 through head 7 in one command buffer;
2. `projection_grouped`: all Q, all K, all V, one grouped Q/K/V visibility
   barrier, then each head's remaining attention stages;
3. `eight_sequential_m42`: audit baseline with the same device-resident X and
   weights but eight command-buffer submissions and eight compact output
   transfers.

The preferred strategy is **projection-grouped**. It is the only strategy that
stably preserved grouped projection throughput on the more-token and larger-
head rows. On the primary row it measured 4.652 ms GPU / 8.263 ms end-to-end;
complete-head measured 4.620 ms / 8.156 ms in the final sweep, effectively tied.
On more tokens, complete-head rose to 10.496 ms GPU while projection-grouped
remained 4.776 ms; the larger-head rows were tied at 4.774/4.793 ms. The alternate remains
implemented and measured, but is not preferred.

The all-stage-grouped variant was not added. Projection grouping already
isolates the dominant experiment; reordering every softmax/P-by-V stage would
add synchronization surface without evidence that those stages dominate the
normal shapes.

## Projection grouping and optional fusion

Every Q/K/V projection is a real existing SGEMM. Descriptor sets reuse the one
shared-X view but retain distinct weights and outputs. Per-head cooperative
primary projection medians in nanoseconds were:

| Family | heads 0 through 7 |
|---|---|
| Q | 170784, 167936, 167936, 166912, 172224, 167552, 167936, 166912 |
| K | 167936, 183520, 167648, 166912, 166912, 167936, 183648, 166720 |
| V | 166912, 167936, 166912, 183552, 166656, 166912, 166912, 166912 |

The primary projection sum was 4.072 ms, 87.5% of the 4.652 ms grouped GPU
interval. The eight-sequential cooperative baseline spent 4.023 ms in
projections, so primary shared-X grouping did not improve projection kernel
time in the final sweep. The more-token, larger-head, and awkward rows did show
large grouped projection wins; the result is shape-dependent rather than a
universal cache claim.

Fused QKV projection is not implemented. The existing SGEMM writes one
contiguous row-major C. Concatenating weights would require an explicit wide
output slicing/strided-view contract, complicate 24 independent persistent
generations, and potentially require a new layout operation. The measured
24-dispatch grouped path already removes the central primary bottleneck enough
to expose tiny, host-X, and 1024-token rollback boundaries. Fusion is therefore
not yet warranted as M43's canonical path.

## Per-head attention reuse

After projection visibility, each head reuses M42 exactly:

```text
Q pack
K packed transpose/layout
V pack
QK-transpose SGEMM
in-place scale
M39b stable softmax
P pack
P-by-V SGEMM
```

The primary grouped totals were:

| Stage | GPU median |
|---|---:|
| Q/K/V projections | 4.072 ms |
| Q pack | 38.9 us |
| K layout | 43.8 us |
| V pack | 36.9 us |
| QK-transpose | 150.8 us |
| scale | 32.4 us |
| softmax | 67.6 us |
| P pack | 36.9 us |
| P-by-V | 142.1 us |
| post-projection sum | 550.9 us |
| full grouped attention | 4.652 ms |

No Q/K/V, Scores, or P host copy occurs. Each head may select cooperative,
conventional FP16, or A2x4 independently. A single-head rollback selects the
same-precision conventional path while unrelated heads remain cooperative; the
aggregate eligibility trace records the rollback mask.

## Command trace and synchronization

The preferred resident-X primary trace contains:

- 88 dispatches: 24 projections and 64 remaining per-head stages;
- 59 barrier calls covering 89 exact buffer ranges;
- 1,024 compact final copy regions (`8 * Tokens`);
- 199 timestamp queries;
- one primary command buffer and one queue submit;
- no queue ownership transfer and no host wait between heads or stages.

Host X adds one pack dispatch, one upload copy, and three exact visibility
barriers: 89 dispatches, 62 barrier calls, and 92 ranges. Complete-head resident
uses 66 barrier calls / 89 ranges. The eight-sequential baseline uses 88
dispatches, 80 barrier calls / 96 ranges, eight submits, and eight final
readback stages.

Projection-grouped Q/K/V visibility is one `vkCmdPipelineBarrier` containing
24 exact output-buffer barriers. Subsequent barriers name only the producing
head buffer. Score scale uses compute-write to compute-read/write; softmax and
P-by-V inputs use compute-write to compute-read. Final output visibility is one
eight-buffer compute-to-transfer barrier; the grouped readback then receives
one transfer-to-host barrier. Queue-family indices are always
`VK_QUEUE_FAMILY_IGNORED`.

The 199-query region reserves shared-X upload/pack and aggregate boundaries,
24 queries per head for eleven exact compute intervals plus the sequential
readback baseline, and grouped end/readback boundaries. The capacity is fixed
below the 256-query per-slot stride.

## Lifecycle and failure ownership

M43 extends the existing M39b/M40b/M42 ring owner. Family state owns 24
weights, optional resident X, pipelines, layouts, descriptor pool, and query
pool. One physical slot owns host X, eight complete intermediate sets, eight
outputs, one grouped readback, descriptors, command buffers, fence, slot
generation, and logical group identity.

One fence represents the complete grouped submission. The slot cannot recycle
until every head and final readback complete. Logical failure remains separate
from physical recyclability:

- shared-X upload, mid-projection, one head QK, one head softmax, and final
  readback faults submit a bounded partial plan, wait to known completion, and
  return logical failure with a recyclable slot;
- a post-P-by-V uncertainty on one head quarantines the whole group;
- weight or shared-X replacement blocks, observes the group fence, reaps the
  slot, then installs the newer generation;
- destruction releases every grouped buffer before the shared Vulkan owners.

Permanent hardware facts prove quarantine/reap, no early recycling, stale
single-weight and shared-X rejection, recovery, unchanged unrelated weight
generations, and zero remaining quarantined slots.

## Memory and capacity

The primary exact one-slot/family model is 37,748,748 bytes:

| Resource class | Bytes |
|---|---:|
| shared X upload / FP32 / packed | 524288 / 524288 / 262144 |
| 24 weight uploads | 12582912 |
| 24 FP32 device weights | 12582912 |
| 24 packed device weights | 6291456 |
| eight Q/K/V FP32 sets | 1572864 |
| Q-pack/K-layout/V-pack capacity | 1048576 |
| eight score buffers | 524288 |
| eight probability buffers | 524288 |
| eight packed-P buffers | 262144 |
| eight output buffers | 524288 |
| reduction minimum bindings | 12 |
| grouped readback | 524288 |

Actual retained capacity after exercising both host and resident modes was
39,059,468 bytes. Buffers grow and are reused; the two physical ring slots are
warmed in the permanent hardware fact, after which repeated group and baseline
execution performs no Vulkan buffer allocation.

Eight independent one-head owners would retain the same weights and head-local
storage but duplicate the three X resources eight times: 46,923,788 bytes by
the same exact model. Shared X therefore removes 9,175,040 bytes, or 19.6%,
before considering additional per-owner pipeline/layout overhead. No system-
memory spill is planned. A 512 MiB exact-request cap rejects before slot
acquisition while retaining an auditable capacity reason.

## Correctness oracle and replay identity

The grouped CPU oracle calls the M42 oracle once per head under the exact
selected precision. It validates finite inputs/outputs, stable-softmax
nonnegativity and row sums, and head-major layout. The first mismatch reports
head, stage, row, column, expected/actual, absolute/relative error,
logical/padded shape, three weight generations, per-head replay ID, reduction
replay ID, and aggregate replay ID.

The primary resident projection-grouped aggregate replay ID is
`12832534940513279545`. Its eight per-head IDs are:

```text
972767035684802868
6996841797284540562
17975865627865807932
9121874610937946146
6153432745104217334
294713990127766581
10282121551690533995
2518051632558621964
```

Other primary aggregate identities are host projection-grouped
`18331233493753172963`, resident complete-head `7924750622373292315`,
eight-sequential cooperative `13618927766442587705`, conventional FP16
`4034656163417050127`, and A2x4 FP32 `4144128367867725301`. Changing only head
3 Wk changes only that head's replay ID plus the aggregate identity.

## RTX 3070 benchmark corpus

Medians use five warmups and five measured groups. GPU is the central attention
interval and excludes final readback. Resident end-to-end includes command
recording/submission, completion, and one compact host copy.

| Workload T/M/H | grouped GPU | 8x M42 GPU | GPU speedup | grouped resident E2E | 8x M42 E2E | E2E speedup |
|---|---:|---:|---:|---:|---:|---:|
| 16/128/16 | 0.735 ms | 0.717 ms | 0.97x | 2.018 ms | 4.529 ms | 2.24x |
| 128/1024/128 | 4.652 ms | 4.541 ms | 0.98x | 8.263 ms | 10.831 ms | 1.31x |
| 256/1024/128 | 4.776 ms | 10.227 ms | 2.14x | 10.738 ms | 19.743 ms | 1.84x |
| 128/1024/256 | 4.793 ms | 10.440 ms | 2.18x | 10.028 ms | 18.985 ms | 1.89x |
| 127/1001/127 | 4.564 ms | 9.611 ms | 2.11x | 8.112 ms | 15.932 ms | 1.96x |
| 1024/128/64 | 11.613 ms | 10.982 ms | 0.95x | 27.510 ms | 31.948 ms | 1.16x |

Primary resident projection-grouped versus the other sequential baselines:

| Baseline | baseline GPU | grouped speedup | baseline E2E | grouped speedup |
|---|---:|---:|---:|---:|
| cooperative M42 | 4.541 ms | 0.98x | 10.831 ms | 1.31x |
| conventional FP16 | 10.403 ms | 2.24x | 15.143 ms | 1.83x |
| A2x4 FP32 | 16.698 ms | 3.59x | 21.625 ms | 2.62x |

Primary host-X grouped end-to-end was 8.696 ms versus 8.263 ms resident.
Warm resident 10-group and 100-group medians were 8.218 and 8.255 ms
end-to-end, with stable 4.652 and 4.652 ms GPU intervals.

Host-X is a measured rollback outside the primary/tiny rows: its same-command
upload/pack-preceded central interval rose to 13.5-22.7 ms on more tokens,
larger head, awkward, and the softmax boundary, versus 4.6-11.5 ms resident.
M43 records this evidence without assigning an unmeasured cause.

## Performance questions answered

1. **Does shared X reduce preparation?** Yes. Host mode validates/uploads/packs
   once; resident mode removes all warm X work. It also saves 9.18 MB versus
   eight independent owners.
2. **Does one command buffer reduce CPU submission?** Yes. Primary submission
   was 72.3 us versus 195.3 us, 2.70x lower; recording was 1.225 versus
   1.418 ms.
3. **Does stage grouping improve GPU execution?** Not on the primary final
   sweep: it measured 0.98x versus sequential cooperative. It wins the
   more-token/larger/awkward rows 2.11x-2.18x and loses tiny and 1024-token GPU
   time. The result is shape-dependent.
4. **Do projections dominate?** Yes on normal rows: 4.072 of 4.652 ms on the
   primary. At 1024 tokens, post-projection attention is 9.656 of 11.476 ms.
5. **Does shared-X cache/reuse improve projection throughput?** Not on the
   primary final sweep: 4.072 ms grouped versus 4.023 ms sequential. It does on
   the more-token/larger/awkward rows. No universal cache claim is justified.
6. **Is fused QKV warranted next?** Not yet. Output slicing/generation costs and
   host/1024 rollback classification should be resolved first.
7. **Is memory a new bottleneck?** No. Exact primary retention is 36.0 MiB,
   actual warmed retention 37.3 MiB, with a fixed 512 MiB rejection cap.
8. **Where can conventional FP16 win?** M42's tiny single-head evidence remains
   a conventional rollback. Grouped cooperative also loses tiny GPU time to
   the sequential cooperative baseline; no tiny production claim is made.
9. **What happens at 1024 tokens?** Query and command capacity remain safe and
   correctness is clean, but P-by-V/softmax/P conversion dominate and grouped
   GPU time is 0.95x the sequential baseline. Readback is 8.3 ms.
10. **Is fixed eight-head grouping production-worthy?** Not yet. The primary
    and middle corpus are materially positive, but host-X, tiny, and 1024-token
    GPU boundaries plus one-device evidence require experimental status.

## Eligibility, fallback, and classification

The experimental predicate requires exactly eight heads, executable
cooperative capability, f16-rounded permission, bounded padded dimensions, 24
prepared weights, one valid shared-X identity, all generations valid, memory
within 512 MiB, and no rollback head. It records exact head-count, capability,
precision, shape, weight, X, padding, capacity, or rollback reasons.

Execution remains available when the predicate is ineligible: extension
absence selects conventional FP16 independently for all heads; explicit FP32
selects A2x4; a bounded single-head rollback selects conventional FP16 for only
that head. The global SGEMM production selector is not consulted or modified.

Classification is **experimental grouped operator candidate**. The full fixed
operator is correct, lifecycle-safe, validation-clean, memory-bounded, and
materially faster end-to-end on the primary resident workload. Its primary GPU
interval is a measured 0.98x rollback. Production promotion is
withheld because:

- tiny, primary, and 1024-token GPU time roll back;
- host-X central GPU timing regresses on four non-primary rows;
- complete-head and projection-grouped performance is not uniform by shape;
- evidence is one RTX 3070 and one exact cooperative tuple;
- head-major output has not yet been consumed by a real output projection.

## SDSL-V and validation

M43 adds no SDSL-V source. It reuses the exact M40a cooperative SGEMM, M42 pack,
transpose, and scale modules, plus production M39b reduction shaders. The four
reused SPIR-V modules pass `spirv-val --target-env vulkan1.3`; no deterministic
shader regeneration was needed because no source or artifact changed.

Permanent non-hardware and hardware facts cover fixed head count, shared-X
identity, independent generation matrix, padding, output indexing, exact
memory, command order, barrier ranges, one upload/submit, deterministic replay,
per-head fallback, capacity, CPU oracle, mismatch localization, host/resident
X, both grouped strategies, eight-submit baseline, all fault points,
quarantine/reap, one-Wk replacement, shared-X replacement, extension-disabled
fallback, warm allocation reuse, and validation cleanliness.

Validation results:

- M43 facts: 5/5 passed on the RTX 3070;
- M43 six-workload benchmark: passed, zero output mismatch;
- validation warnings/errors: zero; device loss: zero;
- M42 regression: 5/5 passed;
- M39b reduction regression: 7/7 passed;
- full requested Go matrix, native manifest, workspace check, shell syntax,
  and `git diff --check`: passed;
- authoritative MSVC Windows native rebuild: passed;
- Linux GCC shared library, ELF test binaries, and harness smoke: passed.

The committed artifact is
`DevelopmentReport/artifacts/M43/bounded_grouped_attention_rtx3070.json`.

## Exact next model-facing workload

The next bounded workload is:

```text
T=128, Heads=8, HeadDim=128
eight resident head-major O views
  -> explicit device-resident [128,1024] aggregation contract
  -> one persistent Wo[1024,1024] output projection
```

That follow-up should decide, with measurements, whether one narrow interleave
kernel or a strided eight-view projection is the correct consumer. It should
retain M43's one-submit lifecycle and must not add residuals, normalization,
rotary embeddings, a transformer block, or a graph scheduler.
