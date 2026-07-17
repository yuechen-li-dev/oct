# Prometheus M49a controlled stage gain and mitigation

Date: 2026-07-17  
Device: NVIDIA GeForce RTX 3070  
Backend/compiler: Vulkan with validation / MSVC 19.51.36231  
Convergence outcome: **MEANINGFUL PROGRESSION**  
Milestone state: **IN PROGRESS**  
M48 EVT state: **POSTPONED**

## Outcome

M49a now has a real, audit-only matched-input M47 FFN suffix and the numerical contracts needed for controlled identification. The suffix accepts an exact host activation, verifies its bit hash and generation, reuses the canonical M47 weights, executes Gate/Up, SiLU-gating, Wdown, and second residual without changing product selection, and performs one explicit final audit readback. It can capture Gate, Up, FP32 Hidden, Wdown, second residual, or the full FFN result. Warm audit execution allocates no Vulkan buffers, repeated hashes are bitwise stable, and the validation-enabled identification fact is clean.

This is not M49a completion. The completed hardware corpus is one tiny identification case, one dense perturbation direction, A2x4 FP32 and conventional FP16. Cooperative matched-input suffix execution, M44/M46/full-block entry points, remaining perturbation directions and magnitudes, held-out shapes/families, the complete FP64 GPU comparison, checkpoint mitigation, native canary calibration, and the required mitigation matrix remain open. No mitigation or envelope is certified.

## M49 handoff

M49 established deterministic, depth-correlated divergence on the primary four-layer stack. Both reduced-precision paths first crossed tolerance at layer-1 FFN, while A2x4 FP32 remained within the established boundary tolerances. Those trajectory observations remain inherited evidence; this report does not relabel them as matched-input disturbance or controlled gain.

## Suffix-injection architecture

`prom_reactor_runtime_m49a_execute_ffn_suffix` is intentionally outside product selectors. Its request includes the exact logical input, input generation, expected input hash, exact source hash, projection/gating/residual realization, shape, and three independent weight generations. A mismatch rejects before submission. The audit owns its upload and readback buffers and adds no product submission, dispatch, barrier, or readback.

The executor reuses M47 planning, descriptor, shader, canonical FP16 packing, and timestamp authorities. Gate, Up, and Hidden working buffers declare transfer-source capability so the audit can copy them; product command recording still issues no transfer or intermediate readback. A missing transfer-source declaration was caught by validation and fixed before accepting evidence.

The attempted cooperative suffix and mixed-path fixed-stack checkpoint exposed a bounded architectural blocker: direct audit capture across the cooperative working set and heterogeneous path selection inside the fixed-stack shared working set are not yet safe. Both prototypes were withdrawn rather than retained as brittle or validation-dirty paths.

## Reference authorities

- CPU FP32 semantic evaluation is the inspectable operation reference.
- CPU reduced-precision simulation uses canonical binary16 round-to-nearest-even at the specified boundaries.
- GPU A2x4 FP32 is an independent hardware witness, not absolute truth.
- Conventional and cooperative FP16 remain distinct reduced-precision realizations.
- Selected CPU FP64 dot and RMS witnesses isolate accumulation sensitivity; the GPU-to-FP64 selected-operation matrix is not complete.

The tiny conventional result is exactly equal to its canonical reduced-precision CPU oracle. The A2x4 result differs from the scalar CPU FP32 oracle at small accumulation-order scale. Comparing those two raw D values as though they shared one reference would be invalid.

## Matched-input local disturbance

Shape: Tokens 16, ModelWidth 128, FfnWidth 256. Input: deterministic near-FP16-midpoint family, seed 4901. Weights: deterministic modular family. All rows below are validation-clean identification evidence.

| Path | Stage | L2 D | L-infinity D | Bias | p95 abs | p99 abs |
|---|---|---:|---:|---:|---:|---:|
| A2x4 FP32 | Gate | 1.27589e-6 | 5.96046e-8 | 6.87464e-10 | 4.09782e-8 | 4.84288e-8 |
| A2x4 FP32 | Up | 1.48932e-6 | 7.82311e-8 | -4.42014e-10 | 4.47035e-8 | 5.96046e-8 |
| A2x4 FP32 | Hidden | 6.57150e-8 | 6.28643e-9 | -1.21666e-11 | 2.15368e-9 | 3.49246e-9 |
| A2x4 FP32 | Wdown | 4.08901e-8 | 2.79397e-9 | 1.53610e-12 | 1.73168e-9 | 2.09548e-9 |
| A2x4 FP32 | full FFN | 3.15398e-7 | 1.19209e-7 | 5.82077e-11 | not emitted | not emitted |
| conventional FP16 direct | full FFN | 0 | 0 | 0 | 0 | 0 |
| conventional FP16 retained Hidden | full FFN | 0 | 0 | 0 | 0 | 0 |

For the A2x4 scalar-reference comparison, Up has the largest L2 fresh discrepancy in this case. This does not answer the milestone-wide “highest injection stage” question because M44, M46, cooperative M47, primary, and held-out cases are absent.

## Controlled inherited-error gain

The permanent generator covers all ten requested deterministic families, hashes identity, records support and L1/L2/L-infinity norms, and normalizes to a requested L2 magnitude. The tiny hardware case executes all ten families at deterministic magnitudes from `1e-5` (FP32-only relative to FP16 spacing) through `0.0625`, including aligned/decorrelated residuals and a natural discrepancy scale.

| Path | L1 gain | L2 gain | L-infinity gain | Interpretation |
|---|---:|---:|---:|---|
| A2x4 FP32 FFN | 1.00037 | 1.00064 | 1.07216 | nearly neutral globally, local maximum amplification |
| conventional FP16 direct FFN | 1.00018 | 1.00032 | 1.05662 | nearly neutral globally, local maximum amplification |
| conventional FP16 retained Hidden FFN | 1.00018 | 1.00032 | 1.05662 | same final boundary behavior as direct on this case |

The gain is direction dependent and multimodal. A2x4 L2 ranges from 0.999505 to 1.00713 and L-infinity peaks at 1.07216 for the dense pattern. Conventional L2 ranges from 0.99499 (one-token contraction) to 1.00128, while L-infinity peaks at 1.08909 for the near-ULP one-token impulse. The conventional FP16-bin-boundary pattern is exactly neutral in all three norms in this case, a visible threshold effect. Retained-Hidden final outputs and all ten gains are bitwise identical to direct packing. Saturation and sign asymmetry still need explicit sweeps, and no cross-stage “highest gain stage” conclusion is supported.

## FP64 local witnesses

A selected six-term cancellation-heavy dot produces 0.9999 with scalar FP32 accumulation and 1.9999 with FP64 accumulation, an absolute difference of 1.0. With a bounded canonical-FP16 operand witness, FP32 and FP64 accumulation both print as 1.99976, separating operand quantization from this selected accumulation order. The selected RMS row prints sum-of-squares 2.0e16 and InvRms 1.73205e-8 in both precisions at recorded display precision.

These are CPU local witnesses. Selected hardware Gate, Up, Wdown, M44, and RMS rows have not yet been compared against the FP64 values, so no claim is made about which hardware accumulation order is closer.

## Mitigation A/B

| Mitigation | Identification error reduction | Held-out reduction | GPU latency | Added memory | Generalizes? |
|---|---:|---:|---:|---:|---|
| conventional direct baseline | baseline | not run | 37.888–63.488 us | 0 | no |
| retain FP32 Hidden | 0% at final FFN | not run | 38.912–64.512 us (0–2.048 us delta) | 16 KiB FP32 Hidden working storage | no |
| A2x4 FP32 FFN path | not comparable across unequal references | not run | 122.752–165.440 us (+83.904–101.952 us) | not isolated | no |
| periodic FP32 checkpoint | not complete | not run | not available | not available | no |
| full A2x4 FP32 | inherited M49 witness only | not run | not comparable here | not isolated | no |

Retaining FP32 Hidden changes tiny Hidden bits (`L2 8.02676e-9` versus its reduced CPU oracle) but the immediate canonical FP16 pack before Wdown maps both policies to the same effective Wdown input. The final capture hash is identical. The policy therefore changes storage without improving this case. Three diagnostic timing pairs put its measured delta between 0 and 2.048 us; this uncontrolled range is not a stable cost claim. A future useful variant would need an empirically selected Wdown path that consumes the retained value without immediately recreating the same FP16 boundary.

Stage-selective FP32 is a real measured M47 path, but this run cannot report error reduction because A2x4 and conventional rows use different precision-matched CPU reference authorities. A common FP64 selected-dot comparison and held-out block trajectories are required before ranking error reduction per microsecond.

The periodic checkpoint attempt did not converge. Heterogeneous per-layer projection inside the current fixed-stack owner conflicts with its shared path-specific working-set assumptions. Expanding that into a generic scheduler would violate M49a scope. The prototype was removed; a future bounded solution should use an explicit audit-only checkpoint suffix owner or a verified homogeneous block boundary.

## Depth propagation and envelopes

M49’s primary trajectory remains the only depth evidence: conventional FFN maximum absolute discrepancies `[1.15156e-4, 3.54118e-2, 1.36608, 2.17557]`, cooperative `[1.15156e-4, 3.55415e-2, 1.20667, 1.66956]`, and A2x4 `[3.93391e-6, 5.3215e-4, 1.25732e-2, 3.93524e-2]` across layers 0–3.

Provisional tiny identification relations can be written as `Eout <= Gbound * Ein + Dbound`, but held-out support is zero. They are recorded with `certified=false` and extrapolation policy `reject`. The permanent fitter consumes identification rows only and reports held-out failures without retuning.

## Canary and Shadow HSFM precursor

The native contract now carries stage-local D, inherited E, gain regime, bias, recurrence, channel concentration, activation magnitude, FP16-boundary density, canary projections, current path, candidate cost, confidence, and held-out validity. Explicit regimes remain nominal, bounded drift, high injection, high gain, checkpoint recommended, FP32 promotion recommended, reference suspect, and audit required. There is no product authority.

Canary calibration code reports Pearson correlation, confusion counts, false-positive rate, and false-negative rate and has a permanent calculation fact. No native full-audit or held-out calibration cases have been run. Current canaries therefore cannot yet be said to detect the first FFN breach, high-gain attention, mitigation success, or a semantic defect.

## Oct integration and friction

M1 of `PrometheusNumericalHeterogeneityLab` introduces a native mode separate from M0 synthetic mode. It loads a typed Octagon projection of the RTX JSON evidence, binds provenance to the JSON SHA-256, fits only identification records, reports held-out support independently, refuses certification with zero held-out records, and emits Octagon, CSV, JSON, Markdown, and a PNG chart. Loading is fallible; there is no synthetic fallback.

Oct friction: the standard library has JSON construction but no typed JSON parsing API, so native JSON cannot be consumed directly. M1 uses an explicit typed Octagon projection and hash binding. This is a general standard-library gap, not an experiment-specific workaround hidden from the report. No language defect was changed.

## Validation

Completed in this progression:

- Windows MSVC native rebuild;
- eight M49a calculation/identity facts plus one validation-enabled RTX suffix fact;
- exact matched-input enforcement and repeat hashes;
- zero warm audit allocation;
- Vulkan validation clean after transfer-source usage was made explicit;
- existing M47 product cooperative regression and M48 cooperative fixed-stack regression preserved in focused runs;
- `git diff --check` clean at the recorded checkpoint.

The full Go, Linux GCC, manifest/workspace, Oct compiled artifact, primary/held-out hardware, canary, consistency, and complete M42–M49 matrix requested by M49a are not yet complete. No shader changed, so regeneration, `spirv-val`, and disassembly assertions are not applicable in this slice.

## Recommendation gate

1. **Which stage injects most?** Up has the largest matched-input A2x4 L2 D in the tiny completed case. No milestone-wide answer is supported.
2. **Which stage has largest inherited gain?** Only the complete FFN was measured across directions on hardware; its maximum observed L-infinity gain is 1.08909, but cross-stage ranking is unavailable.
3. **Are they the same stage?** Unknown.
4. **Does FP32 Hidden help?** No final-output benefit on the tiny identification case; the immediate FP16 Wdown boundary erases it.
5. **Best selective FP32 policy per cost?** Not rankable without common-reference and held-out error reduction.
6. **Does a periodic checkpoint contract error?** Unknown; the checkpoint path is not complete.
7. **Can an empirical envelope predict held-out growth?** Framework yes, evidence no; held-out support is zero.
8. **Do current canaries detect dangerous regimes?** Unknown; native calibration is pending.
9. **Enough evidence for M49b calibration?** No. The evidence contract exists, but calibration data does not.
10. **Any mitigation mature enough for M48 EVT?** No.

## Recommendation

M49b should not begin controller-threshold calibration. The next bounded phase should finish cooperative and M44/M46/full-block matched-input suffixes, run multiple directions and magnitudes on primary plus machine-separated held-out shapes/families, add selected GPU-to-FP64 oracles, and implement a checkpoint through a dedicated audit owner rather than heterogeneous mutation of the fixed-stack working set.

M48 EVT remains **POSTPONED**. No policy has survived held-out hardware validation, so neither broad resumption nor a narrower certified envelope is justified.

## Artifacts

- `internal/prometheus/DevelopmentReport/artifacts/M49a/controlled_stage_gain_and_mitigation_rtx3070.json`
- `Experiments/PrometheusNumericalHeterogeneityLab/M1/M1.native_rtx3070.octagon`
- generated M1 Octagon/CSV/JSON/Markdown/PNG outputs
