# Library Modernization M7c — Full High-Confidence `Require` Sweep

## Search patterns used
- `rg -n "return error\(" Libraries -g '*.oct'`
- `rg -n "-> .* ! Error" Libraries/Optimization Libraries/Cooking Libraries/Octomata -g '*.oct'`
- `rg -n "\?" Libraries/Optimization Libraries/Cooking Libraries/Octomata -g '*.oct'`

## Packages/files changed
- `Libraries/Optimization/Optimization.Core.oct`
- `Libraries/Cooking/Cooking.Core.oct`
- `Libraries/Octomata/Octomata.AntiWindup.oct`

## Functions converted to `Require`

### Optimization
- `ValidateGoldenSectionInputs`
- `ValidateStepSize`
- `ValidateGradientDescentInputs`

### Cooking
- `CelsiusToGasMark`
- `BakersPercentage`
- `FromBakersPercentage`
- `TotalDoughWeight`
- `ScaleIngredient`
- `ScaleLeavening`
- `ScaleSalt`
- `PanAreaScaleFactor`

### Octomata
- `ClampedPIDUpdate`
- `Clamp`
- `Deadband`
- `RateLimit`

## Signatures changed
- Optimization:
  - `GoldenSectionSearch` now non-fallible.
  - `GradientDescentStep` now non-fallible.
  - `GradientDescentSolve` now non-fallible.
  - `ValidateGoldenSectionInputs` now non-fallible.
  - `ValidateStepSize` now non-fallible.
  - `ValidateGradientDescentInputs` now non-fallible.
- Cooking:
  - `CelsiusToGasMark` now non-fallible.
  - `BakersPercentage` now non-fallible.
  - `FromBakersPercentage` now non-fallible.
  - `TotalDoughWeight` now non-fallible.
  - `ScaleIngredient` now non-fallible.
  - `ScaleLeavening` now non-fallible.
  - `ScaleSalt` now non-fallible.
  - `PanAreaScaleFactor` now non-fallible.
- Octomata:
  - `ClampedPIDUpdate` now non-fallible.
  - `Clamp` now non-fallible.
  - `Deadband` now non-fallible.
  - `RateLimit` now non-fallible.

## Call sites updated
- Optimization internal call sites updated to remove `?` from:
  - `ValidateGoldenSectionInputs`
  - `ValidateStepSize`
  - `ValidateGradientDescentInputs`
  - `GradientDescentStep`

## Cases intentionally left fallible
- Cooking lookup/data-domain APIs:
  - `IsSafeTemperature` unknown-protein path.
  - `GramsPerCup` unknown-ingredient path.
  - `CupsToGrams` remains fallible because it depends on fallible ingredient lookup.
- Broader deferred classes from M7 audit remain unchanged here (e.g. IO/parse/external validation, convergence/unsupported capability, singular-domain runtime failures).

## Ambiguous cases deferred
- Validator-style public APIs and domain-runtime failures outside the high-confidence precondition bucket were deferred.
- This pass intentionally did not mass-convert crypto helper invalid-argument paths in `Libraries/Random`.

## Validation results
- `go test ./...`
- `go run ./cmd/oct test Libraries`
- `go run ./cmd/oct test Libraries/Optimization`
- `go run ./cmd/oct test Libraries/Cooking`
- `go run ./cmd/oct test Libraries/Octomata`


## M7c follow-up: Cooking targeted failure triage (2026-04-30)

### Investigation findings
1. Exact targeted failure output:
   - `test failed: manifest function returned invalid package metadata`
2. `Libraries/Cooking/manifest.oct` was **not** changed in the prior M7c Require migration commit (`9ece124`).
3. `go run ./cmd/oct test Libraries` does not execute Cooking package `.octest` files in the observed run; it only ran a small smoke subset (Random/Statistics/UI fail-suite entries).
4. Cooking manifest metadata shape was valid structurally, but invalid semantically: `Name` was set to `"Optimization"` instead of `"Cooking"`.
5. Other manifests use package-consistent `Name` fields (for example `Interpolation`, `Distributions`, `LinearAlgebra`, `Thermofluids`).

### Root cause
- Pre-existing Cooking manifest package name mismatch (`Name: "Optimization"`) caused targeted package metadata validation to fail.

### Fix applied
- Updated `Libraries/Cooking/manifest.oct` to set `Name: "Cooking"`.

### Scope note
- No additional Require migrations or Cooking algorithm/API behavior changes were made in this follow-up.
