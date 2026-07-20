# Prometheus DVT-2 M5b: barrier-conscious attention topology

Date: 2026-07-20

## Convergence outcome

- Convergence outcome: **HONEST STOP**
- Milestone state: **IN PROGRESS**
- DVT-2 state: **READY WITH REQUIRED ADAPTATIONS**

M5b reached a genuine production compiler-target limitation before a subgroup
candidate could be created. The accepted M4 serial attention route remains
unchanged and is the only retained route.

## M5a regression explanation

M5a used one 256-thread workgroup per row, 8 fixed max-reduction rounds, 13
full-workgroup barriers, and 9,476 bytes of shared storage for a 1056-key joint
row. M4 uses two barriers and 8,448 bytes. M5a retained the serial canonical
denominator and one-row workgroup lifetime, so its synchronization cost was
paid for every query/head row. The exact canonical Prefetch result regressed
from 165.439 s to 175.232 s (+5.92%), despite passing numerical authority and
the accepted PNG hash. This is sufficient quantitative evidence to reject that
topology.

## Frozen subgroup capability and compiler boundary

The RTX 3070 is frozen at subgroup size 32 by the native runtime and the M40A
capability probe. A minimal attention-scoped HLSL probe with `WaveActiveMax`
and `WaveReadLaneFirst` compiled at Vulkan 1.1 and disassembled to
`OpGroupNonUniformFMax` and `OpGroupNonUniformBroadcastFirst`.

The production SDSL-V toolchain unconditionally uses `-fspv-target-env=vulkan1.0`
for non-cooperative compute shaders. Under that target DXC rejects the same
probe: “Vulkan 1.1 is required for Wave Operation.” No SDSL-V subgroup primitive
or target-contract declaration exists. Therefore the requested one-subgroup row,
multi-row 256-thread workgroup, and subgroup-sized workgroup cannot be emitted,
registered, or validated through the real production route.

Hand-authoring a Vulkan 1.1 SPIR-V header would bypass the source-to-generated
production contract. Raising the universal target or adding a new target and
runtime admission/fallback policy is a compiler/runtime compatibility milestone,
not a narrow attention-topology edit. It is deliberately not folded into M5b.

## Candidate disposition

| Candidate | Result |
| --- | --- |
| M4 serial | retained authority |
| M5a 256-lane tree | rejected; full image -5.92% |
| 32-lane subgroup per row | compiler-target blocked |
| eight 32-lane rows in a 256-thread workgroup | compiler-target and subgroup-memory-semantics blocked |
| 32-thread workgroup per row | compiler-target blocked |

The proposed multi-row mapping is recorded in the artifacts, including
`subgroup_id -> GroupID.x * 8 + subgroup_id`, lane-to-key strides, and
four PV channels per lane. It remains design-only.

## Validation and next decision

The production M4 shaders still pass SDSL-V checks, and the native reactor is
rebuilt from restored M4 sources. M5a's real retained-stream replay passed
relative L2 `8.38066e-7`; final 30-layer authority was `1.02005e-5` against
`5e-5`; its canonical PNG hash was the accepted
`7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613`.
The restored M4 real replay was repeated in M5b: GPU median `438.181 ms` over
ten warm samples, with the same representative and 30-layer authority values.
No subgroup candidate numerical or full-image claim is made because it never
reached a real production pipeline.

Exact next decision: **retain the accepted serial attention route and move to
another kernel**. A future, separately scoped compiler/runtime milestone may
add explicit Vulkan 1.1 subgroup target admission; only then should M5c repeat
the topology experiment from M4.

All deterministic evidence is in `artifacts/Dvt2M5b/`.
