# Arrays

## Overview

Arrays are homogeneous container values.
Array literals, nested array literals, indexing, assignment, and element-wise arithmetic are supported.
Type matching is exact, including dimensions and nominal names.

## Rules

- Array literal form is `[a, b, c]`.
- Empty array literal form is `[]`, but only in explicit array-typed context.
- All array literal elements must have one exact type.
- `value ... count` in an array literal stands for `count` elements, and `value ...` as the last element fills an array whose length is already fixed. See [Repeated elements](#repeated-elements).
- Array type forms are `T[]`, `T[][]`, and deeper nested container forms.
- Indexing form is `xs[i]`.
- Index expressions must have type `Int`.
- Indexed assignment requires a mutable array binding (`var`).
- Indexed assignment values must match the element type exactly.
- `array[i] = value` replaces one element of a 1D array.
- For a nested two-dimensional array, `rows[i, j] = value` replaces one scalar element and `rows[i] = row` replaces a whole row.
- Whole-row assignment requires an exact row element type and the same runtime length as the destination row. It copies the RHS row value; it does not create mutable aliasing between rows.
- Whole-row assignment checks the row index before writing. Negative and out-of-range indices fail with the normal array bounds error; a length mismatch fails distinctly as `row length mismatch`.
- Nested arrays may still be jagged. A whole-row replacement must match the selected destination row's existing length; it does not resize or regularize the outer array.
- `Append(xs, value)` requires `xs` to be an array.
- `Append` values must match the array element type exactly.
- `Array.CrossSection(xs, range)` requires a 1D array and a `Range`, and returns a new `T[]` copy.
- `Array.CrossSection` preserves the exact array element type, including SI dimensions and nominal record/enum types.
- `Array.CrossSection` resolves omitted range start to `0`, omitted range end to `Len(xs)`, and omitted step to `1`.
- `Array.CrossSection` checks that step is positive, bounds are non-negative and within `Len(xs)`, and start is not after end.
- `Array.Where(values, mask)` requires a 1D array and a `Bool[]` mask, and returns a new `T[]` array containing values whose mask element is `true`.
- `Array.Where` preserves the exact array element type, including SI dimensions and nominal record/enum types.
- `Array.Where` requires `Len(mask) == Len(values)` at runtime; a scalar `Bool` is not a mask, and a length-1 `Bool[]` is not broadcast.
- Element-wise arithmetic requires arrays with the same element type.
- Element-wise arithmetic requires equal runtime lengths.
- Nested arrays are still arrays (containers), not matrix values.
- Nested arrays may be ragged/jagged (`[[1], [2, 3]]`) because they are container-of-container values.
- `[]` never means null/nil; it is a zero-length array with a known element type.
- `[]` can be passed directly as a function or flow call argument when the
  corresponding parameter has a declared array type — the parameter type
  supplies the "expected array type" context, e.g. `Combine([])` is valid
  when `Combine` is declared as `fn Combine(xs: Measurement[]) -> ...`.
- In any other position (e.g. assigned to `var`/`let`, or returned), `[]` still requires an explicit array-typed annotation: `var xs: Int[] = []`.
- Oct does not have Python colon slice syntax: `xs[1:3]` is invalid.
- Oct does not have bracket range extraction in M0: `xs[1..3]` is invalid. Use `Array.CrossSection(xs, 1..3)`.
- Oct does not have logical indexing syntax yet: `values[mask]` is future sugar, not part of ARR2. Use `Array.Where(values, mask)`.
- `Array.CrossSection` is not a view; mutating the result array storage does not mutate the source array storage.
- `Array.CrossSection` and `Array.Where` are for 1D arrays only. Vectors, matrices, and tensors have separate rank-aware APIs and are not accepted as direct values.
- `Array.Where` remains a compiler-owned polymorphic array operation; do not spell it `Array.Where<T>`. User-authored bounded generic helpers use explicit `template fn Name<T>` declarations instead.
- `Array.Where` does not add NumPy-style broadcasting, scalar masks, or matrix/vector/tensor mask indexing syntax.
- Whole-row assignment does not add slices, column assignment, submatrix assignment, broadcasting, shape coercion, or implicit resizing. `rows[i] = [value ...]` writes a new row of the same length; it does not resize.
- Negative indices, reverse ranges, lazy views, `Array.TryCrossSection`, `Array.Copy`, `Array.Take`, `Array.Drop`, and `Array.Window` are deferred/not part of M0.

## Repeated elements

`...` is the ellipsis, and it means what it means in prose: "and so on".

```oct
let zeros = [0.0 ... n]            // n elements, each 0.0
let mixed = [1 ... 2, 2 ... 3, 9]  // 1, 1, 2, 2, 2, 9
let grid = [[0.0 ... cols] ... rows]
```

### `value ... count`

- `value ... count` is an element of an array literal that stands for `count` elements. It can be mixed with ordinary elements and with other repeated elements, in any order.
- `count` is an `Int` expression. It is everything between `...` and the next `,` or `]`, and it need not be a constant.
- A count of zero adds no elements. `[1.5 ... 0]` is an empty `Float[]`: the element still names the type, so no type annotation is needed.
- A negative count is an error: a compile error when the count is a constant, and the runtime error `repeat count must not be negative, got <n>` otherwise.
- The count is evaluated once, before the value. The value is then evaluated once for each element, exactly as if it had been written out that many times. `[Entropy.Seed()! ... 8]` reads the random source eight times, and a `?` in the value or in the count returns its error from the enclosing function.
- Each element is its own value. Writing to `grid[0]` above does not change `grid[1]`.
- The element type is the type of the value. The usual rule holds: every element of the literal has that one type.

### `value ...`

- `value ...` with no count fills the rest of an array whose length something else has already fixed. It is the last element of its literal, and the elements before it come first: `[1, 2, 0 ...]`.
- Three places fix a length:
  - a column of a `record table` literal, which is filled to the table's row count; see [Records](11-records.md#record-tables);
  - a replacement column in a table `with`, which is filled to the table being updated;
  - whole-row assignment, `rows[i] = [value ...]`, which is filled to the length of the row being replaced. The target is a two-dimensional array in a local variable or in a board field.
- Anywhere else nothing fixes the length, and `value ...` is a compile error that says to write a count. A declared type such as `Float[]` says what the elements are, not how many.
- The value is evaluated once for each element it adds. When the literal already has as many elements as the length, it adds none.
- More elements before `...` than the length allows is an error: a compile error where both are constants, and the runtime error ``array literal has <n> elements before `...` and its length is fixed at <m>`` otherwise.
- `rows[i] = [value ...]` checks the index first. A negative or out-of-range index fails with `index <i> out of bounds for array of length <n>`.

### What `...` is not

- It does not spread a collection into arguments or into another literal: `F(xs ...)` is an error.
- It is not a range. A range is `a..b`, with two dots.
- It is not a slice: `xs[1 ...]` is an error. Use `Array.CrossSection`.
- It takes one count. A repeated group is repeated by nesting: `[[0 ... n] ... m]`.
- `.octagon` data does not accept it; a data file writes every element.

`oct fmt` writes one space on each side of `...`, and none before a closing `]`: `[0.0 ... n]`, `[true ...]`.

## Examples

Valid:

```oct
package Main

fn Main() -> Int {
    let grid = [[1, 2], [3, 4]]
    return grid[1][0]
}
```

```oct
package Main

fn Main() -> Int {
    let cols = 3
    var rows = [[0 ... cols] ... 2]
    rows[0] = [1, 2 ...]
    rows[1] = [7 ...]
    return rows[0][2] + rows[1][0]
}
```

```oct
package Main

fn Main() -> Int[][] {
    var rows = [[1, 2], [3, 4]]
    rows[0] = rows[1]
    rows[1, 0] = 9
    return rows
}
```

```oct
package Main

fn Main() -> Int {
    var xs: Int[] = []
    var ys: Float<m>[] = []
    return Len(xs) + Len(ys)
}
```

```oct
package Main

fn Main() -> Int[] {
    let samples = [10, 20, 30, 40, 50]
    return Array.CrossSection(samples, 1..5 step 2)
}
```

```oct
package Main

fn HotSamples() -> Float<K>[] {
    let temps: Float<K>[] = [280.0K, 310.0K, 295.0K, 320.0K]
    let hot = Array.Where(temps, temps > 305.0K)
    return hot
}
```

Invalid:

```oct
package Main

fn Main() -> Int<m>[] {
    var xs = [1m, 2m]
    return Append(xs, 3s)
}
```

```oct
package Main

fn Main() -> Int {
    let zeros = [0.0 ...]
    return Len(zeros)
}
```
