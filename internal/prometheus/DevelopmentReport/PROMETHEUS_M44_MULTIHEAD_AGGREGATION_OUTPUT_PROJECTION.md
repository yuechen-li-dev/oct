# Prometheus M44 multi-head aggregation and output projection

## Outcome

Convergence outcome: **SUCCESS**  
Milestone state: **COMPLETE**  
Production classification: **2 — experimental complete attention-module candidate**

M44 turns M43's eight resident head outputs into one complete bounded attention
module result:

```text
eight O[h][Tokens, HeadDim] device views
  -> C[token, h * HeadDim + column]
  -> persistent Wo[8 * HeadDim, ModelWidth]
  -> Y[Tokens, ModelWidth] FP32 device view
  -> optional one-final-Y correctness readback
```

Both required layout strategies are implemented. Explicit interleave can write
FP32 C for A2x4 or round and pack F16x2 C directly for cooperative and
conventional-FP16 projection. Direct segmented projection reads all eight head
views without materializing C and accumulates one Y element across the eight
segments. No device plan reads an intermediate head or C back to the host.

The production-facing layout decision is **retain M43's head-major output and
use the narrow packed interleave at the output-projection boundary**. On the
primary RTX 3070 row, interleave is 4.640 us, only 4.2% of M44 GPU time;
interleave plus cooperative projection is 109.824 us in one submit and 101.216
us in two bounded submits. Direct segmented is 224.608 us. Changing M43's
natural per-head P-by-V output layout is not justified by this consumer.

The module remains experimental rather than production-ready. It is correct,
lifecycle-safe, validation-clean, device-resident, and materially faster than
the host-bounce baseline, but the preferred submit plan varies at tiny and
larger-head edges, tiny favors direct/conventional projection, M43 itself still
has documented rollback regions, and the evidence is one RTX 3070 cooperative
tuple.

No residual connection, normalization, rotary embedding, mask system, batch
dimension, arbitrary head count, graph scheduler, model import, training,
Direct3D, PTX, CUDA interoperability, or distributed execution was added. The
global SGEMM selector, production shader IDs, ordinary M42/M43 APIs, and their
manifests remain unchanged.

## Logical and physical tensor contract

The bounded logical tensors are:

| Tensor | Shape | Logical layout |
|---|---|---|
| H[h] | `[Tokens, HeadDim]` | row-major, h in `[0,7]` |
| C | `[Tokens, 8 * HeadDim]` | token-major concatenation |
| Wo | `[8 * HeadDim, ModelWidth]` | row-major |
| Y | `[Tokens, ModelWidth]` | row-major FP32 |

The canonical ordering is exact:

```text
C[token, h * HeadDim + column] = O[h][token, column]
```

M43's physical source is not reinterpreted as C. It supplies eight explicit,
non-overlapping device-buffer views, each with logical shape, physical row
stride, FP32 element type, producer/consumer access, device, lifetime, slot,
and slot generation. M44 validates all eight and performs the layout operation
explicitly when the interleave strategy is selected.

The bounded request envelope is exactly eight heads, Tokens in `[1,1024]`,
ModelWidth in `[1,4096]`, HeadDim in `[1,1024]`, and `8 * HeadDim <= 8192`.
The primary contract is `128 / 8 / 128 / 1024`, so C and Wo are both 1024
wide in their reduced dimension.

Y is one device buffer with logical shape `[Tokens, ModelWidth]`. Cooperative
and conventional-FP16 storage has padded row stride `roundUp16(ModelWidth)` and
padded rows `roundUp16(Tokens)`; A2x4 and direct segmented storage has compact
row stride `ModelWidth`. The exposed internal view records the stride rather
than implying compact storage. The optional final readback copies only the
logical ModelWidth columns into compact host Y.

## Bounded request and result

`prom_m44_plan_request` is the smallest internal contract needed by the
operator. It carries:

- exactly eight `prom_device_buffer_view` head sources;
- Tokens, HeadDim, ModelWidth, precision, aggregation, projection, and submit
  policies;
- one nonzero persistent Wo generation and content hash;
- one M43 aggregate replay identity;
- cooperative capability and bounded rollback facts.

`prom_m44_output_projection_plan` records logical/padded dimensions, source and
output strides, precision and strategy, exact stage order, timestamps,
dispatches, barriers and buffer counts, copy regions, submit count, memory,
eligibility reason, command-plan replay ID, and operation replay ID.
`prom_m44_composed_request/result` joins that plan to the existing fixed M43
request and reports one logical operation, physical slot/generation,
recyclability, retained output view, allocations/reuse, and stage timings.

The contract remains inside Prometheus-native code. No raw Vulkan handle or
scatter/gather SGEMM abstraction is exposed publicly, and no graph owner was
introduced.

## Persistent Wo ownership

One family resource owns output-projection weight
`Wo[8 * HeadDim, ModelWidth]`. Initial authority is finite host FP32. Preparing
Wo:

1. validates exact shape, element count, and finite FP32 content;
2. rejects zero or non-increasing generation;
3. waits and reaps every in-flight M43+M44 slot before replacement;
4. hashes the exact logical values;
5. uploads retained FP32 once;
6. retains device FP32 and padded packed F16x2 representations;
7. publishes generation, shape, and hash only after known GPU completion.

Warm execution does not pack, upload, allocate a Vulkan buffer, create a
pipeline, or create a descriptor layout. Stale generation and shape mismatch
reject before slot acquisition. Primary one-time Wo preparation measured
3.394 ms wall and 171.552 us GPU. The exact primary retained Wo representations
are 4,194,304-byte upload, 4,194,304-byte FP32 device, and 2,097,152-byte packed
F16 device buffers.

## Strategy 1: explicit interleave

`interleave_heads.sdslv` is one fixed-eight-head compute shader with ten
bindings: eight read-only head views and FP32/packed output destinations. One
invocation owns two adjacent destination elements. It derives token, head, and
column deterministically, reads only a valid logical source, and either:

- writes contiguous FP32 C for A2x4; or
- rounds the two FP32 values to F16-RNE and writes one packed F16x2 word for
  cooperative or conventional-FP16 projection.

Padded rows and columns are zero. The shader dispatches once, never writes both
representations in one plan, and reuses a grow-only slot buffer. There is no
host concatenate and no second full conversion pass.

The dependency is eight exact M43 output ranges from compute-write to
compute-read, then the exact C range from interleave compute-write to SGEMM
compute-read. Primary interleave measured 4.640 us in the one-submit plan and
4.448 us in the two-submit plan.

## Strategy 2: direct segmented projection

The bounded direct route implements:

```text
Y[token, out] = sum(h=0..7, column=0..HeadDim-1)
    roundF16(O[h][token,column]) * packedF16(Wo[h*HeadDim+column,out])
```

One dispatch binds the eight head views, persistent packed Wo, and final FP32
Y. It has no concatenate, partial-output, or accumulation buffer. It does not
generalize SGEMM to arbitrary gather input, and it does not introduce a broad
cooperative fragment/layout system.

The user's suggested SDSL-V flow/state and payload-enum features were useful
here. `HeadSample.Value { X, WeightRow }` carries the selected source and exact
Wo row, while a `ProjectOutput` flow board separates coordinate resolution,
fixed-eight accumulation, and store. This removes eight copies of the
round/unpack/weight/accumulate body. DXC `-O3` inlines and constant-folds the
comptime head loop: disassembly contains no function calls and retains eight
static head-buffer accesses. The same abstraction was not forced into the
interleave shader, whose low/high dynamic head selection would add tag
dispatch without removing its actual resource-selection work.

Direct segmented is correct but loses on the normal envelope: primary is
224.608 us, 2.05x the one-submit packed-interleave cooperative route. It is
useful at tiny, where 11.296 us beats one-submit cooperative's 18.176 us and is
slightly below conventional FP16's 12.896 us. Eight partial SGEMMs plus
accumulation were therefore not added: the smaller no-temporary direct attempt
already answers the segmented-layout question, and partials would require
`8 * Tokens * ModelWidth * 4` additional bytes plus nine dispatches.

## Precision contract

All M43 P-by-V head outputs are FP32. The output-projection boundaries are:

| Path | C/head boundary | Wo boundary | Accumulation/output |
|---|---|---|---|
| interleave cooperative | F16-RNE packed in interleave | persistent F16-RNE packed | FP32 |
| interleave conventional FP16 | same packed C | same packed Wo | FP32 |
| direct segmented | F16-RNE at each head load | persistent F16-RNE packed | FP32 |
| interleave A2x4 | exact FP32 C | persistent FP32 | FP32 |

The CPU oracle applies the same boundary. It creates C in canonical token/head
order, rounds both multiplicands for rounded paths, preserves exact FP32 for
A2x4, and accumulates/outputs FP32. Numerical tolerances were not weakened for
performance. The first mismatch reports strategy, token, output column,
expected/actual, absolute/relative error, Wo generation, M43 aggregate replay
ID, and M44 replay ID; source head/column follows directly from the canonical
reduced index.

## Composition, command plans, and synchronization

M44 extends the bounded M43 ring owner. The composed path records M43 without
its old eight-head readback, leaves its command buffer open for one-submit, and
consumes the real retained head views. There is one logical M43+M44 identity,
one slot fence, one final Y buffer, and one final correctness readback.

Two command plans are implemented:

1. one command buffer: M43, head visibility, interleave/direct projection,
   projection, and final Y readback in one submit;
2. two bounded submits: M43 signals the existing bounded dependency and M44
   waits on the same queue before projection. No secondary queue or ownership
   transfer exists.

The machine-readable M44 stage trace for interleave is:

| Seq | Operation | Dispatches | Barrier calls/ranges | Dependency |
|---:|---|---:|---:|---|
| 0 | heads ready | 0 | 1 / 8 | eight O compute-write -> compute-read |
| 1 | interleave | 1 | 1 / 1 | C compute-write -> compute-read |
| 2 | output projection | 1 | 1 / 1 | Y compute-write -> transfer-read |
| 3 | final readback | 0 | 1 / 1 | readback transfer-write -> host-read |

Direct has heads-ready, one direct-projection dispatch/barrier, and final
readback: three barrier calls covering ten ranges. Queue-family fields are
always `VK_QUEUE_FAMILY_IGNORED`; no whole-device barrier is used. Interleave
composition totals 90 dispatches, 61 barrier calls covering 91 buffers, and
Tokens final copy regions when joined to resident projection-grouped M43.
Direct totals 89 dispatches and 60/90 barriers/ranges. Query slots 199–206 are
disjoint from M43's 0–198 region.

At primary, two-submit cooperative measured 101.216 us M44 versus 109.824 us
one-submit, and 2.513 versus 2.738 ms complete M43+M44 GPU. End-to-end was
5.210 versus 5.559 ms. Two-submit is the current primary winner, but it is not
universal: one-submit wins tiny and larger-head end-to-end, while two-submit
wins primary, more-token, awkward, and 1024-token. The extra submit's CPU cost
is small but visible. Both remain bounded selectable plans; production
promotion waits for a stable multi-device decision.

## Lifecycle and faults

The M43 slot owns all eight heads through their final M44 consumer. M44 slot
state owns reusable concatenate upload/FP32/packed buffers, Y, readback,
descriptors, command buffer identity, and generation. Family state owns Wo,
the two M44 pipelines, layouts, and descriptor pool. M44 state is destroyed
before the shared Vulkan owners.

Known-completion logical faults remain distinct from physical recycling. The
permanent suite injects before aggregation, during interleave, after
interleave/before projection, mid-direct projection, after projection submit,
and before final readback. Each reports the exact logical failure and returns
the slot only after known fence completion. An uncertain-completion fault
quarantines the whole M43+M44 slot; reap observes the known fence and restores
it. No head or Wo generation can recycle while referenced.

Wo replacement waits/reaps both ring slots, changes the strictly increasing
generation, and stale-generation execution rejects. The two-slot warm fact
then proves no Vulkan buffer allocation occurs in repeated execution.

## Memory model

All arithmetic is checked before allocation. Buffers are grow-only and reused;
the exact M44 request is capped at 1 GiB. No system-memory spill is planned.
The final corpus primes both physical slots for every strategy before sampling,
so all device plans report the same retained-capacity assumption.

Primary M44 bytes are:

| Resource | packed interleave | FP32 interleave | direct segmented |
|---|---:|---:|---:|
| eight source head views | 524,288 | 524,288 | 524,288 |
| contiguous FP32 C | 0 | 524,288 | 0 |
| contiguous packed C | 262,144 | 0 | 0 |
| eight partial outputs | 0 | 0 | 0 |
| accumulation buffer | 0 | 0 | 0 |
| Wo upload | 4,194,304 | 4,194,304 | 4,194,304 |
| Wo FP32 | 4,194,304 | 4,194,304 | 4,194,304 |
| Wo packed | 2,097,152 | 2,097,152 | 2,097,152 |
| final Y | 524,288 | 524,288 | 524,288 |
| final readback | 524,288 | 524,288 | 524,288 |
| exact M44 bytes | 12,320,768 | 12,582,912 | 12,058,624 |

Each plan reports two reusable descriptor sets and fourteen bindings: the
ten-binding fixed-head layout plus the existing four-binding SGEMM layout.
Vulkan does not expose a portable descriptor-object byte size, so the exact
machine-readable contract reports counts rather than inventing bytes.

After capacity priming, every primary device strategy retains 49,545,228 bytes
for the complete M43+M44 owner. The packed-interleave M43+M44 exact request is
50,069,516 bytes; the difference reflects exact logical accounting versus
grow-only physical capacity and shared owner storage. Only one C
representation is logically required by each selected plan.

## RTX 3070 corpus and baselines

Every strategy uses two untimed executions to prime both grow-only slots,
thirty-two discarded warm operations to avoid idle-clock contamination, and a
five-operation median. Primary also records independent 10- and 100-operation
warm medians. All 42 records are correct and validation-clean.

Times below are nanoseconds. `coop-1 agg/proj/M44` is the central one-submit
device-resident metric. The strategy columns are complete M44 GPU intervals.

| Workload | coop-1 agg | coop-1 projection | coop-1 M44 | direct | A2x4 | conventional F16 | coop-2 M44 | M43+M44 GPU | coop-1 E2E | host-bounce E2E |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| tiny | 4,192 | 13,984 | 18,176 | 11,296 | 33,632 | 12,896 | 13,024 | 409,088 | 1,341,300 | 1,565,600 |
| primary | 4,640 | 104,448 | 109,824 | 224,608 | 352,864 | 443,712 | 101,216 | 2,738,016 | 5,559,200 | 8,560,800 |
| more tokens | 6,944 | 189,440 | 196,768 | 427,328 | 372,000 | 858,496 | 191,008 | 2,775,968 | 7,043,400 | 13,002,300 |
| larger head | 6,496 | 191,488 | 197,888 | 777,600 | 697,888 | 882,368 | 196,672 | 3,301,760 | 6,311,500 | 11,899,500 |
| awkward | 4,896 | 99,328 | 104,448 | 219,488 | 359,872 | 439,104 | 101,984 | 3,089,792 | 6,082,300 | 9,257,200 |
| 1024-token | 11,520 | 58,592 | 70,624 | 139,488 | 219,552 | 274,208 | 69,504 | 2,085,056 | 5,506,800 | 23,280,400 |

Primary host bounce reads all M43 heads, concatenates for 69.0 us on CPU,
packs for 425.5 us on CPU, uploads in 13.568 us GPU, projects in 96.256 us,
and incurs both head and Y readbacks. Its 8.561 ms end-to-end is 54.0% slower
than one-submit device residency and 64.3% slower than the primary two-submit
winner. The similar projection kernel time proves that residency and lifecycle,
not a different multiply, account for the product win.

M43-only primary GPU was 2.405 ms in the audit baseline. One-submit M44 adds
109.824 us, 4.6% of that M43 interval; output projection is not the dominant
complete-module cost. Its 104.448 us projection is 3.8% of the measured 2.738
ms product GPU interval. Primary warm 10/100-operation medians were 2.510/2.505
ms GPU and 5.440/5.229 ms end-to-end.

## Performance questions answered

1. **Is explicit interleave cheap enough?** Yes. It is 4.640 us primary and
   4.2% of M44 GPU.
2. **Is direct segmented actually faster?** No on normal shapes. It is 2.05x
   slower primary and increasingly worse at HeadDim 256. It wins tiny.
3. **Does packed interleave preserve cooperative benefit?** Yes. Cooperative
   is 3.21x faster than A2x4 and 4.04x faster than conventional FP16 primary.
4. **Is output projection dominant?** It dominates M44 but not M43+M44; it is
   only 3.8% of primary product GPU time.
5. **Does one-submit remain better?** Not consistently. Two-submit wins the
   primary central/product metrics and four of six E2E rows; one-submit wins
   tiny and larger-head E2E.
6. **Does head-major storage remain acceptable?** Yes. Its measured conversion
   cost is negligible and direct consumption is available as a bounded audit.
7. **Would token-major M43 output have been better?** Not enough to justify
   redesign. It could remove 4–12 us, while disturbing eight natural P-by-V
   destinations and their ownership.
8. **Is fused QKV or an output-layout change justified next?** Not by M44.
   M43 projection work remains the larger optimization target; changing output
   layout saves too little.
9. **Where do cases roll back?** Tiny selects direct/conventional; normal,
   awkward, and 1024-token M44 favor cooperative interleave. Submit choice is
   not stable at tiny and larger-head E2E. M43's own 1024-token and tiny
   rollback evidence remains relevant.
10. **Is the bounded attention module production-worthy?** It is complete and
    useful, but not production-promoted because strategy/device evidence is
    still limited and the complete M43 envelope is not uniformly positive.

## Eligibility and fallback

The deterministic M44 predicate rejects exact reasons for head count, invalid
view, shape/stride mismatch, zero generation, overlap, missing/stale Wo, shape,
precision, cooperative capability, padding, strategy combination, capacity,
or rollback. Direct segmented is allowed only with its F16-rounded policy;
A2x4 requires FP32; cooperative/conventional require F16-rounded input.

If cooperative capability is unavailable, the composed request records and
executes interleave plus conventional FP16 with the same rounded precision.
Explicit A2x4 remains the FP32 fallback, and direct segmented remains the
no-concatenate bounded alternative. Host bounce exists only as an audit
baseline. The permanent extension-disabled fact verifies conventional
fallback and all eight consumed views. No global SGEMM selector changes.

## Replay identities and provenance

Replay identity hashes exact shapes, precision, aggregation/projection/submit
policy, Wo generation/hash, M43 aggregate identity, both M44 shader hashes,
and the deterministic command plan. Primary resident identities are:

| Plan | M44 replay ID | M43 aggregate replay ID |
|---|---:|---:|
| interleave cooperative one-submit | 15610795104405013023 | 2525274942473759781 |
| interleave A2x4 one-submit | 18119834671965669215 | 2525274942473759781 |
| interleave conventional one-submit | 14268367080758783365 | 2525274942473759781 |
| direct segmented one-submit | 251721194714206753 | 2525274942473759781 |
| interleave cooperative two-submit | 9109543952165686668 | 2525274942473759781 |

The two new SDSL-V assets remain experimental and selector-ineligible. Both
target Vulkan 1.0, use DXC 1.9.0.5347 (`fe2615732`), pass deterministic
regeneration, `spirv-val`, and interface disassembly assertions.

| Shader | source SHA-256 | SPIR-V SHA-256 |
|---|---|---|
| interleave | `67de5fac9fc51ee483b07e2c38d48d077f89e5942b8d08ca3307b3896695ad43` | `ef5bd1d4aac8cce92c0548f92e63f9c8be508217bc62e1c4d14c40d07ace041e` |
| direct segmented | `08a4106010c596ac9ac45743087e683804fb8891152f09a0080e19b009244b79` | `1f2c7914051d28c23605731f95125d4d048327c1c8e0af769f38b16693032365` |

The committed RTX 3070 artifact is
`DevelopmentReport/artifacts/M44/multihead_aggregation_output_projection_rtx3070.json`.
The workspace checker verifies its 42-record corpus, shapes, plan matrix,
correctness, validation, precision, memory, identical primed retention,
residency, warm evidence, and exact shader hashes.

## Validation

Permanent non-hardware facts cover eight-view validation, exact concatenate
indexing, output layout/stride, Wo shape/generation, overflow/capacity,
interleave/direct plans, zero partial storage, barrier order/ranges, one final
readback, no device-plan host concatenate, deterministic replay, eligibility
reasons, stale Wo, and fault ownership. Hardware facts cover all device paths,
real M43 composition, one/two submit, host bounce, warm reuse, Wo replacement,
all fault points, quarantine/reap, extension-disabled fallback, and validation.

Validation results:

- M44 focused facts: 5/5 passed on the RTX 3070, including all projection
  paths, both submit plans, host bounce, warm allocation reuse, Wo replacement,
  every fault point, quarantine/reap, extension-disabled fallback, and zero
  validation warnings/errors;
- M43 regressions: 5/5 passed; M42 regressions: 5/5 passed;
- M44 six-workload benchmark: 42/42 plan/workload records correct, zero
  validation warnings/errors, zero device loss, and deterministic artifact
  consistency passed;
- full normal Marionette lane: 386 tests, 354 passed, 32 hardware-gated legacy
  skips, zero failed; focused M42/M43/M44 hardware lanes ran separately and did
  not skip;
- all requested Go lanes passed: `internal/source`, `internal/diagnostic`,
  `internal/sdslv/...`, both octxiliary trees, `internal/cli`, `cmd/oct`, the
  combined `internal/... ./cmd/oct`, and `tools/build_sidecars`;
- authoritative MSVC Windows rebuild passed; native manifest parity and the
  SDSL-V workspace/artifact checker passed;
- both M44 shaders reproduced byte-identical HLSL, SPIR-V, and header word
  streams; `spirv-val --target-env vulkan1.0` and disassembly interface
  assertions passed;
- `bash -n`, `git diff --check`, Linux GCC shared-library/test builds, ELF
  verification, and the Linux Marionette smoke passed.

The Linux builder surfaced an existing warning in `sdslv_test_host.c` line 312
about comparing a pointer to a zero character constant. It is outside the M44
runtime/shaders and did not affect the successful build or smoke; it was not
silently changed as part of this milestone.

## Exact next transformer-facing workload

The next bounded consumer should be one device-resident residual add:

```text
resident X[128,1024] generation Gx
resident M44 Y[128,1024] generation Gy
  -> Z[128,1024] = X + Y
  -> one retained FP32 device view
```

It should reuse M44's slot/lifetime and explicit stride contract, compare
in-place versus separate-output ownership, and preserve one final optional
readback. Normalization should remain a separate later milestone so residual
ownership and precision are measured without introducing a transformer graph
or a fused residual-normalization abstraction.

## M45 first-consumer status

M45 now consumes this real Y before the M43/M44 slot can recycle. It combines
the immutable resident X with Y under either a disjoint Z allocation or an
exclusive in-place-Y ownership transition, assigns a distinct post-residual
content generation, and exposes one retained FP32 Z view. M44's APIs, shader
identities, selector behavior, replay identities, and experimental
classification remain unchanged; the composed M45 replay wraps a
readback-stripped internal M44 identity because only final Z may cross the
host boundary.
