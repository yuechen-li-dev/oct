# Prometheus M48 multi-block golden path and EVT closeout

## Outcome

Convergence outcome: **MEANINGFUL PROGRESSION**  
Milestone state: **IN PROGRESS**  
EVT state: **IN PROGRESS**

M48 has crossed the hardware-corpus boundary. Four real M43-M47 blocks now run
inside one fixed stack owner with four ordered parameter bundles, direct
device-resident activation handoff, one reused block working set, bounded
per-layer descriptor/query banks, and either one queue submit or four
same-queue semaphore-linked submits. The permanent validation-enabled tiny
hardware fact compares the final output with the four-block CPU oracle, proves
resident and host-fed A0, executes one/two-layer audits, recovers known and
uncertain faults, replaces all four requested resource granularities, and
observes zero warm Vulkan buffer allocation. The Windows RTX 3070 has now
executed the primary, more-token, smaller-expansion, awkward-width, and
token-boundary rows through this final M48 authority, including primary
cooperative, conventional-FP16, A2x4-FP32, and extension-disabled fallback
routes.

This report preserves the earlier planning-boundary history: M48 first landed
with truthful null hardware timing because M43 still owned command begin/reset
and singular parameters. The current continuation removed that blocker. EVT is
not yet marked complete because standalone M47 still has its legacy composed
terminal recorder and the explicit stack-level host-wait/no-readback and
host-bounce audit baselines have not yet been made into runtime paths. Linux
live Vulkan is not an EVT requirement here: no native Linux Vulkan service is
available, while the Windows RTX 3070 witness is the selected EVT platform.

## System-level transformer runtime integration

### Old terminal assumptions

Before this pass, grouped attention selected one family-wide Wq/Wk/Wv matrix,
resident input meant only family-prepared X, and
`prom_m43_record_grouped_internal` reset and began its own command buffer.
M44-M47 could continue an internal composed path, but the terminal M47 owner
submitted and waited. Four calls would therefore have required resource
replacement, host fence boundaries, and host-mediated activation authority.

### New ownership hierarchy

The internal vocabulary is now centralized in
`reactor_vulkan_transformer_internal.h`:

```text
transformer runtime family
  shared pipelines, layouts, query pool, descriptor pools
  four prom_transformer_layer_resources bundles
  reusable physical slots

layer resources
  24 attention parameter resources
  Wo
  RMSNorm weight
  Wgate / Wup / Wdown
  shape, generation, and content hash per resource

physical stack slot
  optional host A0 upload + device A0
  activation ping A + activation ping B
  one M43-M47 working set
  four descriptor banks
  four bounded command buffers + three semaphores + one fence
  compact final readback
  logical request identity, slot generation, quarantine state
```

Each `prom_transformer_parameter_resource` owns its upload, FP32, and packed
device forms. Replacing one resource waits/reaps the bounded stack ring before
publishing the new generation and does not mutate another bundle. Destruction
walks each ordered bundle explicitly.

### Caller-owned recorder contract

`prom_transformer_record_context` supplies an already-open command buffer, an
exact query base/count, the layer index, and that layer's descriptor bank.
`prom_transformer_prepare_block` selects the explicit layer resources and
builds the existing M43-M47 plans against a caller-provided resident input and
caller-selected output activation. `prom_transformer_record_block` then
appends:

```text
M43 grouped attention
M44 aggregation and Wo projection
M45 first residual
M46 RMSNorm
M47 gated FFN and second residual
```

It does not reset, begin, end, submit, wait, or read back. M43's internal
recorder now has an append mode that consumes the caller command buffer and
explicit descriptor handles while the existing default wrapper retains its
standalone ownership behavior. M44-M47 append their existing tails; numerical
operators and shader artifacts were not rewritten.

Standalone M42-M47 APIs still compile and retain their previous public/internal
behavior. The remaining organizational debt is concrete: standalone M47 still
enters `prom_m45_execute_composed_core` through its M46 continuation rather
than entering `prom_transformer_prepare_block` / `prom_transformer_record_block`.
That is a parallel terminal block recorder, not a semantic distinction, and is
kept explicitly open rather than hidden behind a compatibility claim.

### Resident activation contract

Stack inputs are explicit FP32 row-major device resources with exact logical
Tokens/ModelWidth, supported stride/range, nonzero semantic generation,
same-device ownership, compute-write to compute-read provenance, and current
slot lifetime. Layer zero accepts either the immutable prepared resident A0 or
a host-fed A0 copied once before the first block. Later layers consume the
prior ping output directly. Reduced paths reuse the existing FP32-to-packed
input machinery; there is no host authority or packed-X family identity for an
internally produced activation.

The public fixed-stack execution request remains bounded to four product
layers, with one/two/four available only in explicit audit mode. No node list,
operator registry, DAG, scheduler, or general graph API was introduced.

### Descriptors and timestamps

Each physical slot owns four complete descriptor banks (134 sets per layer,
536 total). Pipelines/layouts remain shared. A descriptor bank is written before
recording and never mutated while a submit may consume it, which is correct for
both one-submit and four-submit modes.

Each layer owns a deterministic 236-query region. The query pool stride was
expanded to 1024 per physical slot. Collection reads only the spans actually
written by M43, M44, M45, M46, M47, and optional final readback; intentionally
unused gaps remain unavailable rather than being misreported as timestamps.

### Submit, fence, and fault ownership

The one-submit plan resets/begins one command buffer, appends all layers, adds
the optional final readback, ends once, submits once, and waits one final fence.
The four-submit plan records one block per command buffer and submits all four
in one queue call with three compute-stage semaphore dependencies; there is no
inter-layer host wait. Both plans have one final fence.

Known record-time failures recycle the slot without quarantine. An uncertain
post-submit fault quarantines the complete slot and retains its resources until
the fence reaper observes completion. The permanent hardware fact proves a
successful complete stack after both recovery paths.

## Fixed topology and identities

Each layer owns exactly 29 resources:

```text
0..23  Wq/Wk/Wv for heads 0..7
24     Wo
25     RMSNorm Weight
26     Wgate
27     Wup
28     Wdown
```

The complete product has 116 independently identified persistent resources.
Intrinsic layer replay identity uses the selected layer's shape, strategies,
generations, and hashes. Activation generation chains from A0 through all
ordered layer replay identities and is independent of physical slot reuse.
Live replacement is proven for layer 2/head 5/Wk, layer 1/Wo, layer 3/Wdown,
and layer 0/RMSNorm. Each changes aggregate replay identity and the subsequent
execution consumes the requested generation coherently.

## Activation and working-set reuse

A0 is immutable and separate from the two output roles:

```text
A0 -> layer 0 -> ping B
ping B -> layer 1 -> ping A
ping A -> layer 2 -> ping B
ping B -> layer 3 -> ping A (final)
```

The output role becomes the next immutable input only after the complete block
has emitted its compute-write to compute-read dependency. Q/K/V, scores,
probabilities, packed layouts, heads, M44 output, residual/RMSNorm state, and
Gate/Up/Hidden/Down are one serial working set. Persistent weights and the two
live activation roles are outside that working set. The warm hardware repeat
reported `buffer_allocation_count=0`.

## Exact primary memory

The earlier plan incorrectly counted only one output ping buffer. The live
owner requires immutable A0 plus ping A and ping B; the corrected primary
resident-A0 capacity is:

| Resource | Bytes |
|---|---:|
| four persistent layer bundles | 671,121,408 |
| immutable resident A0 | 524,288 |
| two padded activation pings | 1,048,576 |
| one reusable block working set | 10,486,272 |
| compact final readback | 524,288 |
| one complete quarantinable slot reserve | 12,059,136 |
| **exact retained capacity** | **695,763,968** |

Per-layer output retention would hold four padded outputs; two ping roles save
1,048,576 bytes. The fixed descriptor and timestamp capacities are 536 sets and
944 queries. Their Vulkan objects are counted exactly without inventing opaque
driver allocation byte sizes. The default 2 GiB capacity gate remains checked
before submission.

## Live correctness and lifecycle evidence

The permanent Windows hardware fact uses the coherent tiny shape
`[Layers=4, Tokens=16, ModelWidth=128, Heads=8, HeadDim=16, FfnWidth=256]`
with four distinct generated parameter bundles and conventional FP16
projections. It proves:

- final one-submit output agrees with the four-block reduced-precision CPU
  oracle;
- one-submit and four-submit outputs agree;
- all four layers complete without intermediate readback or host wait;
- host A0 and prepared resident A0 both execute;
- one- and two-layer live audits use the same recorder;
- validation error count does not increase;
- all ten defined known fault locations recycle and uncertain-completion
  quarantine/reap recovers;
- all four required granular replacements execute coherently;
- a warm repeated stack performs zero Vulkan buffer allocation.

One captured validation-enabled tiny run on the RTX 3070 measured:

| Tiny-shape metric | ns |
|---|---:|
| one-layer GPU | 466,944 |
| two-layer GPU | 914,432 |
| four-layer GPU, one submit | 2,113,536 |
| four-layer GPU, four submits | 1,989,344 |
| host-fed four-layer end-to-end | 14,182,600 |
| resident four-layer end-to-end | 5,736,100 |
| warm 10 median GPU | 1,849,856 |
| warm 100 median GPU | 1,845,184 |

The complete workload timing corpus, captured serially through the same
authority, is in the machine-readable artifact. Its primary conventional row measured one/two/four
GPU intervals of 8,477,696 / 16,978,432 / 38,849,760 ns for one-submit and
38,312,480 ns for the semaphore chain. The primary 100-sample median was
32,610,272 ns (p10 32,583,872; p90 32,981,408), proving warm allocation count
zero after both slots were primed. The primary cooperative route was 15,270,784
ns one-submit and the A2x4 FP32 route was 53,285,152 ns; these are matched
precision-route comparisons, not numerical-equivalence claims. The selected
topology is shape-dependent: the four-submit chain won the captured primary,
more-token, and token-boundary samples, while one-submit won smaller-expansion
and awkward samples. No universal winner is claimed.

The two audit-only synchronization baselines now exist in the same executor.
At the captured primary shape, host-wait retained device activations and
measured 34,119,040 ns GPU, 41,118,600 ns E2E, and 34,595,400 ns of accumulated
CPU fence wait across four submits. Host-bounce measured 34,115,104 ns GPU,
51,927,400 ns E2E, 34,890,900 ns CPU wait, and 9,825,600 ns host copy time;
it performs three intermediate readbacks and three reuploads. These are audit
baselines, not product plans. The arithmetic intervals remain comparable; the
extra E2E cost makes the value of resident activation handoff explicit.

## Source organization

- `reactor_vulkan_transformer_internal.h`: enduring private ownership types for
  parameter resources, layer bundles, descriptor banks, and caller recording.
- `reactor_vulkan_transformer.c`: bounded M42-M48 planning, identities, memory
  accounting, comparison, and CPU reference/oracle code.
- `reactor_vulkan_fused_reduction.c`: existing shared Vulkan operator runtime;
  now also owns resource preparation, reusable block recording, and fixed-stack
  lifecycle because those functions depend on its private pipeline/slot helpers.
- `reactor_attention_tests.cpp`: planning/reference facts plus validation-enabled
  live stack lifecycle proof.

The new header makes the ownership seam visible, but the Vulkan implementation
file remains too broad. Moving the block and stack implementations into
`reactor_vulkan_transformer_block.c` and `reactor_vulkan_transformer_stack.c`
requires first extracting a cohesive private runtime interface; doing a textual
`.inc` split would only disguise coupling and was rejected.

## Validation status and remaining evidence

Passed in this continuation:

- GCC C11 syntax with warnings limited to two pre-existing unused timing locals;
- authoritative Windows MSVC native library and Marionette builds;
- targeted Go Prometheus tests;
- all six M48 Marionette facts, including live hardware;
- validation-enabled one/four-submit, resident/host A0, all known logical fault
  points, uncertain quarantine/reap, replacement, deterministic extension
  fallback, and zero-warm-allocation facts;
- full RTX 3070 primary/corner corpus with 10/100 warm distributions and all
  three requested primary precision routes;
- MSVC native rebuild after the permanent fault/fallback assertions;
- `git diff --check`.

Still required before EVT completion:

- migrate standalone M47 to a thin compatibility wrapper over the common
  record-block authority, preserving its bounded two-submit compatibility
  policy without retaining a second block recorder;
- prove replacement while an actual stack is in flight (the current replacement
  proof is coherent and serial after bounded lifecycle reaping);
- record primary precision-oracle audit rows and per-layer warm distribution /
  drift rows in the artifact (current bounded primary correctness proof is
  finite-output only, while tiny retains the complete CPU oracle);
- rerun the full requested Go/workspace matrix after those code changes.

### Primary precision-audit contradiction

The new deterministic primary conventional-FP16 audit executes the existing
four-block CPU reference once, outside product timing, then compares twelve
fixed final-output coordinates. It does **not** pass. The failure is therefore
not hidden by the previous finite-output-only primary check. One representative
coordinate is layer 3/final, token 31, column 511: CPU reference `-106.834`,
live output `-113.538`, absolute error `6.70318`, relative error `0.0627436`.
The captured input generation is `480000`, layer replay identity is
`18276622376753282516`, and aggregate stack replay identity is
`9856943563942416748`. Multiple other fixed coordinates diverge.

This is a genuine numerical-authority contradiction between the existing
primary CPU reference and the live conventional-FP16 execution, not a timing
or audit-readback issue. EVT remains open. The machine-readable artifact keeps
the failing evidence and exact diagnostic rather than promoting finite output
to correctness.

Accordingly the standalone block is an **experimental complete one-block
transformer fragment with a legacy compatibility owner**, the fixed stack is an
**experimental live fixed-stack operator**, the four-layer golden path is
**hardware-corpus proven but not compatibility/baseline complete**, and EVT
remains **IN PROGRESS**.

## Exact DVT handoff after EVT

The first DVT workload remains four layers at `[Tokens=128, ModelWidth=1024,
Heads=8, HeadDim=128, FfnWidth=4096]`, or the largest coherent common shape if
the AMD device rejects it. Preserve the RTX artifact, query AMD subgroup/wave,
cooperative tuples, memory types, limits, and timestamps, then compare FP32,
FP16, cooperative, and bounded fallback behavior. Under fixed eight-head
semantics awkward widths must remain divisible by eight; use 1000/125 or
1016/127 rather than silently accepting 1001 or 1023.
