# M27b DifferentialEquations compiled Int/Float step coercion delta

Date: 2026-06-04

## Baseline from M27a

M27a fixed the callback blocker for the ODE helper surface: named top-level derivative functions can be passed to `fn(Float, Float) -> Float` parameters, and callback parameters can be invoked from compiled functions.

The M27a DifferentialEquations compiled baseline was:

| Package | M27a result | Remaining diagnostic category |
| --- | ---: | --- |
| `Libraries/DifferentialEquations --execution compiled` | 2 passed / 4 failed | generated-Go `Int`/`Float` coercion around `steps: Int` at solve call sites, not unknown callback identifiers. |

The representative failure shape was a single Oct `steps: Int` binding being used in Float arithmetic and then later passed to an `Int`-typed helper/solver argument. The generated Go must use `float64(steps)` only at the Float expression site and must keep `steps` as an `int` when calling an `Int` parameter.

## Reproduction result on the M27b branch

The requested reproduction commands were run after building the wrapper sidecars into `.tmp/m27b-wrappers`:

```sh
OCT_WRAPPER_PATH=$PWD/.tmp/m27b-wrappers go run ./cmd/oct test Libraries/DifferentialEquations --execution compiled
go run ./cmd/oct test Libraries/DifferentialEquations --execution interpreted
```

Current M27b branch result:

| Execution mode | Result |
| --- | ---: |
| compiled | 6 passed / 0 failed |
| interpreted | 6 passed / 0 failed |

No current DifferentialEquations test emits generated-Go errors. The previously described failures are therefore fixed rather than reclassified: there are no remaining helper-call-site or helper-body generated-Go failures for `EulerSolve`, `RK4Solve`, or `ValidateSolveInputs` in the focused package run.

## Root cause and fix shape

The failure class was an expression-local coercion leak. `steps` is a binding whose real compiled type is `Int`; using it in a Float expression requires a temporary expression wrapper such as `float64(steps)`, but that wrapper must not replace the binding's stored type/name or be reused for later `Int` arguments.

The fixed lowering shape is:

- identifier expressions read the local/parameter binding's real type and stable Go name;
- `coerceExprToType` and `goCoerceArg` return a wrapped expression string only for the current expression or argument;
- local and parameter bindings are not permanently retyped by expected-type context;
- user-function and callback-function calls coerce arguments at the call site without mutating the source expression or binding metadata.

A focused internal/build regression now compiles and runs a solver-like program where `steps: Int` is:

1. explicitly converted for Float arithmetic;
2. multiplied in a Float context before a callback-shaped helper call;
3. later passed to helpers expecting `Int`.

The regression also inspects the MIR dump and rejects the old bad shape, such as `UseSteps(float64(steps))` or callback-helper calls with `float64(steps)` for an `Int` parameter.

## Focused package results after M27b

| Package / fixture | Result after M27b | Notes |
| --- | ---: | --- |
| `Libraries/DifferentialEquations --execution compiled` | 6 passed / 0 failed | DifferentialEquations is compiled-green in the focused run. |
| `Libraries/DifferentialEquations --execution interpreted` | 6 passed / 0 failed | Interpreted parity remains green. |
| `Language/Testing/CompiledCallbacks/valid --execution compiled` | 4 passed / 0 failed | M27a callback behavior remains green. |
| `Language/Testing/CompiledCallbacks/valid --execution interpreted` | 4 passed / 0 failed | Interpreted callback fixture remains green. |
| `Libraries/Mathematics --execution compiled` | 21 passed / 0 failed | M27a callback-green package remains green. |
| `Libraries/Numerics --execution compiled` | 6 passed / 0 failed | M27a callback-green package remains green. |
| `Libraries/Optimization --execution compiled` | 7 passed / 0 failed | M27a callback-green package remains green. |

## Remaining deferred categories

M27b intentionally does not broaden any of the deferred M27a/M26 categories:

- anonymous lambdas, closures, partial application, returned function values, aggregate-stored function values, and fallible callbacks;
- Einstein/tensor notation;
- new Complex features beyond the current scalar Complex surface;
- wrapper migrations, Octxiliary protocol or transport changes, Pdf image interop, UI live/native bridge work, or package-manager sidecar lifecycle changes;
- DifferentialEquations public API redesign.

## Recommended next milestone

Proceed to the next compiled-stdlib inventory milestone outside DifferentialEquations. The best next target is another focused package or package cluster whose remaining failures are generated-Go lowering issues rather than broad deferred features, while continuing to keep callback support intentionally narrow until a real standard-library case requires a wider function-value surface.
