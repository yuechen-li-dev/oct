# M27a standard-library compiled callback lowering delta

Date: 2026-06-04

## Baseline callback failures

M27a started from the M26 callback inventory. The focused compiled baseline was:

| Package | Baseline result | Representative diagnostic |
| --- | ---: | --- |
| `Libraries/Mathematics` | 14 passed / 7 failed | calculus helpers failed with `unknown identifier 'f'` when invoking `fn(Float) -> Float` parameters. |
| `Libraries/DifferentialEquations` | 0 passed / 6 failed in M22; reproduced callback failures around `DerivativeIdentity` before implementation work | ODE helpers required callback parameters shaped as `fn(Float, Float) -> Float`. |
| `Libraries/Numerics` | 0 passed / 6 failed in M22 | root finder helpers passed named scalar callbacks and failed in compiled mode. |
| `Libraries/Optimization` | 3 passed / 4 failed in M22 | golden-section and gradient helpers passed named scalar objective/gradient callbacks. |

## Callback signatures required by current tests

The current focused standard-library tests require these monomorphic callback shapes:

- `Libraries/Mathematics`: `fn(Float) -> Float` for differentiation and integration helpers.
- `Libraries/Numerics`: `fn(Float) -> Float` for bisection and Newton functions, plus a second `fn(Float) -> Float` derivative callback for Newton.
- `Libraries/Optimization`: `fn(Float) -> Float` objective callbacks and `fn(Float) -> Float` gradient callbacks.
- `Libraries/DifferentialEquations`: `fn(Float, Float) -> Float` derivative callbacks for Euler and RK4 helpers.

No current focused tests require fallible callback signatures, closures, lambdas, returning function values, storing function values in aggregates, or callbacks crossing Octxiliary boundaries.

## Implementation choices

M27a keeps this intentionally narrow:

- Function type references now have deterministic compiler type strings such as `fn(Float) -> Float` and `fn(Float, Float) -> Float`.
- Compiled Go types lower these function types to Go `func(...) ...` types.
- Named top-level Oct functions used as values lower to their generated Go symbols, such as `fn_Mathematics_f`.
- Callback parameters lower as ordinary Go function parameters and are invoked as direct Go calls.
- Selected-reachable compiled test lowering now treats named function arguments as dependencies while avoiding false dependencies on callback parameter names.

## Post-M27a focused package results

| Package | Result after M27a | Notes |
| --- | ---: | --- |
| `Libraries/Mathematics --execution compiled` | 21 passed / 0 failed | Calculus callback failures are fixed; Mathematics is compiled-green in the focused run. |
| `Libraries/Numerics --execution compiled` | 6 passed / 0 failed | Root-finder callback cases are compiled-green. |
| `Libraries/Optimization --execution compiled` | 7 passed / 0 failed | Objective and gradient callback cases are compiled-green. |
| `Libraries/DifferentialEquations --execution compiled` | 2 passed / 4 failed | Callback step helpers now pass. Remaining solve failures are generated-Go Int/Float argument coercion around `steps`, not unknown callback identifiers. |

## Deferred callback shapes

Deferred deliberately:

- anonymous lambdas and closure capture;
- partial application;
- higher-rank or polymorphic function values;
- function values in records/arrays;
- returning function values;
- dynamic function dispatch beyond named functions and local callback parameters;
- fallible callbacks, because no focused current tests require them;
- callbacks crossing Octxiliary boundaries.

## Recommended next milestone

M27b should address the remaining `Libraries/DifferentialEquations` compiled solve failures by narrowing the generated-Go Int/Float coercion issue where `steps: Int` is incorrectly emitted as `float64(steps)` at `EulerSolve`/`RK4Solve` call sites after arithmetic expressions use the same variable in Float contexts.
