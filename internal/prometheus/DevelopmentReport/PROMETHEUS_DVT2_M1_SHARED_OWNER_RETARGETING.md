# PROMETHEUS DVT-2 M1 — Shared Owner Retargeting

## Closeout

**Convergence outcome:** SUCCESS  
**Milestone state:** COMPLETE  
**DVT-2 state:** READY FOR M2

M1 retains one native execution owner for the complete nine-evaluation image
run. The bridge creates it once, resets only evaluation-lifetime state between
scheduler evaluations, and destroys it once during session close.

The generated-lock transition position is authoritative:

```
NoiseRefiner0 -> NoiseRefiner1 -> ContextRefiner0 -> ContextRefiner1
-> MainTransformer0..29 -> complete
```

ContextRefiner manifest-local tensors map to the existing semantic physical
weight slots `[2,3,4,5,6,7,8,9,10,11,12]`; no array-index family window was
retained. Production Context W3 uses the inactive, storage-capable audit-arena
view, while its immutable all-ones vector is a separate small host-visible
buffer. It is not aliased to timestep state. This preserves the one-window
device accounting ceiling.

PreparedImage, PreparedContext, JointWorking, activation, output, replay, and
generation evidence are invalidated on each reset. Qwen conditioning and the
34 validated host packages remain session-lifetime state. ContextRefiner0/1
runs for every scheduler evaluation.

## Canonical validation

Prompt `A lighthouse in fog at dawn`; seed 42; 512x512; 9 steps;
`res_multistep + simple`; AuraFlow shift 3.0:

| run | wall time | PNG SHA-256 |
| --- | ---: | --- |
| M0 repeat baseline | 209.8794111 s | `7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613` |
| M1 canonical | 205.3001912 s | `7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613` |
| M1 repeat | 203.6386687 s | `7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613` |

The M1 repeat is 6.2407424 s (2.97%) below the accepted M0 repeat while
performing ContextRefiner work in all nine evaluations. Each M1 run executed
270 MainTransformer layers, reported 34 host-cache hits per evaluation,
reported zero Context reuse, and held the model-owned device ceiling at
643,587,076 bytes.

## Reuse and lifecycle evidence

- owner constructions: 1 per session; owner destructions: 1 at close;
- evaluations 1--9: zero owner reconstruction, zero family arena allocation,
  zero pipeline/shader/descriptor-pool creation;
- logical binds: 34 targets per evaluation (first target is owner creation in
  evaluation 1; all remaining targets are retargets), 306 weight packages
  uploaded from the session cache across the full smoke;
- shared resources preserved: Vulkan runtime, command infrastructure,
  pipelines, descriptor pools, main-sized arenas, weight staging/window,
  audit/readback infrastructure, lock authority, and host package cache;
- retargeted/reset: active portfolio, semantic slot bindings, descriptor and
  binding generations, target identity, streams, output, audit, and replay.

Validation passed: full Windows native build; bridge Go build; focused
Marionette malformed-program/pipeline-fault test; payload authority check;
Python syntax checks; canonical and repeat PNG smokes; and `git diff --check`.

## M2 handoff

M2 remains double-buffered layer-weight upload/prefetch. M1 shows each
MainTransformer package is 361,820,672 bytes, current upload/compute overlap
is zero, and the current one-window ceiling is 643,587,076 bytes. M2 must add
a second staging path and a second bounded weight window only behind a
single-window fallback, with transfer/compute barriers and no change to this
lock-derived owner or stream contract. See `dvt2_m2_handoff.json`.
