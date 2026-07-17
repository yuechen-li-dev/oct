# Prometheus M49 numerical heterogeneity control research

Date: 2026-07-16  
Device: NVIDIA GeForce RTX 3070  
Backend/compiler: Vulkan with validation / MSVC 19.51.36231  
Convergence outcome: **MEANINGFUL PROGRESSION**  
Milestone state: **IN PROGRESS**  
M48 EVT state: **POSTPONED**

Continuation: [M49a controlled stage gain and mitigation](PROMETHEUS_M49A_CONTROLLED_STAGE_GAIN_AND_MITIGATION.md) adds an audit-only matched-input M47 suffix and first controlled hardware D/G evidence. M49a remains **IN PROGRESS**; no mitigation has held-out support and M48 EVT remains postponed.

## Outcome

M49 establishes the first bounded numerical-heterogeneity identification and shadow-control framework for the M43–M48 transformer path. It also resolves an important ambiguity on the primary `128/1024/8/128/4096` workload:

- all three GPU paths are bitwise stable over 100 warm repetitions;
- conventional FP16 and cooperative FP16 first exceed established stage tolerances at layer-1 FFN and then amplify the discrepancy through depth;
- the all-FP32 A2x4 witness stays inside every established boundary tolerance across four layers;
- conventional and cooperative FP16 have similar error trajectories despite different kernels and latency;
- signed bias is tiny relative to RMS error at the large-error boundaries, so scalar post-hoc correction is not supported;
- normal product execution, allocation, readback, and authority are unchanged.

This is meaningful progression, not completion. Arbitrary matched-input suffix injection, controlled real-stage gain, multi-shape/weight held-out hardware validation, mitigation A/B, canary sensitivity, and FP64 selected-dot oracles remain open.

## Motivation and M48 context

M48 EVT closeout is intentionally postponed. The earlier contradiction included a real CPU reference defect: binary16 conversion did not use canonical round-to-nearest-even. Fixing it reduced disagreement but did not eliminate the depth-dependent FP16 trajectory. The remaining phenomenon is not usefully described as random noise or one bad coordinate.

The requested `M47_M48_FP16_DEPTH_DISCREPANCY_DIAGNOSIS.md` is not present in the repository. No conclusion here depends on that missing hypothesis document.

## Plant and vocabulary

For stage or block `i` and realization `p`:

```text
x[i+1,p] = F[i,p](x[i,p], parameters)
e[i,p] = x[i,p] - x[i,reference]
delta[i,p](x) = F[i,p](x) - F[i,reference](x)
e[i+1,p] approximately J[i] * e[i,p] + delta[i,p]
```

- **Plant:** one stage, one M43–M47 block, or repeated M48 stack.
- **Reference model:** a selected semantically matched implementation, not automatic truth.
- **Realization:** backend, precision, shader, accumulation/storage, and submit plan.
- **State:** activation entering a stage or block.
- **Measurement:** audit-only tensor readback, coordinate, projection, norm, or hash.
- **Residual:** realization output minus reference output.
- **Disturbance injection:** matched-input residual introduced by a stage.
- **Disturbance gain:** output perturbation norm divided by input perturbation norm.
- **Observer/controller:** research-only regime estimation and shadow path/precision recommendation.
- **Reproducibility floor:** the smallest remaining path discrepancy after semantic alignment without copying one realization's exact machine order.

No Gaussian or white-noise model is assumed.

## Authorities

| Authority | Role | Limitation |
| --- | --- | --- |
| CPU semantic reference | Inspectable FP32 stage oracle with canonical binary16 RNE at reduced boundaries | Scalar order is not GPU order; it previously held an RNE defect |
| GPU A2x4 FP32 | Independent all-FP32 hardware witness | Slower upper bound, not absolute truth |
| GPU conventional FP16 | Reduced-precision witness | Deterministic reduced-precision disturbance |
| GPU cooperative FP16 | Independent reduced-precision witness | Numerically tracks conventional on the primary path |

Reference-suspect evidence can be quarantined. CPU/GPU disagreement is classified before any correction or tolerance decision.

## Recorder and metrics

The shared audit recorder copies five full logical tensors per layer before the working set is overwritten: attention, M44 projection, first residual, RMSNorm output, and FFN/second-residual layer output. One-, two-, three-, and four-layer prefixes use the same runtime. M43 retains its matched-input attention audit.

M49 computes L1, L2, L-infinity, MAE, RMS, signed bias, p50/p90/p95/p99 absolute error, cosine similarity, near-zero-aware relative error, token/channel concentration, residual/reference correlation, lag-1 correlation, signed persistence, and channel recurrence. Buffers and calculations are audit-only. Normal execution has no intermediate readback; each execution in the 100-stack warm loop reports zero Vulkan buffer allocation.

Selected Q/K/V, score/softmax, per-head, InvRms, Gate, Up, Hidden, and Down surfaces are not yet unified in the M49 trajectory schema. Existing component audits cover portions, but this remains a Part C gap.

## Determinism

Full outputs were hashed after every warm stack. The original printed `distinct_outputs` field counted hashes differing from the first rather than cardinality; it is now named `hash_changes` and the classification is explicit.

| Path | Runs | First full-output hash | Hash changes | Classification |
| --- | ---: | ---: | ---: | --- |
| Conventional FP16 | 100 | `11954385417472968597` | 0 | Bitwise deterministic within plan |
| Cooperative FP16 | 100 | `12759720903673541007` | 0 | Bitwise deterministic within plan |
| A2x4 FP32 | 100 | `4962826545120659717` | 0 | Bitwise deterministic within plan |

Independent 10-run loops also had zero changes. These validation-enabled runs used warm one-stack submission; existing M48 coverage also executes per-layer, host-wait, and host-bounce topologies. Cold restart and validation-disabled comparisons remain open, so the scope is explicit.

## Stage-local disturbance and depth

Values are maximum absolute residual; parentheses contain coordinates beyond established stage tolerance out of 131,072.

| Path | Layer | Attention | Projection | Residual 1 | RMSNorm | FFN/output |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Conventional | 0 | 2.86e-6 (0) | 0 (0) | 0 (0) | 1.01e-6 (0) | 1.15e-4 (0) |
| Conventional | 1 | 2.23e-4 (0) | 1.41e-4 (0) | 1.39e-4 (0) | 1.56e-4 (0) | 3.54e-2 (2) |
| Conventional | 2 | 1.875e-1 (0) | 1.258e-1 (61) | 1.272e-1 (27) | 6.88e-3 (11) | 1.366 (361) |
| Conventional | 3 | 9.0 (22,680) | 4.912 (15,180) | 4.959 (120) | 5.60e-3 (7,500) | 2.176 (0) |
| Cooperative | 0 | 2.86e-6 (0) | 0 (0) | 0 (0) | 1.01e-6 (0) | 1.15e-4 (0) |
| Cooperative | 1 | 2.24e-4 (0) | 1.41e-4 (0) | 1.39e-4 (0) | 1.56e-4 (0) | 3.55e-2 (2) |
| Cooperative | 2 | 1.875e-1 (0) | 1.258e-1 (61) | 1.270e-1 (27) | 7.34e-3 (17) | 1.207 (301) |
| Cooperative | 3 | 8.25 (22,680) | 4.219 (15,180) | 4.280 (7,320) | 4.46e-3 (3,138) | 1.670 (0) |
| A2x4 | 0 | 4.77e-7 (0) | 1.30e-6 (0) | 7.63e-6 (0) | 1.01e-6 (0) | 3.93e-6 (0) |
| A2x4 | 1 | 2.12e-6 (0) | 1.03e-6 (0) | 3.93e-6 (0) | 3.81e-6 (0) | 5.32e-4 (0) |
| A2x4 | 2 | 2.08e-3 (0) | 1.30e-3 (0) | 1.31e-3 (0) | 2.42e-5 (0) | 1.26e-2 (0) |
| A2x4 | 3 | 1.64e-1 (0) | 8.89e-2 (0) | 8.60e-2 (0) | 7.46e-5 (0) | 3.94e-2 (0) |

The first primary failure is not matched-input M43 attention. It is layer-1 FFN in both FP16 paths. By layer 2 inherited residual is visible at attention and subsequent boundaries begin exceeding bounds. By layer 3 it is large and highly recurrent. Similar conventional/cooperative trajectories implicate shared reduced-precision semantics rather than random kernel behavior; A2x4 independently witnesses coherent topology and reference behavior.

## Gain and error structure

Permanent facts prove exact global, maximum-token, and maximum-channel gain on a controlled linear witness. Real-stage gain is not yet certified: the depth matrix mixes fresh injection and inherited drift, and arbitrary `F(x + epsilon) - F(x)` injection for impulses, channels, seeded signs, aligned residuals, and quantization boundaries is still absent. No global Lipschitz curve is fitted.

At conventional layer-3 attention, L1 is 536,239, L2 1,767.92, L-infinity 9.0, MAE 4.09118, RMS 4.88322, bias 0.000154972, p95/p99 9.0/9.0, residual/reference correlation 0.529877, and channel recurrence 0.987482. A2x4 at the same boundary has RMS 0.0224049 and L-infinity 0.163574, within the bound. Tiny signed bias relative to RMS rejects a scalar-bias explanation. High recurrence with low mean supports structured signed residual, not independent white noise.

Covariance/SVD and histogram artifacts remain open; Gaussianity, low rank, and sawtooth structure are unsupported.

## Reproducibility floor

Canonical RNE removed an accidental semantic mismatch. The remaining primary-shape floor is bounded, not certified for transfer:

| Realization | Layer-3 FFN max absolute residual | Result |
| --- | ---: | --- |
| A2x4 FP32 | 0.0393524 | All 20 boundaries passed |
| Conventional FP16 | 2.17557 | Nine boundaries failed overall |
| Cooperative FP16 | 1.66956 | Nine boundaries failed overall |

Conversion placement, duplicate rounding, FMA contraction, accumulation order, transcendental/rsqrt/SiLU paths, lane order, padding, and selected FP64 dot oracles are not fully isolated.

## Mitigation and performance

| Candidate | Numerical result | Four-submit GPU time | Assessment |
| --- | --- | ---: | --- |
| Conventional FP16 | First failure layer-1 FFN | 37.297 ms | Baseline |
| Cooperative FP16 | Same first failure/nine failing boundaries | 15.248 ms | 2.446x faster, not a numerical mitigation |
| A2x4 FP32 | All 20 boundaries pass | 48.474 ms | 1.300x conventional; accuracy upper bound |
| Retain FP32 Hidden | Prior M47 candidate | 1.043 ms fused vs 1.035 ms packed stage | +2 MiB; numerical held-out A/B pending |
| Periodic FP32 checkpoint | Oct simulation candidate | Synthetic only | Hardware A/B pending |

The Oct experiment contains 12 deterministic cases, 48 trajectories, a 6/6 identification/held-out split, and four bounded candidates. `when utility` selects an FP32 checkpoint with synthetic 91.85% held-out improvement. This is design evidence only and has no product authority.

Three hardware mitigation A/B results and cross-shape held-out generalization are incomplete; M49 therefore cannot claim success.

## Compensation, observer, and controller

The native framework fits/evaluates scalar bias, and a permanent fact rejects it when held-out RMS worsens. No correction enters product execution, and nothing is called Kalman because no stochastic state-space/noise model is justified.

Observer states are `Unidentified`, `Nominal`, `BoundedDrift`, `HighInjection`, `HighGain`, `ReferenceSuspect`, `PrecisionPromotionRecommended`, `BackendFallbackRecommended`, `AuditRequired`, and `Quarantined`. Explicit transitions use enter/exit confirmation hysteresis; suspect references cannot recommend correction and repeated invalid evidence quarantines the record.

The shadow controller generates bounded candidates: accept, conventional, cooperative, A2x4, selective FP32, periodic checkpoint, audit, or quarantine. Impossible actions are ineligible with reasons. Named considerations cover risk reduction, latency, memory, portability, complexity, and confidence with deterministic tie-breaking. The recommendation never changes product authority.

## Canary and envelope

The canary combines L1/L2/L-infinity, two seeded signed projections, an absolute projection, and a full bit hash. A permanent structured-cancellation witness proves opposite signed errors cannot silently cancel. Hardware sensitivity, false rates, transfer, and overhead correlation remain unmeasured.

The machine-readable envelope is stage/path/shape specific:

```text
error_out <= gain_bound * error_in + local_disturbance_bound + bias_bound
```

Records include semantic identity, corpus, bounds, confidence, held-out state, and extrapolation policy. Unsupported path/shape extrapolation is rejected. Framework mechanics are complete; certified empirical bounds are not.

## DVT transfer

Identity uses backend, precision, cooperative tuple, shader hash, accumulation/storage policy, transcendental policy, dispatch topology, and memory regime—not device name. Norm/gain/cancellation features may be operator intrinsic; arithmetic policies are backend specific; latency/capacity are device specific; shapes, weights, and activation regimes are workload specific. AMD unified-memory DVT must rerun identification and held-out validation rather than reuse RTX thresholds.

## Artifacts

- `internal/prometheus/DevelopmentReport/artifacts/M49/numerical_heterogeneity_rtx3070.json`: primary raw-summary matrix, hashes, costs, distributions, identities, and unsupported claims.
- `Experiments/PrometheusNumericalHeterogeneityLab/M0/M0.synthetic_report.octagon`: complete typed synthetic report.
- `Experiments/PrometheusNumericalHeterogeneityLab/M0/synthetic_trajectories.csv`: all 432 depth rows.
- `Experiments/PrometheusNumericalHeterogeneityLab/M0/FINDINGS.md` and milestone-prefixed PNGs: interpretation and charts.

## Limitations

- Only the primary hardware shape/current deterministic weights were rerun across every path.
- Arbitrary matched-input suffix injection and controlled real-stage perturbation gain are absent.
- FP64 selected-dot oracles and eight-layer hardware depth are absent.
- Source-side hardware mitigation A/B and held-out transfer are incomplete.
- Covariance/SVD/histograms, cold restart, and validation-disabled repeatability are incomplete.
- Canary quality is not hardware calibrated; AMD DVT is not run.

These are machine-readable unsupported claims, not hidden fallback behavior.

## Validation

Completed: Windows MSVC rebuild; 11/11 M49 native facts; five compiled Oct facts with zero fallback; Octagon/CSV/JSON/Markdown/PNG artifact generation; validation-enabled conventional/cooperative/A2x4 audits; 10/100 output hashing; A2x4 full-boundary pass. Conventional/cooperative assertion failures were preserved rather than tolerated away. The final Go/Linux/manifest/workspace/diff matrix is recorded in the handoff. No shader source changed, so shader regeneration, `spirv-val`, and disassembly checks are not applicable.

## Recommendation for M48 and DVT

Do **not** resume M48 EVT closure until:

1. arbitrary matched-input M44–M47 suffix injection separates fresh disturbance from inherited drift;
2. controlled perturbation gain is measured for major stages/directions;
3. at least three source-side mitigations are hardware-measured, including selective retention and a periodic/mixed FP32 policy;
4. mitigation generalizes to held-out inputs, weights, tiny/primary/awkward/token-boundary shapes;
5. FP64 selected-dot witnesses bound reference uncertainty;
6. canary sensitivity/false rates correlate with full audit;
7. stage/path envelopes pass held-out validation without device-name thresholds;
8. AMD DVT reruns the identification plan before threshold transfer.

Then M48 may resume with A2x4 as accuracy witness, cooperative as throughput witness, and a validated source-side mixed policy as the candidate golden path. Until then, no automatic correction or selector authority should enter product execution.
