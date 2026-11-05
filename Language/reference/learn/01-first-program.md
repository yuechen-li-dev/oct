# 1. Your first Oct program

[Reference home](../README.md) · Next: [Values with meaning](02-domain-values.md)

You can read most Oct immediately if you know another statically typed
language. Save this whole example as `first_program.octest`:

<!-- verified-example: ../../ReferenceExamples/valid/first_program.octest -->
```oct
package ReferenceExamples

fn SumSquares(values: Int[]) -> Int {
    var total = 0
    for i in 0..Len(values) {
        total = total + values[i] * values[i]
    }
    return total
}

[Fact]
fn AFirstOctProgram() -> Void {
    Assert.Equal(14, SumSquares([1, 2, 3]), "sum of squares")
    Assert.Equal(0, SumSquares([]), "an empty input has no terms")
}
```

Run it with `oct test first_program.octest --execution compiled`. From this
checkout, the ready-made file runs with:

```text
go run ./cmd/oct test Language/ReferenceExamples/valid/first_program.octest --execution compiled
```

The source is a package. A function uses `fn`, typed parameters, and a declared
result after `->`. A function can appear before or after its caller. `Int[]`
is an array of integers; array indices begin at zero.

`let` is the default binding. Here `total` changes, so it is `var`. The loop
`0..Len(values)` includes zero and excludes the array length. Use `for` for
counted iteration; reserve `while` for an evolving termination condition.

`[Fact]` marks a test function returning `Void`. The assertion takes expected
value, actual value, and an explanation. Tests live in `.octest` files; normal
program source uses `.oct`. There is no required semicolon.

## Run a program instead of a test

A normal `.oct` file declares a `Main` entry function. For example,
replace the test above with `fn Main() -> Int { return SumSquares([1, 2, 3]) }`
and keep the package and helper function. `oct run file.oct` interprets it;
`oct build file.oct` produces a native executable. A non-`Void` entry result is
printed. Build output names are described in the [CLI](../tooling/cli.md).

## Familiar syntax, a few deliberate choices

| Intent | Oct spelling |
| --- | --- |
| Immutable binding | `let count = 3` |
| Mutable binding | `var count = 3` |
| Explicit binding type | `let count: Int = 3` |
| Boolean operations | `and`, `or`, `not` |
| Equality | `==`, `!=` |
| Line comment | `// explanation` |
| Function result | `-> Int` |
| Ascending / descending range | `0..10 step 2` / `10..0 descend 2` |

Write ordinary local `if` blocks for conditional work. Use `switch` when
selecting among several outcomes; nested `else { if ... }` decision ladders
are rejected. Oct has no ordinary `break` or `continue`; early `return`, a
guarded body, or explicit `Loop` state covers the supported patterns. See
[control flow](../language/control-flow.md) for examples.

More detail: [variables](../language/variables-and-values.md),
[functions](../language/functions.md), [tests](../tooling/testing.md).
