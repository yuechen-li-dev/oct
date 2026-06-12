# Uncertainty

Linear (first-order, "delta method") propagation of measurement
uncertainty through arithmetic and common transcendental functions —
the kind of thing Python's third-party `uncertainties` package provides,
but not in the stdlib, and here as explicit named functions instead of
operator overloading.

## Core type

```oct
record Measurement {
    Value: Float
    Uncertainty: Float   // 1-sigma standard uncertainty
}
```

All operations assume `Uncertainty` values are independent and
approximately Gaussian, and propagate them via:

- addition/subtraction: uncertainties add in quadrature
- multiplication/division: relative uncertainties add in quadrature
- `f(x)`: `unc(f(x)) ~= |f'(x)| * unc(x)`

## Construction

- `Exact(value)` — a known constant, `Uncertainty: 0.0`
- `New(value, uncertainty)` — validates `uncertainty >= 0`
- `FromSamples(xs)` — sample mean +/- standard error of the mean
  (`SampleStandardDeviation(xs) / sqrt(len(xs))`), requires `len(xs) >= 2`

## Arithmetic

`Add`, `Sub`, `Neg`, `Mul`, `Scale(a, k)` (exact scalar), `DivBy`
(named to avoid colliding with the builtin `Div`), `PowInt(a, n)` for
integer exponents.

## Transcendentals

`UncertaintySqrt`, `UncertaintyExp`, `UncertaintyLn`, `UncertaintySin`,
`UncertaintyCos` — prefixed to avoid colliding with the builtin scalar
math functions of the same name.

## Reporting and combination

- `RelativeUncertainty(a)` — `unc(a) / |a.Value|`
- `ContainsWithin(a, x, k)` — whether `x` is within `k` standard
  uncertainties of `a.Value`
- `Combine(measurements)` — inverse-variance-weighted combination of
  independent measurements of the same quantity
- `ToDisplayString(a)` — `"value +/- uncertainty"`

## Example

```oct
import Uncertainty

fn Main() -> Int ! Error {
    // length = 12.50 +/- 0.05 cm, width = 8.20 +/- 0.03 cm
    let length = Uncertainty.Exact(12.50)
    let lengthM = Assert.LGTM(Uncertainty.New(12.50, 0.05), "length")
    let widthM = Assert.LGTM(Uncertainty.New(8.20, 0.03), "width")

    let area = Uncertainty.Mul(lengthM, widthM)
    Print(Uncertainty.ToDisplayString(area))  // ~ "102.5 +/- 0.5915..."
    return 0
}
```

## Not covered (v1)

Correlated-error propagation (covariance-aware combination of
non-independent quantities) is intentionally out of scope for v1. A
natural follow-up would add a `CovarianceMatrix`-based variant that
takes correlation into account when combining or transforming
measurements, building on `LinearAlgebra` (including `JacobiSVD`).
