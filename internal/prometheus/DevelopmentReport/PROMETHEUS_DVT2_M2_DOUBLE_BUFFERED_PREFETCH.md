# Prometheus DVT-2 M2: Double-buffered layer-weight prefetch

## Outcome

M2 retains the M1 one-window execution as the closed `MinimumMemory` profile
and adds the closed `Prefetch` profile.  `Prefetch` owns exactly two bounded
weight windows.  While compute reads the active window, a scoped prefetch
goroutine fills the inactive mapped staging slot and submits the lock-derived
successor to the RTX 3070 transfer-only queue.  It is not a task graph and it
does not permit a third window or arbitrary depth.

The generated evidence bundle is under `artifacts/Dvt2M2/`.  Its canonical
smokes freeze exact timings, PNG hashes, allocation ceilings, per-evaluation
evidence, interval overlap, and the M3 handoff.

The post-change MinimumMemory canonical smoke completed in **203.468 s** at
the accepted 643,587,076-byte ceiling.  Prefetch completed in **196.288 s**
and repeated in **195.452 s** at 1,005,407,748 bytes.  The repeat improves on
the accepted M1 repeat (203.639 s) by 8.187 s (4.0%).  This does not meet the
aspirational 15% target, so the report treats compute/Python work as the next
bottleneck rather than overstating the result.

## Profiles

| Profile | Device windows | Model-owned ceiling | Host staging | Execution |
| --- | ---: | ---: | --- | --- |
| `MinimumMemory` | 1 | 643,587,076 bytes | one active slot | serialized upload then compute |
| `Prefetch` | 2 | 1,005,407,748 bytes | two 88,473,600-byte mapped slots | current compute overlaps the one next legal upload |

Each window is 361,820,672 bytes, the largest lock-authorized package.  M2
does not duplicate activations, attention/FFN scratch, resident stream slots,
audit/readback, or pipelines.  The existing 12,286,112,768-byte immutable host
package cache remains the only full package cache and remains reread-free during
evaluations.  Driver bookkeeping is not part of model-owned accounting.

## Queue, visibility, and ownership

The validated RTX 3070 has a universal compute family and a distinct
transfer-only family.  The two windows are created with concurrent sharing
across exactly those families, so role swaps do not need queue-family ownership
transfers.  A transfer fence proves completion.  Before descriptors are changed,
the compute queue records a transfer-write to shader-read acquire barrier;
only then does the code commit descriptors, increment binding/descriptor
generations, and swap the active/prefetch roles.

This matters: host-side asynchronous submission alone is not treated as
overlap or visibility.  The interval trace intersects the scoped host prefetch
call with the current native compute call, and the Vulkan contract establishes
the corresponding device write/read ordering.

The first Prefetch canonical trace recorded 8.430 s of bounded prefetch work,
8.375 s of intersecting compute/prefetch intervals (99.35%), and 0.052 s of
exposed prefetch wait across 279 transitions.  The repeat recorded 8.488 s,
8.434 s, and 0.051 s respectively.  Those timings are host-monotonic trace
evidence; driver GPU timestamp telemetry remains unavailable in this runtime.

## Closed state and schedule

The inactive window progresses through `Empty -> HostPrepared ->
UploadSubmitted -> DeviceReady -> Active -> Empty`; uncertain submitted work
is quarantined.  Activation rejects a wrong lock successor, stale target
position, incomplete upload, mixed payload identity, or an active-window
target.  Evaluation reset reaps completion and invalidates old generations.

The generated schedule contains every transition from `NoiseRefiner0` through
`MainTransformer29`, including the three cross-family edges.  ContextRefiner
continues to use M1 semantic-slot mapping into the shared physical layout; no
family-index-shaped duplicate activation layout was introduced.

## Validation and next target

Both canonical profiles run the nine-evaluation lighthouse configuration and
must produce PNG SHA-256
`7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613`.
The deterministic artifacts record the concrete measurements, all 30 main
layers per evaluation, zero immutable package rereads, fallback behavior, and
the remaining exposed prefetch wait.

M3 is deliberately not implemented here.  The evidence selects typed transport
and residency generations as the next bounded target: after a second workbench
is available, remaining coordination/bridge cost is more actionable than
adding depth or changing the model/Python boundary.
