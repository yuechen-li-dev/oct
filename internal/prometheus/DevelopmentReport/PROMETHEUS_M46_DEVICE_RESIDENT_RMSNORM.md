# Prometheus M46 device-resident RMSNorm

## Outcome

Convergence outcome: **SUCCESS**  
Milestone state: **COMPLETE**  
RMSNorm operator classification: **2 — experimental RMSNorm candidate**  
Composed M43+M44+M45+M46 classification: **experimental transformer fragment**

M46 consumes the exact retained unread M45 Z buffer, performs bounded FP32
RMSNorm entirely on the Vulkan device, and retains one FP32 N view. The
operator supports separate output and exclusive in-place Z-to-N ownership,
one-submit and same-queue semaphore split-submit topology, a one-workgroup row
reduction through width 1024, and deterministic staged partial reduction above
1024. Correctness optionally reads only final N. There is no Z readback,
host-side normalization, or intermediate host copy in the product path.

The RTX 3070 corpus covers seven workloads, all four normal device plans, a
forced-staged primary audit, M43+M44+M45 without normalization, and a favorable
CPU host-bounce lower bound. All 43 records are correct and validation-clean.
M42–M45 APIs, manifests, shader IDs, selectors, replay behavior, and existing
classifications remain unchanged.

No LayerNorm, mean subtraction, bias, FFN/MLP, gated activation, residual
fusion, rotary embedding, graph scheduler, model importer, training path,
Direct3D, PTX, CUDA interop, or distributed execution was added.

## Tensor, storage, and precision contract

```text
Z:       FP32 row-major [Tokens, ModelWidth], explicit physical stride
Weight:  FP32 [ModelWidth], persistent family-owned device resource
InvRms:  FP32 [Tokens], slot-owned temporary
N:       FP32 row-major [Tokens, ModelWidth], compact or exact Z alias

Tokens      in [1,1024]
ModelWidth  in [1,4096]
epsilon     finite and > 0; corpus value 1e-5
```

The exact operation is:

```text
sumsq[token] = sum_column(Z[token,column] * Z[token,column])
inv_rms[token] = rsqrt(sumsq[token] / ModelWidth + epsilon)
N[token,column] = Z[token,column] * inv_rms[token] * Weight[column]
```

Input, accumulation, scale, epsilon, reciprocal square root, and output are
FP32. Logical width controls the reduction; padded columns never contribute
and remain untouched. There is no hidden reduced-precision boundary and the
only broadcast is `Weight[column]`.

## Request and eligibility

`prom_m46_plan_request` carries one Z view, exact shape, epsilon, ownership
strategy, submit policy, optional bounded reduction audit policy, expected Z
generation, Weight generation/hash, Z exclusivity, remaining pre-normalization
consumer count, M45 replay identity, and final-readback policy.
`prom_m46_composed_request` joins that contract to the real resident M45
producer. Raw Vulkan handles do not cross the internal reactor boundary.

Eligibility requires:

- one non-null FP32 row-major Z view with sufficient explicit stride/range;
- exact shape within `[1,1024] x [1,4096]`;
- nonzero matching Z content and physical slot generations;
- nonzero matching Weight generation/hash and exact width;
- finite positive epsilon;
- separate output, or exclusive Z with zero pre-normalization consumers;
- checked sizing below the 1 GiB request cap;
- automatic reduction selection, legal forced fused, or legal forced staged.

Stale Z/Weight, invalid epsilon, insufficient stride, missing exclusivity,
unsupported force-fused width, overflow, and capacity failure reject before
submission.

## Persistent Weight ownership

`prom_reactor_runtime_m46_prepare_weight` accepts exactly one finite host FP32
vector. It computes a stable hash before upload, requires a strictly increasing
nonzero generation, waits/reaps all in-flight family slots before replacement,
and retains one host-visible upload authority plus one device-local FP32
buffer. Both M46 pipelines are created during preparation. Invocation performs
no Weight upload, conversion, pipeline creation, descriptor-layout creation,
or Vulkan allocation after warm capacity.

Primary one-time Weight preparation measured **0.517 ms** and retains 4,096
upload plus 4,096 device bytes. Replacement after injected uncertain completion
waits/reaps the quarantined composed slot, publishes the newer generation, and
recovers. A stale generation rejects before M43 execution.

## Reduction strategy and InvRms audit

M46 reuses M39b's deterministic 256-lane, 1024-element chunking and shared
memory reduction principles without changing the production M39b API or
adding a generic reduction-expression engine.

- Width `<= 1024`: one workgroup per row accumulates `Z*Z`, derives InvRms,
  and writes one FP32 value per token. Apply is the second dispatch.
- Width `> 1024`: workgroups write compact 1024-column partial sums, one final
  workgroup per row derives InvRms, and apply is the third dispatch.
- A bounded forced-staged audit is available where fused is legal. It is not
  selected automatically and participates in replay identity.

No full squared tensor exists. Explicit InvRms costs 512 bytes primary and
makes the in-place safety boundary mechanical. Primary forced staged adds 512
partial bytes and measured 11.040 us versus 8.224 us for fused in-place
one-submit. Fused is about 25.5% faster for this comparison, while explicit
InvRms remains worthwhile for bounded lifecycle clarity.

Staged execution becomes necessary above width 1024. The 2048 case uses two
partials per row; `[128,4096]` uses four partials per row and exactly 2,048
partial bytes. Both execute correctly on hardware.

## Output strategies and chosen plan

Separate output keeps Z read-only and writes compact N into one grow-only
slot buffer. It is the correctness baseline and device-resident fallback.

In-place Z requires exclusive ownership and zero remaining consumers of the
pre-normalized value. All sum-of-squares reads and the InvRms write complete
before the exact Z range transitions from shader-read to
shader-read|shader-write. Apply renames the same physical buffer to N. Physical
slot generation remains stable; content generation changes. The buffer is
returned exactly once under its final N role.

The chosen product strategy is **in-place Z**. Primary best M46 time is 7.936
us versus 8.992 us for best separate output, and it saves 524,288 exact bytes.
The chosen primary submit plan is the two-submit semaphore split, while both
topologies remain selectable because the winner is not universal across the
corpus.

## Command buffers, submits, and synchronization

One-submit records M43, M44, M45, M46 reduction/final reduction/apply, and
optional final N readback in one open slot command buffer and performs one
queue submission.

Two-submit records M43+M44+M45 in the producer command buffer and M46 in the
consumer command buffer. The first submission signals the existing slot
semaphore; the second waits at compute stage on the same queue. There is no
host wait between submits, secondary queue, ownership transfer, or generic
graph scheduler.

The normalized dependency trace is:

```text
M45 Z compute write -> M46 reduction compute read
partial compute write -> final reduction compute read       (staged only)
InvRms compute write -> apply compute read
all Z reduction reads -> apply Z read or read/write
N compute write -> transfer read or next compute read
readback transfer write -> host read                        (optional)
```

Every barrier uses the exact buffer range and `VK_QUEUE_FAMILY_IGNORED`.
Permanent tests distinguish separate Z-read from in-place Z-read/write and
assert staged partial ordering. No whole-device barrier exists.

## Generation, aliasing, and replay

Separate output creates a distinct physical N buffer and new N content
generation while preserving Z. In-place preserves VkBuffer and slot generation
but consumes the logical Z generation and assigns a new N generation.

Replay identity hashes shape/stride, exact epsilon bits, prior Z generation,
Weight generation/hash, strategy, submit topology, selected/forced reduction
plan, both shader hashes, M45 replay identity, normalized barrier plan, and N
generation inputs. Repeated planning is deterministic.

Representative primary identities are:

| Strategy / submit | M46 replay | M45 replay | N generation |
|---|---:|---:|---:|
| separate / one | 6342321793607224740 | 17219951903141793311 | 3958728033471764877 |
| separate / two | 4109337325948332888 | 4700567342960732326 | 967547882129673482 |
| in-place / one | 3099473545049820896 | 14259086788021863236 | 18249265685062525422 |
| in-place / two | 13083499866804136308 | 5186783483116879492 | 7636889614622832201 |
| in-place / forced staged | 16791387565752705103 | 3443878108640965607 | 2641598856151942638 |

The final forced-staged values above are taken from the committed M46 artifact;
the artifact checker rejects missing, duplicate, zero, or structurally invalid
identities.

## Memory model

Primary `[128,1024]` plan-visible bytes with final readback are:

| Resource | separate N | in-place Z |
|---|---:|---:|
| Z physical view | 524,288 | 524,288 |
| Weight device | 4,096 | 4,096 |
| partial sums | 0 | 0 |
| InvRms | 512 | 512 |
| separate N | 524,288 | 0 |
| final compact readback | 524,288 | 524,288 |
| exact total | 1,577,472 | 1,053,184 |

The upload authority adds 4,096 family-retained bytes outside the per-request
total. Forced-staged primary adds 512 partial bytes. `[128,4096]` in-place uses
2,048 partial plus 512 InvRms bytes and saves a 2,097,152-byte N tensor.
Token-boundary in-place saves 4,194,304 bytes.

The warmed audit owner retains about 49.6–50.1 MB primary because it primes
both physical slots and both output strategies. A product owner selecting only
in-place does not create the separate N allocation. All temporaries are
grow-only; warm corpus assertions prove no Vulkan allocation after both slots
reach capacity.

## RTX 3070 timing corpus

The committed artifact contains 43 records over tiny, primary, more-token,
wider, awkward, 4096-width, and 1024-token workloads. Each normal plan primes
both slots, runs 16 warm operations, and records a five-operation median.

| Workload | best strategy / submit | plan | M46 GPU | complete GPU | end-to-end | saved bytes |
|---|---|---|---:|---:|---:|---:|
| tiny | in-place / two | fused | 6.368 us | 0.431 ms | 1.568 ms | 8,192 |
| primary | in-place / two | fused | 7.936 us | 2.527 ms | 5.309 ms | 524,288 |
| more tokens | in-place / two | fused | 9.664 us | 2.716 ms | 7.937 ms | 1,048,576 |
| wider | in-place / two | staged | 12.640 us | 4.942 ms | 9.551 ms | 1,048,576 |
| awkward | separate / one | fused | 8.608 us | 2.497 ms | 5.244 ms | 508,508 |
| staged 4096 | in-place / one | staged | 16.320 us | 10.396 ms | 20.586 ms | 2,097,152 |
| token boundary | in-place / two | fused | 28.128 us | 5.229 ms | 23.295 ms | 4,194,304 |

Primary in-place/two stage medians are 4.512 us reduction/InvRms, 3.072 us
apply, 7.936 us total M46, 2.527 ms complete GPU, 0.664 ms CPU recording,
0.083 ms CPU submission, 1.672 ms final readback, and 5.309 ms end-to-end.

Primary 10-operation medians are 8.224 us M46, 2.520 ms complete GPU, and
5.683 ms end-to-end. Primary 100-operation medians are 8.000 us, 2.521 ms,
and 5.546 ms respectively.

M43+M44+M45 without normalization measures 2.509 ms complete GPU and 5.512 ms
end-to-end primary. RMSNorm is about 0.31% of the best complete GPU interval.

The favorable host-bounce audit reads final Z, normalizes on CPU, and omits
reupload. Primary CPU normalization is 0.184 ms and the lower-bound end-to-end
cost is 5.702 ms. At 4096 width it is 0.735 ms and 20.682 ms. This lower bound
is only modestly worse when both paths already pay final readback; it is not a
valid retained-N product fallback, and reupload would widen the gap.

## Performance questions

1. RMSNorm is small relative to the complete fragment: about 0.31% primary.
2. In-place saves meaningful memory: 0.5 MiB primary and 4 MiB token-boundary.
3. In-place beats separate primary M46 time by about 11.7%; awkward favors
   separate, so the timing win is not universal.
4. Explicit InvRms is worthwhile: 512 bytes primary buys a simple, proven
   destructive-write boundary.
5. Fused beats forced staged primary: 8.224 versus 11.040 us.
6. Staging is required above width 1024 and remains small at 2048/4096.
7. Submit topology is material only at the margin and has no universal winner;
   primary favors two submits.
8. Padded stride is correct. Awkward width 1001 passes every plan and remains
   near 8–9 us best M46 time.
9. Host bounce is a predictable product violation. Its no-reupload lower bound
   is only modestly slower under final-readback measurement; retained execution
   avoids the mandatory reupload needed by the next device consumer.
10. The retained N view, generation, alias, and lifecycle contracts are stable
    enough for the gated FFN milestone.

## Correctness, lifecycle, and faults

The CPU oracle accumulates in column order, applies exact epsilon and scale,
rejects nonfinite Z/Weight/sum/InvRms/N, supports zero and very small rows, and
leaves padding untouched. First mismatch reports strategy, token, column,
expected/actual, absolute/relative error, sumsq, InvRms, epsilon,
Z/Weight/N generations, and M45/M46 replay identities.

Known-completion fault points before reduction, after the first partial,
before final reduction, after InvRms, before/during apply, after submission,
and before final readback submit a bounded prefix, wait known completion, and
return the slot exactly once. Uncertain completion quarantines the whole
composed slot. Weight replacement waits/reaps it and a newer generation
recovers. M46 state is destroyed before M45/M44/shared Vulkan resources.

Permanent non-hardware facts cover request and epsilon validation, awkward
stride, auto/forced fused and staged planning, temporary sizing, exclusivity,
barrier ranges/order, optional final readback, no intermediate copy, overflow,
generation transition, deterministic replay, CPU oracle, padding, and mismatch
localization. Hardware facts cover both strategies/topologies, fused/staged,
Weight replacement, all fault points, and validation cleanliness.

## Shader provenance

| Asset | source SHA-256 | HLSL SHA-256 | SPIR-V SHA-256 |
|---|---|---|---|
| reduce | `937cdc8e4adc6d79d5b629a71bf9d3825f34297685849f9a2dd80d6d2e6909f3` | `fc37afe0690519c4811ca7a8392392b9a12e780b7a2236d22bdf3bb850752cd2` | `a9af5205bcd60571ec11384ff2499c2bb8f548ce87514d038504d7f596ecb3fa` |
| apply | `360f69373898bdd38d6d36f1be424a260f7d32e4d29b8008a2f9e80303c9b672` | `6fa77a1d12341a42e58192bbb30ced9a8c4d0e79d499731f66624ad83c8cfce2` | `78750b592ac5470ebd5ee0e91dad25ce6f33d1e2c9c519c8ea1dde204b0caaa1` |

DXC 1.9.0.5347 targets Vulkan 1.0. `spirv-val` accepts both modules.
Disassembly proves local size 256, reduction bindings 0/3, apply bindings
0/1/2/3, FP32 multiply/add/divide, `InverseSqrt`, and intended entry points.
Second generation reproduces byte-identical HLSL, SPIR-V, and header words.
Both assets have experimental authority and remain selector-ineligible.

## Validation, classification, and rollback

Validation evidence includes:

- Windows MSVC native library, facts, and benchmark rebuild;
- focused M42–M46 regressions and four M46 hardware facts;
- 43/43 benchmark records correct, zero validation warnings/errors;
- real fused, forced staged, 2048 staged, and 4096 staged execution;
- one-submit and semaphore split, separate and in-place, warm allocation proof,
  Weight replacement, all fault points, and deterministic aggregation;
- full required Go test matrix;
- native manifest and M46 artifact consistency checks;
- SDSL-V workspace, deterministic generation, `spirv-val`, and disassembly;
- Linux GCC shared-library/test builds, ELF verification, and smoke;
- `bash -n` and `git diff --check`.

The operator is an **experimental RMSNorm candidate**, not research-only.
Correctness, bounded reduction, ownership, lifecycle, replay, memory, and
performance evidence are complete on the RTX 3070. Production promotion is
withheld because shader authority remains experimental and validation covers
one Vulkan device family. The complete transformer fragment remains
experimental because M43/M44/M45 retain their classifications.

Rollback is localized to the M46 structs/functions/state in
`reactor_vulkan.h` and `reactor_vulkan_fused_reduction.c`, the two experimental
shader assets and manifest entries, M46 facts/benchmark, artifact checker,
committed artifact, this report, and the M45 consumer-status paragraph.
Removing those regions restores M45 without changing its public behavior.

## Exact next FFN-facing workload

```text
N: FP32 row-major [128,1024]
physical stride: explicit (1024 primary)
device: same Vulkan device
content generation: nonzero M46 N generation
producer access: RMSNorm compute write
required consumer access: compute read
owner: same bounded composed slot until the FFN fence completes
readback: none before the gated FFN
```

The next milestone may implement one bounded gated feed-forward operator over
this retained N. M46 does not preselect FFN hidden width, activation, second
projection, second residual, or graph scheduling.
