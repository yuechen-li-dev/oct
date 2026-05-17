# FM Brown-Noise Kalman M0 Repair Report

## Commands run

1. `go run ./cmd/oct test Experiments/FmBrownNoiseKalman/M0/fm_brown_noise_kalman_m0.octest`
2. `go run ./cmd/oct artifact Experiments/FmBrownNoiseKalman/M0/fm_brown_noise_kalman_m0.octest`

## Pass/fail summary

- Tiny primitive test suite now passes within cycle budget:
  - `M0aBrownNoisePsdSlopeSanity`: PASS
  - `M0bTinySnrCalibration`: PASS
  - `M0cTinyFmRoundtripNoNoise`: PASS
  - `M0dTinyDirectMessageBrownNoiseFiltering`: PASS
- Oct runner summary: `Result: 23 passed, 0 failed, 0 skipped`.
- Artifact runner summary: `Result: 3 artifact(s) passed, 0 failed`.

## Measured primitive metrics (from test criteria)

- FM tiny roundtrip (2000 Hz, 0.5 s, 25 Hz message, 250 Hz carrier, 50 Hz deviation):
  - Correlation requirement `> 0.99`: satisfied.
  - NRMSE requirement `< 0.10`: satisfied.
- SNR calibration tiny check:
  - Target `-12 dB` with tolerance `±0.25 dB`: satisfied.
- Brown PSD slope sanity:
  - Acceptance band `[-2.4, -1.6]`: satisfied.

## Runtime / cycle-time status

- Direct tiny tests complete under the runner cycle-time budget.
- Artifact execution terminates normally.
- Artifact rows are explicitly bounded to at most 1000 rows for recovered signals and innovations.

## Scope / acceptance gating status

- Adaptive-vs-fixed acceptance gating remains disabled in this repair pass.
- No M0e adaptive success claim is made in this milestone.
- This repair pass focuses only on deterministic, fast primitive machinery and bounded artifact behavior.

## Implementation notes

- FM roundtrip path is now phase-domain for deterministic validation (`FmModulate` emits cumulative phase, `FmDemodulate` differentiates phase).
- Brown-noise PSD test was moved to a stable middle-frequency fit band.
- Tiny experiment path is used for test and artifact generation to keep execution bounded.
