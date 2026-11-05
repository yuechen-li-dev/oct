# 3. Functions and alternatives

[Previous](02-domain-values.md) · [Reference home](../README.md) · [Next](04-collections.md)

Functions are values with exact parameter, result, and fallibility types.
An anonymous function lists captured values explicitly, and each capture is a
snapshot evaluated when the function is constructed.

<!-- verified-example: ../../ReferenceExamples/valid/functions_and_options.octest -->
```oct
package ReferenceExamples

fn MakeOffset(offset: Int) -> fn(Int) -> Int {
    return fn(value: Int) -> Int with { offset: offset } {
        return value + offset
    }
}

fn FirstPositive(values: Int[]) -> Option<Int> {
    for i in 0..Len(values) {
        if values[i] > 0 { return Option.Some(values[i]) }
    }
    return Option.None
}

fn PositiveOrZero(values: Int[]) -> Int {
    return match FirstPositive(values) {
        case Option.Some(value) => value
        case Option.None => 0
    }
}

[Fact]
fn FunctionsAndMissingValuesAreExplicit() -> Void {
    let addFive = MakeOffset(5)
    Assert.Equal(8, addFive(3), "a function keeps its captured value")
    Assert.Equal(4, PositiveOrZero([-2, 0, 4]), "match reads the payload")
    Assert.Equal(0, PositiveOrZero([]), "absence is an ordinary alternative")
}
```

Run the [source](../../ReferenceExamples/valid/functions_and_options.octest)
with `oct test <source> --execution compiled`.

The result of `MakeOffset` has type `fn(Int) -> Int`. The `with` after its
signature names the environment: the left `offset` is the capture's name,
the right `offset` is the value in the surrounding invocation. The function
keeps that value even after its creator returns. Captures are immutable;
there are no implicit captures or shared mutable capture cells.

## Absence and error answer different questions

`Option<Int>` is the builtin enum `None | Some(Int)`. It needs no import.
An empty search is a normal alternative, so `FirstPositive` returns `None`.
A malformed input or failed operation can instead use `T ! Error` to explain
the failure. A function can return `Option<T> ! Error` when it needs both.

The result type supplies the option's `T` to `Option.Some` and `Option.None`.
Where no expected option type exists, write it explicitly, such as
`let missing = Option<Int>.None`. Oct does not infer it from a payload.

Enum `match` is an exhaustive expression with `case` arms. It reads payloads
and produces a value. Fallible `match` is a statement with `ok`/`err` blocks.
`?` and `!` handle fallible results; they do not unwrap options.

For your own alternatives, declare `enum Response { Missing Found(Reading) }`.
One payload may itself be a domain-shaped concept, so multi-field information
does not require tuple or nested-pattern syntax.

## Reuse the type you mean

Use an ordinary named function for reusable computation. Pass a function value
for a callback, an explicit capture when it needs configuration, and a
`template fn` when several concrete types need the same checked source.
Template applications always write type arguments; Oct does not have general
template argument inference or a trait search system.

More detail: [function values](../language/function-values.md),
[enums and options](../language/enums-and-options.md), [errors](../language/errors.md),
[templates](../language/templates-and-selectors.md).
