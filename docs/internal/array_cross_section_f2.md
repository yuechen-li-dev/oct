# F2 — Range values and `Array.CrossSection` M0 design

## Executive summary

F2 recommends a v0.1-friendly design for readable 1D array subsetting:

- Add/complete compiler-owned first-class range values as `Range` if the parser/typechecker/compiler work remains tractable for v0.1.
- Add `Array.CrossSection(values, range)` as the only M0 array subsetting API.
- Keep `Array.CrossSection` limited to `T[]` collection arrays and returning a new `T[]` copy.
- Preserve exact element type, including nominal record/enum element types and SI-dimensioned numeric element types such as `Float<K>[]`.
- Reject Python-style colon slicing and bracket range extraction in M0.
- Defer vector/matrix/tensor slicing, lazy views, reverse ranges, negative indexing, maps, user-defined generics, and iterator protocols.

Convergence state: **success for design**. This milestone intentionally does not change production behavior. It narrows the F1 array-subsetting blocker into a readable API and a bounded range-value implementation plan.

## Evidence inspected

F2 was written against the current F1 report, reference surface, and implementation shape:

- `docs/internal/array_map_generics_friction_f1.md`
- `Language/reference/language/07-arrays.md`
- `Language/reference/language/04-control-flow.md`
- `Language/reference/language/03-expressions.md`
- `Language/reference/language/09-builtins.md`
- `Language/reference/language/16-vectors-and-matrices.md`
- `Language/Types/Arrays/...`
- `Language/ControlFlow/Loops/...`
- `Experiments/LanguageFriction/ArrayMapGenerics/...`
- `internal/lex/lex.go`
- `internal/parse/parse.go`
- `internal/ast/program.go`
- `internal/typecheck/typecheck.go`
- `internal/interpret/interpret.go`
- `internal/build/compiler.go`
- `internal/builtin/builtin.go`

Current implementation note: Oct already has a closed `Range` value path in parts of the implementation. The parser represents `start..end [step n]` as `ast.RangeExpr`; the typechecker gives it `Range`; the interpreter can evaluate it; and `for` loops consume it. However, open-ended forms (`start..`, `..end`, `..`) are not represented today, and compiled expression lowering rejects general range expressions outside its dedicated for-loop lowering. F2 therefore treats "first-class Range" as a design recommendation to complete and document, not as an assertion that every proposed M0 form already works.

## 1. Terms

### Array cross-section

An array cross-section is a new array copy selected from a 1D Oct array by a readable range expression. It is not Python slicing, and it is not rank-aware vector/matrix/tensor slicing.

In M0, a cross-section is a collection operation over `T[]` only. It does not reinterpret arrays as mathematical vectors or matrices.

### Range value

A range value is a compiler-owned immutable value describing integer index selection:

- optional start expression;
- optional end expression;
- optional positive step expression.

The consumer resolves omitted endpoints because the correct defaults are collection-dependent. For arrays, omitted start means `0`; omitted end means `Len(values)`.

### Open start

An open start is a range with no explicit start endpoint:

```oct
..200
..
```

For `Array.CrossSection`, open start resolves to `0`.

### Open end

An open end is a range with no explicit end endpoint:

```oct
100..
..
```

For `Array.CrossSection`, open end resolves to `Len(values)`.

### Step

A step is a positive `Int` stride between selected indices:

```oct
0..Len(values) step 10
```

An omitted step resolves to `1`. M0 rejects `step <= 0` and does not support reverse ranges.

### Copy semantics

`Array.CrossSection` returns a newly allocated array. The result contains copies of the selected element values in order. Mutating the result array does not mutate the source array's array storage, and mutating the source array after the call does not mutate the result array storage.

For record/enum/string/scalar element values, this follows Oct's ordinary value semantics. If future reference-like element values are added, F2 does not define deep-clone behavior; the cross-section guarantee is about array storage, not arbitrary deep object graphs.

### View semantics deferred

A view is a lazy or aliasing window over source storage. Views are explicitly deferred. M0 must not expose view/lazy slice semantics or lower `Array.CrossSection` to a Go slice alias.

## 2. Why not bracket slicing

F2 explicitly rejects Python-like colon slices:

```oct
xs[1:3]
xs[:3]
xs[1:]
xs[::2]
xs[1:5:2]
```

Reasons:

- Colon slices are opaque to non-Python readers.
- `::`-style syntax is terse but not self-explanatory.
- Oct already has readable `start..end step n` range syntax.
- Brackets already mean scalar indexing for arrays, vectors, matrices, and tensors.
- Matrix/tensor bracket range syntax would prematurely imply rank-aware slicing.
- v0.1 should prefer explicit scientific operation names over punctuation compression.

F2 also rejects bracket range extraction for M0:

```oct
xs[1..3]
xs[..3]
xs[1..]
```

This is not because Oct range syntax is bad. It is because bracket range extraction creates syntax and rank expectations that should wait until vector/matrix/tensor cross-section design is ready. M0 should keep scalar indexing visually distinct from array subsetting:

```oct
let one = xs[3]
let many = Array.CrossSection(xs, 1..4)
```

## 3. Recommended API shape

M0 should introduce one readable array API:

```oct
Array.CrossSection(values, 100..200)
Array.CrossSection(values, 100..)
Array.CrossSection(values, ..200)
Array.CrossSection(values, ..)
Array.CrossSection(values, 0..Len(values) step 10)
```

Rules:

- `Array.CrossSection` operates only on `T[]`.
- It returns `T[]`.
- It preserves exact element type, including:
  - nominal record types;
  - nominal enum types;
  - SI dimensions, for example `Float<K>[]`;
  - `String[]`, `Bool[]`, `Int[]`, and `Float[]`.
- It returns a new array copy.
- It does not produce a view.
- It does not mutate the source array.
- It does not accept vectors, matrices, or tensors.
- It does not accept maps/dictionaries.
- It does not require user-defined generics.

Naming decisions:

- Prefer `Array.CrossSection`, not bare `CrossSection`, for M0.
- The `Array` namespace means collection/data-series arrays only.
- Future rank-aware APIs should live under `Vector`, `Matrix`, or `Tensor` namespaces, not under `Array`.

## 4. Range expression design

F2 evaluates two implementation shapes.

### Option A — first-class compiler-owned `Range`

Examples:

```oct
let Window = 100..200
let Tail = 100..
let Prefix = ..200
let All = ..
let EveryTenth = 0..Len(values) step 10

let SamplesWindow = Array.CrossSection(Samples, Window)
```

Pros:

- Consistent reusable concept.
- Works beyond `Array.CrossSection` later.
- Avoids a one-off parser special case for one builtin call.
- Can support future APIs like `Analysis.Window(values, range)`.
- Clean documentation story: range expressions are values, and APIs decide how to consume open endpoints.
- Aligns with the existing implementation direction where closed `start..end [step n]` already has `ast.RangeExpr`, `Range` typechecking, and interpreted `Range` values.

Cons:

- Requires range expressions to have a fully documented type.
- Open-ended ranges need representation of omitted start/end.
- Parser changes are needed for prefix/open-start forms such as `..200` and all-open `..`.
- Compiler support must be completed for general range expressions, not only dedicated for-loop lowering.
- May expose range values before many APIs consume them.

### Option B — range-argument syntax only for `Array.CrossSection`

Examples:

```oct
Array.CrossSection(Samples, 100..200)
Array.CrossSection(Samples, ..200)
```

but:

```oct
let r = 100..200
```

would not be supported yet.

Pros:

- Smaller intended implementation surface.
- Avoids exposing a broad `Range` type in the language reference.
- Easier to keep as a v0.1 built-in special form.

Cons:

- Requires special-case grammar around a specific builtin call.
- Harder to reuse in other APIs.
- Users may reasonably expect `let r = 100..200` to work if the same syntax appears in function arguments.
- Risks creating an awkward half-feature.
- Conflicts with the current implementation direction, where range expressions are already represented as expressions rather than as a call-local special form.

### Recommendation

Prefer **Option A: first-class compiler-owned `Range`**, provided F3 confirms parser/typechecker/compiler changes are tractable.

Accept **Option B** only if open-ended first-class range expressions prove too risky for v0.1. If Option B is chosen, it must be documented as an intentionally temporary built-in special form and should not introduce bracket extraction.

In either option, do not implement bracket range syntax in M0.

## 5. Range M0 rules

### Type name

F2 recommends the type name:

```oct
Range
```

Rationale:

- The implementation already uses `Range` as a compiler-owned base type/value kind.
- M0 endpoints are `Int`, so `Range` is not currently ambiguous in user code.
- Future non-index ranges remain speculative.

Alternative names:

- `IndexRange` is clearer if future non-Int endpoint domains are likely.
- `IntRange` is precise but may overfit the implementation detail into user-facing language.

Decision: use `Range` for M0. Revisit only if a concrete non-Int range proposal appears.

### Forms considered

Full conceptual range forms:

```oct
start..end
start..
..end
..
start..end step n
start.. step n
..end step n
.. step n
```

Recommended M0 forms:

```oct
start..end
start..
..end
..
start..end step n
```

Open-ended ranges with `step` should be deferred in M0:

```oct
start.. step n
..end step n
.. step n
```

Reason: `start..end step n` already matches the current for-loop surface and is enough for decimated fixed windows. Deferring open-ended `step` forms keeps parser recovery and diagnostics simpler. Once `Range` is stable, F4 or a small follow-up can add open-ended stepped ranges without changing `Array.CrossSection` semantics.

### Semantics

- `start`, `end`, and `step` are `Int` expressions.
- `start` is inclusive.
- `end` is exclusive.
- Omitted start is resolved by the consumer as `0` for arrays.
- Omitted end is resolved by the consumer as the collection length for arrays.
- Omitted step is `1`.
- `step` must be positive.
- `step` cannot be zero.
- Negative `step` is rejected in M0.
- Negative start/end are ordinary `Int` values syntactically, but produce bounds errors in `Array.CrossSection`; they are not Python-style "from end" indices.
- No reverse ranges in M0.
- Range values are immutable.

### Open endpoint representation

Parser/AST/typechecker should preserve whether start and/or end were omitted. Do not eagerly replace omitted endpoints with `0` or `Len(...)` at parse time because endpoint defaults are consumer-dependent.

Suggested AST shape for F3:

```go
type RangeExpr struct {
    Start Expr // nil when omitted
    End   Expr // nil when omitted
    Step  Expr // nil when omitted
}
```

The existing `RangeExpr` shape is close, but current parsing assumes both `Start` and `End` are present. F3 should extend it rather than inventing a separate syntax node.

## 6. `Array.CrossSection` semantics

Given:

```oct
let Out = Array.CrossSection(values, r)
```

Resolve:

- `start = r.Start` if present, otherwise `0`;
- `end = r.End` if present, otherwise `Len(values)`;
- `step = r.Step` if present, otherwise `1`.

Validation:

- `step > 0`;
- `start >= 0`;
- `end >= 0`;
- `start <= Len(values)`;
- `end <= Len(values)`;
- `start <= end`.

Error model recommendation:

- `Array.CrossSection(values, range) -> T[]` should bounds-check and fail with a clear runtime error when the range is invalid.
- This matches the current direct style of array indexing and `Require`: programmer-error preconditions fail immediately rather than returning fallible values everywhere.
- A future fallible helper may be useful for data-cleaning pipelines:

```oct
Array.TryCrossSection(values, range) -> T[] ! Error
```

Do not require `Array.TryCrossSection` for M0 unless the language first adds fallible variants for ordinary array indexing or establishes a broader fallible collection-access convention.

Allocation:

- Allocate a new result array.
- Precompute capacity when straightforward.
- Copy exact selected element values.
- Leave source array unchanged.

Step behavior:

- If `step == 1`, perform a contiguous copy.
- If `step > 1`, perform a decimated copy.
- Reject `step <= 0`.

Examples:

```oct
let a = [10, 20, 30, 40, 50]

Array.CrossSection(a, ..)          // [10, 20, 30, 40, 50]
Array.CrossSection(a, ..3)         // [10, 20, 30]
Array.CrossSection(a, 2..)         // [30, 40, 50]
Array.CrossSection(a, 1..4)        // [20, 30, 40]
Array.CrossSection(a, 0..5 step 2) // [10, 30, 50]
```

## 7. Typechecking and generics

M0 type rule:

```oct
Array.CrossSection<T>(values: T[], range: Range) -> T[]
```

This is compiler-owned polymorphism. It does not introduce user-defined generics.

Rules:

- The first argument must be a 1D array `T[]`.
- The second argument must be `Range`.
- The result array element type is exactly the source array element type.
- SI dimensions are preserved naturally because the element type is copied exactly.
- Nominal record and enum element types are preserved exactly.
- Wrong arity is rejected statically.
- Non-range second arguments are rejected statically when their type is known.
- Invalid literal/static steps may be rejected statically; dynamic invalid steps fail at runtime.

Reject:

- non-array first argument;
- `Vector<T>`;
- `Matrix<T>`;
- tensor/indexed tensor values;
- maps/dictionaries;
- scalar `String`;
- wrong arity;
- non-range second argument.

Recommended diagnostics:

```text
Array.CrossSection expects a 1D array as its first argument
```

```text
Array.CrossSection operates on arrays, not Matrix<Float>; use Matrix-specific APIs
```

```text
Array.CrossSection expects a Range as its second argument
```

```text
array cross-section step must be positive
```

```text
array cross-section range is out of bounds
```

The vector/matrix/tensor diagnostic should name the offending type when possible and should not suggest using array APIs to perform rank-aware mathematical slicing.

## 8. Interpreted and compiled implementation plan

This section is a plan for F3, not an F2 implementation.

### Parser

If Option A is chosen:

- Extend existing range grammar from for-loops/general expressions into full expression parsing.
- Support `start..end`, `start..`, `..end`, `..`, and `start..end step n`.
- Preserve `step` as a contextual keyword. It must remain legal as an ordinary identifier in non-ambiguous binding/reference positions.
- Produce targeted diagnostics for malformed ranges such as missing endpoints where not allowed by the M0 form set.

If Option B is chosen:

- Parse range arguments specially in `Array.CrossSection`.
- Keep the special case narrow and documented as temporary.

F2 recommends Option A.

### AST

- Extend `RangeExpr` with optional start/end and optional step.
- Preserve source spans for the full range expression and each endpoint/step expression.
- Preserve whether endpoints were omitted.

### Typechecker

- Endpoint and step expressions typecheck as `Int`.
- Range expression type is `Range`.
- `Array.CrossSection` is a compiler-owned namespaced builtin.
- Verify `T[] + Range` and produce `T[]`.
- Reject vector/matrix/tensor inputs explicitly rather than allowing a generic non-array error when a better diagnostic is available.

### Interpreter

- Evaluate present start/end/step expressions once.
- Resolve omitted endpoints with the source array length inside `Array.CrossSection`.
- Validate bounds and positive step.
- Allocate a result array.
- Append selected elements in order.
- Return the new array.

### Compiler/build

- Generate Go code that copies selected elements into a new slice/array.
- Do not lower to a Go slice alias/view.
- Preserve element type exactly.
- Support `step > 1`.
- Preserve compiled/interpreted parity.
- Ensure general `Range` expression lowering works wherever first-class ranges are accepted, or keep range lowering intentionally restricted if F3 selects Option B.

Potential Go lowering sketch:

```go
start := ...
end := ...
step := ...
if step <= 0 || start < 0 || end < 0 || start > end || end > len(values) {
    return error(...)
}
out := make([]T, 0, ((end-start)+step-1)/step)
for i := start; i < end; i += step {
    out = append(out, values[i])
}
```

## 9. Docs plan for implementation milestone

Update these docs when F3 implements behavior:

- Array reference doc: add `Array.CrossSection` examples and copy semantics.
- Range/for-loop/control-flow reference doc: document first-class `Range` if Option A lands.
- Expression reference doc: document range expressions if Option A lands.
- Builtin reference doc: add `Array.CrossSection` under namespaced compiler-owned builtins.
- F1 doc: retain as historical audit and point to F2/F3 for the post-F1 direction.
- Compiled support docs: mark interpreted/compiled support and any remaining gaps.

Docs must explicitly say:

- This is not Python slicing.
- There is no colon slice syntax.
- `Array.CrossSection` is for arrays only.
- Vectors, matrices, and tensors have separate APIs.
- M0 has copy semantics.
- M0 has no negative indices.
- M0 has no reverse ranges.
- User-defined generics are not involved.

## 10. Tests plan for F3

F2 is design-only, so these tests are planned for the implementation milestone.

### Parser tests

- `100..200`
- `100..`
- `..200`
- `..`
- `100..200 step 10`
- malformed ranges with clear diagnostics
- existing `for i in start..end` parsing still works
- existing `for i in start..end step n` parsing still works
- `step` remains contextual and can still be used as an identifier where currently allowed

### Typechecker tests

- Range start must be `Int`.
- Range end must be `Int`.
- Range step must be `Int`.
- Literal/static `step 0` rejected.
- Literal/static negative step rejected.
- `Array.CrossSection(Float[], Range) -> Float[]`.
- Preserve `Float<K>[]`.
- Preserve record arrays.
- Preserve enum arrays.
- Reject non-array first argument.
- Reject `Vector<T>`.
- Reject `Matrix<T>`.
- Reject tensor/indexed tensor values.
- Reject scalar `String`.
- Reject non-range second argument.
- Reject wrong arity.

### Interpreter tests

- Copy all: `Array.CrossSection(a, ..)`.
- Prefix: `Array.CrossSection(a, ..n)`.
- Suffix: `Array.CrossSection(a, n..)`.
- Middle window: `Array.CrossSection(a, start..end)`.
- Step: `Array.CrossSection(a, 0..Len(a) step 2)`.
- SI-dimensioned array.
- Record array.
- Enum array.
- Out-of-bounds start.
- Out-of-bounds end.
- Negative start/end reject as bounds errors, not from-end indices.
- Zero step.
- Negative step.
- Source array unchanged.
- Result array storage is a copy, not an alias, where mutation can prove it.

### Compiled tests

- Same valid cases as interpreted.
- Same error cases where compiled runtime errors are supported.
- Generated result is copy, not alias, where mutation can prove it.
- `step > 1` parity.
- SI-dimensioned element preservation parity.
- Record/enum element preservation parity if compiled mode supports those array element cases.

### Language corpus

Add valid and invalid language contracts under `Language/`:

- Valid `Array.CrossSection` examples under the array language area.
- Invalid expected failures for non-array, vector/matrix/tensor, non-range, wrong arity, and malformed ranges.
- Keep user-visible language semantics in `.octest` and `.octfail`, not embedded Go tests.

Go tests should orchestrate parser/compiler integration only and should not duplicate semantic contracts already captured under `Language/`.

## 11. Open design questions answered

### 1. Should Range be called `Range`, `IndexRange`, or something else?

Use `Range` for M0. The current implementation already uses that compiler-owned name. Revisit only if a concrete non-Int endpoint proposal appears.

### 2. Should range values be first-class in M0?

Yes, if F3 confirms the parser/typechecker/compiler changes are tractable. This avoids a one-off `Array.CrossSection` grammar and aligns with the existing range-expression implementation direction.

### 3. Should open-ended ranges with `step` be supported in M0?

No. Support `start..end step n` in M0 and defer `start.. step n`, `..end step n`, and `.. step n`. The deferred forms are useful but not necessary for the first cross-section milestone.

### 4. Should `Array.CrossSection` be fallible or runtime-erroring?

Runtime-erroring for M0. It returns `T[]`, bounds-checks, and fails clearly for invalid ranges. A future `Array.TryCrossSection(values, range) -> T[] ! Error` can be considered after the language has a broader fallible collection-access convention.

### 5. Should step be included in M0 or deferred?

Include `step` for closed ranges: `start..end step n`. It is already part of readable loop syntax and supports common scientific decimation. Defer open-ended stepped ranges only.

### 6. Should `Array.CrossSection` support arrays of records/enums/SI-dimensioned numbers?

Yes. It should preserve exact element type for every supported `T[]`, including records, enums, and SI-dimensioned numeric elements.

### 7. Should `Array.Copy(values)` exist as an alias for `Array.CrossSection(values, ..)`?

No for M0. `Array.CrossSection(values, ..)` is explicit and sufficient. Add `Array.Copy` only if broader copy APIs are designed for arrays and other value categories.

### 8. Should there be convenience helpers `Array.Take`, `Array.Drop`, `Array.Window`, or should `CrossSection` be the only M0?

`Array.CrossSection` should be the only M0 helper. `Take`, `Drop`, and `Window` are useful but would expand the API before the core range/cross-section semantics have proven themselves.

## 12. Recommended next milestone

Split implementation into two milestones unless F3 estimates both pieces as small:

```text
F3a — implement first-class Range expressions M0
F3b — implement Array.CrossSection M0
```

If implementation inspection shows open-ended first-class ranges and `Array.CrossSection` are both low risk, they may be combined as:

```text
F3 — implement Range expressions and Array.CrossSection M0
```

The split is safer because current code already has partial closed-range support but not open endpoints or general compiled range expression lowering.

## 13. F2 validation

F2 acceptance criteria status:

1. `docs/internal/array_cross_section_f2.md` exists. **Done.**
2. Python-style colon slicing is rejected. **Done.**
3. Bracket range extraction is rejected for M0. **Done.**
4. `Array.CrossSection` is recommended. **Done.**
5. Array vs Vector/Matrix/Tensor distinction is preserved. **Done.**
6. First-class `Range` vs special range argument options are evaluated. **Done.**
7. M0 range syntax and semantics are specified. **Done.**
8. `Array.CrossSection` M0 behavior is specified. **Done.**
9. Copy semantics are specified. **Done.**
10. Typechecking rules and diagnostics are specified. **Done.**
11. Interpreted/compiled implementation plan is described. **Done.**
12. F3 tests plan is included. **Done.**
13. Next milestone is recommended. **Done.**
14. Production behavior changes. **None intended.**
15. Full tests. **Run as part of this F2 change.**
