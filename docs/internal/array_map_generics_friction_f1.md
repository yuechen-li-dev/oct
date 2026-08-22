# F1 — Array/Map/Generics friction audit

## Executive summary

F1 tested everyday scientific data-shaping tasks against the current Oct reference surface and runnable probes under `Experiments/LanguageFriction/ArrayMapGenerics/`.
The main result is convergence state **meaningful progression**: array/vector/matrix scalar element access, manual loop transforms, concrete function-value helpers, SI-dimensioned array elements, and SI-dimensioned Octomata board snapshots work; the next blockers are now isolated as missing slice syntax, missing dictionary/map values, absent user-defined generics, and the now-resolved F4 compiled support gap for scalar `String.From<T>` in the probe package.

The current reference explicitly supports homogeneous arrays, `xs[i]` indexing, mutable indexed assignment, `Append`, `Len`, exact element type matching including SI dimensions, and element-wise array arithmetic.
It does not document slices.
Vectors and matrices are intentionally separate from arrays: `Float[]` is not a `Vector<Float>`, `Float[][]` is not a `Matrix<Float>`, vectors use `v[i]`, matrices use `m[r, c]`, matrix shape is exposed as `m.rows`/`m.cols`, and vector/matrix slicing is not documented.
The builtin surface documents `String.From<T>` only for compiler-known `Int`, `Float`, `Bool`, and `String`, and it explicitly says this does not introduce user-defined generics.

Post-F1 direction update: F2 supersedes the earlier "slice syntax" shorthand with a readable `Array.CrossSection(values, range)` design and explicitly rejects Python-style colon slicing and bracket range extraction for M0. See `docs/internal/array_cross_section_f2.md`.

Recommended v0.1 direction:

- **Must-have for v0.1:** add conservative 1D array slice syntax if schedule allows; keep the F4 compiled parity fix for promised `String.From<T>` scalar conversions covered; document the current map/dictionary absence; keep SI board-field behavior covered because the probe shows interpreted and compiled snapshot preservation works.
- **Nice-to-have for v0.1:** add concrete, non-generic `Array`/`Analysis` helpers for common `Float[]`, `Int[]`, and `String[]` workflows where they remove large boilerplate; add explicit matrix row/column extraction helpers returning arrays, not vectors, if they can be implemented without a full tensor-slicing design.
- **Defer to v0.2:** full `Map<K,V>`, map literals, user-defined generics, bounded generics, iterator protocols, vector/matrix/tensor slicing, and automatic `String.From` for enums/records/arrays.

## Probe matrix

| Category | Representative probes | Current status | Evidence |
|---|---|---:|---|
| A. Array slicing/subsetting | first N, drop N, window, every kth, last N, fallible bounds checks | **Works but awkward** with manual loops; slice syntax is **impossible today** | `array_transform_manual.octest`; `expected_fail/slice_syntax_not_supported.oct.disabled` |
| B. Vector/matrix row and column operations | row extraction, column extraction, row/column reductions, SI-unit row extraction | **Works but awkward** when returning arrays; vector conversion and matrix slicing are **not available** | `vector_matrix_manual.octest`; `expected_fail/matrix_slice_not_supported.oct.disabled` |
| C. Map/transform/filter/reduce/zip | Float map, Int-to-String map, filter, sum, normalize, moving average, z-score, map-with-index, zip with shape check | **Works but awkward** with concrete helper functions; reusable generic helpers are **impossible today** | `array_transform_manual.octest`; function-value docs show exact concrete signatures only |
| D. Lookup tables/dictionaries/maps | String-to-Float lookup, enum score lookup, frequency counting, grouping, CSV named fields, missing-key lookup | **Impossible as a general data structure today**; records/enums substitute only for static/fixed cases | `expected_fail/map_literal_not_supported.oct.disabled` |
| E. `String.From<T>` | `Int`, `Float`, `Bool`, `String`, enum, unit value, array, formatting | **Works interpreted and, after F4, compiled for compiler-known scalar types**; enum/unit/array are intentionally rejected | `string_and_board_probe.octest`; `expected_fail/string_from_enum_not_supported.oct.disabled` |
| F. Generics/bounded generics | user-defined `Identity<T>`, generic map/filter/reduce, typed `Map<K,V>` | **Impossible today** outside compiler-owned type arguments | `expected_fail/user_defined_generic_function_not_supported.oct.disabled` |
| G. SI board fields | `board.LastTemp: Float<K>`, assignment, return, snapshot | **Resolved for v0.1/F6** in interpreted and compiled execution; `BoardSnapshot` preserves `Int<D>`/`Float<D>` scalar fields | `dimensioned_scalar_snapshot_surface.octest`; `si_board_baseunit_f6.md` |

## Works today

### Arrays with manual loops

The array reference gives enough primitives to build shaped copies: array literals, exact `T[]` types, `Len`, `Append`, and integer indexing.
The working F1 probe implements:

- first N elements;
- drop first N elements;
- `[start:end]`-style middle window;
- every kth element;
- last N elements;
- explicit bounds-safe fallible windowing;
- concrete `Float[] -> Float[]` map;
- `Int[] -> String[]` conversion through `String.From<Int>`;
- filter by predicate logic inlined in the helper;
- sum/reduce;
- normalize by sum;
- moving average;
- z-score transform;
- map with index;
- zip with equal-length shape check.

This proves the language can express the operations, but it also proves the friction: every operation needs custom loops, explicit empty-array typed contexts, and repeated append-based allocation.
The implementation must be specialized by element type because user-defined generics are absent.

### Concrete function values

Function values are available for named functions with exact signatures.
That is enough for concrete helpers like `fn ApplyLengthScale(values: Float<m>[], op: fn(Float<m>) -> Float<m>) -> Float<m>[]`, but not for a reusable `Array.Map<T,U>`.
This is useful for narrow scientific hooks and not enough for a general collection library.

### Vector and matrix scalar element access

Matrix row and column extraction can be written manually by looping over `m.cols` or `m.rows` and returning `Float[]`.
Dimensioned matrix elements preserve units when copied into a dimensioned array.
This works because matrix element access has exact element type, including dimensions.

### SI unit board fields and snapshots

The F1 board probe uses:

```oct
flow Controller(temp: Float<K>) -> Float<K> {
    board {
        LastTemp: Float<K>
    }

    state Tick {
        board.LastTemp = temp
        suspend
    }

    state Done {
        return board.LastTemp
    }
}
```

Interpreted mode passed the snapshot assertion, and compiled mode also passed this particular board probe.
This was better than expected from the older scalar-only wording in the Octomata reference. The reference now reflects implementation truth: `BoardSnapshot` supports the accepted scalar board types and arrays of those types.
Because `Float<K>` is a dimensioned `Float`, the implementation preserves the unit type in the typed snapshot; F6 tightened the reference wording and added interpreted/compiled language coverage for dimensioned scalar board fields.

## Awkward today

### Slices are boilerplate and allocation semantics are implicit

Today, `xs[start:end]` does not parse.
The equivalent manual helper allocates a new array through repeated `Append`.
The code can be made bounds-safe by returning `T[] ! Error`, but every caller and every element type must repeat that pattern.
There is no visible copy-vs-view contract because no slice surface exists.

The practical result for scientific scripts is high friction for the most common sequence operations: trimming warmup samples, extracting a calibration window, dropping headers, selecting decimated samples, and keeping a tail window all become helper definitions instead of one expression.

### Generic-looking containers exist only as compiler-owned surfaces

The reference uses generic-looking forms for arrays (`T[]`), vectors (`Vector<T>`), matrices (`Matrix<T>`), and compiler-owned builtins such as `String.From<T>` and `Matrix.zeros<T>`.
However, user-defined generic functions do not parse, and there is no documented way to define a generic collection helper.
This is a cognitive trap: users can see `Vector<Float>` and `String.From<Int>`, but cannot write `fn Map<T,U>(...)`.
The audit should keep this distinction explicit in v0.1 docs.

### Matrix row/column workflows lose mathematical shape

Manual row extraction returns `Float[]`, not `Vector<Float>`.
That is honest with the current model: arrays are collections and vectors are mathematical rank-1 tensor values.
But scientific users often expect `m[row, :]` to produce a vector-like object.
Adding that without a tensor-slicing design would force a premature answer to orientation, row-vector vs column-vector, shape labels, copy/view behavior, and SI-unit preservation.

### Standard-library helpers are concrete

`Libraries/Analysis` already contains concrete `Float[]` helpers such as cumulative sums, normalization, centering, and standardization.
This validates the manual-loop style as current Oct, but also shows why generic array helpers are not yet available: the same patterns must be duplicated for every type and dimension family.

## Impossible today

### Slice syntax

Probe:

```oct
return xs[1:3]
```

Current diagnostic:

```text
run failed: parse .../slice.oct: expected ']' after index expression at 5:16 near ":"
```

The parser treats `:` inside brackets as invalid for array element indexing.

### Matrix slicing

Probe:

```oct
return m[:, 0:1]
```

The current reference only documents matrix concrete indexing as `m[r, c]` with exactly two `Int` indices, and indexed tensor notation as `m[i, j]` with symbolic `Index` labels.
There is no mixed scalar/range matrix indexing form today.

### Maps/dictionaries

Probe:

```oct
let m = map[String, Float] {
    "alpha": 1.0
}
return m["alpha"]
```

Current diagnostic:

```text
run failed: parse .../map.oct: expected statement at 4:32 near "{"
```

There is no documented `Map<K,V>` type, no map literal, and no string-key lookup indexing.
Records can model fixed named fields known at authoring time.
Enums plus `switch` can model small static score tables.
Neither can model dynamic string-key lookup, frequency counting, grouping, CSV headers, parameter tables, or adjacency maps.

### User-defined generics

Probe:

```oct
fn Identity<T>(x: T) -> T {
    return x
}
```

Current diagnostic:

```text
run failed: parse .../generic.oct: expected '(' after function name at 3:12 near "<"
```

This confirms that generic type parameters are not merely undocumented; the syntax is unavailable for user functions.

### Automatic string conversion beyond scalar compiler-known types

Probe:

```oct
return String.From<Mode>(Mode.Fast)
```

Current diagnostic:

```text
run failed: function Main: String.From<T> supports Int, Float, Bool, and String in M0
```

The same M0 limitation applies to arrays and unit-qualified numeric values such as `String.From<Float<K>>(2.0K)`.
`FormatFloat(value, precision)` covers dimensionless `Float` display precision; unit-aware formatting is not available as a generic conversion.

## Slice recommendation

### Should Oct add slice syntax?

Yes, **1D array slicing is the smallest high-impact feature surfaced by this audit**.
It removes widespread boilerplate without requiring maps, generics, iterators, or tensor slicing.
It is also easy to document deterministically.

### Recommended M0 syntax

```oct
xs[start:end]
xs[start:]
xs[:end]
xs[:]
```

### M0 rules

- Applies to **1D arrays only** in M0 (`T[]`).
- `start` is inclusive.
- `end` is exclusive.
- `start` and `end` expressions must be `Int`.
- Omitted `start` means `0`.
- Omitted `end` means `Len(xs)`.
- Bounds are checked at runtime with clear diagnostics or fallible helper equivalent behavior.
- Result type is `T[]` with exact element type preservation, including SI dimensions and nominal names.
- Result is a **new array copy** in M0.
- `xs[:]` is an explicit shallow element copy of the array value.
- No negative indices in M0.
- No step slices in M0.
- No vector, matrix, or tensor slicing in M0.

### Why no step slices in M0?

`xs[start:end:step]` is useful, but it multiplies the design surface:

- step must reject zero;
- negative step implies reversed bounds semantics or requires rejection;
- allocation size calculation becomes more complex;
- users may expect Python/NumPy negative indexing if step exists;
- the manual `EveryKth` probe already works as a library helper.

Recommended path: ship contiguous copy slices first; provide `EveryKth`/`Decimate` as concrete helper functions if needed; revisit step slices with iterators/tensor slicing in v0.2.

### Copy vs view

M0 should choose copy semantics.
Views would need aliasing, mutation, lifetime, and compiled lowering rules.
Copy semantics align with current `Append`-style array construction and are easiest to reason about in scientific scripts.
If views are introduced later, they should be a separate explicit type or API, not a silent change to `xs[start:end]`.

## Map/dictionary recommendation

### Does Oct need maps?

Yes for scientific scripts that handle named parameters, CSV headers, grouping, frequency counts, graph adjacency, and dynamic lookup tables.
No current combination of records/enums/arrays substitutes for dynamic key-value lookup.

### Is `Map<K,V>` a v0.1 must-have?

Probably **not** unless v0.1 explicitly targets CSV/dataframe-like workflows.
A sound map feature likely needs several adjacent decisions:

- type parameter representation for `K` and `V`;
- allowed key types and equality/hash semantics;
- missing-key behavior;
- literal syntax;
- mutation/immutability;
- iteration order;
- compiled representation.

That is too broad for a late v0.1 release unless it is sharply constrained.

### Conservative M0 shape if included

If maps are pulled into v0.1 anyway, choose a deliberately small surface:

```oct
let m: Map<String, Float> = Map.fromPairs([("alpha", 1.0), ("beta", 2.0)])
let alpha = m["alpha"]?
```

Recommended constraints:

- `Map<K,V>` is builtin/compiler-owned at first.
- Key types in the first milestone: `String`, `Int`, and possibly enum types if hashing/equality is already deterministic for enums.
- Defer `Bool` keys unless a real use case appears; `Map<Bool,V>` is usually better modeled as a two-field record or branch.
- Lookup must be fallible: missing key returns `Error` or a future option/result value.
- Missing key must never return default zero.
- Mutation can be deferred; immutable construction plus lookup is enough for parameter tables.
- Map literal syntax can be deferred if `Map.fromPairs` or a small builder exists.
- If map literals are included, prefer one syntax and avoid overloading record literals until the grammar is clear.

Candidate literal syntax remains attractive but should be v0.2 unless maps become a must-have:

```oct
let m = map[String, Float] {
    "alpha": 1.0
    "beta": 2.0
}

let alpha = m["alpha"]?
```

The alternative:

```oct
let m: Map<String, Float> = {
    "alpha": 1.0
    "beta": 2.0
}
```

is terser, but it risks ambiguity with future record/object literal forms and depends more heavily on expected-type parsing.

## Generics recommendation

### Are user-defined generics needed for slices?

No.
Array slices can be implemented as a compiler/runtime operation over the existing `T[]` array type without exposing user-defined type parameters.
The result type is mechanically the same array element type.

### Are user-defined generics needed for maps?

For a general user-facing `Map<K,V>`, yes eventually.
A builtin/compiler-owned map could exist before user-defined generics, but a reusable map API will quickly need type parameters for constructors, lookups, values, and helper functions.

### Are bounded generics needed for array helpers?

For high-quality reusable helpers, yes eventually:

- `Equatable` for `Contains`, `IndexOf`, deduplication, group keys;
- `Ordered` for sort, min/max, clamp over generic ordered values;
- `Numeric` for sum/mean/reductions;
- `Hashable` for map keys and sets;
- `UnitNumeric` or dimension-preserving numeric constraints for `Float<D>`/`Int<D>` scientific helpers.

Without bounded generics, v0.1 can still ship concrete helpers for common cases (`Float[]`, `Int[]`, `String[]`, `Complex[]`) and rely on compiler-owned surfaces for special cases.

### Can v0.1 ship without user-defined generics?

Yes.
This audit recommends v0.1 ship without user-defined generics if the release includes:

- clear documentation that generic-looking forms are compiler-owned only;
- 1D array slice syntax or concrete slice helpers;
- concrete `Float[]` scientific helpers in `Analysis`/`Mathematics` where they materially reduce scripts;
- no promise of user-authored `Map<T,U>`/`Reduce<T>` until v0.2.

### Minimum generic capability that would unblock library helpers

For v0.2, the smallest useful library-author feature is:

```oct
fn Map<T, U>(xs: T[], f: fn(T) -> U) -> U[]
fn Filter<T>(xs: T[], pred: fn(T) -> Bool) -> T[]
fn Reduce<T, U>(xs: T[], initial: U, f: fn(U, T) -> U) -> U
fn ZipWith<T, U, V>(xs: T[], ys: U[], f: fn(T, U) -> V) -> V[] ! Error
```

Then add constraints in a second step:

```text
Equatable
Ordered
Numeric
Hashable
UnitNumeric
```

Do not start with bounded generics before plain type parameters work; otherwise the design surface becomes too large.

## String.From<T> recommendation

### Current behavior

`String.From<T>` is documented and works in interpreted mode for compiler-known `Int`, `Float`, `Bool`, and `String` in the F1 probe.
It rejects enum, array, and dimensioned numeric type arguments with a clear M0 diagnostic.
`FormatFloat(value, precision)` works for dimensionless `Float` precision control.

### F1 compiled behavior gap (resolved by F4 for scalar conversions)

Before F4, running the F1 probes with `--execution compiled` produced:

```text
compiled execution required: function LanguageFrictionArrayMapGenerics.FloatLabels: unknown function 'String.From'
compiled execution required: function LanguageFrictionArrayMapGenerics.StringFromCompilerKnownScalarsWorks: unknown function 'String.From'
```

Auto mode fell back to interpreted for the affected tests. F4 resolves this scalar lowering gap for `Int`, `Float`, `Bool`, and `String`; enum, array, record, and dimensioned numeric conversions remain intentionally unsupported.

### Recommended API direction

Keep:

```oct
String.From<Int>(x)
String.From<Float>(x)
String.From<Bool>(x)
String.From<String>(x)
```

as the preferred explicit namespaced API for report/library code.
Keep `ToString(x)` as compatibility/backing surface for existing code.
Do not add automatic record/array formatting in M0; require explicit formatting APIs so scientific reports remain deterministic and avoid accidental huge output.
Enums can be considered after enum reflection/name design is explicit.
Dimensioned numeric formatting should likely be a separate unit-aware API, for example `Units.Format(value, precision)`, rather than overloading scalar `String.From<Float>`.

## SI board fields recommendation

The requested SI board probe is materially supported today:

- `Float<K>` board field parses and typechecks.
- Assignment from a `Float<K>` flow parameter works.
- Interpreted execution preserves the unit in `BoardSnapshot`.
- Compiled execution also passed the F1 snapshot probe.

Recommended v0.1 action is documentation/test hygiene rather than feature work:

1. Done in F6: Octomata board snapshot wording says scalar numeric board fields include dimensioned `Int<D>`/`Float<D>`.
2. Add/keep a normative language test under `Language/ControlFlow/...` only if this is intended as a stable language contract.
3. If compiled support is not intended to be guaranteed, update `docs/COMPILED_SUPPORT.md` instead.

F1 did not add a normative test because it is an audit task and the probe remains non-normative.

## Proposed v0.1 feature list

### Must-have for v0.1

1. **Conservative 1D array slices** if any new ergonomics feature is accepted:
   - `xs[start:end]`, `xs[start:]`, `xs[:end]`, `xs[:]`;
   - copy semantics;
   - no negative indices;
   - no step;
   - no vector/matrix/tensor slicing.
2. **Compiled scalar `String.From<T>` parity** for documented M0 type arguments (`Int`, `Float`, `Bool`, `String`) is resolved by F4 and should remain covered.
3. **Documentation clarity** that user-defined generics and maps/dictionaries are absent in v0.1, even though compiler-owned generic-looking forms exist.
4. **SI board snapshot documentation cleanup** if dimensioned scalar board fields are intended to be supported.

### Nice-to-have for v0.1

1. Concrete `Float[]` helpers in `Analysis` or `Mathematics` for common workflows:
   - `Window`/`Take`/`Drop` if slice syntax does not land;
   - `Sum`, `Mean`, `Standardize`, `Normalize`, `MovingAverage` if not already covered by the relevant library;
   - `ZipAdd`/shape-check helpers where scientific scripts repeatedly need them.
2. Concrete `Int[]` and `String[]` helpers only where real scripts need them; avoid broad pre-generic API sprawl.
3. Matrix row/column extraction helpers returning arrays:
   - `Matrix.rowArray(m, row) -> T[] ! Error`;
   - `Matrix.columnArray(m, col) -> T[] ! Error`;
   - defer vector-returning helpers until row/column orientation is designed.
4. Unit-aware formatting helper exploration, separate from automatic `String.From` for dimensioned values.

## Proposed v0.2 deferrals

- Full typed `Map<K,V>`.
- Map literals.
- User-defined generic functions/types.
- Bounded generics and constraints (`Equatable`, `Ordered`, `Numeric`, `Hashable`, `UnitNumeric`).
- `Array.Map`, `Array.Filter`, `Array.Reduce`, `Array.ZipWith` as generic standard-library helpers.
- Iterator protocols and lazy views.
- Step slices (`xs[start:end:step]`).
- Negative indices, if ever wanted.
- Vector/matrix/tensor slicing:
  - `m[row, col]` remains scalar access;
  - `m[rowStart:rowEnd, colStart:colEnd]`;
  - `m[:, col]`;
  - `m[row, :]`;
  - rank/shape/orientation rules;
  - copy/view policy for tensor storage.
- Automatic enum/record/array string conversion.
- Unit-aware formatting and report rendering beyond scalar `FormatFloat`.

## Appendix: probe snippets and diagnostics

### Working probe commands

```sh
go run ./cmd/oct test Experiments/LanguageFriction/ArrayMapGenerics --execution interpreted
```

Result:

```text
Result: 6 passed, 0 failed, 0 skipped
```

```sh
go run ./cmd/oct test Experiments/LanguageFriction/ArrayMapGenerics --execution auto
```

Result:

```text
Result: 6 passed, 0 failed, 0 skipped
```

During F1, auto mode reported interpreted fallback for tests using `String.From` and several Assert-backed cases, while the SI board snapshot probe compiled. After F4, scalar `String.From<T>` no longer accounts for fallback; remaining strict compiled probe failures are separate Assert/package lowering gaps.

### Compiled probe command

```sh
go run ./cmd/oct test Experiments/LanguageFriction/ArrayMapGenerics --execution compiled
```

Historical F1 result: failed for 5 probe tests due to compiled-support limitations, including then-missing `String.From` lowering and Assert package lookup in these experiment tests; after F4, the scalar `String.From<T>` cases compile while Assert package lookup remains separate.
This is useful audit evidence, not a production regression.

### Expected-fail probes

Expected-fail snippets are stored as `.oct.disabled` files so normal `oct test` discovery does not parse them as package sources.
They are design probes, not normative tests.

#### Array slice syntax

File: `Experiments/LanguageFriction/ArrayMapGenerics/expected_fail/slice_syntax_not_supported.oct.disabled`

```oct
return xs[1:3]
```

Diagnostic:

```text
expected ']' after index expression ... near ":"
```

#### Matrix slice syntax

File: `Experiments/LanguageFriction/ArrayMapGenerics/expected_fail/matrix_slice_not_supported.oct.disabled`

```oct
return m[:, 0:1]
```

Expected current outcome: parser rejects range components in matrix brackets.

#### Map literal / dictionary

File: `Experiments/LanguageFriction/ArrayMapGenerics/expected_fail/map_literal_not_supported.oct.disabled`

```oct
let m = map[String, Float] {
    "alpha": 1.0
}
```

Diagnostic:

```text
expected statement ... near "{"
```

#### User-defined generic function

File: `Experiments/LanguageFriction/ArrayMapGenerics/expected_fail/user_defined_generic_function_not_supported.oct.disabled`

```oct
fn Identity<T>(x: T) -> T {
    return x
}
```

Diagnostic:

```text
expected '(' after function name ... near "<"
```

#### `String.From` enum conversion

File: `Experiments/LanguageFriction/ArrayMapGenerics/expected_fail/string_from_enum_not_supported.oct.disabled`

```oct
return String.From<Mode>(Mode.Fast)
```

Diagnostic:

```text
String.From<T> supports Int, Float, Bool, and String in M0
```
