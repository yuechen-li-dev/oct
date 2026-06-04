# M24 numeric and array/matrix shape lowering delta

Date: 2026-06-03
Scope: targeted generated-Go/compiler lowering hardening for numeric coercions, expected array element shape, and shadowed local/result shapes after M23.

## Summary

M24 hardened deterministic generated-Go lowering without changing Octxiliary transport, wrapper protocols, Complex support, Einstein/tensor support, callback/function-value lowering, or public standard-library APIs.

Fixed or improved M23 categories:

- Mixed accepted `Int`/`Float` scalar arithmetic now emits explicit `float64(...)` conversions when the lowered Oct expression result is `Float`. Integer-only arithmetic remains integer-shaped.
- Return, assignment, array-literal, and record-field lowering now propagate expected element types where available, including integer literal values in expected `Float` / `Float[]` contexts.
- Index and range-bound expressions suppress unrelated outer expected `Float` contexts so array indices and loop induction variables remain Go `int` values.
- Shadowed logical names with different generated-Go shapes now receive distinct Go locals while retaining earlier locals needed by already-lowered code. This fixes loop variables later shadowed by Float locals and record-result names reused for different result records.

## Baseline from M23 and M24 results

Sidecars were built into `.tmp/m24-wrappers` with the M22/M23-style `go build` matrix for IO, Hash, Compression, Time, Text, Archive, Json, Csv, Plot, Xlsx, Image, and Pdf wrappers.

| Package | M23 result | M24 result | Status |
| --- | ---: | ---: | --- |
| `Interpolation` | 20 passed, 8 failed | 28 passed, 0 failed | Improved to compiled-green; spline array temporary shape was fixed by expected return/array shape propagation and local-shadow hardening. |
| `LinearAlgebra` | 35 passed, 7 failed | 42 passed, 0 failed | Improved to compiled-green; eigen loop-variable/shadowing shape issues fixed. |
| `Statistics` | 4 passed, 31 failed | 32 passed, 3 failed | Major improvement from Float/Int arithmetic coercion; remaining failures are runtime `SortedCopy` index behavior, not generated-Go type errors. |
| `Random` | 21 passed, 1 failed | 22 passed, 0 failed | Improved to compiled-green; reused record result logical name now lowers to distinct Go locals. |
| `Analysis` | 35 passed, 1 failed | 35 passed, 1 failed | Unchanged; remaining `LocalMaximaDoesNotIncludeEndpoints` reaches runtime and exits with zero assertions. |

## Failures fixed

- `Statistics` generated-Go build failures such as `sum / n`, `accum / n`, and percentile rank arithmetic now compile through explicit Int-to-Float conversion when the Oct typechecker has already accepted a Float result.
- `LinearAlgebra` Jacobi eigen helpers now compile because `for c in ...` remains an integer induction variable even when a later `let c = ...` Float binding appears in the same function.
- `Interpolation` spline tests now compile and run because array-typed temporaries retain their full `Float[]` shape instead of being narrowed by later scalar expressions.
- `Random.BernoulliAndRangesAndNormalAreNonDegenerate` now compiles because reused `draw` bindings with distinct record result shapes get separate generated Go locals.
- Focused M24 regression coverage now exercises mixed numeric arithmetic, typed `Float[]` literals, record field expected array shape, loop-index/shadowing behavior, and record result shape shadowing.

## Deferred or still open

- `Statistics` has three remaining compiled runtime failures in `MedianHandlesOddAndEvenDeterministically`, `IQRSpansFiftyPercentOfData`, and `SummarizeProducesCoherentRecord`; all panic in `SortedCopy` with index `-1`. These are now isolated from the prior generated-Go numeric type errors.
- `Analysis.LocalMaximaDoesNotIncludeEndpoints` still exits with zero assertions in compiled mode and appears to be a test/runner assertion-shape issue rather than numeric lowering.
- Complex support remains deferred.
- Einstein notation/tensor support remains deferred.
- Broad callback/function-value lowering remains deferred.
- Pdf image interop, UI live/native bridge builtins, legacy structured IO APIs, package-manager sidecar lifecycle, and syntax/reference cleanup remain deferred.

## Recommended next milestone

**M25 — compiled runtime-shape cleanup.**

Suggested focus:

1. Statistics `SortedCopy` compiled runtime/index behavior now that Float/Int generated-Go type failures are removed.
2. Analysis compiled zero-assertion runner/test-shape issue if it is not a language-contract problem.
3. Additional array/list temporary-shape probes in Mechanics/RF that do not require Complex or Einstein/tensor support.
4. Keep Complex, Einstein/tensor notation, broad callback lowering, new wrapper migrations, new Octxiliary transports, and public API redesign out of scope unless explicitly reprioritized.
