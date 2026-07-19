# EVT-2 M1F Z-Image compiled-block facade and warm timing freeze

## Outcome

**SUCCESS / COMPLETE / READY FOR M2.** M1F adds the narrow callable,
model-specific native facade over the already accepted M1B -> M1C -> M1D
resident sequence. It adds no model arithmetic, shader, resource-plan, or
precision-policy change.

`prometheus_reactor_runtime_noise_refiner0_execute` accepts only the validated
BF16 `[1,1024,3840]` input, BF16 `[1,256]` timestep, and immutable identities.
It rejects audit-enabled execution, drives the fixed resident sequence, and
returns evidence plus a resident FP32 `ModelEmbedding` output state. M1C/M1D
audit readbacks remain explicit replay operations; the facade omits them so no
warm execution performs a host intermediate bounce.

## Real RTX evidence

The rebuilt, validation-enabled RTX 3070 witness passed every M1B/M1C/M1D O19
audit and then passed ten M1F complete resident runs. The final audited output
remains FP32 `[1,1024,3840]`, relative L2 `1.30438e-6` against the unchanged
`5e-5` limit, with peak device memory `654,891,776` bytes.

| Metric | Result |
| --- | ---: |
| Median complete warm time | 385,966,600 ns |
| Mean | 451,764,580 ns |
| Minimum | 380,028,000 ns |
| P95 | 734,894,700 ns |
| Standard deviation | 112,806,000 ns |
| Warm allocations/uploads/pipelines/descriptors | 0 / 0 / 0 / 0 |
| Host intermediate bounce | 0 |

The first two raw samples are retained in the deterministic timing artifact;
no favorable outlier was selected. Per-M1D timestamp splits are still not
exposed and are recorded as an evidence limitation, not fabricated.

## Lifecycle and replay

The facade preserves the existing quarantine/reap and stale-output behavior:
M1B ingress establishes the prefix replay identity, the facade derives fixed
M1C/M1D identities internally, and uncertain completion rejects the output.
It neither accepts arbitrary shader IDs nor permits graph traversal or mutable
topology. Existing stage-group fault coverage continues to apply; there is no
claim of per-pipeline fault injection.

The deterministic package was regenerated twice by
[`tools/evt2_m1e_assembly`](../../../tools/evt2_m1e_assembly), including the
updated callable API, timing, and M2 handoff.

## Next milestone

**EVT-2 M2A — Compile `noise_refiner.1` by assembly reuse.** First obtain and
validate the pinned full-model source/tensor inventory, then prove the exact
weight/topology/witness differences. Do not infer reuse from block names.
