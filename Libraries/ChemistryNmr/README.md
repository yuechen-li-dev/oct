# ChemistryNmr

## Shelf boundary

`ChemistryNmr` owns NMR frequency, chemical-shift, peak, multiplicity, spectrum, and spectrum-analysis operations. [`Chemistry`](../Chemistry/README.md) owns general solution chemistry and kinetics; [`Units.Spectroscopy`](../Units/README.md) owns reusable ppm/wavenumber conversion records. Start with `ChemistryNmr.Core.octest`, then the spectrum constructors in `ChemistryNmr.Spectrum.octest`.

NMR spectroscopy library for Oct. 29 tests, all compiled, zero interpreted fallback.

## Packages

- `ChemistryNmr.Core` — constants, Larmor frequency, chemical shift arithmetic
- `ChemistryNmr.Spectrum` — Peak/Spectrum records, multiplicity constructors, analysis functions

## Dimensional coverage

| Quantity | Oct type | Notes |
|---|---|---|
| Larmor frequency | `Float<Hz>` | = `Float<s^-1>` ✓ |
| Coupling constant (J) | `Float<Hz>` | Dimensioned, distinct from shift ✓ |
| Gyromagnetic ratio (γ) | `Float<A*s*kg^-1>` | = rad/(s·T) with rad dimensionless ✓ |
| Magnetic field (B₀) | `Float<kg*s^-2*A^-1>` | Tesla ✓ |
| Chemical shift (δ) | `Float` | ppm is dimensionless — only gap |
| Integration | `Float` | Dimensionless ratio ✓ |

Larmor equation is dimensionally verified at compile time:
`Float<A*s*kg^-1> × Float<kg*s^-2*A^-1> = Float<s^-1> = Float<Hz> ✓`

## Known friction

Chemical shift (δ, ppm) cannot be `Float<ppm>` because ppm is a dimensionless ratio (10⁻⁶),
not an SI unit. This is correct — Oct's type system is right to not have it. Coupling
constants (Hz) are dimensioned and enforced. You cannot confuse a coupling constant with
a spectrometer frequency. You can confuse a chemical shift with any other bare Float.
