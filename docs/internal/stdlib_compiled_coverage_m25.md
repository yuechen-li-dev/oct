# M25 runtime/index cleanup delta

Date: 2026-06-04
Scope: narrow compiled runtime/test-shape cleanup after M24 numeric array lowering.

## Baseline from M24

M24 left the standard-library numeric packages in this state:

| Package | M24 compiled result | Remaining M24 note |
| --- | ---: | --- |
| `Libraries/Statistics` | 32 passed, 3 failed | `MedianHandlesOddAndEvenDeterministically`, `IQRSpansFiftyPercentOfData`, and `SummarizeProducesCoherentRecord` reached runtime and panicked in `SortedCopy` with index `-1`. |
| `Libraries/Analysis` | 35 passed, 1 failed | `LocalMaximaDoesNotIncludeEndpoints` reached runtime but completed with zero assertions. |

Baseline reproduction for M25 used sidecars built into `.tmp/m25-wrappers` and these focused compiled commands:

```sh
OCT_WRAPPER_PATH=$PWD/.tmp/m25-wrappers go run ./cmd/oct test Libraries/Statistics --execution compiled
OCT_WRAPPER_PATH=$PWD/.tmp/m25-wrappers go run ./cmd/oct test Libraries/Analysis --execution compiled
```

Observed baseline failures before fixes:

- `Statistics.MedianHandlesOddAndEvenDeterministically`: `panic: runtime error: index out of range [-1]` in generated `fn_Statistics_SortedCopy`.
- `Statistics.IQRSpansFiftyPercentOfData`: `panic: runtime error: index out of range [-1]` in generated `fn_Statistics_SortedCopy`.
- `Statistics.SummarizeProducesCoherentRecord`: `panic: runtime error: index out of range [-1]` in generated `fn_Statistics_SortedCopy`.
- `Analysis.LocalMaximaDoesNotIncludeEndpoints`: compiled test runner exited with `0` and `test completed with zero assertions`.

## Fixes made

### Statistics `SortedCopy` index panic

Root cause: the compiled lowering for logical `and` / `or` eagerly lowered both operands to temporaries before emitting the final Go `&&` / `||` expression. In `SortedCopy`, the loop guard:

```oct
while j > 0 and out[j - 1] > value {
```

therefore evaluated `out[j - 1]` even when `j == 0`, producing the generated-Go index `-1` panic. The library algorithm was valid and interpreted behavior was already correct.

M25 changes generated-Go lowering so logical binary expressions use control-flow blocks and preserve short-circuit evaluation before lowering the right operand. This keeps `out[j - 1]` unreachable when `j > 0` is false and also handles `or` symmetrically.

Regression coverage added:

- A compiler/runtime test covering compiled `and` and `or` short-circuiting around index expressions.
- A focused Statistics fact covering reverse-sorted input, repeated values, and input preservation through median/sorted-copy behavior.

### Analysis zero-assertion test shape

`LocalMaximaDoesNotIncludeEndpoints` computed `peaks` and only asserted inside a loop over the returned peaks. The motivating input has exactly one interior peak at index `2`, so the test now asserts that intended shape directly before checking that endpoints are absent. No compiler or test-runner workaround was added.

## Results after M25

| Package | M24 compiled result | M25 compiled result | Status |
| --- | ---: | ---: | --- |
| `Libraries/Statistics` | 32 passed, 3 failed | 36 passed, 0 failed | Improved to compiled-green. |
| `Libraries/Analysis` | 35 passed, 1 failed | 36 passed, 0 failed | Improved to compiled-green. |

Focused interpreted checks also pass for both packages:

- `Libraries/Statistics --execution interpreted`: 36 passed, 0 failed, 0 skipped.
- `Libraries/Analysis --execution interpreted`: 36 passed, 0 failed, 0 skipped.

## Remaining deferred categories

M25 deliberately did not change these deferred areas:

- Complex support.
- Einstein notation or tensor support.
- Broad callback/function-value lowering.
- New wrapper migrations or Octxiliary transport types.
- Octxiliary protocol changes.
- Pdf image interop.
- UI live/native bridge support.
- Package-manager sidecar build lifecycle.
- Public Statistics or Analysis APIs.

Documentation gap surfaced: `Language/reference/language/03-expressions.md` lists logical operators and deterministic evaluation order, but does not explicitly state the short-circuit contract for `and` and `or`. The compiler now matches the behavior that existing Oct code relies on; the reference should make that explicit in a future syntax/reference cleanup.

## Recommended next milestone

**M26 — remaining compiled standard-library parity inventory.**

Suggested focus:

1. Re-run the broader compiled standard-library coverage now that Statistics and Analysis are green.
2. Investigate any narrow Mechanics/RF array-shape cases that do not require Complex or Einstein/tensor support.
3. Update the language reference to explicitly document logical short-circuit semantics if accepted as the intended contract.
4. Keep Complex, Einstein/tensor notation, callback/function-value lowering, new wrapper transports, package-manager sidecar lifecycle, and public API redesign out of scope unless explicitly reprioritized.
