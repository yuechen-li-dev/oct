# FM Brown-Noise Kalman Experiment Report (M0)

## Scope
This report summarizes execution results for:

- `Experiments/FmBrownNoiseKalman/M0/fm_brown_noise_kalman_m0.oct`
- `Experiments/FmBrownNoiseKalman/M0/fm_brown_noise_kalman_m0.octest`

The experiment target is deterministic FM modulation/demodulation under brown noise at `-12 dB` SNR, with baseline and Kalman filter comparisons.

## What was run

### 1) Oct test run
Command:

- `go run ./cmd/oct test Experiments/FmBrownNoiseKalman/M0/fm_brown_noise_kalman_m0.octest`

Observed outcomes:

- `M0aBrownNoisePsdSlopeSanity`: **FAIL** (assertion)
- `M0bSnrCalibration`: **PASS**
- `M0cFmRoundtripNoNoise`: **FAIL** (assertion)
- `M0dDirectMessageBrownNoiseFiltering`: **FAIL** (exceeded cycle time `30.0<s>`)
- `M0eFullFmBrownNoiseComparison`: **FAIL** (exceeded cycle time `30.0<s>`)

Summary line from runner:

- `Result: 20 passed, 4 failed, 0 skipped`

### 2) Artifact run
Command:

- `go run ./cmd/oct artifact Experiments/FmBrownNoiseKalman/M0/fm_brown_noise_kalman_m0.octest`

Observed behavior:

- Started `M0ArtifactInnovationsOctagon` and continued running without completion during observation window (several minutes).
- Process was manually terminated to avoid indefinite resource use.

## Conclusions

Current M0 implementation is **not yet converged** for the intended acceptance criteria. Specifically:

1. The brown-noise PSD slope sanity bound is currently violated in the implemented test path (`M0a` fails).
2. FM no-noise roundtrip correlation target (`> 0.99`) is currently not met (`M0c` fails).
3. The heavier filtering/comparison tests exceed the runner cycle-time budget (`M0d`, `M0e`), so acceptance criteria for adaptive vs fixed KF are not presently demonstrated under this harness.
4. Artifact generation path exists and is wired (`[Artifact]` + `WriteOctagon`), but at least one artifact run does not complete in practical time with current code/test settings.

## Convergence state
Per repository convergence rule, this lands in:

- **Meaningful progression**: deterministic experiment scaffolding and artifact plumbing exist, and one core calibration check passes (`M0b`), but key technical blockers are now isolated with evidence:
  - FM demodulation quality mismatch in no-noise loop,
  - PSD slope mismatch in current estimator windowing,
  - cycle-time pressure on full comparisons/artifact generation.

## Inconsistency callout (as requested by AGENTS.md)

The prior PR narrative claimed broad pass-level readiness, but direct execution in this environment shows clear failures/timeouts above. This is an explicit inconsistency between claimed and observed behavior and should be treated as unresolved until corrected with passing evidence.
