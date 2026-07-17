# Prometheus M49a controlled stage gain and mitigation

Date: 2026-07-17  
Device: NVIDIA GeForce RTX 3070  
Backend/compiler: Vulkan with validation / MSVC 19.51.36231  
Convergence outcome: **MEANINGFUL PROGRESSION**  
Milestone state: **IN PROGRESS**  
M48 EVT state: **POSTPONED**

## Outcome

M49a now has audit-only matched-input owners for M44 output projection, M46 RMSNorm, and the M47 FFN suffix. Each verifies logical shape, physical stride, content hash, nonzero generation, weight identities, precision realization, strategy, and deterministic source identity before submission. The M47 owner now runs A2x4 FP32, conventional FP16, and cooperative FP16 through safe explicit working storage. It also has a research-only mixed path in which retained FP32 Hidden is genuinely consumed by A2x4 FP32 Wdown. Warm audit execution allocates no Vulkan buffers, repeated hashes are stable, and all three focused validation-enabled facts are clean.

This is not M49a completion. Cooperative suffix execution and two upstream stage owners remove a real execution boundary, and all ten perturbation families now run across the suffix paths, but the hardware corpus is still identification-only and tiny. Complete M43–M47 matched-input execution, primary and held-out shapes/families, cross-stage magnitude sweeps, selected coordinate GPU-to-FP64 comparisons, narrow Gate/Up and Wdown promotion, checkpoint stacks, canary calibration, and the certification matrix remain open. No mitigation or envelope is certified.

## M49 handoff

M49 established deterministic, depth-correlated divergence on the primary four-layer stack. Both reduced-precision paths first crossed tolerance at layer-1 FFN, while A2x4 FP32 remained within the established boundary tolerances. Those trajectory observations remain inherited evidence; this report does not relabel them as matched-input disturbance or controlled gain.

## Suffix-injection architecture

`prom_reactor_runtime_m49a_execute_ffn_suffix` is intentionally outside product selectors. Its request includes the exact logical input, input generation, expected input hash, exact source hash, projection/gating/residual realization, shape, and three independent weight generations. A mismatch rejects before submission. The audit owns its upload and readback buffers and adds no product submission, dispatch, barrier, or readback.

The executor reuses M47 planning, descriptor, shader, canonical FP16 packing, and timestamp authorities. Gate, Up, and Hidden working buffers declare transfer-source capability so the audit can copy them; product command recording still issues no transfer or intermediate readback. A missing transfer-source declaration was caught by validation and fixed before accepting evidence.

The cooperative suffix now uses a dedicated audit owner that explicitly ensures the selected SGEMM pipeline before recording. This fixes the missing-pipeline lifecycle fault without reusing the withdrawn validation-dirty prototype. M44 deterministically densifies an exact strided input before delegating the existing safe output-projection audit. M46 uses dedicated per-slot input storage and an explicit transfer-to-shader barrier, avoiding stale working-set contents. Normal product selectors and command recording remain unchanged.

The heterogeneous fixed-stack checkpoint remains incomplete. Its earlier prototype was withdrawn; no generic scheduler was introduced.

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
| cooperative FP16 | Hidden | 8.02676e-9 | 1.39698e-9 | 1.83564e-12 | 2.32831e-10 | 4.65661e-10 |
| cooperative FP16 | full FFN | 0 | 0 | 0 | 0 | 0 |
| FP32 Hidden + FP32 Wdown | Wdown | 1.07151e-8 | 1.16415e-9 | 1.67119e-11 | 4.65661e-10 | 6.98492e-10 |
| FP32 Hidden + FP32 Wdown | full FFN | 1.03238e-7 | 5.96046e-8 | 8.73115e-11 | not emitted | not emitted |
| M46 FP32 | RMSNorm | 1.21645e-6 | 4.76837e-7 | -1.48298e-11 | 3.72529e-9 | 7.45058e-9 |
| M44 A2x4 FP32 | output projection | 0 | 0 | 0 | 0 | 0 |

The M44 witness uses dyadic operands and is therefore lifecycle evidence, not a discriminating numerical case. M46's wide/cancellation input has the largest recorded L-infinity D, while Up retains the largest recorded L2 D. These rows use named local authorities and must not be ranked as mitigation error reduction across unequal references. Primary and held-out cases are absent, so the milestone-wide highest-injection question remains open.

## Controlled inherited-error gain

The permanent generator covers all ten requested deterministic families, hashes identity, records support and L1/L2/L-infinity norms, and normalizes to a requested L2 magnitude. The tiny hardware case executes all ten families at deterministic magnitudes from `1e-5` (FP32-only relative to FP16 spacing) through `0.0625`, including aligned/decorrelated residuals and a natural discrepancy scale.

| Path | L1 gain | L2 gain | L-infinity gain | Interpretation |
|---|---:|---:|---:|---|
| A2x4 FP32 FFN | 1.00037 | 1.00064 | 1.07216 | nearly neutral globally, local maximum amplification |
| conventional FP16 direct FFN | 1.00018 | 1.00032 | 1.05662 | nearly neutral globally, local maximum amplification |
| conventional FP16 retained Hidden FFN | 1.00018 | 1.00032 | 1.05662 | same final boundary behavior as direct on this case |
| cooperative FP16 FFN | 1.00018 | 1.00032 | 1.05662 | bitwise same reduced realization on this case |
| FP32 Hidden + FP32 Wdown FFN | 1.00018 | 1.00032 | 1.05576 | no material global-gain reduction |

The gain is direction dependent and multimodal. A2x4 L2 ranges from 0.999505 to 1.00713 and L-infinity peaks at 1.07216 for the dense pattern. Conventional L2 ranges from 0.99499 (one-token contraction) to 1.00128, while L-infinity peaks at 1.08909 for the near-ULP one-token impulse. The conventional FP16-bin-boundary pattern is exactly neutral in all three norms in this case, a visible threshold effect. Retained-Hidden final outputs and all ten gains are bitwise identical to direct packing. Saturation and sign asymmetry still need explicit sweeps, and no cross-stage “highest gain stage” conclusion is supported.

## FP64 local witnesses

A selected six-term cancellation-heavy dot produces 0.9999 with scalar FP32 accumulation and 1.9999 with FP64 accumulation, an absolute difference of 1.0. With a bounded canonical-FP16 operand witness, FP32 and FP64 accumulation both print as 1.99976, separating operand quantization from this selected accumulation order. The selected RMS row prints sum-of-squares 2.0e16 and InvRms 1.73205e-8 in both precisions at recorded display precision.

These are CPU local witnesses. Selected hardware Gate, Up, Wdown, M44, and RMS rows have not yet been compared against the FP64 values, so no claim is made about which hardware accumulation order is closer.

## Mitigation A/B

| Mitigation | Identification error reduction | Held-out reduction | GPU latency | Added memory | Generalizes? |
|---|---:|---:|---:|---:|---|
| conventional direct baseline | baseline | not run | 37.888–63.488 us | 0 | no |
| retain FP32 Hidden | 0% at final FFN | not run | 38.912–64.512 us (0–2.048 us delta) | 16 KiB FP32 Hidden working storage | no |
| FP32 Hidden + FP32 Wdown consumer | 0.98% L2; -2.74% L-infinity | not run | 78.848 us (+40.960 us) | 16 KiB retained Hidden | no |
| complete A2x4 FP32 FFN upper bound | 99.716% L2 against common FP64 authority | not run | 122.752–165.440 us (+83.904–101.952 us) | not isolated | no |
| periodic FP32 checkpoint | not complete | not run | not available | not available | no |
| full A2x4 FP32 | inherited M49 witness only | not run | not comparable here | not isolated | no |

Retaining FP32 Hidden changes tiny Hidden bits (`L2 8.02676e-9` versus its reduced CPU oracle) but the immediate canonical FP16 pack before Wdown maps both policies to the same effective Wdown input. The final capture hash is identical. The policy therefore changes storage without improving this case. Three diagnostic timing pairs put its measured delta between 0 and 2.048 us; this uncontrolled range is not a stable cost claim. A future useful variant would need an empirically selected Wdown path that consumes the retained value without immediately recreating the same FP16 boundary.

The suffix test now also evaluates every policy against one precisely defined common authority: the full FFN is computed in FP64 and rounded once to float at the final output. Conventional and cooperative FP16 both have L2 `1.14947e-4` and L-infinity `8.70228e-6`; full A2x4 FP32 has L2 `3.26468e-7` and L-infinity `1.19209e-7`. Genuine FP32 Hidden plus FP32 Wdown has L2 `1.13820e-4` but L-infinity `8.94070e-6`. Thus it improves L2 by only 0.98%, worsens L-infinity by 2.74%, and takes 78.848 us versus 37.888 us for the direct conventional path. It is ineligible even before considering its zero held-out support.

Complete-FFN A2x4 is an upper-bound witness, not a selective policy. Gate/Up-only and Wdown-only block policies are still required before any error-reduction-per-cost ranking.

The periodic checkpoint attempt did not converge. Heterogeneous per-layer projection inside the current fixed-stack owner conflicts with its shared path-specific working-set assumptions. Expanding that into a generic scheduler would violate M49a scope. The prototype was removed; a future bounded solution should use an explicit audit-only checkpoint suffix owner or a verified homogeneous block boundary.

## Depth propagation and envelopes

M49’s primary trajectory remains the only depth evidence: conventional FFN maximum absolute discrepancies `[1.15156e-4, 3.54118e-2, 1.36608, 2.17557]`, cooperative `[1.15156e-4, 3.55415e-2, 1.20667, 1.66956]`, and A2x4 `[3.93391e-6, 5.3215e-4, 1.25732e-2, 3.93524e-2]` across layers 0–3.

Provisional tiny identification relations can be written as `Eout <= Gbound * Ein + Dbound`, but held-out support is zero. They are recorded with `certified=false` and extrapolation policy `reject`. The permanent fitter consumes identification rows only and reports held-out failures without retuning.

## Canary and Shadow HSFM precursor

The native contract now carries stage-local D, inherited E, gain regime, bias, recurrence, channel concentration, activation magnitude, FP16-boundary density, canary projections, current path, candidate cost, confidence, and held-out validity. Explicit regimes remain nominal, bounded drift, high injection, high gain, checkpoint recommended, FP32 promotion recommended, reference suspect, and audit required. There is no product authority.

Canary calibration code reports Pearson correlation, confusion counts, false-positive rate, and false-negative rate and has a permanent calculation fact. No native full-audit or held-out calibration cases have been run. Current canaries therefore cannot yet be said to detect the first FFN breach, high-gain attention, mitigation success, or a semantic defect.

## Oct integration and friction

M1 of `PrometheusNumericalHeterogeneityLab` introduces a native mode separate from M0 synthetic mode. It loads a typed Octagon projection of the RTX JSON evidence, binds provenance to the JSON SHA-256, fits only identification records, reports held-out support independently, refuses certification with zero held-out records, and emits Octagon, CSV, JSON, Markdown, and a PNG chart. Loading is fallible; there is no synthetic fallback.

The expanded fit has 10 identification rows and zero held-out rows. Fitted, authored, and uniform models all select conventional FP16 in shadow mode, so no held-out ranking improvement exists. The fitted model is additionally rejected on two independent robustness gates: its domain-constrained disturbance coefficients have the wrong signs (`CoefficientSignsValid=false`) and leave-one-row-out maximum weight delta is `2.63107` against a `1.0` limit. For the selected conventional row, the fitted contribution breakdown is `[-0.117001, -0.0353390, -0.0559474]` for L2 disturbance, L-infinity disturbance, and time, plus bias `-0.529778`, total score `-0.738066` before explicit `1e6` Int quantization. `EvaluateLinearUtilityEvidence`, both fitted-versus-baseline comparisons, and the score breakdown are persisted in the Octagon artifact. The model remains shadow-only and `ProductAuthorityChanged: false`.

Oct friction: the standard library has JSON construction but no typed JSON parsing API, so native JSON cannot be consumed directly. M1 uses an explicit typed Octagon projection and hash binding. This is a general standard-library gap, not an experiment-specific workaround hidden from the report. No language defect was changed.

## Validation

Completed in this progression:

- Windows MSVC native rebuild;
- full validation-enabled native lane: 427 facts, 395 passed, 32 intentional skips, zero failures;
- validation-clean M44, M46, and M47 suffix hardware facts plus adjacent M47/M48 product regressions;
- exact matched-input enforcement and repeat hashes;
- zero warm audit allocation;
- Vulkan validation clean after transfer-source usage was made explicit;
- existing M47 product cooperative regression and M48 cooperative fixed-stack regression preserved in focused runs;
- standard targeted and aggregate Go test matrices, native manifest, and SDSL-V workspace checks;
- Linux GCC produced an ELF reactor and Marionette harness; the explicit Linux smoke passed. The complete helper exceeded the 120-second host limit while compiling auxiliary binaries, so this is not recorded as a full helper pass;
- `bash -n` and `git diff --check` clean at the recorded checkpoint.

Primary/held-out hardware, canary calibration, the checkpoint stack, and the requested exhaustive slow wrapper/hardware corpus remain incomplete. No shader changed, so regeneration, `spirv-val`, and disassembly assertions are not applicable in this slice.

## Recommendation gate

1. **Which stage injects most?** Up has the largest recorded L2 D; M46 RMSNorm has the largest recorded L-infinity D. Authorities and cases differ, so no milestone-wide answer is supported.
2. **Which stage has largest inherited gain?** Only the complete FFN was measured across directions on hardware; its maximum observed L-infinity gain is 1.08909, but cross-stage ranking is unavailable.
3. **Are they the same stage?** Unknown.
4. **Does FP32 Hidden help?** Storage alone does not. When Wdown genuinely consumes FP32 Hidden, tiny-case L2 improves only 0.98%, L-infinity regresses 2.74%, and latency roughly doubles; this is not a useful mitigation result.
5. **Best selective FP32 policy per cost?** Not rankable without common-reference and held-out error reduction.
6. **Does a periodic checkpoint contract error?** Unknown; the checkpoint path is not complete.
7. **Which mitigation generalizes?** None has held-out support; no generalization claim is allowed.
8. **Can an empirical envelope predict held-out growth?** Framework yes, evidence no; held-out support is zero.
9. **Do current canaries detect dangerous regimes with acceptable false rates?** Unknown; native calibration is pending.
10. **Does the fitted utility model beat authored and uniform ranking on held-out data?** No comparison is available. All three choose conventional FP16 on identification data; the fitted model also violates sign and leave-one-out stability gates.
11. **Is M49b controller calibration justified?** No. The evidence contract exists, but calibration and held-out data do not.
12. **Is M48 ready to resume?** No. There is neither a held-out mixed policy nor a certified envelope.

## Recommendation

M49b should not begin controller-threshold calibration. The next coherent boundary is complete-block/depth mitigation identification and held-out calibration: finish M43/full-block exact entry, run primary plus machine-separated held-out shapes and families, add selected GPU-to-FP64 coordinate oracles, compare narrow promotions, and implement checkpoints through a dedicated audit stack owner.

M48 EVT remains **POSTPONED**. No policy has survived held-out hardware validation, so neither broad resumption nor a narrower certified envelope is justified.

## Artifacts

- `internal/prometheus/DevelopmentReport/artifacts/M49a/controlled_stage_gain_and_mitigation_rtx3070.json`
- `Experiments/PrometheusNumericalHeterogeneityLab/M1/M1.native_rtx3070.octagon`
- generated M1 Octagon/CSV/JSON/Markdown/PNG outputs

The native JSON SHA-256 is `e1cebf242a99d1a06cd761ba1541fc20fe87efd2eb8dd94c81b199718d1d1406`. Two consecutive final artifact generations were byte-identical: Octagon `7a937c9ff53078e53d1d1344c67f86c1776c772125069436c80d32c6d1e69452`, CSV `60f73abb749326e1600e2442c0b399f15878df6e0f54feb16809fe1f71f54252`, JSON `c8a5d469f403ba55661e014e2cbb57746a0226f16d2e953f8923bc919ab216a0`, and PNG `22f0b05438b395b8e4136c1cb2cd5fab9811566bd086db46009b11f67d7dc3bd`.
