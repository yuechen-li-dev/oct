# REPORT

## 1. Goal

This experiment checks whether RF M2b makes deterministic mobility/time-variation reasoning feel natural in Oct without introducing simulator complexity. The focus is first-principles trends: Doppler scaling, coherence-time inverses, coherence-bandwidth inverses, and optional deterministic phase evolution.

## 2. Doppler

Part A builds deterministic velocity and carrier-frequency sweeps and computes Doppler shift across each axis.

Observed behavior:

- zero radial velocity maps to zero Doppler
- Doppler magnitude increases monotonically over the rising velocity sweep
- Doppler magnitude increases monotonically over the rising carrier-frequency sweep
- scalar and series Doppler computations agree at sampled indices

This keeps the Doppler surface readable and composable for mobility intuition.

## 3. Coherence time

Part B feeds the Doppler sweep into two coherence-time approximations:

- `CoherenceTimeSecondsFromMaxDopplerJakes`
- `CoherenceTimeSecondsFromMaxDopplerHalfCycle`

Observed behavior:

- both approximations decrease as Doppler increases
- formulas differ numerically but remain directionally consistent
- for positive Doppler points, half-cycle values are larger than Jakes values
- zero Doppler follows deterministic sentinel behavior

This gives a compact first-principles view of time selectivity.

## 4. Coherence bandwidth

Part C sweeps RMS delay spread and applies both coherence-bandwidth approximations:

- `CoherenceBandwidthHzFromRmsDelaySpreadWideSense`
- `CoherenceBandwidthHzFromRmsDelaySpreadStrict`

Observed behavior:

- increasing delay spread decreases coherence bandwidth under both formulas
- wide-sense values are consistently above strict values for matching delay spread
- monotonic ordering is stable and deterministic across the sweep

This provides a clear multipath-frequency-selectivity intuition layer.

## 5. Optional phase probe

Part D uses deterministic `DopplerPhaseRadians` on a fixed time grid for low and high Doppler values.

Observed behavior:

- phase evolves deterministically with time for both Doppler settings
- higher Doppler produces larger per-step phase increments
- scalar and series phase calculations agree at sampled indices

This confirms the minimal phase helper is useful for small time-variation demonstrations.

## 6. Observations

What feels clean for this scope:

- deterministic sweep builders make small mobility/coherence studies readable
- Doppler → coherence-time chaining is direct and testable
- delay-spread → coherence-bandwidth chaining is direct and testable
- optional phase evolution can be shown without introducing stochastic channel-process machinery

Friction intentionally left visible:

- no built-in higher-level sweep utilities (manual loop construction remains in experiment code)
- deterministic helpers provide intuition surfaces only; no richer time-varying fading-process abstraction is included at M2b

## 7. Conclusion

For small deterministic mobility/coherence probes, RF M2b is sufficient and composable. It supports clear first-principles reasoning about time and frequency selectivity while remaining standards-agnostic and lightweight.

## Deliverable summary

1. **Experiment name/path**: `Experiments/RfDopplerCoherenceProbe`
2. **What Oct artifacts were added**:
   - `manifest.oct`
   - `M0/rf_doppler_coherence_probe.oct`
   - `M0/rf_doppler_coherence_probe.octest`
   - this `REPORT.md`
3. **What RF M2b behaviors were exercised**:
   - Doppler scaling via `DopplerShiftHz`, `DopplerShiftHzWithPropagationSpeed`, `MaxDopplerShiftHz`
   - coherence-time approximations vs Doppler (`...Jakes`, `...HalfCycle`)
   - coherence-bandwidth approximations vs delay spread (`...WideSense`, `...Strict`)
   - deterministic phase evolution via `DopplerPhaseRadians`
4. **What tests validate the claims**:
   - zero/monotonic Doppler trend tests across velocity and carrier sweeps
   - inverse-trend coherence-time tests with formula ordering checks
   - inverse-trend coherence-bandwidth tests with formula ordering checks
   - deterministic phase progression and faster-high-Doppler checks
   - scalar-vs-series consistency and stage-shape preservation checks
5. **What friction or limitations were intentionally left visible**:
   - manual sweep loop construction
   - no stochastic/time-varying fading-process model in this experiment scope
