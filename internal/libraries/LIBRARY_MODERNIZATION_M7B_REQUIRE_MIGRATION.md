# Library Modernization M7b — High-Confidence `Require` Migration

## Scope completed

This pass migrated high-confidence precondition-style `return error(...)` checks in `Libraries/Statistics` to `Require(...)`.

## 1) Functions converted

### `Libraries/Statistics/Statistics.Core.oct`
- `Mean`
- `Variance`
- `StandardDeviation`
- `Median`
- `Percentile`

### `Libraries/Statistics/Statistics.Regression.oct`
- `Covariance`
- `SampleCovariance`
- `SampleVariance`
- `SampleStandardDeviation`
- `Correlation` (only argument shape/cardinality preconditions)
- `LinearRegression` (only argument shape/cardinality preconditions)

### `Libraries/Statistics/Statistics.Summary.oct`
- `Summarize`
- `MinOf`
- `MaxOf`
- `RangeOf`
- `IQR`
- `ZScores` (empty-input precondition only)

## 2) Signatures changed

Converted from fallible to non-fallible where only precondition failure remained:

- `Mean(xs: Float[]) -> Float ! Error` → `Mean(xs: Float[]) -> Float`
- `Variance(xs: Float[]) -> Float ! Error` → `Variance(xs: Float[]) -> Float`
- `StandardDeviation(xs: Float[]) -> Float ! Error` → `StandardDeviation(xs: Float[]) -> Float`
- `Median(xs: Float[]) -> Float ! Error` → `Median(xs: Float[]) -> Float`
- `Percentile(xs: Float[], p: Float) -> Float ! Error` → `Percentile(xs: Float[], p: Float) -> Float`
- `Covariance(xs: Float[], ys: Float[]) -> Float ! Error` → `Covariance(xs: Float[], ys: Float[]) -> Float`
- `SampleCovariance(xs: Float[], ys: Float[]) -> Float ! Error` → `SampleCovariance(xs: Float[], ys: Float[]) -> Float`
- `SampleVariance(xs: Float[]) -> Float ! Error` → `SampleVariance(xs: Float[]) -> Float`
- `SampleStandardDeviation(xs: Float[]) -> Float ! Error` → `SampleStandardDeviation(xs: Float[]) -> Float`
- `Summarize(xs: Float[]) -> StatSummary ! Error` → `Summarize(xs: Float[]) -> StatSummary`
- `MinOf(xs: Float[]) -> Float ! Error` → `MinOf(xs: Float[]) -> Float`
- `MaxOf(xs: Float[]) -> Float ! Error` → `MaxOf(xs: Float[]) -> Float`
- `RangeOf(xs: Float[]) -> Float ! Error` → `RangeOf(xs: Float[]) -> Float`
- `IQR(xs: Float[]) -> Float ! Error` → `IQR(xs: Float[]) -> Float`

## 3) Call sites/tests updated

- Removed obsolete fallible operators (`?` / `!`) for newly non-fallible functions.
- Kept fallible handling where behavior remains recoverable (e.g., constant-sequence correlation and z-score paths).
- Updated statistics tests to match new signatures and handling.

## 4) Intentionally left fallible

- `Correlation`: kept fallible for `non-constant sequences` domain condition.
- `LinearRegression`: kept fallible for `non-constant x values` domain condition.
- `ZScores`: kept fallible for `non-constant sequence` domain condition.

## 5) Ambiguous/deferred

Deferred policy cases were not mass-converted in this pass (consistent with M7 audit guidance).

## 6) Validation results

- `go test ./...` passed.
- `go run ./cmd/oct test Libraries/Statistics` passed.
