# Library Modernization After Pow / Units / Random Updates

## 1) Features now available
- `Pow(base, exponent)` for direct power expressions.
- Signed unit exponents (e.g. `s^-1`) and `Hz` alias.
- `Require(condition, message)` for production preconditions.
- Typed empty arrays in explicit typed contexts.
- Random modules use record-result APIs (`Next`, `Value`) instead of tuple-threading.

## 2) Audit patterns searched
- Power workaround patterns: `Exp(Ln(` and `Exp(x * Ln(y))`.
- Frequency-related identifiers (`frequency`, `freq`, `Hz`, `rate`, `sampleRate`).
- Loop/style patterns (`while` counting loops).
- Random API shape and typed empty-array usage in `Libraries/Random`.

## 3) Files/libraries updated
- `Libraries/RF/RF.PathLoss.oct`
- `Libraries/RF/RF.LinkBudget.oct`
- `Libraries/Wireless/Wireless.Core.oct`
- `Libraries/RF/RF.PathLoss.octest`

## 4) Pow replacements
- Replaced `Exp(pathLossExponent * Ln(distanceRatio))` with `Pow(distanceRatio, pathLossExponent)`.
- Replaced `Exp(Ln(10) * valueDb / 10.0)` with `Pow(10.0, valueDb / 10.0)`.
- Replaced `Exp(snrDb / 10.0 * Ln(10.0))` with `Pow(10.0, snrDb / 10.0)`.

## 5) Hz / s^-1 unit updates
- No broad signature conversion to `Float<Hz>`/`Float<s^-1>` was applied in this pass due cascade risk across RF/Wireless call sites.
- Frequency unit modernization is deferred for a dedicated API-signature migration pass.

## 6) Require / precondition updates
- `Libraries/Random` already used `Require` for deterministic invalid-input programmer errors (`FlipCoins`, `RollDie`, `RollDice`).
- No additional deterministic silent-normalization hotspots were changed in this subset.

## 7) Loop/style updates
- No loop-shape changes were applied in this subset; existing loops touched here were already clear range loops.

## 8) Random API updates
- Verified current `Libraries/Random` usage is record-result API based (`draw.Next`, `draw.Value`) and not tuple-threaded.

## 9) Tests / validation
- Added regression test asserting the log-distance path-loss power-law expression using `Pow(...)`.
- Ran required validation commands listed in task request (see terminal log / summary).

## 10) Deferred suspicious patterns
- Dimensionless frequency fields remain in `Libraries/Wireless` and parts of `Libraries/RF` and should be migrated to `Float<Hz>` / `Float<s^-1>` with coordinated call-site updates.
- Additional counting-`while` modernizations outside edited files remain for a broader style sweep.
- Potential log-space-intent expressions were left unchanged unless clearly algebraic power workarounds.

## Inconsistency notes (Language/reference vs existing code)
- Language reference now supports unitized frequency and signed exponents, but multiple RF/Wireless public APIs still expose dimensionless `Float` for Hertz-like values. This is an explicit repository-level inconsistency.
