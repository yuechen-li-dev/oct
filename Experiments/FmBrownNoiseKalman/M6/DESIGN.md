# FM Brown-Noise Kalman M6 Design Lab

## 1) Question

Can we design an adaptive policy that better balances two partially conflicting objectives in this synthetic direct-message brown-noise setup:

1. innovation residual whitening, and
2. recovered-message preservation/recovery quality,

instead of using residual whitening as the sole adaptation proxy?

M6 is a design lab. It is not a realism pass, and not a broad algorithm expansion.

## 2) Background from M4b/M5

### M4b scalar incremental sweep (27 cases)

From `M4b/sweep_summary.json` and report:

- totalCases = 27
- AdaptiveWin = 15
- WhitenessOnly = 12
- RecoveryOnly = 0
- NoMeaningfulWin = 0
- meanDeltaOutputSNRDb = -0.10323719568627238 dB
- meanWhitenessRatio = 0.4599244600302107
- finalA range = [0.8796228912983445, 0.99], mean 0.9727125543420385
- clampTotal = 12512

Implication: scalar incremental adaptation robustly improved whiteness, but average output-SNR moved slightly negative and many wins were whiteness-only.

### M5 scalar vs windowed (27 cases)

From `M5/sweep_summary.json` and report:

- scalarAdaptiveWin = 15, windowedAdaptiveWin = 1
- windowedBetterRecovery = 10
- windowedBetterWhiteness = 0
- scalarMeanWhitenessRatio = 0.4599244600302107
- windowedMeanWhitenessRatio = 0.9206405349794723
- scalarMeanDeltaOutputSNRDb = -0.10323719568627238 dB
- windowedMeanDeltaOutputSNRDb = -0.01281063531133515 dB

Derived from `M5/sweep_metrics.csv`:

- windowed better recovery than scalar in 10/27 cases
- scalar better recovery than windowed in 17/27 cases
- windowed better recovery spread across all input SNR values
- windowed had zero clamp events while scalar had many clamp hits

Implication: estimator form alone does not solve the objective mismatch. Windowing softened adaptation and sometimes helped recovery, but gave up much of scalar whitening power.

## 3) Why whitening alone is insufficient

The M4b+M5 pattern is consistent:

- whitening gains are frequent and large,
- recovery gains are less consistent,
- and improving residual whiteness does not guarantee improved recovered message quality.

So M6 should test objective-aware adaptation policies that explicitly guard recovery while still pursuing whitening.

## 4) Candidate adaptive policies

All candidates below preserve scalar board state and keep history in external arrays.

### Candidate A — Whiteness-only scalar incremental (control)

- Definition: existing M4/M4b scalar incremental lag-1 adaptive update.
- Scientific value: high as baseline control only.
- Implementation effort: already done.
- Oracle dependency: none.
- Generalization potential: high (already deployable in principle).

### Candidate B — Windowed lag-1 estimator (control)

- Definition: existing M5 windowed estimator with smoothed update.
- Scientific value: high as comparison control (tradeoff anchor).
- Implementation effort: already done.
- Oracle dependency: none.
- Generalization potential: high.

### Candidate C — Recovery-guarded scalar adaptation (recommended new policy)

- Definition: produce `proposedA` (scalar incremental source), then gate or attenuate update if short-window recovery proxy worsens beyond tolerance even when whiteness improves.
- Minimal guard form:
  - compute short-window recovery proxy trend (e.g., correlation/NRMSE to known message),
  - if whitening improves but recovery drops beyond `epsRecovery`, reject update or scale learning rate by `guardScale < 1`.
- Scientific value: very high (directly tests core M6 hypothesis).
- Implementation effort: moderate, bounded.
- Oracle dependency: yes for this synthetic design lab if using true message correlation/NRMSE.
- Generalization potential: medium-high (oracle proxy can be replaced later by non-oracle surrogates).

### Candidate D — Conservative blended objective over candidate A updates

- Definition: score update by
  - `objective = whitenessImprovement - lambda * recoveryDegradation`
- Test small lambda set `{0, 0.25, 0.5, 1.0}` only.
- Scientific value: high (maps objective tradeoff frontier).
- Implementation effort: moderate-high (more bookkeeping and policy tuning surface).
- Oracle dependency: yes in this lab form.
- Generalization potential: medium.

### Candidate E — Stability-gated adaptation

- Definition: reject/scale updates when `|proposedA-currentA|` too large, `finalA` nears clamp boundary, or recent recovered-output changes spike.
- Scientific value: medium (regularization/safety; indirect objective link).
- Implementation effort: low-moderate.
- Oracle dependency: none required.
- Generalization potential: high.

## 5) Candidate ranking for M6

Ranked for this pass (best first):

1. **Candidate C (Recovery-guarded scalar adaptation)**
   - best direct test of objective-mismatch hypothesis
   - feasible in current scalar-board architecture
2. **Candidate D (Blended objective)**
   - strong scientific follow-up if Candidate C shows signal
3. **Candidate E (Stability gate)**
   - useful fallback if oracle-guard complexity is too high
4. **Candidate B (Windowed baseline)**
   - comparison baseline only
5. **Candidate A (Scalar baseline)**
   - comparison baseline only

## 6) Architecture constraints and per-candidate mapping

Non-negotiable constraints:

- scalar board state only
- no board arrays
- BoardSnapshot(machine)! used for observation/export
- history and window computations external to board state

### Candidate mapping table

| Candidate | Board fields needed | External arrays/history needed | Octomata-state compatible | Uses synthetic oracle | Deployable later without oracle |
| --- | --- | --- | --- | --- | --- |
| A Scalar incremental | existing `a`, `innovationPrev`, Kalman scalars | innovation trace, recovered trace | yes (already) | no | yes |
| B Windowed | existing Kalman scalars + current `a` | innovation window arrays, smoothed estimates | yes (already) | no | yes |
| C Recovery-guarded | existing + scalar guard inputs (`proposedA`, `guardScale`/`accept`) | short-window recovery proxy history + innovation history | yes (driver computes guard; flow consumes chosen scalar) | yes (for M6 lab variant) | potentially, with non-oracle proxy |
| D Blended objective | existing + scalar chosen `a` | rolling whiteness and recovery proxy streams; lambda selection | yes (selection external, scalar applied internal) | yes (lab variant) | potentially |
| E Stability-gated | existing + scalar gate params | short history of `a`, optional output-delta history | yes | no | yes |

## 7) Recommended M6 probe

Recommended probe: **Candidate C (Recovery-guarded scalar adaptation)** on the same 27-case grid used in M4b/M5, comparing:

1. Fixed
2. Scalar incremental
3. Windowed
4. Recovery-guarded scalar (new)

### Proposed per-step policy (auditable minimal form)

1. Compute `proposedA` from scalar incremental estimator.
2. Compute short-window metrics (external arrays):
   - whitening proxy trend,
   - recovery proxy trend (oracle: correlation or NRMSE vs known message).
3. Apply guard rule:
   - if whitening improves and recovery degradation <= `epsRecovery`, accept full update,
   - if whitening improves but recovery degradation > `epsRecovery`, reject update or attenuate (`a <- currentA + guardScale*(proposedA-currentA)`),
   - if whitening does not improve, keep currentA (or fallback attenuation).
4. Clamp as today.

Fallback if local-horizon simulation is too costly: trend-based gate only (no dual-simulation branch).

## 8) Metrics and success criteria

### Thresholds (reuse unless evidence demands change)

- `snrEpsilonDb = 0.01`
- `whitenessRelativeEpsilon = 0.01`

### Required reporting

For new policy vs fixed:

- AdaptiveWin
- WhitenessOnly
- RecoveryOnly
- NoMeaningfulWin

For new policy vs scalar incremental:

- BetterRecovery count
- BetterWhiteness count
- BothBetter count
- WorseBoth count

Also report:

- mean ΔSNR
- mean whiteness ratio
- finalA min/max/mean
- clamp totals and per-case distribution

### Promising-candidate criteria

A candidate is promising if it:

1. improves mean ΔSNR vs scalar incremental,
2. preserves acceptable whitening (does not collapse toward fixed behavior),
3. reduces WhitenessOnly frequency,
4. increases AdaptiveWin and/or RecoveryOnly without large NoMeaningfulWin growth,
5. avoids pathological clamp saturation.

Note: scientific value remains valid even if whiteness weakens somewhat, if recovery quality improves materially.

## 9) Risks

1. **Oracle dependence risk:** using true message for guard is not deployable as-is.
   - mitigation: clearly label oracle-only and treat as hypothesis probe.
2. **Objective leakage risk:** short-window proxy noise may over-gate adaptation.
   - mitigation: include acceptance-rate and guard-trigger diagnostics.
3. **Complexity creep risk:** local-horizon candidate simulation may overcomplicate M6.
   - mitigation: prefer minimal trend-based guard first.
4. **False confidence risk:** bounded 27-case synthetic grid may overstate stability.
   - mitigation: explicitly avoid robustness claims.

## 10) Implementation plan (next pass)

1. Add `M6` experiment folder files by forking M5 structure minimally.
2. Implement recovery-guarded scalar policy using external short-window proxy streams and scalar board consumption.
3. Run bounded 27-case sweep (same grid) across fixed/scalar/windowed/new policy.
4. Emit metrics + summary with the same label policy and add comparative tables versus scalar.
5. Record clamp/finalA patterns and guard-trigger diagnostics.

No FM/IQ/audio realism, no runtime/compiler changes, no board arrays, no broad retuning.

## 11) What not to claim

- Do not claim robustness or production readiness.
- Do not claim improved real FM receiver performance.
- Do not claim oracle-dependent policy is deployable.
- Do not claim objective solved beyond this bounded synthetic grid.
