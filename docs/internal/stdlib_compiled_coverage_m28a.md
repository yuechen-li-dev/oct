# M28a RF generated-Go placeholder and array coercion delta

Date: 2026-06-04

## Baseline from M26/M27

M26 made scalar Complex lowering sufficient for `Libraries/Complex`, `Libraries/Signal`, and the Complex-dependent RF S-parameter surface. The focused RF S-parameter run was already green at 7 passed / 0 failed, but the full RF package remained at 45 passed / 12 failed in compiled mode.

M27a/M27b did not target RF. They made `Libraries/Mathematics`, `Libraries/Numerics`, `Libraries/Optimization`, and `Libraries/DifferentialEquations` compiled-green through narrow callback lowering and expression-local `Int`/`Float` coercion fixes.

The M28a reproduction after building sidecars into `.tmp/m28a-wrappers` matched the known RF shape:

| Command | Baseline result | Representative failure class |
| --- | ---: | --- |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m28a-wrappers go run ./cmd/oct test Libraries/RF --execution compiled` | 45 passed / 12 failed | generated Go used `_` as a loop value in discard `for _ in ...` loops, and emitted `[]int` / `Int<unit>[]` where `[]float64` / `Float<unit>[]` was expected. |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m28a-wrappers go run ./cmd/oct test Libraries/RF/RF.SParameters.octest --execution compiled` | 7 passed / 0 failed | S-parameters remained compiled-green. |
| `go run ./cmd/oct test Libraries/RF --execution interpreted` | 57 passed / 0 failed | Interpreted RF remained green. |

Representative generated-Go excerpts before the fix:

```go
case 6:
    if (_ < _t4) { pc = 7 } else { pc = 9 }
case 8:
    _ = (_ + _t5)
```

and:

```text
cannot use distance (variable of type []int) as []float64 value in argument to fn_RF_FreeSpacePathLossLinearSeries
cannot use _t0 (variable of type []int) as []float64 value in argument to fn_RF_ThermalNoisePowerSeries
```

## Root causes

1. **Discard `for` bindings were treated like value bindings.**
   `for _ in 1 .. count` assigned the range cursor into the Go blank identifier, then reused `_` in generated loop condition and increment expressions. Go permits `_` only as an assignment target, not as a value expression.

2. **Expected array element type propagation was incomplete at user-call sites and returns.**
   Array literals in function arguments were lowered before the callee parameter type was pushed as an expected context, so `[1, 2, 3]` could become `[]int` even when the parameter expected `Float[]`. Return lowering also pushed the return type but did not coerce the final expression before emitting the return terminator.

3. **Integer unit literals preserved `Int<unit>` in unconstrained locals.**
   RF uses values like `[1m]` and local variables later passed to helpers expecting `Float<m>[]`. The generated Go representation for `Int<unit>[]` is `[]int`, which requires an explicit elementwise conversion before passing to a `Float<unit>[]` parameter.

## Fixes made

- Discard `for _ in ...` lowering now allocates a real compiler temporary for the loop cursor and does not bind `_` as a readable local. The temporary is used in loop condition and increment emission, preserving Go's `_` rule.
- User-function and callback-function argument lowering now pushes the expected parameter type while lowering each argument and then applies call-site coercion without mutating the source binding.
- Return lowering now applies expression-local coercion to the declared return type after lowering the returned expression.
- Explicitly typed locals now coerce the initializer to the declared type before assignment.
- Integer and float literals now preserve unit-bearing scalar types in compiled lowering (`Int<m>`, `Float<Hz>`, etc.).
- `Int[]` / `Int<unit>[]` to `Float[]` / `Float<unit>[]` coercion in expected contexts now uses a small generated helper that converts slices element by element. Unconstrained arrays still keep their inferred integer element type.

## Regression coverage

Focused `internal/build` regressions now cover:

- passing `[1, 2, 3]` to a `Float[]` parameter;
- returning `[1, 2, 3]` from a `Float[]` function;
- initializing a `Float[]` record field with integer literals;
- initializing an explicitly typed `Float[]` local with integer literals;
- compiling and running a `for _ in ...` loop, then inspecting generated Go to reject `_` as a loop value.

## Post-fix RF results

After the fixes and sidecar build into `.tmp/m28a-wrappers`:

| Command | Result |
| --- | ---: |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m28a-wrappers go run ./cmd/oct test Libraries/RF --execution compiled` | 57 passed / 0 failed |
| `OCT_WRAPPER_PATH=$PWD/.tmp/m28a-wrappers go run ./cmd/oct test Libraries/RF/RF.SParameters.octest --execution compiled` | 7 passed / 0 failed |
| `go run ./cmd/oct test Libraries/RF --execution interpreted` | 57 passed / 0 failed |

`Libraries/RF` is compiled-green in the focused package run.

## Deferred categories

M28a did not implement or broaden any deferred category: no Einstein/tensor notation, no new Complex features, no broad callback/function-value expansion, no wrapper migrations, no new Octxiliary transports or protocol changes, no Pdf image interop, no UI bridge, no package-manager sidecar lifecycle, and no RF public API redesign.

No RF failures remain in the focused compiled/interpreted verification. Any future RF expansion that requires tensors, broader Complex algebra, or new transports should be treated as a separate milestone.

## Recommended next milestone

Proceed to the next standard-library compiled coverage target whose remaining failures are generated-Go lowering issues rather than a deferred feature family. Keep the RF fixes bounded to code generation and avoid folding tensor, wrapper, or protocol work into the next cleanup pass.
