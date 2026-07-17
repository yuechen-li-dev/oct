# Prometheus M49b numerical Shadow-HSFM — pre-DVT baseline

Date: 2026-07-17  
Convergence outcome: **SUCCESS**  
Milestone state: **COMPLETE**  
M48 EVT state: **POSTPONED TO DVT**  
DVT state: **READY**

M49b ends here. This freezes the RTX 3070 implementation and its bounded live
evidence loop; calibration is now DVT work, not a reason to retain the milestone.

## Live paired evidence

At controller cadence the real M48 owner records the selected fixed four-block
stack, then records one internal all-A2x4 stack using the same initial activation
generation, persistent layer resources, block order, shape, and pinned parameter
generation. The selected result remains product authority. The witness has no
output-tensor readback and cannot recurse.

Each stack transfer-copies the same 16 deterministic shape-derived final-output
coordinates to its slot-owned 64-byte host-visible capture. No full tensor is
read back and product output is not mutated. The paired control unit derives
sample deltas, sampled L2/L-infinity, two signed projection deltas, an absolute
projection delta, finite agreement, coordinate disagreement count, and a
deterministic paired identity from both replay identities, shape, parameter
generation, and delta bits.

The estimator is authored and inspectable. A complete finite 16-coordinate pair
with nonzero shape/replay/parameter identities receives confidence `0.80`.
Missing identity or finite disagreement gives zero confidence and marks the
reference suspect. Gain is explicitly neutral (`1.0`): paired final outputs do
not justify inventing an input-disturbance gain model. History stores delta
samples, so its compact projections are discrepancy projections, not raw output.

The current canary is 16 transfer copies, not a new shader or compute dispatch.
It is real bounded output evidence, but transfer-GPU time is not separately
timestamped and must not be represented as a compute-shader overhead result.

## Cadence, lifecycle, and authority

`CanaryInterval = 4` is measured in selected completed stacks; the internal
witness does not consume an index. A pair is due in `Unidentified`,
`ReferenceSuspect`, or `Quarantined`, and otherwise when
`execution_index % CanaryInterval == 0`. A parameter publication resets to
`Unidentified`, forcing the next selected request to pair. A one-slot runtime
does not recycle selected output early: it requests bounded audit instead.

The selected request is copied and policy-applied before planning. It cannot
change after recording begins. Completion precedes capture observation. An
uncertain completion quarantines slot and controller; a known witness failure
produces `AuditRequired`, never accepted selected evidence.

- Stage 0/1 retain product path and emit shadow evidence/action scores.
- Stage 2 applies the first-class interval-two pattern (blocks 1 and 3 A2x4).
- Stage 3 applies all-A2x4 during cooldown and recovers through checkpoint.
- Audit and controller override fields are distinct and planner-rejected when mixed.

Controlled finite-evidence tests exercise Nominal, drift, high gain, checkpoint,
fallback, cooldown/recovery, parameter generation, and quarantine. The live RTX
rows below are Stage-0 paired observer rows; their `BoundedDrift` result is not
a claim of hardware-triggered Stage-2/3 promotion.

## Source organization

`reactor_vulkan_transformer_control.c` owns pure shape identity, cadence, paired
estimation, fixed policy application, and quarantine adaptation. It owns no
Vulkan state. `reactor_vulkan_fused_reduction.c` remains the fixed-stack owner
of slot buffers, recording, descriptors, barriers, submission, fences, witness
invocation, and destruction. `reactor_vulkan_transformer.c` owns plan validation
and replay identity.

The fused-reduction file was 14,453 lines before the runtime pass and is 14,733
at freeze. The lifecycle seam accounts for the increase; 155 lines of pure
policy/estimation now live in the new transformer-control unit. No
transformer-specific logic was added to reduction pipeline implementation.

## Frozen parameters

| Field | Default |
| --- | ---: |
| CanaryInterval / CheckpointInterval | 4 / 2 |
| EnterHighGainCount / ExitHighGainCount | 2 / 3 |
| MaxL2 / MaxLinf / MaxGain | 0.02 / 0.05 / 1.25 |
| ConfidenceFloor / FallbackCooldown | 0.75 / 8 |
| AuditSampleCount / RolloutStage | 16 / 0 |

All fields are range-validated, generation-identified, and live-tunable without
shader recompilation. Authored utility is `[-0.60, -0.30, -0.10]`; uniform and
fitted-shadow scores are recorded, while fitted authority remains disabled.

## RTX 3070 paired runs

Device: NVIDIA GeForce RTX 3070, driver 596.36, 8192 MiB. Both runs were
validation-enabled live fixed-stack tests, cooperative requested, with bitwise
repeatable conventional M48 warm corpus output.

| Shape | Selected GPU | Selected E2E* | Observer CPU | A2x4 GPU | A2x4 E2E | Confidence | State/action |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| 4 x 128 x 1024, FFN 4096 | 13.988 ms | 20.456 ms | 1.1 us | 48.121 ms | 52.758 ms | 0.80 | BoundedDrift / shadow continue |
| held-out FFN 2048 | 12.452 ms | 19.085 ms | 1.5 us | 42.379 ms | 47.196 ms | 0.80 | BoundedDrift / shadow continue |

*Selected E2E excludes separately reported witness E2E. The live test reaches
the next cadence boundary with another paired witness and asserts zero Vulkan
buffer allocations there. The initial pair warms two 64-byte slot buffers.

Primary identities: canary `13133589003333920077`, witness replay
`5233646395579545000`, evidence `7963429468135628739`, controller
`17706179060646069574`. Held-out identities: canary `8225047763399955016`,
witness replay `16433110970959169920`, evidence `11274842124219885905`,
controller `4676268274102808232`.

These rows prove nonzero bounded confidence and lifecycle-safe evidence. They do
not claim full-tensor discrepancy, separately timed canary transfer GPU cost,
sub-1% amortized production overhead, or complete Stage-2/3 RTX outcome matrix.
Those measurements move to portable DVT.

## Faults and validation

Focused tests cover invalid parameters, deterministic paired evidence,
nonfinite/low-confidence containment, bounded history, authority, cooldown,
parameter update, and audit/controller separation. The live stack suite covers
paired capture, known fault recycling, uncertain completion quarantine/reap,
validation-enabled execution, and warm paired zero allocation.

Passed: Windows MSVC native rebuild;
`PrometheusM49bControllerParametersEvidenceAndBoundedAuthority`;
`PrometheusM49bFixedStackAdapterKeepsAuditAndAuthoritySeparate`;
`PrometheusM48ValidationSubmitPlansAndCapacity`; and
`PrometheusM48LiveFixedStackUsesFourLayerBundlesAndTwoSubmitTopologies`,
including primary and held-out paired rows. No shader changed, so no new
SDSL-V/SPIR-V lane was required. The MSYS2 GCC/Linux-script attempt reached
native compilation but is blocked on this Windows host by missing Vulkan SDK
headers; it is not reported as a Linux pass.

## Pre-DVT freeze and AMD procedure

Base revision: `be4bfd13` plus this M49b change set. Source/binary hashes and
machine-readable rows are in the companion artifact. Stage 0 is the rollout
default; RTX history/confidence must be segregated before a new backend. The
artifact is the Oct lab handoff; no second Oct controller exists and
`ProductAuthorityChanged` is false for the frozen rows.

1. Build on the AMD unified-memory laptop; record GPU/driver, memory behavior,
   subgroup/wave size, timestamps/limits, cooperative tuples, and shader hashes.
2. Reset M49b to `Unidentified`; publish portable Stage-0 defaults and force a
   paired witness.
3. Run FP32 witness, conventional FP16 if supported, then cooperative only when
   its tuple is verified.
4. Run four layers, 128 tokens, width 1024, eight heads, head dimension 128,
   FFN 4096; if capacity requires it, record the reason and use FFN 2048.
5. Measure paired evidence, checkpoint/fallback, topology, memory, faults,
   primary/held-out overhead, and replay identity. Tune only the central record.

Do not import RTX thresholds or confidence as AMD certification. Remaining
uncertainty is field-tunable DVT work, not another RTX-only M49b milestone.
