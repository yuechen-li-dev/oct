# REPORT

## 1. Goal

This experiment checks whether RF M0 is already usable for small, first-principles RF/channel studies in Oct. The package intentionally stays narrow: deterministic link budget and path-loss composition, deterministic AWGN composition, and deterministic Rayleigh fading composition.

## 2. Deterministic baseline

The baseline computes received power across distance using both free-space path loss and a log-distance model anchored at 1 m. It also computes thermal noise (`kTB`) with a linear noise figure term and shows SNR at fixed received power while varying bandwidth.

Distance sweep: 10 m, 30 m, 100 m, 300 m at 2.4 GHz.

Key outcomes:

- received power decreases monotonically with distance for both path-loss models
- log-distance (`n = 3`) aligns with free-space near the anchor point and diverges to lower received power at longer range
- increasing bandwidth raises noise floor and reduces SNR

## 3. AWGN probe

The AWGN probe applies deterministic additive noise samples to a deterministic baseline sample vector. This validates that RF M0 AWGN helpers are enough to express a repeatable channel perturbation study and quantify point-wise deltas from baseline.

Key outcomes:

- sample shape is preserved
- sample magnitudes shift in controlled positive/negative directions
- baseline and AWGN outputs are observably different while staying reproducible

## 4. Rayleigh probe

The Rayleigh probe applies deterministic `(I, Q)` component tuples through RF M0 Rayleigh helpers to produce element-wise fading gains.

Key outcomes:

- channel output preserves sample shape
- fading can express both attenuation and amplification relative to deterministic baseline
- resulting behavior is qualitatively distinct from pure deterministic link budget output

## 5. Observations

What felt clean:

- RF M0 link-budget pieces compose directly into small scalar/array studies
- linear-domain units are explicit and keep dimensions understandable
- deterministic AWGN/Rayleigh helpers are sufficient for reproducible probes

What felt limited (intentionally left visible):

- sample-series helpers currently assume a fixed 4-sample shape in practice, which constrains experiment expressiveness
- AWGN and Rayleigh surfaces are deterministic composition primitives only; no richer channel-process tooling exists in M0
- no built-in plotting/report artifacts are used here, so interpretation remains numeric/test-driven

These are good RF M1 pressure points without expanding M0 in this pass.

## 6. Conclusion

RF M0 already supports meaningful first-principles RF experiments in Oct when the study is kept small and deterministic. The next pressure point is richer sequence/channel ergonomics (shape-general helpers and broader channel-process modeling), not core deterministic link-budget correctness.

## Deliverable summary

1. **Experiment name/path**: `Experiments/RfAwgnRayleighProbe`
2. **Added Oct artifacts**:
   - `manifest.oct`
   - `M0/rf_awgn_rayleigh_probe.oct`
   - `M0/rf_awgn_rayleigh_probe.octest`
   - this `REPORT.md`
3. **RF M0 behaviors exercised**:
   - free-space path loss, log-distance path loss
   - thermal noise (`kTB`) and noise figure scaling
   - received power and SNR composition
   - deterministic AWGN sample composition
   - deterministic Rayleigh fading composition
4. **Tests validating claims**:
   - monotonic received-power trends vs distance
   - model divergence between free-space and log-distance at longer ranges
   - noise/SNR trend vs bandwidth
   - AWGN shape+delta checks
   - Rayleigh shape+attenuation/amplification checks
5. **Friction left visible for future RF work**:
   - fixed-size sample helper ergonomics
   - deterministic-only noise/fading surfaces in M0
