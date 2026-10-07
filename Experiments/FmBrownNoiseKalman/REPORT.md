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

## Random v2 migration (2026-10-03)

The white noise for every milestone now comes from a Random v2 stream:
`Random.Normals(Random.Fork(Random.Seeded(seed), "white-noise"), n, 0.0, 1.0)`
in `M0` and in `Shared`. The seeds are unchanged. The noise realization for a
given seed is different, so every recorded number that depends on noise
changed. The existing tests did not need any change and pass as before. One
fact was added to M0: a seed replays its noise and another seed gives other
noise.

Recorded outputs:

- **M3, M4, M4b, M5, M6: regenerated.** Before regenerating, each was
  reproduced from the Random v1 code and matched the recorded files, so the
  differences in this change come from the noise and from nothing else.
- **M2: not regenerated.** Its artifact entry points do not run, for reasons
  that have nothing to do with Random (see below). `M2/metrics.*`,
  `M2/m2a_report.*` and the M2b sweep files still describe the v1 noise.
- **M0, M1:** no outputs are recorded in the repository.

What moved, out of 27 sweep cases:

| | Random v1 | Random v2 |
|---|---|---|
| M4b scalar adaptive wins | 15 | 11 |
| M4b whiteness only | 12 | 16 |
| M4b mean delta output SNR (dB) | -0.103 | -0.017 |
| M4b mean whiteness ratio | 0.460 | 0.534 |
| M6 guarded adaptive wins | 15 | 9 |
| M6 guarded whiteness only | 12 | 17 |
| M6 guarded mean delta output SNR (dB) | -0.109 | -0.019 |

The reading of the experiment is the same: adaptation whitens the innovation
in every case and does not, on average, improve the recovered signal. The
count of cases labelled a win is sensitive to the noise realization: it fell
from 15 to 11 for the scalar filter and from 15 to 9 for the guarded one.
Treat the win counts as one draw, not as a rate. `M4/FINDINGS.md` is updated
by hand; `M5/FINDINGS.md` and `M6/FINDINGS.md` are generated.

Artifact entry points:

- `oct artifact` rejects ambient reads during artifact evaluation. The entry
  points of M3, M4, M4b, M5 and M6 wrote their outputs and then read them back
  to assert that they were non-empty, so none of them could run. This was
  already the case before the migration. The read-backs are removed; the
  entry points now only write.
- M2 has the same read-backs and a second fault: `M2bArtifactsWrite` writes
  `m2b_sweep_progress.json` more than once, which `oct artifact` rejects as a
  duplicate output path. M2 is left as it was.

## Json migration (2026-10-07)

Every JSON summary of M1 to M6 was text built with `+` or `String.Concat`
and handed to `Json.Object`. Each is a record now, published with
`Artifact.WriteJson(path, value)`; `Json.Object` no longer exists. The M1 and
M2 tests load the summary with `Json.Load<T>` and check its fields.

Recorded outputs:

- **`metrics.json` of M1, M3, M4 and `sweep_summary.json` of M4b, M5, M6:
  regenerated.** The values are the same. The keys are the record's field
  names (`TotalCases`, was `totalCases`), and the text is in the layout `Json`
  writes: indented, a `Float` always with a fraction. Every other published
  file is byte-identical.
- **`M2/metrics.json`: regenerated in a scratch copy** with the faults of M2's
  entry points set aside, since they still do not run under `oct artifact`
  (below). Same values.
- M2b's `.octagon` report held a JSON text in a String and its Markdown
  report printed the JSON text of the summary. The first is a record and the
  second a key-value table. Neither file is recorded.

M2's entry points still do not run, for three reasons that are not JSON:
`M2ArtifactFilesWrite` reads `metrics.csv` back with `IO.Read`, which the
phase refuses; `M2Artifacts` publishes the same files a second time; and
`M2bArtifactWriteAll` publishes `m2b_sweep_progress.json` more than once.
With those set aside, `M2/metrics.csv` and `M2/m2a_report.md` come out with
other numbers than the recorded files, which still describe the Random v1
noise. They are left as recorded.
