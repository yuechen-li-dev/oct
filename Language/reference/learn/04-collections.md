# 4. Collections and mathematics

[Previous](03-functions-and-alternatives.md) · [Reference home](../README.md) · [Next](05-choosing-features.md)

Choose a collection by the operations and meaning you need. An array stores
an ordered sequence; a vector or matrix expresses a mathematical value.

<!-- verified-example: ../../ReferenceExamples/valid/collections.octest -->
```oct
package ReferenceExamples

[Fact]
fn CollectionsChooseTheirOwnOperations() -> Void {
    let samples = [1.0, 2.0, 3.0]
    let doubled = samples * 2.0
    let selected = Array.Where(doubled, doubled > 3.0)
    Assert.Equal(2, Len(selected), "an eager mask keeps the matching elements")
    Assert.Equal(4.0, selected[0], "array order is preserved")

    var changed = samples
    changed[0] = 9.0
    Assert.Equal(1.0, samples[0], "arrays have value semantics")

    let direction = vector[1.0, 2.0]
    let identity = matrix[[1.0, 0.0] [0.0, 1.0]]
    let projected = identity @ direction
    Assert.Equal(2.0, projected[1], "@ contracts a matrix with a vector")

    let zeros: Float[] = [0.0 ... 3]
    Assert.Equal(3, Len(zeros), "a repeated element constructs an array")
}
```

Run the [source](../../ReferenceExamples/valid/collections.octest) with
`oct test <source> --execution compiled`.

## Eager array work

An array has one element type. `[1.0, 2.0]` is `Float[]`; `[[1, 2], [3, 4]]`
is `Int[][]`. Bind a typed empty array as `var output: Float[] = []`.
Use a counted `for` loop when the computation needs each index, and append
with `output = Append(output, value)` when building a result.

Array arithmetic is element-wise. One-dimensional numeric arrays also accept
scalar arithmetic and comparisons: the comparison creates a `Bool[]` mask,
and `Array.Where(values, mask)` returns a new filtered array. Equal-length
array operands stay strict; a one-element array is not a broadcast scalar.
There is no NumPy-style rank broadcasting.

Arrays have value semantics. Binding or assigning an array does not give the
new variable a mutable alias to the original. Indexed writes require `var`.
`value ... count` constructs repeated elements; final `value ...` fills a
length already fixed by an assignment or table context. Neither is a spread
operator. `..` is the separate range spelling.

## Mathematical values

Use `vector[...]` and `matrix[[...] [...]]`, or the typed constructors, for
linear algebra. `@` contracts the supported vector/matrix axes; `*` is
element-wise. Explicit Einstein indices and differential operators build on
these same types. Extents are checked but are not type parameters.

An array is not implicitly a vector. A declared numeric element type can admit
documented widening, but it does not reinterpret the container category. For
example, `let values: Float[] = [1, 2]` is allowed; declaring `Vector<Float>`
does not make an ordinary array literal a vector.

For tables, `record table` declares cell types and construction supplies one
array per column. Every column has the same row count. `table[i]` projects a
typed immutable row; `table.Column` projects a typed column. Use a
[query](../runtime/queries.md) for a yielding read-only pipeline.

More detail: [arrays and repetition](../language/arrays.md),
[record tables](../language/records-and-tables.md#record-tables),
[vectors and tensor notation](../language/vectors-matrices-and-tensors.md).
