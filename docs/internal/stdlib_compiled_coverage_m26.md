# M26 compiled Complex support delta

_Date: 2026-06-04._

## Baseline before M26 edits

Wrappers were built into `.tmp/m26-wrappers` and the focused compiled package checks were run with `OCT_WRAPPER_PATH=$PWD/.tmp/m26-wrappers`.

| Command | Baseline result | Complex-related failures |
| --- | ---: | --- |
| `go run ./cmd/oct test Libraries/Complex --execution compiled` | 0 passed / 9 failed | All failures were blocked by `function Complex.ComplexSin` or `Complex.ComplexSinh`: `compiled mode does not yet support builtin Real`. |
| `go run ./cmd/oct test Libraries/Mathematics --execution compiled` | 7 passed / 14 failed | FFT/complex tests failed through `NearComplexAt` using `Real`, `Abs` over `Complex`, and one generated-Go `complex128 * float64` mismatch in inverse FFT scaling. The remaining 7 failures were callback/function-value calculus tests (`unknown identifier 'f'`). |
| `go run ./cmd/oct test Libraries/Signal --execution compiled` | 34 passed / 3 failed | `MagnitudeSpectrum*` and `PowerSpectrum*` failed because `MagnitudeSpectrum` used `Abs` for `Complex`. |
| `go run ./cmd/oct test Libraries/RF --execution compiled` | 41 passed / 16 failed | S-parameter tests failed through `Real` and `Abs` over `Complex`; other RF failures were pre-existing generated-Go array/placeholder issues unrelated to Complex. |

## Implemented in M26

- Oct `Complex` continues to lower to Go `complex128`, now with complete scalar builtin support for the current interpreted/typechecked Complex surface used by the standard libraries.
- Compiled builtin lowering now covers:
  - `I() -> Complex`
  - `Complex(real, imag) -> Complex`
  - `ComplexPolar(r, theta) -> Complex`
  - `Real(z)`, `Imag(z)`, `Arg(z) -> Float`
  - `Conj(z) -> Complex`
  - `Abs(z: Complex) -> Float`
  - existing real `Exp`/`Ln` plus Complex overloads via `math/cmplx`
- Compiled scalar arithmetic now coerces `Int`/`Float` operands to `complex128` when the typechecker has already selected a Complex arithmetic result. This fixes `Complex * Float`, `Float * Complex`, and related `+`, `-`, `*`, `/` generated-Go type mismatches without adding a generic numeric tower.
- `Real` and `Imag` lower through generated helper functions instead of direct Go builtin calls so Oct locals named `real` or `imag` cannot shadow Go's predeclared `real`/`imag` functions inside generated functions.
- Focused compiler coverage was added for Complex constructor/component helpers, polar conversion, conjugate, `Abs`, `Arg`, `I`, `Exp`/`Ln`, arithmetic, numeric-to-Complex coercion, and local `real`/`imag` shadowing.

## Post-M26 focused compiled status

| Command | M26 result | Notes |
| --- | ---: | --- |
| `go run ./cmd/oct test Libraries/Complex --execution compiled` | 9 passed / 0 failed | `Libraries/Complex` is compiled-green. |
| `go run ./cmd/oct test Libraries/Mathematics --execution compiled` | 14 passed / 7 failed | All FFT/Complex failures are fixed. Remaining failures are callback/function-value calculus cases (`unknown identifier 'f'`), which are outside M26. |
| `go run ./cmd/oct test Libraries/Signal --execution compiled` | 37 passed / 0 failed | `Libraries/Signal` is compiled-green in the focused run. |
| `go run ./cmd/oct test Libraries/RF/RF.SParameters.octest --execution compiled` | 7 passed / 0 failed | The Complex-dependent S-parameter surface is compiled-green. |
| `go run ./cmd/oct test Libraries/RF --execution compiled` | 45 passed / 12 failed | All S-parameter Complex failures are fixed. Remaining failures are non-Complex generated-Go placeholder/array coercion issues. |

## Deferred work

M26 intentionally did not implement Einstein/tensor notation, broad callback/function-value lowering, new wrappers, new Octxiliary transports, Octxiliary protocol changes, arbitrary generic algebra, FFT-specific compiler magic, PDF image interop, live UI/native bridges, package-manager sidecar lifecycle, or public Complex API redesign.

Remaining Complex-adjacent blockers after M26 are not scalar Complex support blockers:

1. `Libraries/Mathematics` calculus tests still require callback/function-value lowering.
2. Full `Libraries/RF` still has generated-Go `_` placeholder emission and `Int[]` to `Float[]` array coercion failures in non-S-parameter tests.

## Recommended next milestone

The next bounded milestone should target either:

1. callback/function-value lowering for the `Libraries/Mathematics` calculus helpers, or
2. generated-Go placeholder/array coercion cleanup for the remaining full `Libraries/RF` failures.

Do not combine those with Einstein/tensor notation or new Octxiliary transport work.
