# RF Library (M1)

RF M1 keeps the deterministic M0/M0a math and sequence ergonomics, and adds modest channel-realism helpers:

- Rician fading
- Log-normal shadowing (dB offsets mapped to linear factors)
- Tapped-delay-line (TDL) multipath helpers

## Design boundaries

RF remains first-principles and standards-agnostic:

- scalar-first helpers with explicit `...Series` variants
- deterministic, pure, reproducible helper surfaces
- no hidden state and no random-number subsystem
- no giant channel/simulator object framework

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

## Deferred by design

Still intentionally out of scope:

- Doppler/time variation
- coherence-time/coherence-bandwidth metrics
- MIMO channel models
- standards-specific channel profiles
- packet/protocol or full wireless simulation frameworks
