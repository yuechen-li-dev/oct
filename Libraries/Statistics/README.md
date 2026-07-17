# Statistics

## Surface

`Libraries/Statistics` provides descriptive statistics over finite `Float[]` collections:

- `Mean(xs: Float[]) -> Float`
- `Variance(xs: Float[]) -> Float`
- `StandardDeviation(xs: Float[]) -> Float`
- `Median(xs: Float[]) -> Float`
- `Percentile(xs: Float[], p: Float) -> Float`

## Definitions

- `Variance` uses the **population** definition: `sum((x - mean)^2) / n`.
- `StandardDeviation` is `Sqrt(Variance(xs))`.
- `Median` sorts ascending; for even length it returns the average of the two middle values.
- `Percentile` accepts `p` in `[0, 100]` and uses linear interpolation on rank `r = (p/100) * (n-1)`.

## Edge-case policy

- Empty input is rejected with `Error` for all functions.
- Single-element input is valid:
  - `Mean`, `Median`, and `Percentile` return that value.
  - `Variance` and `StandardDeviation` return `0.0`.

## Dimension behavior

Statistics M0 currently targets `Float[]` (dimensionless scalars) only.
Dimensioned or mixed-dimension arrays are rejected by typing and are covered by invalid tests.
No function silently drops units.

## Bounded utility fitting

`Statistics.UtilityFit.oct` adds a 1-to-32-feature weighted linear fit for
continuous utility targets. Evidence is a validated `UtilityObservations`
record table. Normalization is fit only on identification rows, held-out rows
are evaluated without refitting, and zero-held-out models are never certified.
Models are ordinary immutable records containing ordered names, normalization,
weights, bias, metrics, stability evidence, evidence hashes, and a readable
identity. `ScoreLinearUtility` checks exact name/order equality;
`QuantizeLinearUtilityScore` is the explicit Float-to-Int bridge for the
existing `when utility`. See `docs/OCT_UTILITY_AND_TABLES.md` for the full
contract and exclusions.
