# Oct Standard Libraries — RF/Wireless Typed-Frequency Audit (M5)

## 1) Executive summary

Scope completed:
- Mandatory references reviewed: `internal/libraries/LIBRARY_MODERNIZATION_AUDIT.md` and `Language/reference` unit docs.
- Code/test/docs audited under `Libraries/RF/` and `Libraries/Wireless/`.

Audit totals:
- Files audited: **22** (`.oct`, `.octest`, `README.md`, `manifest.oct` where relevant naming/docs appeared).
- Candidate symbols found/classified: **26**
- High-confidence `Float<Hz>`: **16**
- High-confidence `Float<s^-1>`: **0**
- Keep dimensionless `Float`: **9**
- Unclear / needs design decision: **1**

Reference alignment notes:
- `Language/reference/language/08-units.md` explicitly supports signed exponents and `Hz` as alias of `s^-1`; therefore frequency-like public concepts should prefer `Float<Hz>` over untyped `Float`.
- No explicit special type for dB/dBm is documented in `Language/reference`; these remain dimensionless `Float` with naming/docs discipline.

## 2) RF findings

### RF.Noise (`Libraries/RF/RF.Noise.oct`)

1. **`ThermalNoisePower(bandwidthHz: Float, ...)` `bandwidthHz`**
- Current type/name: `Float`, `bandwidthHz`
- Recommended type: `Float<Hz>`
- Confidence: High
- Risk: Medium
- Reasoning: Explicit Hz semantics in parameter name/docs and direct use in kTB.
- Call-site/test impact: All direct calls and `*_Series` arrays in `RF.Noise.octest` will require typed literals/arrays.

2. **`ThermalNoisePowerSeries(bandwidthHz: Float[], ...)` `bandwidthHz[]`**
- Current type/name: `Float[]`, `bandwidthHz`
- Recommended type: `Float<Hz>[]`
- Confidence: High
- Risk: Medium
- Reasoning: Same as above, vectorized bandwidth inputs.
- Impact: Series callers/tests migrate arrays to typed Hz.

3. **`ThermalNoisePowerWithNoiseFigure(... bandwidthHz: Float, ...)` `bandwidthHz`**
- Recommended type: `Float<Hz>` (High, Medium risk)
- Reasoning/impact: Same contract as base thermal-noise function.

4. **`ThermalNoisePowerWithNoiseFigureSeries(... bandwidthHz: Float[], ...)` `bandwidthHz[]`**
- Recommended type: `Float<Hz>[]` (High, Medium risk)
- Reasoning/impact: Same contract as above.

### RF.PathLoss (`Libraries/RF/RF.PathLoss.oct`)

5. **`FreeSpacePathLossLinear(..., frequencyHz: Float)` `frequencyHz`**
- Recommended type: `Float<Hz>`
- Confidence: High
- Risk: Medium
- Reasoning: Carrier frequency in Friis relation; explicit Hz naming/docs.
- Impact: propagates to path-loss call-sites/tests.

6. **`FreeSpacePathLossLinearSeries(..., frequencyHz: Float)` `frequencyHz`**
- Recommended type: `Float<Hz>`
- Confidence: High
- Risk: Medium
- Reasoning: Shared carrier frequency with explicit Hz semantics.
- Impact: all series users update.

### RF.Doppler (`Libraries/RF/RF.Doppler.oct`)

7. **`DopplerShiftHz(carrierFrequencyHz: Float, ...)` `carrierFrequencyHz`**
- Recommended type: `Float<Hz>`
- Confidence: High
- Risk: Medium
- Reasoning: Carrier frequency input.
- Impact: multiple tests and any package consumers update.

8. **`DopplerShiftHz(...) -> Float` return (Doppler shift)**
- Recommended type: `Float<Hz>`
- Confidence: High
- Risk: Medium
- Reasoning: Return is explicitly Hz-valued Doppler shift.
- Impact: downstream helper signatures and tests.

9. **`DopplerShiftHzWithPropagationSpeed(carrierFrequencyHz: Float, ...)` input/return**
- Recommended type: `carrierFrequencyHz: Float<Hz>`, return `Float<Hz>`
- Confidence: High
- Risk: Medium
- Impact: helper and callers migrate in lock-step.

10. **`MaxDopplerShiftHz(carrierFrequencyHz: Float, ...) -> Float`**
- Recommended type: carrier `Float<Hz>`, return `Float<Hz>`
- Confidence: High
- Risk: Medium
- Reasoning: Maximum Doppler magnitude is Hz.

11. **`CoherenceTimeSecondsFromMaxDopplerJakes(maxDopplerHz: Float)`**
- Recommended type: `maxDopplerHz: Float<Hz>`
- Confidence: High
- Risk: Medium
- Reasoning: input is physically maximum Doppler frequency.

12. **`CoherenceTimeSecondsFromMaxDopplerHalfCycle(maxDopplerHz: Float)`**
- Recommended type: `Float<Hz>`
- Confidence: High
- Risk: Medium
- Reasoning: same.

13. **`CoherenceBandwidthHzFromRmsDelaySpreadWideSense(...) -> Float`**
- Recommended type: return `Float<Hz>`
- Confidence: High
- Risk: Medium
- Reasoning: output is coherence bandwidth in Hz.

14. **`CoherenceBandwidthHzFromRmsDelaySpreadStrict(...) -> Float`**
- Recommended type: return `Float<Hz>`
- Confidence: High
- Risk: Medium
- Reasoning: same.

15. **`DopplerPhaseRadians(dopplerShiftHz: Float, ...)` `dopplerShiftHz`**
- Recommended type: `Float<Hz>`
- Confidence: High
- Risk: Medium
- Reasoning: phase progression uses Hz·s.

### RF.SParameters (`Libraries/RF/RF.SParameters.oct`)

16. **`SParameters2Port.Frequencies: Float[]`**
- Recommended type: `Float<Hz>[]`
- Confidence: High
- Risk: High
- Reasoning: frequency axis for measured network data is physical frequency.
- Impact: broad API surface + many call sites/tests.

17. **`InterpolateTraceAtFrequency(... frequencies: Float[], f: Float)`**
- Recommended type: `frequencies: Float<Hz>[]`, `f: Float<Hz>`
- Confidence: High
- Risk: High
- Reasoning: interpolation index/axis are physical frequencies.
- Impact: helper and all wrapper interpolation APIs.

18. **`InterpolateS11/S21/S12/S22(..., f: Float)`**
- Recommended type: `f: Float<Hz>`
- Confidence: High
- Risk: High
- Reasoning: public query frequency.
- Impact: public API + tests.

### RF dimensionless keep list (in-scope)

19. **`MagnitudeDb`, `ReturnLossDbFromS11`, `InsertionLossDbFromS21` returns**
- Keep: dimensionless `Float` (dB)
- Confidence: High
- Risk: Low
- Reasoning: dB quantities are logarithmic ratios, not physical base units in current type system.

## 3) Wireless findings

### Wireless.Core (`Libraries/Wireless/Wireless.Core.oct`)

20. **`Band.CenterFreqHz`, `ChannelWidthHz`, `MaxChannelWidthHz`**
- Current: `Float`
- Recommended: `Float<Hz>`
- Confidence: High
- Risk: High
- Reasoning: public record fields, explicit Hz semantics.
- Impact: all record constructors/accessors/tests.

21. **`LinkBudget.FreqHz`, `BandwidthHz`**
- Recommended: `Float<Hz>`
- Confidence: High
- Risk: High
- Reasoning: core public link budget contract.

22. **`MloConfig.Link1FreqHz`, `Link2FreqHz`, `Link1WidthHz`, `Link2WidthHz`**
- Recommended: `Float<Hz>`
- Confidence: High
- Risk: High
- Reasoning: public multi-link config fields.

23. **`ThroughputEstimate.ChannelWidthHz`**
- Recommended: `Float<Hz>`
- Confidence: High
- Risk: High
- Reasoning: record field persists channel width.

24. **`OfdmaAllocation.ToneSpacingHz`**
- Recommended: `Float<Hz>`
- Confidence: High
- Risk: High
- Reasoning: subcarrier/tone spacing is a frequency interval.

25. **`FreeSpacePathLossDb(..., freqHz: Float)`**
- Recommended: `freqHz: Float<Hz>`
- Confidence: High
- Risk: Medium
- Reasoning: explicit free-space frequency term.

26. **`ThermalNoiseFloorDbm(bandwidthHz: Float)` and `EstimateThroughput(... channelWidthHz: Float, ...)`**
- Recommended: `bandwidthHz: Float<Hz>`, `channelWidthHz: Float<Hz>`
- Confidence: High
- Risk: Medium
- Reasoning: direct use as bandwidth in noise/throughput models.

### Wireless dimensionless keep list (in-scope)

- `SpectralEfficiencyBitsPerHz`: keep dimensionless `Float` (units are bits/s/Hz; represented as scalar efficiency multiplier in current model).
- `MimoEfficiency`, `OverheadFraction`: keep dimensionless `Float`.
- dB/dBm fields and params (`TxGainDb`, `RxGainDb`, `NoiseFigureDb`, `PathLossDb`, `SnrDb`, etc.): keep dimensionless `Float`.

### Wireless unclear case

- **`AllocateOfdma` local `toneSpacingHz` + derived `symbolDurS`**
  - Classification: **unclear / needs design decision** between:
    - keep as `Float<Hz>` because concept is subcarrier spacing in Hz, or
    - express as `Float<s^-1>` if the API wants generic inverse-time notation for rate-like spacing.
  - Current recommendation trend: prefer `Float<Hz>` for externally visible OFDM spacing (per policy), but the repo should confirm if OFDM internal rate conventions should standardize on `Hz` or generic `s^-1` for future DSP APIs.

## 4) Safe migration set (for M5b)

High-confidence, low/medium-risk targets:
- `RF.Noise`: all `bandwidthHz` scalar/series params to `Float<Hz>`.
- `RF.PathLoss`: `frequencyHz` scalar/series params to `Float<Hz>`.
- `RF.Doppler`: carrier/max-doppler/doppler-shift params and returns to `Float<Hz>`; coherence-bandwidth returns to `Float<Hz>`.
- `Wireless.Core` non-record function params:
  - `FreeSpacePathLossDb(... freqHz ...)`
  - `ThermalNoiseFloorDbm(bandwidthHz ...)`
  - `EstimateThroughput(... channelWidthHz ...)`

## 5) High-risk/public API migration set

Changes with broad surface/call-site impact:
- `Wireless.Core` public record field migrations:
  - `Band`, `LinkBudget`, `MloConfig`, `ThroughputEstimate`, `OfdmaAllocation` Hz fields.
- `RF.SParameters` public record and interpolation signatures:
  - `SParameters2Port.Frequencies`
  - interpolation query frequencies (`f`) and helper axis arrays.

## 6) Dimensionless keep list

Keep plain `Float` (intentional non-Hz):
- dB/dBm/dBi/log-domain quantities (`*Db`, `*Dbm` fields/functions).
- `noiseFigureLinear`, `pathLossExponent`.
- `SpectralEfficiencyBitsPerHz` and output of `SpectralEfficiencyFromSnrDb`.
- `MimoEfficiency`, `OverheadFraction`, link margin scalar arithmetic.

## 7) Naming cleanup recommendations

1. If any future symbol is normalized frequency (cycles/sample), do **not** suffix with `Hz`.
2. Keep `*Hz` suffix for physical-frequency concepts that migrate to `Float<Hz>`.
3. Optional future cleanup for readability (after type migration):
   - `FreqHz` -> `Frequency`
   - `ChannelWidthHz` -> `ChannelWidth`
   - `ToneSpacingHz` -> `ToneSpacing`
   where the unit is already encoded in type.

## 8) Proposed implementation milestones

- **M5b — RF typed-frequency low-risk function migration**
  - Migrate RF.Noise, RF.PathLoss, RF.Doppler function signatures/returns and tests.
- **M5c — Wireless function + record API typed-frequency migration**
  - Migrate Wireless.Core signatures, then records and call-sites.
- **M5d — RF S-parameters frequency-axis migration**
  - Migrate `SParameters2Port.Frequencies` + interpolation APIs; update test fixtures.
- **M5e — Docs/examples and naming cleanup**
  - Remove legacy compatibility comments, tighten naming where needed, ensure reference-consistent unit guidance.

## Inconsistencies explicitly surfaced

- Existing RF/Wireless APIs currently encode Hz semantics mostly in names/comments while using plain `Float`; this conflicts with reference-supported `Float<Hz>`/`Float<s^-1>` capability.
- dB/dBm are widely used but not represented as dedicated unit-typed aliases in `Language/reference`; this is a documentation/modeling gap to keep tracking.


## 10) M5b implementation status (completed)

Date: 2026-04-30

- Tests updated first as oracle in:
  - `Libraries/RF/RF.Noise.octest`
  - `Libraries/RF/RF.PathLoss.octest`
  - `Libraries/RF/RF.Doppler.octest`
- APIs migrated (low/medium-risk RF function set):
  - `RF.Noise`: bandwidth inputs to `Float<Hz>` / `Float<Hz>[]`, temperature inputs to `Float<K>` / `Float<K>[]` for dimensional consistency with existing `K` usage.
  - `RF.PathLoss`: free-space carrier-frequency inputs to `Float<Hz>` (scalar + series), speed-of-light helper typed as `Float<m/s>`.
  - `RF.Doppler`: carrier/max-doppler/doppler-shift frequency inputs and outputs migrated to `Float<Hz>`; coherence-bandwidth returns migrated to `Float<Hz>`; coherence-time max-Doppler inputs migrated to `Float<Hz>`.
- Formulas preserved: numeric relationships/constants unchanged (kTB, Friis geometry, Doppler linear relation, Jakes/half-cycle coherence-time approximations, and coherence-bandwidth approximations).
- Intentionally unchanged dimensionless quantities: dB/dBm, linear noise figure, path-loss exponents, scalar power/loss ratios remain `Float`.
- Deferred by scope: Wireless records and RF.SParameters were intentionally not migrated in this milestone.
- Validation executed:
  - `go test ./...`
  - `go run ./cmd/oct test Libraries/RF`
  - `go run ./cmd/oct test Libraries`
  - `go run ./cmd/oct test Language/Types/UnitsM1`
  - `go run ./cmd/oct test Libraries/Wireless`

Reference consistency note surfaced:
- This migration uses `Hz` typed literals/arrays and signed-exponent-compatible unit behavior per `Language/reference/language/08-units.md` (`Hz = s^-1`).

## M5c execution update (RF.SParameters typed-frequency migration)

Date: 2026-04-30

1. Tests updated first
- Updated `Libraries/RF/RF.SParameters.octest` to use typed frequency axis arrays (`Float<Hz>[]`) and typed query frequencies (`Float<Hz>`) before changing implementation.
- Added compatibility assertions that bind `data.Frequencies` to `Float<Hz>[]` and interpolation query to `Float<Hz>`.

2. Record fields migrated
- Migrated `SParameters2Port.Frequencies` from `Float[]` to `Float<Hz>[]` in `Libraries/RF/RF.SParameters.oct`.

3. Interpolation helpers migrated
- Migrated `InterpolateTraceAtFrequency(... frequencies, f)` to `frequencies: Float<Hz>[]` and `f: Float<Hz>`.
- Migrated public wrappers `InterpolateS11`, `InterpolateS21`, `InterpolateS12`, and `InterpolateS22` query argument `f` to `Float<Hz>`.

4. Interpolation semantics preserved
- Preserved range checks, exact-knot behavior, and piecewise linear interpolation over real/imaginary components.
- Ratio expression `(f - f0) / (f1 - f0)` remains dimensionless by unit cancellation; no algorithmic changes were introduced.

5. Intentionally unchanged dimensionless quantities
- `S11/S21/S12/S22` trace values remain `Complex` (dimensionless/network-ratio representation).
- `MagnitudeDb`, `ReturnLossDbFromS11`, and `InsertionLossDbFromS21` remain dimensionless/log-domain `Float` outputs.

6. Limitations/deferred items
- Wireless typed-frequency migration remains deferred by design for this milestone (M5c scope is RF.SParameters only).
- No field renaming performed (kept `Frequencies` as required).

7. Validation results
- Executed required validation matrix for Go tests and Oct test suites, including RF, Libraries-wide, UnitsM1, and Wireless non-regression.
