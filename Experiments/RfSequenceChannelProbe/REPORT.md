# REPORT

## 1. Goal

This experiment probes whether RF M0a removed the sequence-ergonomics friction seen in the first RF experiment. The focus is not new RF physics; it is whether longer, deterministic sequence studies now feel natural with the `...Series` and elementwise gain/noise helpers.

## 2. Sequence baseline

Part A builds 16-element distance and bandwidth sweeps and then composes RF M0a series helpers end-to-end:

- `FreeSpacePathLossLinearSeries` and `LogDistancePathLossLinearSeries`
- `ThermalNoisePowerWithNoiseFigureSeries`
- `SNRLinearSeries`

The baseline confirms expected physical trends (path loss rises with distance, received power falls, noise rises with sweep, SNR falls) while staying purely sequence-driven rather than hand-writing fixed-size arrays. Scalar-vs-series consistency checks show the series helpers are compositional wrappers over the same deterministic math.

## 3. AWGN sequence probe

Part B applies deterministic AWGN over a 16-element baseline signal. A deterministic scale-factor sequence is converted into elementwise noise power and applied with `ApplyAwgnSamples`.

Observed behavior:

- input/output shape is preserved at length 16
- elementwise perturbations match deterministic factors at early/middle/late indices
- longer-sequence expression remains concise without fixed-size helper code

## 4. Rayleigh sequence probe

Part C applies deterministic Rayleigh fading over the same 16-element baseline using two equivalent composition paths:

1. `ApplyRayleighFadingSeries`
2. `RayleighPowerGainSeries` + `ApplyPowerGainSeries`

The probe verifies elementwise equivalence between these paths, and demonstrates both attenuation and amplification in-sequence.

## 5. Observations

What is cleaner after RF M0a:

- no fixed-length plumbing is required for AWGN/Rayleigh/path-loss/noise studies
- sequence shape preservation is explicit and testable
- scalar helpers and series helpers compose predictably
- gain-first composition (`RayleighPowerGainSeries` + `ApplyPowerGainSeries`) makes channel pipelines clearer

Remaining ergonomic pressure (left intentionally visible):

- sentinel-array error signaling (`[0.0]` / `[0kg*m^2/s^3]`) still requires explicit shape checks in higher-level probes
- sequence construction still needs explicit loops/append patterns; there is no higher-level sweep generator in RF itself

Based on this probe, the next pressure point appears to be RF M1 channel richness (or lightweight sequence-construction ergonomics), not fixed-size channel-helper limitations.

## 6. Conclusion

For deterministic, first-principles studies, RF M0a is sufficient to make longer sequence-based RF/channel experiments feel natural in Oct. The specific fixed-size awkwardness from the original probe is removed for this class of workflow.

## Deliverable summary

1. **Experiment name/path**: `Experiments/RfSequenceChannelProbe`
2. **Added Oct artifacts**:
   - `manifest.oct`
   - `M0/rf_sequence_channel_probe.oct`
   - `M0/rf_sequence_channel_probe.octest`
   - this `REPORT.md`
3. **RF M0a behaviors exercised**:
   - sequence path-loss helpers (`FreeSpacePathLossLinearSeries`, `LogDistancePathLossLinearSeries`)
   - sequence thermal noise + SNR helpers (`ThermalNoisePowerWithNoiseFigureSeries`, `SNRLinearSeries`)
   - deterministic AWGN sequence composition (`ApplyAwgnSamples`)
   - deterministic Rayleigh sequence composition (`RayleighPowerGainSeries`, `ApplyPowerGainSeries`, `ApplyRayleighFadingSeries`)
4. **Tests validating claims**:
   - length-preservation and `Len > 4` assertions
   - monotonic trend checks for distance/bandwidth studies
   - scalar-vs-series consistency checks
   - AWGN elementwise deterministic delta checks
   - Rayleigh elementwise attenuation/amplification and composition-equivalence checks
5. **Remaining friction intentionally left visible**:
   - sentinel return conventions for mismatched shapes
   - manual loop-based sweep construction in experiment code
