# M23 generated-Go hardening delta

Date: 2026-06-03
Scope: targeted generated-Go/compiler-codegen hardening after the M22 compiled standard-library inventory.

## Summary

M23 made focused generated-Go fixes without changing Octxiliary transport, wrapper protocols, Complex support, Einstein/tensor support, callback/function-value lowering, or public standard-library APIs.

Fixed or improved M22 categories:

- `M22-F001`: Oct identifiers that collide with Go keywords are now rendered through generated-Go-safe names at the codegen boundary. This fixes the Analysis normalization helpers whose local variable named `range` previously produced malformed Go syntax.
- `M22-F002`: generated Go emission now treats non-final MIR blocks with nil terminators as fallthrough jumps to the following block instead of failing before code generation. The Analysis local-maxima path now reaches execution; its remaining failure is a zero-assertion/runtime-test-shape issue rather than the previous unsupported MIR terminator.
- `M22-F005`: generated result type names now sanitize dimension exponent and operator characters such as `^`, preventing raw Oct dimension syntax from leaking into Go type declarations. Geometry and Thermofluids dimensioned helpers now compile green.
- `M22-F008`: generated imports are pruned after source emission, removing stale conservative imports such as unused `math`. Octomata now compiles green.
- Related dimensioned scalar hardening: Int literals passed to Float/dimensioned-Float user-function parameters are lowered with explicit `float64(...)` argument coercions only after the typechecker has accepted the call.

## Commands run

Sidecars were built into `.tmp/m23-wrappers` with the M22-style `go build` matrix for IO, Hash, Compression, Time, Text, Archive, Json, Csv, Plot, Xlsx, Image, and Pdf wrappers.

Focused compiled checks after fixes:

| Command | Result |
| --- | --- |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m23-wrappers go run ./cmd/oct test Libraries/Analysis --execution compiled` | 35 passed, 1 failed; improved from 31 passed, 5 failed. Remaining failure: `LocalMaximaDoesNotIncludeEndpoints` exits with zero assertions. |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m23-wrappers go run ./cmd/oct test Libraries/Geometry --execution compiled` | 9 passed, 0 failed; improved from 1 passed, 8 failed. |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m23-wrappers go run ./cmd/oct test Libraries/Octomata --execution compiled` | 92 passed, 0 failed; improved from 80 passed, 12 failed. |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m23-wrappers go run ./cmd/oct test Libraries/Interpolation --execution compiled` | 20 passed, 8 failed; remaining spline failures are array/shape lowering issues. |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m23-wrappers go run ./cmd/oct test Libraries/Thermofluids --execution compiled` | 11 passed, 0 failed. |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m23-wrappers go run ./cmd/oct test Libraries/LinearAlgebra --execution compiled` | 35 passed, 7 failed; remaining failures are loop-variable int/float shape mismatches in eigen helpers. |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m23-wrappers go run ./cmd/oct test Libraries/Random --execution compiled` | 21 passed, 1 failed; remaining failure not addressed in M23 hardening subset. |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m23-wrappers go run ./cmd/oct test Libraries/Statistics --execution compiled` | 4 passed, 31 failed; remaining failures include unresolved numeric shape/coercion issues such as `sum / n` where `sum` is Float and `n` is Int. |

## Failures fixed

- Analysis normalization generated-Go syntax failures from a local `range` binding are fixed.
- Geometry and Thermofluids dimensioned return/result type syntax failures are fixed.
- Octomata stale `math` import failures are fixed.
- Fallthrough-like nil MIR terminators no longer stop code generation before a real compiled run.

## Deferred or still open

- `M22-F003` Complex support remains deferred.
- Einstein notation and tensor support remain deferred.
- `M22-F004` callback/function-value lowering remains deferred.
- `M22-F006` matrix/list/array shape mismatches are only partially improved; LinearAlgebra eigen and Interpolation spline generated-Go shape errors remain open.
- `M22-F007` broad empty-literal and `_` placeholder hardening remains open except where import pruning/identifier hardening made affected packages compile.
- `M22-F009`, `M22-F010`, `M22-F012`, and `M22-F013` remain deferred as wrapper/API, PDF image interop, live UI bridge, and manifest/dependency work.
- `M22-F014` runtime/index failures were not expanded into broad runtime semantics work.

## Recommended next milestone

**M24 — generated numeric/shape lowering pass.**

Suggested focus:

1. Loop variable declared-type preservation when an existing typed local is reused as a `for` induction variable.
2. Float/Int arithmetic coercion rules in generated Go where the typechecker already accepts mixed numeric expressions.
3. Matrix/vector/list expected-shape propagation for Interpolation spline and LinearAlgebra eigen helpers.
4. Empty literal expected-type propagation and discard placeholder emission once the numeric/shape blockers are narrower.

Keep Complex, Einstein/tensor notation, broad callback/function-value lowering, new wrapper migrations, new Octxiliary transports, and public API redesign out of M24 unless explicitly reprioritized.
