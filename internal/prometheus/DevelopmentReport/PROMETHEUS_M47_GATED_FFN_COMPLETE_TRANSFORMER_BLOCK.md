# Prometheus M47 gated FFN and complete bounded transformer block

## Outcome

Convergence outcome: **SUCCESS**  
Milestone state: **COMPLETE**  
Bounded gated-FFN classification: **experimental production candidate**  
Complete M43–M47 block classification: **experimental complete transformer-block candidate**

M47 consumes the real retained M46 N view and executes the complete bounded
transformer suffix on the Vulkan device:

```text
X -> M43 grouped attention -> M44 output projection -> M45 first residual
  -> M46 RMSNorm -> N
  -> Gate = N x Wgate
  -> Up = N x Wup
  -> Hidden = SiLU(Gate) * Up
  -> Down = Hidden x Wdown
  -> BlockOutput = N + Down
```

The topology is the Prometheus EVT golden path, not a claim about a universal
transformer standard. The second residual source is N. Gate and Up share N.
Wgate, Wup, and Wdown are distinct persistent parameters. M47 adds no second
normalization, bias, dropout, GELU, activation selector, graph scheduler, model
import, multi-block runtime, logits, sampling, training, or distributed path.

The preferred product route is direct-packed SiLU gating, cooperative
projections, in-place Down-to-BlockOutput ownership, and the same-queue split
after M46. It performs no intermediate readback; correctness optionally reads
only final BlockOutput. All seven requested workload classes execute correctly
on the RTX 3070 across 49 benchmark records with zero validation warnings or
errors.

## Request, tensor, and precision contract

`prom_m47_plan_request` carries one retained FP32 row-major N view with explicit
physical stride, exact Tokens/ModelWidth/FfnWidth, three weight generations and
hashes, projection path, gating strategy, Hidden storage, residual alias plan,
submit policy, expected N generation, M46 replay identity, and final-readback
policy. `prom_m47_composed_request` joins that contract to the real M46 producer.
No raw Vulkan handle is exposed outside the internal reactor boundary.

```text
Tokens      in [1,1024]
ModelWidth  in [1,4096]
FfnWidth    in [1,8192]
primary     [128,1024,4096]
request cap 1 GiB
```

All element and byte products are checked before slot acquisition. Views require
the same Vulkan device, sufficient explicit stride/range, nonzero content and
physical generations, compute-write producer access, and compute-read consumer
access. Stale N or any stale weight rejects before M43 recording. A2x4 requires
compact FP32 N; reduced paths pack a strided N view into a padded F16x2 boundary.

Gate and Up SGEMMs produce FP32 logical values with FP32 accumulation/output.
SiLU uses the stable exact FP32 forms:

```text
x >= 0: x / (1 + exp(-x))
x <  0: x * exp(x) / (1 + exp(x))
```

The product is FP32. Cooperative and conventional-FP16 Wdown round Hidden to
F16-RNE explicitly; A2x4 consumes exact FP32 Hidden. The CPU oracle applies the
same N/weight/Hidden boundaries and rejects nonfinite inputs, weights,
intermediates, and output. No clamp or GELU substitution exists.

## Persistent three-weight ownership

The family owns independent resources for:

```text
Wgate [ModelWidth,FfnWidth]
Wup   [ModelWidth,FfnWidth]
Wdown [FfnWidth,ModelWidth]
```

Each accepts finite host FP32 authority, validates exact shape/count, hashes the
logical values, requires its own strictly increasing nonzero generation, and
retains upload, device FP32, and padded packed-F16 representations. Replacement
waits/reaps all family slots before publishing only the selected generation.
The hardware fact independently replaces Gate, Up, and Down after uncertain
completion, verifies stale rejection, and recovers with the three new
generations. Warm invocation performs no pack, upload, pipeline creation,
descriptor-layout creation, or Vulkan allocation.

Primary aggregate Wgate/Wup/Wdown preparation is **46.485 ms wall** and
**2.076 ms GPU upload/pack**. Each primary weight retains 16,777,216 upload,
16,777,216 FP32, and 8,388,608 packed bytes; all three retain 125,829,120 bytes.

## Projection and gating strategies

Gate and Up are two independent SGEMMs. A concatenated projection was audited
but not implemented: existing SGEMM can write one wide output, but preserving
independent weight replacement/hash/generation identity would add a fourth
combined representation and slicing ownership. Measurements show the two
projections dominate but do not justify that retained-state complexity yet.

The separate baseline dispatches SiLU into ActivatedGate, barriers it, then
multiplies by Up into FP32 Hidden. Reduced Wdown adds the existing generic pack.
The fused-FP32 strategy writes Hidden in one elementwise dispatch and then packs
where required. The preferred direct-packed shader performs SiLU and multiply,
rounds once to F16-RNE, zeroes padded rows/columns, and writes the exact packed
Wdown input without a full FP32 Hidden allocation or pack pass.

Primary cooperative M47 medians:

| Gating | M47 GPU | Complete block GPU | Exact M47 bytes |
|---|---:|---:|---:|
| separate activation/multiply | 1.067 ms | 3.887 ms | 137,101,312 |
| fused FP32 + pack | 1.043 ms | 3.815 ms | 135,004,160 |
| fused direct-packed | 1.035 ms | 3.805 ms | 132,907,008 |
| fused direct-packed, split | **0.958 ms** | **3.504 ms** | 132,907,008 |

Fusion saves one 2,097,152-byte ActivatedGate tensor primary. Direct packing
also removes the 2,097,152-byte FP32 Hidden tensor and a measured 5.120 us pack
pass. The primary separate-to-direct M47 improvement is about 10.2%; the split
direct route improves another 7.4% relative to one-submit direct in this run.

## Wdown and second residual ownership

Wdown is a real persistent SGEMM and remains device-resident. The chosen
residual strategy aliases exclusive Down as BlockOutput. N stays immutable for
replay/audit and retains its content generation. Down keeps its physical buffer
and slot generation but receives a distinct BlockOutput content generation. The
alias is returned once under its final role.

Separate BlockOutput remains the device fallback and audit baseline. In-place N
is explicitly rejected because N is both the FFN source and retained M46 audit
state. Primary direct-packed separate output measured 0.961 ms M47 versus
0.958 ms in-place Down; the computational difference is noise-scale, while
in-place saves exactly 524,288 logical bytes. Residual itself is **3.072 us**
primary, 0.32% of M47, so it is computationally free but ownership-significant.

## Command plans and synchronization

One-submit records M43, M44, M45, M46, M47, and optional final readback in one
command buffer and submits one fence. Two-submit records M43 through M46 in the
producer buffer, signals the existing slot semaphore, and records all M47 work
in the consumer buffer. The second submit waits at compute stage on the same
queue. There is no host wait, second queue, ownership transfer, or graph owner.

The normalized M47 dependency plan is:

```text
M46 N compute write -> N pack or Gate/Up read
packed N write -> Gate/Up SGEMM read                       (reduced paths)
Gate write -> activation/fused-gating read
Up write -> multiply/fused-gating read
ActivatedGate write -> multiply read                      (separate only)
Hidden FP32 write -> pack or A2x4 Wdown read
packed Hidden write -> cooperative/conventional Wdown read
Down write -> residual read/write
N read -> second residual read
BlockOutput write -> final transfer read or next compute read
readback transfer write -> host read                      (optional)
```

All barriers use exact buffer ranges and `VK_QUEUE_FAMILY_IGNORED`. There is no
whole-device or unrelated-head barrier. Seven fixed four-binding descriptor sets
are allocated with the family, reused by every invocation, and keep the command
trace bounded and inspectable.

## Lifecycle, faults, and replay

N remains alive through both projections and the residual. Gate/Up remain alive
through gating, Hidden through Wdown, and Down through residual. The composed
two-slot owner quarantines the complete M43–M47 slot on uncertain completion and
reaps only after known fence completion. Known-completion faults submit a safe
prefix, wait, and return the slot once.

Hardware injection covers before Gate, between Gate/Up, before gating, separate
activation, fused gating, after Hidden, Wdown, before residual, after residual
submission, before final readback, and uncertain completion. Independent weight
replacement reaps the quarantined slot. Warm repeated execution proves stable
allocation and pipeline counts.

Replay identity hashes exact shapes/strides/padding, N generation, all weight
generations/hashes, projection route, gating and Hidden storage, residual alias,
submit topology, both shader hashes, M46 replay identity, command/barrier plan,
and BlockOutput generation. Primary preferred M47 replay is
`10475302116931058988`; its BlockOutput generation is `9606319251323345418`.
Physical handle reuse never implies semantic identity reuse.

## Memory model

Primary preferred exact M47-visible bytes, including one final readback:

| Resource | Bytes |
|---|---:|
| retained N view | 524,288 |
| packed N | 262,144 |
| three weights: upload + FP32 + packed | 125,829,120 |
| Gate + Up | 4,194,304 |
| direct-packed Hidden | 1,048,576 |
| Down / aliased BlockOutput | 524,288 |
| final readback | 524,288 |
| exact total | **132,907,008** |

Weights are 94.7% of the exact M47 footprint. No simultaneous redundant FP32
and packed Hidden exists in the selected direct route. The fully primed audit
owner retains 185,606,672 bytes because both physical slots and alternate
strategies were exercised. No system-memory spill occurred.

## RTX 3070 corpus and timings

The committed artifact contains 49 correct records: seven workloads times seven
strategy combinations. Each plan primes both slots, performs four discarded
warm operations, and records a five-operation median. Primary preferred stage
medians are:

```text
N pack                 3.648 us
Gate projection      283.648 us
Up projection        283.648 us
fused direct gating   11.264 us
Wdown projection     373.760 us
second residual        3.072 us
M47 total            958.464 us
M43->M47 total         3.504 ms
CPU recording          0.683 ms
CPU submission         0.083 ms
final readback          1.600 ms
end-to-end              6.282 ms
```

Primary 10-operation medians are 0.958 ms M47, 3.504 ms complete GPU, and
6.267 ms end-to-end. Primary 100-operation medians are 0.956 ms, 3.495 ms, and
6.329 ms respectively.

| Workload | Best M47 strategy | M47 GPU | Complete GPU | End-to-end |
|---|---|---:|---:|---:|
| tiny `[16,128,256]` | direct conventional / one | 37.888 us | 0.471 ms | 1.466 ms |
| primary `[128,1024,4096]` | direct cooperative / split | 0.958 ms | 3.504 ms | 6.282 ms |
| more tokens `[256,1024,4096]` | direct cooperative / split | 1.859 ms | 4.590 ms | 9.053 ms |
| smaller expansion `[128,1024,2048]` | direct cooperative / separate output | 0.577 ms | 3.111 ms | 5.787 ms |
| wider `[128,2048,4096]` | direct cooperative / split | 1.860 ms | 6.811 ms | 11.265 ms |
| awkward `[127,1001,3001]` | direct cooperative / split | 0.836 ms | 3.362 ms | 6.222 ms |
| token boundary `[1024,256,512]` | direct cooperative / separate output | 0.276 ms | 2.507 ms | 7.786 ms |

Primary conventional FP16 is 5.118 ms M47 and A2x4 FP32 is 2.924 ms, versus
0.958 ms cooperative direct-packed split. The cooperative path retains a 5.34x
M47 advantage over conventional and 3.05x over A2x4 at the primary shape.

The host-bounce audit reads final N through M46, runs the precision-matched CPU
FFN, and deliberately omits reupload. Primary reduced CPU FFN is 9.483 s and the
lower-bound end-to-end cost is 9.497 s. Exact A2x4 CPU is 6.005 s. This is an
honest audit lower bound, not a product fallback; reupload would only widen the
gap.

## Performance answers and rollback regions

1. Gate plus Up are 567 us primary, 59% of M47; they are the largest combined
   component.
2. Fused FP32 beats separate modestly; direct packing is the stable winner.
3. Direct packing removes one full Hidden pass, 2 MiB primary FP32 storage, and
   about 5 us of measured pack work.
4. A fused Gate/Up projection is not yet justified by its generation/layout cost.
5. Wdown is the largest single stage at 374 us primary.
6. The 3 us second residual is effectively free computationally.
7. In-place Down saves 0.5 MiB primary and remains the preferred ownership plan,
   even where separate output wins a noise-scale timing sample.
8. Split wins primary, more-token, wider, and awkward; one-submit wins tiny and
   is close elsewhere. Both remain selectable.
9. Tiny rolls to conventional FP16. Token-boundary and smaller expansion show a
   separate-output timing sample winner but retain in-place for the product
   memory policy. Awkward A2x4 uses compact separate M46 N because A2x4 has no
   strided-A contract.
10. One bounded block is lifecycle-stable for repeated execution. M48 may repeat
    this exact block contract, but must add its own multi-block ownership rather
    than treating M47 as a generic scheduler.

## Shader provenance and validation

| Shader | source SHA-256 | HLSL SHA-256 | SPIR-V SHA-256 |
|---|---|---|---|
| FP32 separate/fused | `4ef651b948505b08e91489a32960884e085e3ee422e31b92c91881e7b93a738e` | `388ed6248037ed0169211f0e40c75f0854caf2d1883e360bbdd6770944570f6a` | `4224253f52d36e3268f4ded56cc58b0a50419b6b6b33b9a279a233c730b8a092` |
| direct/pack | `fccf5c74f4aabbdef9075ffd174222659c249e7508eae5f6eba6b54831971ac4` | `b8382d6ef2fa0124d0a39f00d09ac51f933a8c3ada0050740f729fcdac45a75a` | `6de00e90fd1f3249e51d034dd12f4304a1c8a0d3e08110367503ad7c714bcb84` |

Both experimental assets target Vulkan 1.0, pass `spirv-val`, and reproduce
byte-identical HLSL/SPIR-V plus identical generated header words. Disassembly
proves local size 256, bindings 0/1/2/3, `Exp` math instructions, intended entry
points, and no function-call or dynamic tag-dispatch overhead. Both remain
selector-ineligible outside the bounded M47 path.

Permanent non-hardware facts cover request/view/generation validation, awkward
padding, exact memory arithmetic, separate/fused/direct plans, Hidden precision,
in-place N rejection, barrier order/access, deterministic replay, stable SiLU,
F16 boundaries, stride/padding, nonfinite rejection, and mismatch localization.
Hardware facts cover the complete real M43–M47 composition, all projection and
gating routes, both residual plans, both submits, every fault, quarantine/reap,
independent weight replacement, stale rejection, extension-disabled
device-resident conventional fallback, warm reuse, and validation.

The operator is an experimental production candidate rather than promoted
production because shader authority is experimental and device evidence covers
one Vulkan device family. The complete block remains experimental because it
inherits the M43–M46 classifications. Neither is research-only: correctness,
bounded memory, lifecycle, fallback, replay, and useful primary performance are
demonstrated.

## Exact M48 model-golden-path workload

```text
input: retained FP32 BlockOutput[128,1024], explicit stride and M47 generation
weights per repeated block:
  existing fixed-eight-head M43 Q/K/V generations
  M44 Wo generation
  M46 RMSNorm scale generation
  M47 Wgate/Wup/Wdown generations, FfnWidth=4096
route:
  cooperative F16-RNE boundaries with FP32 accumulation/output
  direct-packed SiLU Hidden
  in-place Down residual
  same-queue bounded split after normalization
readback:
  none between blocks; one optional final output only
scope:
  repeat a small bounded block count with explicit per-block ownership;
  do not introduce a generic graph scheduler, model import, logits, or sampling
```

The committed evidence is
`DevelopmentReport/artifacts/M47/gated_ffn_complete_transformer_block_rtx3070.json`.
