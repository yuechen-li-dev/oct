# RF Library (M2a)

RF M2a keeps the deterministic M0/M0a/M1 RF math and channel helpers, and adds deterministic sequence builders for experiment inputs:

- constant deterministic series
- linear deterministic ramps for scalar and distance sweeps
- explicit piecewise construction through deterministic concatenation

## Design boundaries

RF remains first-principles and standards-agnostic:

- scalar-first helpers with explicit `...Series` variants
- deterministic, pure, reproducible helper surfaces
- no hidden state and no random-number subsystem
- no giant channel/simulator object framework
- no generic time-series utility framework

## Sequence semantics

Series helpers preserve shape for valid inputs and stay elementwise.
Invalid shape/parameter combinations return deterministic sentinel arrays (`[0.0]`, `[0]`, or `[0kg*m^2/s^3]`).

## M1 additions

### Rician fading

- `RicianPowerGainFromComponents`
- `RicianAmplitudeFromComponents`
- `ApplyRicianFading`
- `RicianPowerGainSeries`
- `ApplyRicianFadingSeries`

Rician helpers use explicit K-factor and externally supplied scatter components, preserving deterministic tests.

### Log-normal shadowing

- `ShadowingLinearFromDb`
- `ShadowingLinearFromDbSeries`
- `ApplyShadowingToPathLoss`
- `ApplyShadowingToPathLossSeries`
- `ApplyShadowingToReceivedPower`
- `ApplyShadowingToReceivedPowerSeries`

Shadowing is expressed as explicit dB offsets so callers can supply deterministic or externally generated variation.

### Tapped-delay-line multipath

- `ApplyTappedDelayLinePowerSeries`
- `MaxDelaySamples`

TDL helpers model per-tap delay and power gain with deterministic convolution-style application.


## M2a additions

### Deterministic sequence builders

- `ConstantFloatSeries`
- `ConstantDistanceSeries`
- `LinearFloatSeries`
- `LinearDistanceSeries`
- `ConcatFloatSeries`

These helpers reduce hand-written array verbosity when building deterministic RF experiment inputs (shadowing offsets, gain patterns, scatter trends, and distance sweeps).

Design intent is intentionally narrow:

- deterministic only (no RNG, no hidden state)
- explicit lengths and endpoints
- small composable surface that feeds existing RF `...Series` helpers

## Deferred by design

Still intentionally out of scope:

- Doppler/time variation
- coherence-time/coherence-bandwidth metrics
- MIMO channel models
- standards-specific channel profiles
- packet/protocol or full wireless simulation frameworks
- random-number sequence generators or stochastic input builder layers
