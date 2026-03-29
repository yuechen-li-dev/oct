# RF Library (M0a)

RF M0a keeps the deterministic M0 math intact and improves sequence ergonomics.

## Elementwise sequence semantics

RF helpers remain scalar-first, with explicit `...Series` variants for arrays.

- Series helpers preserve input shape for valid inputs.
- Operations are elementwise and deterministic.
- Mismatched sequence shapes return deterministic sentinel arrays (`[0.0]` or `[0kg*m^2/s^3]`).

## Sequence-focused helpers

- Path loss: `FreeSpacePathLossLinearSeries`, `LogDistancePathLossLinearSeries`
- Noise: `ThermalNoisePowerSeries`, `ThermalNoisePowerWithNoiseFigureSeries`
- AWGN: `ApplyAwgnSamples`
- Rayleigh: `RayleighPowerGainSeries`, `ApplyPowerGainSeries`, `ApplyRayleighFadingSeries`
- Link budget/SNR: `SNRLinearSeries`, `SNRDbSeries`

## Determinism

All helpers are pure and reproducible:

- no randomness
- no hidden state
- no implicit reshaping/broadcast rules
