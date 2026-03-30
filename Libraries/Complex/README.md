# Complex Library (M0b)

`Libraries/Complex` is an extension layer for advanced complex-valued functions.

## Scope

Complex remains a core language type (`Complex`, `I`, `Real`, `Imag`, `Conj`, `Abs`, `Arg`, `ComplexPolar`, `Exp`, `Ln`).

This package adds explicit function names for practical engineering use:

- `ComplexSin(z: Complex) -> Complex`
- `ComplexCos(z: Complex) -> Complex`
- `ComplexTan(z: Complex) -> Complex ! Error`
- `ComplexSinh(z: Complex) -> Complex`
- `ComplexCosh(z: Complex) -> Complex`
- `ComplexTanh(z: Complex) -> Complex ! Error`

## Determinism and singularities

- `ComplexTan` computes `ComplexSin(z) / ComplexCos(z)`.
- `ComplexTan` returns `Error` when `Abs(ComplexCos(z)) <= 1e-12`.
- `ComplexTanh` returns `Error` when `Abs(ComplexCosh(z)) <= 1e-12`.

## Type stance

M0b is intentionally conservative: all functions accept `Complex` and return `Complex` (or `Complex ! Error` for singular quotient cases).
No dimension-aware complex trig is included.

## Non-goals

No inverse complex trig, no branch-cut policy controls, no special-function families, and no symbolic simplification.
