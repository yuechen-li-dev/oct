# RF Library (M2c)

RF M2c keeps the deterministic M0/M0a/M1 RF math and channel helpers, retains M2a sequence ergonomics, keeps M2b Doppler/coherence reasoning, and adds a small deterministic MIMO channel layer:

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


## M2b additions

### Doppler shift helpers

- `DopplerShiftHz`
- `DopplerShiftHzWithPropagationSpeed`
- `MaxDopplerShiftHz`

These helpers provide deterministic mobility-to-frequency-shift reasoning from carrier frequency, radial velocity, and (optionally) propagation speed.

### Coherence-time helpers

- `CoherenceTimeSecondsFromMaxDopplerJakes`
- `CoherenceTimeSecondsFromMaxDopplerHalfCycle`

Both are approximation-based and intentionally explicit:

- Jakes-style approximation: `0.423 / f_d,max`
- half-cycle approximation: `1 / (2 f_d,max)`

### Coherence-bandwidth helpers

- `CoherenceBandwidthHzFromRmsDelaySpreadWideSense`
- `CoherenceBandwidthHzFromRmsDelaySpreadStrict`

Both are approximation-based and intentionally explicit:

- wide-sense approximation: `1 / (5 tau_rms)`
- strict approximation: `1 / (50 tau_rms)`

### Minimal deterministic time-variation helper

- `DopplerPhaseRadians`

This helper provides deterministic phase progression from Doppler shift and time without introducing a stochastic time-varying channel simulator.



## M2c additions

### Simple deterministic MIMO helpers

- `MimoChannel` and `MimoChannelFromRowMajor`
- `ApplyMimoChannel` for explicit `y = Hx` application
- `ReceivedStreamPowerSeries` for per-receive-stream power under a given input
- `RowPowerGainSeries`, `ColumnPowerGainSeries`, and `TotalChannelPowerGain` for small coupling/gain inspection

MIMO helpers are intentionally narrow:

- deterministic only (no random channel generation)
- shape-explicit (row-major coefficients with strict matrix/vector compatibility checks)
- no hidden transpose, reshape, padding, or broadcasting
- no beamforming, precoding/decoding, estimation, scheduler, or standards layers

## Deferred by design

Still intentionally out of scope:

- full time-varying stochastic fading/channel processes
- stochastic or standards-specific MIMO channel models
- beamforming and precoding/decoding frameworks
- channel-estimation and multi-user antenna-stack frameworks
- standards-specific mobility/channel profiles
- packet/protocol or full wireless simulation frameworks
- random-number sequence generators or stochastic input builder layers


## M0 S-parameters additions

### Deterministic 2-port S-parameter core

- `SParameters2Port` with direct fields: `Frequencies: Float<Hz>[]`, `S11`, `S21`, `S12`, `S22`
- `ValidateSParameters2Port` for non-empty and strictly increasing frequency axes (Complex[] length introspection is currently unavailable, so trace-length consistency is caller-maintained in M0)
- `MagnitudeDb`, `PhaseRadians`, `PhaseDegrees` for small complex-trace inspection
- `InterpolateS11`, `InterpolateS21`, `InterpolateS12`, `InterpolateS22` with query frequency `Float<Hz>`
  - exact-knot queries return the stored sample
  - in-range non-knot queries linearly interpolate real/imag parts separately
  - out-of-range queries reject with `Error` (no clamping/extrapolation)
- `ReturnLossDbFromS11` and `InsertionLossDbFromS21` use explicit convention `-20*log10(|Sij|)`

Scope is intentionally narrow and 2-port only for M0: no Touchstone I/O, no N-port framework, no cascading/de-embedding, and no impedance-renormalization surfaces.

## Typed-frequency API note (M5b)

For RF physical frequency and bandwidth function parameters/returns in low/medium-risk helpers, the API now uses unit-typed frequency values (`Float<Hz>`) instead of dimensionless `Float` (notably in RF.Noise, RF.PathLoss, and RF.Doppler function surfaces).

Dimensionless logarithmic and ratio quantities remain plain `Float` by design (for example dB/dBm, noise figure linear factors, path-loss exponents, and SNR-derived scalar ratios).
