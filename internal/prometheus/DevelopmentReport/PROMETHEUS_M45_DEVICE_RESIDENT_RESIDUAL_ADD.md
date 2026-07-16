# Prometheus M45 device-resident residual add and ownership proof

## Outcome

Convergence outcome: **SUCCESS**  
Milestone state: **COMPLETE**  
Residual operator classification: **2 — experimental residual candidate**  
Composed M43+M44+M45 classification: **experimental transformer fragment**

M45 implements one bounded device-resident residual transition:

```text
immutable resident X[Tokens,ModelWidth]
real slot-owned M44 Y[Tokens,ModelWidth]
  -> Z = X + Y
  -> one retained FP32 Z device view
  -> zero intermediate host copies
  -> zero or one final compact Z readback
```

Separate-output and exclusive in-place-Y strategies execute through the same
FP32 shader. In-place X is explicitly rejected after audit because resident X
is family-owned, reused by all eight M43 projection groups, and may outlive one
M44/M45 slot. No normalization, residual fusion, graph scheduler, arbitrary
broadcasting/rank, model importer, training path, Direct3D, PTX, CUDA interop,
or distributed execution was added. M42/M43/M44 APIs, shader IDs, selectors,
replay identities, and production classifications are unchanged.

## Tensor and storage contract

The logical operation is exact FP32 addition:

```text
Z[token,column] = X[token,column] + Y[token,column]
Tokens      in [1,1024]
ModelWidth  in [1,4096]
primary     = [128,1024]
```

All three views are row-major FP32. Logical rows and columns must match
exactly, but X and Y may have different physical row strides. A stride must be
at least ModelWidth. Validation uses the exact physical range
`Tokens * rowStride * 4`, checks offset addition and byte capacity without
overflow, and never infers compactness from shape.

| View | Producer/current access | Residual access | Owner | Mutability |
|---|---|---|---|---|
| X | prepared resident resource, compute-readable | shader read | M43 family, resident-X generation | immutable/shared |
| Y | M44 projection shader write | shader read, or shader read/write | composed physical slot and generation | exclusive only for in-place-Y |
| separate Z | none | shader write, then transfer-read or compute-read | composed physical slot, new content generation | exclusive |
| in-place Y-as-Z | M44 Y storage | shader read/write, then transfer-read or compute-read | same physical slot, new logical content generation | exclusive |

The primary packed-interleave M44 Y is physically
`[paddedTokens,paddedModelWidth]` with stride 1024. Direct/A2x4 output may be
compact. M45 carries the stride in every descriptor/push/trace contract.
Padding is not read as logical data and is left unchanged deterministically.

## Bounded request and eligibility

`prom_m45_plan_request` carries X/Y views, Tokens, ModelWidth, strategy,
submit policy, expected X/Y content generations, FP32 precision, Y exclusivity,
pre-residual Y consumer count, M44 replay identity, and final-readback policy.
`prom_m45_composed_request` joins that contract to the real resident M43/M44
owner without exposing raw Vulkan handles outside the internal Vulkan header.

Eligibility requires:

- two valid FP32 row-major views on the same Vulkan device;
- exact logical shape and sufficient independent physical strides/ranges;
- nonzero, matching X/Y logical generations and nonzero physical slot
  generation;
- disjoint X/Y used ranges;
- separate output, or exact in-place Y with mechanical exclusivity and zero
  remaining pre-residual consumers;
- checked sizing within the 1 GiB request cap;
- one supported FP32 precision policy.

Exact reasons distinguish invalid view, shape, stride, generation, device,
alias, exclusivity, precision, capacity, strategy, and in-place-X rejection.
Invalid and stale requests reject before command submission. The bounded
device fallback is separate output; host execution remains audit-only.

## Strategies and winner

### Separate output

One grow-only slot buffer receives compact Z. X, Y, and Z are disjoint. One
dispatch covers only `Tokens * ModelWidth` logical elements. The plan records
exact X/Y read ranges, Z write range, timestamps, optional per-row compact
readback, and one reusable descriptor set. This is the correctness baseline.

### In-place Y

M44 leaves Y exclusively owned by the same composed slot, with no readback and
no other successor. M45 binds Y as both read source and write destination;
each invocation owns one element. The projection-write to residual-read/write
barrier is exact. Physical buffer and slot generation stay unchanged, while
the view is renamed Z and receives a derived post-residual content generation.
No second Z allocation exists.

### In-place X audit

Rejected. X is one immutable family resource shared by 24 Q/K/V projections,
may be reused by later attention operations, and replacement already waits all
referencing slots. Mutating it would trade a bounded slot allocation for a
broader and less coherent family lifecycle. The plan and runtime return the
exact `IN_PLACE_X`/`PROM_M45_DETAIL_IN_PLACE_X_REJECTED` decision.

The chosen product strategy is **in-place Y**. It is consistently a little
cheaper at the residual kernel itself and saves one exact logical Z tensor.
Separate output remains the on-device fallback and audit baseline.

## Generation, aliasing, and replay

X uses the prepared resident-X content generation. Pre-residual Y derives a
nonzero logical generation from the readback-stripped internal M44 replay,
logical request, and physical slot generation. Z derives a new nonzero content
generation from X, Y, strategy, alias plan, shader hash, M44 identity, and
command plan. Thus an unchanged VkBuffer and slot generation never imply
unchanged contents.

Legal cases are disjoint X/Y plus either disjoint Z or exact exclusive Y/Z.
Partial X/Y overlap, identical X/Y range, cross-device views, stale content,
insufficient stride/range, another Y consumer, and unrelated output overlap
are rejected. M45 never double-returns an aliased buffer: Y and Z are one slot
resource with two sequential logical roles.

Primary readback-stripped M44 replay is `587872734306529766`. Representative
M45 identities from the committed RTX artifact are:

| Strategy/submit | M45 replay | Z generation |
|---|---:|---:|
| separate / one | 16405257448183323603 | 597343316311313610 |
| separate / two | 15274375858381093162 | 17015278552615267547 |
| in-place Y / one | 3713860574087423151 | 8213310196714442444 |
| in-place Y / two | 14635248599580941186 | 15406587092518790256 |

Repeated plan construction is deterministic. Final Z identity is distinct
across physical slot/request generations by design while the structural plan
replay is deterministic for identical plan inputs.

## Command and submit plans

One-submit records M43, M44 aggregation/projection, M45 barriers/dispatch, and
optional Z readback in one command buffer and submits one fence. Two-submit
records M43+M44 together, signals the existing bounded semaphore, then waits
on the same compute queue before the M45 command buffer. There is no second
queue, ownership transfer, host wait between submits, or generic operation
graph.

The normalized residual trace is:

| Seq | Range | Dependency |
|---:|---|---|
| 0 | exact X physical range | compute read -> residual shader read |
| 1 | exact Y physical range | projection shader write -> shader read (separate) or read/write (in-place Y) |
| 2 | exact Z physical range | residual shader write -> transfer read or next compute read |
| 3 | compact readback, when requested | transfer write -> host read |

All barriers use `VK_QUEUE_FAMILY_IGNORED`. There is no whole-device or broad
head-buffer barrier. X's read/read dependency orders its existing composed
consumers without inventing a write. In-place Y specifically uses
`SHADER_WRITE -> SHADER_READ|SHADER_WRITE`, which permanent tests distinguish
from separate output's `SHADER_WRITE -> SHADER_READ`.

## Lifecycle and fault evidence

M45 extends the existing two-slot M43/M44 owner. M43 heads, resident X, M44 Y,
and optional separate Z stay alive through residual completion. A dedicated
preallocated descriptor set prevents M45 updates from changing descriptors
already recorded for M44. M45 state is destroyed before M44/shared Vulkan
owners.

Known-completion faults before barriers, after the X barrier, after the Y
barrier, during dispatch, after submission, and before readback submit the
bounded prefix, wait the fence, report logical failure, and return the slot
once. Uncertain completion quarantines the whole composed slot. Resident-X
replacement waits, observes the fence, reaps it, then publishes the new
generation. Hardware facts also prove stale-X rejection, recovery, immutable
host source values, no early M44 Y recycling, and zero allocation after both
physical slots reach warm capacity.

## Shader and provenance

`experimental/transformer/residual_add.sdslv` is normal-looking FP32 code with
three buffers and 32 bytes of push constants. It supports independent X/Y/Z
strides and exact Y/Z aliasing without broadcasting or generic tensor logic.

| Artifact | SHA-256 |
|---|---|
| SDSL-V source | `e83b60a31f82d9af785710d54a2055814917e1d0e9ecb988ab8eaf489313777d` |
| generated HLSL | `7fedb0c77e12e2b975ae6529e1c50d5e98490a0dcc4f31ff1bfab8808e5b18e6` |
| SPIR-V | `c6e177b9fb86f1e5b01e05c544091577629fb4f51d4e08b9c6b624d85b6d0acc` |

DXC 1.9.0.5347 (`fe261573`) targets Vulkan 1.0. `spirv-val` accepts the
module. Disassembly proves `ResidualAdd_CS`, local size 256x1x1, bindings
0/1/2, push constants, one FP32 `OpFAdd`, and no unrelated interface. The
asset has experimental authority and remains selector-ineligible.

## Memory

Primary exact residual-visible bytes are:

| Resource | separate output | in-place Y |
|---|---:|---:|
| X view | 524,288 | 524,288 |
| Y / aliased Y-Z view | 524,288 | 524,288 |
| separate Z | 524,288 | 0 |
| final readback | 524,288 | 524,288 |
| residual-visible total | 2,097,152 | 1,572,864 |
| complete exact M43+M44+M45 | 50,593,804 | 50,069,516 |

In-place Y saves exactly 524,288 bytes primary, 8,192 tiny, 1,048,576 for
more-token/wider, 508,508 awkward, and 4,194,304 boundary. The benchmark primes
both strategies in one owner, so warmed retained capacity is intentionally the
same 49,545,228 bytes primary; a product owner that selects only in-place Y
does not create the separate-Z buffer. Audit X/Y readbacks are excluded from
product-path memory. All product buffers are grow-only and device-local; no
system-memory spill or convenience copy of Y exists.

## RTX 3070 performance

The committed artifact contains 36 validation-clean records: four device
plans plus M43+M44 and CPU-host-bounce baselines for six workloads. Each device
plan primes both slots, performs 32 warm operations, and records a five-run
median. Primary in-place-one 10/100-operation medians were 3.168/2.515 ms GPU
and 6.373/5.939 ms end-to-end; the spread confirms M44's earlier observation
that GPU state can affect submit-topology samples.

| Workload | best stable residual | residual GPU | complete GPU | end-to-end | saved bytes |
|---|---|---:|---:|---:|---:|
| tiny | in-place Y / two | 3.200 us | 0.417 ms | 1.384 ms | 8,192 |
| primary | in-place Y / two | 5.376 us | 2.514 ms | 5.563 ms | 524,288 |
| more tokens | in-place Y / two | 6.624 us | 2.701 ms | 7.719 ms | 1,048,576 |
| wider | in-place Y / two | 7.360 us | 4.936 ms | 9.562 ms | 1,048,576 |
| awkward | in-place Y / one | 5.216 us | 2.479 ms | 5.043 ms | 508,508 |
| boundary | in-place Y / two | 29.888 us | 4.573 ms | 22.563 ms | 4,194,304 |

Primary separate/two measured 5.664 us residual, 2.519 ms complete GPU, and
5.390 ms end-to-end. In-place/two was 5.376 us, 2.514 ms, and 5.563 ms.
In-place wins kernel time and memory, while the small end-to-end difference is
noise/recording/readback dominated. One versus two is not universal: tiny and
awkward favor one end-to-end; primary/wider/boundary favor two or are close.
Both bounded plans remain selectable, with two-submit the primary central-GPU
winner in the final corpus.

The primary M43+M44 baseline is 2.499 ms GPU and 5.203 ms end-to-end. Residual
adds about 5.4 us of kernel work, 0.21% of the complete GPU interval. It is
effectively free computationally but not ownership-free.

The CPU host-bounce audit reads real M44 Y and real resident X into two host
buffers, then adds on CPU; it deliberately omits optional reupload. Primary
cost is 7.035 ms end-to-end: 1.813 ms X readback plus 34.8 us CPU add in
addition to the M44 Y path. Boundary is 40.975 ms, including 16.577 ms X
readback and 0.827 ms CPU add. Even this favorable no-reupload baseline is
27% slower than primary in-place/two and 82% slower at the boundary. Reupload
would only increase the gap.

## Performance questions

1. Residual addition is effectively free relative to M43+M44: 5.4 us and
   about 0.21% primary.
2. In-place Y saves one complete Z tensor: 0.5 MiB primary and 4 MiB boundary.
3. In-place Y is slightly faster at the residual dispatch; aggregate/E2E
   differences are dominated by GPU state, recording, and readback.
4. Lifecycle complexity is material but bounded: exclusivity, a content
   generation, one alias plan, and no double return are required.
5. Two-submit wins primary central GPU; submit winner is not universal.
6. Different/padded strides are correct. Awkward residual remains about 5 us.
7. Real X/Y host bounce is predictably worse, even without reupload.
8. In-place X is not justified under the reusable resident-X owner.
9. Tiny retains in-place Y but favors one-submit E2E; boundary retains
   in-place Y for memory while two-submit is the stable timing choice.
10. Ownership is stable enough to hand Z to a normalization consumer, but the
    operator remains experimental pending another Vulkan device family.

## Correctness and validation

The CPU oracle applies one FP32 add at each logical coordinate with independent
strides and rejects nonfinite X, Y, or Z. The first mismatch records strategy,
token, column, expected/actual, absolute/relative error, X/Y/Z generations,
and M44/M45 replay identities.

Permanent non-hardware facts cover shape/stride, partial aliasing, cross-device
views, stale generations, exclusivity, in-place-X rejection, separate and
in-place plans, access masks and exact ranges, optional final readback,
overflow envelope, generation transition, deterministic replay, CPU oracle,
padding preservation, and mismatch localization. Gated hardware facts cover
real M43/M44 composition, both ownership strategies, both submit plans,
retained-only Z, every fault point, quarantine/reap, resident-X replacement,
warm allocation reuse, and validation cleanliness.

RTX results are zero validation warnings/errors, zero device loss, no stale
use, no alias corruption, and all 36 benchmark outputs correct. The committed
artifact is `DevelopmentReport/artifacts/M45/device_resident_residual_rtx3070.json`.

Validation results:

- focused M42/M43/M44/M45 facts: 19/19 passed on the RTX 3070, including four
  M45 facts with no hardware skip;
- complete normal Marionette lane: 390 tests, 358 passed, 32 unrelated
  hardware-gated legacy skips, zero failed;
- M45 six-workload benchmark: 36/36 records correct, zero validation warnings
  or errors, exact strategy/submit corpus and artifact consistency passed;
- authoritative MSVC Windows native rebuild passed;
- `go test` passed for `internal/source`, `internal/diagnostic`, all
  `internal/sdslv/...`, both octxiliary trees, `internal/cli`, `cmd/oct`, the
  combined `internal/... ./cmd/oct`, and `tools/build_sidecars`;
- native manifest parity and the SDSL-V workspace/artifact checker passed;
- deterministic source-to-HLSL/SPIR-V/header regeneration reproduced the
  recorded hashes; `spirv-val --target-env vulkan1.0` and disassembly interface
  assertions passed;
- `bash -n`, `git diff --check`, Linux GCC shared-library/test builds, ELF
  verification, and the Linux Marionette smoke passed.

The Linux build repeats the existing `sdslv_test_host.c:312` warning about a
pointer comparison with a zero character constant. It is outside M45 and did
not affect the successful build/smoke; it was not silently changed here.

## Classification and rollback

The bounded operator is an **experimental residual candidate**, not
research-only. Correctness, ownership, replay, lifecycle, and performance are
complete on the RTX 3070. Promotion is withheld because shader authority is
still experimental, validation evidence covers one Vulkan device family, and
one/two-submit performance is not stable across the envelope. The composed
attention fragment inherits M43/M44's experimental classification.

Rollback regions are localized to the M45 structs/functions/state in
`reactor_vulkan.h` and `reactor_vulkan_fused_reduction.c`, the residual shader
asset and manifest entry, M45 facts/benchmark, artifact checker, this report,
the M45 artifact, and the final M44 consumer-status paragraph. Removing those
regions restores M44 without changing its public behavior or identities.

## Exact next normalization-facing workload

The next milestone may consume exactly one retained view:

```text
Z: FP32 row-major [128,1024]
physical row stride: explicit (1024 on the primary path)
device: same Vulkan device
content generation: nonzero M45 Z generation
producer access: residual compute write
required consumer access: compute read
owner: same bounded composed slot until the normalization fence completes
readback: none before normalization
```

That milestone must choose and measure its own normalization contract. M45
does not preselect RMSNorm versus LayerNorm, add scale/bias state, fuse the
residual, or introduce a transformer graph.
