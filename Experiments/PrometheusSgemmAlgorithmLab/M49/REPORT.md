# P13 M8 / Prometheus SGEMM Algorithm Lab M49 — HFSM Resource-Lease Control vs Variant Selection

## Scope

M49 is an executable control-architecture comparison. It does **not** implement kernels, runtime dispatch, or Vulkan changes. It models whether control should primarily live in static variant selection, dynamic HFSM resource leasing, or a hybrid of both.

## Modeled approaches

1. **Feedforward-only**
   - `device_band + shape_class -> kernel_variant`
   - static WIP envelope
   - low implementation overhead, high variance sensitivity

2. **HFSM resource-lease only**
   - slot lifecycle: `Idle -> Buffering -> Request -> Processing -> Yield -> Resume`
   - dynamic WIP and explicit grant/yield flow
   - lower contention/latency stall risk, but reduced compute-bound specialization when variant vocabulary is flat

3. **Hybrid (recommended)**
   - feedforward recipe defines local compute behavior (accumulators/tiles/prefetch profile)
   - HFSM controls request/grant/yield/lookahead at runtime
   - combines compute specialization with robust flow control

## Scenario and dimension coverage

M49 models all required M45 shape classes:

- small-square
- medium-square
- large-square
- tall-skinny
- wide-short
- K-heavy
- ML/FFN-like

Across required scenarios:

- nominal-band
- degraded-band (lower register/shared headroom)
- contention
- latency-dominant

Modeled dimensions include register pressure, shared-memory pressure, bandwidth/compute pressure, pipeline latency pressure, static/dynamic/hybrid WIP control, and product-quality risk vectors.

## Findings from executable model

- **Feedforward-only** performs competitively on compute-dense nominal cases but is brittle under degraded band + contention due to occupancy-cliff and variance amplification.
- **HFSM-only** reduces stall and contention risk in latency-dominant conditions, but with flat recipe space it underperforms on compute-bound shapes (e.g., K-heavy nominal).
- **Hybrid** preserves or improves nominal product score versus feedforward while materially reducing stall and occupancy-cliff risks across variance scenarios.

## Direct answers (required)

1. **Is kernel variant selection alone sufficient?**
   - No. It lacks runtime actuation for contention and variance shocks; occupancy-cliff risk remains high.

2. **Is HFSM resource control alone sufficient?**
   - Not fully. It improves flow robustness, but without recipe diversity it under-delivers compute-bound throughput.

3. **Does hybrid dominate both?**
   - Yes in this model: best aggregate product score and strongest cross-scenario robustness.

4. **What is the minimal hybrid architecture?**
   - A compact feedforward recipe selector (small variant set) + lease arbiter + slot-state HFSM (`buffer/request/process/yield/resume`) + bounded lookahead actuator.

5. **How should lookahead be used?**
   - As bounded outstanding-depth control in the HFSM, driven by runtime pressure signals; never as unbounded prefetch.

6. **How should WIP be controlled?**
   - Runtime lease budgeting per slot/workgroup with explicit yield on critical-section exit and backpressure under contention.

7. **What is the role of kernel variants in the final system?**
   - Variants are local execution recipes (compute behavior), not the global controller.

8. **What should P13 M9 implement?**
   - Resource-lease controller in runtime, lookahead as true actuator, minimal recipe set centered on `SRT-2accum-K`, and recipe-aware lease caps.

## Artifacts emitted

- `m49_strategy_comparison.octagon`
- `m49_resource_flow_table.octagon`
- `m49_wip_control_table.octagon`
- `m49_failure_modes.octagon`
- `m49_final_recommendation.octagon`

## Documentation consistency note

M49 follows current `Language/reference`-style usage already present in nearby Prometheus lab experiments (`enum`, `record`, deterministic `Fact`, and `Artifact` emission patterns). No explicit contradiction was identified during this scoped task.

## Px16 M10 RTX 3070 DVT feedback

Real RTX 3070 Px16 DVT data showed that `memory-conservative` is not only a degraded/register-constrained fallback. It won or remained competitive on high-capability discrete GPU shapes including wide, rectangular, low-K, odd, and awkward cases, so native selector assumptions were widened beyond `REGISTER_CONSTRAINED` devices. M49's broader architecture conclusion still stands: variants are local execution recipes selected by feedforward policy, while HFSM/resource-lease control remains runtime actuation rather than dispatch authority.
