# Enums

## Overview

Enums are nominal sum types with named variants.
Variants are either tag-only or single-payload.
Variant references are qualified.
`match` is the payload-binding analysis form for enums.

## Rules

- Enum declaration form is `enum Name { Variant ... }`.
- Supported variant forms are:
  - `Variant` (tag-only)
  - `Variant(Type)` (single payload)
- Variant construction forms are:
  - `Name.Variant`
  - `Name.Variant(value)` for payload variants
- `match` over enum variants is exhaustive and binds payload names per case:
  - `case Name.Variant(v) => ...` for payload variants
  - `case Name.Tag => ...` for tag-only variants
- Same-enum values support equality and inequality with `==` and `!=`; the result is `Bool`.
- Enum values from different enum types are not comparable.
- Enum ordering comparisons (`<`, `>`, `<=`, `>=`) are rejected; enum declarations do not define ordering.
- Enum values are switched by qualified variants.
- `switch` over an enum is exhaustive when all variants are listed.
- Non-exhaustive enum `switch` requires an `else` arm.
- Duplicate enum case labels are rejected.
- Enum identity is nominal by enum name. See [02 Types](./02-types.md).
- `Option<T>` is the one builtin enum; see "Option" below.
- Intentionally out of scope in this milestone:
  - multi-field payloads
  - tuple or record destructuring patterns
  - nested pattern matching and guards



Equality example:

```oct
enum Regime {
    BrownNoiseKalman
    Stabilized
}

fn IsBrown(r: Regime) -> Bool {
    return r == Regime.BrownNoiseKalman
}

fn Changed(a: Regime, b: Regime) -> Bool {
    return a != b
}
```

## Option

`Option<T>` is the builtin enum for a value that may be absent. It has two
variants: `None`, and `Some(T)`, which holds one value of type `T`.

```oct
fn Half(value: Int) -> Option<Int> {
    if value % 2 == 0 {
        return Option.Some(value / 2)
    }
    return Option.None
}

fn OrZero(value: Option<Int>) -> Int {
    return match value {
        case Option.Some(v) => v
        case Option.None => 0
    }
}
```

Rules:

- `Option` is not declared and not imported. A program cannot name a record,
  an enum, a concept, a function, a flow or a package `Option`.
- `Option` is not a reserved word. A parameter or a local may have the name,
  and where one is in scope `Option.Level` reads a field of that value, as
  `vector[i]` indexes a value named `vector`. The type `Option<T>` and a
  `case Option.None` label are the builtin everywhere.
- `T` is any type a record field may have, except `Void`. `Option<Float<m>>`,
  `Option<Reading>`, `Option<Int[]>` and `Option<Option<Int>>` are types, and so
  is an array of options, `Option<Int>[]`.
- `Option<A>` and `Option<B>` are different types when `A` and `B` differ.
  Neither converts to the other, and that includes `Option<Int>` and
  `Option<Float>`.
- A value is not its option. `let x: Option<Int> = 5` is an error; write
  `Option.Some(5)`.
- An option is an enum in every other respect. Its variants are written
  qualified. `match` over it is exhaustive and binds the payload:
  `case Option.Some(v) => ...` and `case Option.None => ...`. `switch` selects on
  `case Option.None` and needs an `else` arm for `Some`. Two values of one
  option type compare with `==` and `!=`; ordering is rejected.
- Equality compares the payloads by value, at any depth: two `Option<Int[]>`
  are equal when both are `None`, or both are `Some` of arrays with equal
  elements.
- `?` and `!` do not apply to an option. They handle a fallible result,
  `T ! Error`, which is a different thing: an error says why there is no
  value, and `None` says only that there is none. A function may return
  `Option<T> ! Error`.
- There are no helper functions on an option. `match` reads it.

### Which option a variant is

`Option.None` and `Option.Some(value)` do not say what `T` is. The place the
value goes says it:

- a binding with a declared type: `let x: Option<Float> = Option.None`;
- an assignment to a variable, to an array element, or to a board field or
  an element of one;
- an argument, where the parameter is an option, and the second argument of
  `Append`, where the array holds options;
- a returned value, where the function returns an option;
- a flow argument, a turn input passed to `Step`, and a `yield`;
- a field of a record literal or of a `with` update, and a cell of a
  `record table` column;
- an element of an array literal that is itself at one of these places;
- the payload of an enum variant, or of another option;
- an operand of `==` or `!=`, or an argument of `Assert.Equal`, where the
  other one has an option type;
- a candidate of `when utility Option<T>`.

An `if`, `match` or `switch` expression and parentheses pass the type on to
their arms, so `let x: Option<Int> = if ready { Option.Some(1) } else { Option.None }`
is typed by the binding.

Anywhere else, write the type argument: `Option<Float>.None` and
`Option<Float>.Some(1.5)`. The written form is valid everywhere, and it is how
an option gets a type where nothing declares one:

```oct
let none = Option<Float>.None
var seen = [Option<Int>.Some(1), Option<Int>.None]
```

`T` is never worked out from the payload. `let x = Option.Some(1.5)` is an
error, because `Option.None` in the same place could not be typed and the two
variants must read alike. The error says to declare the type or to write
`Option<T>.Some(...)`.

The payload is a value of type `T` as a declared type decides it (see
[02 Types](./02-types.md)): `Option.Some(1)` in an `Option<Float>` holds `1.0`.

`Option` is not supported under `profile Verilog` or by the WebAssembly
target; both say so by the type's name.

## Judgment enum utility selection

Enums can serve as closed judgment spaces for one-shot utility-scored selection.
The enum declaration remains ordinary; judgment behavior is introduced at an expression site by the enum-targeted utility form:

```oct
when utility TreatmentDecision {
    case TreatmentDecision.Observe when risk < 0.3 score 40
    case TreatmentDecision.Treat when risk >= 0.6 score 80
    else TreatmentDecision.Observe
}
```

Payload variants are written with explicit qualified construction:

```oct
enum LabDecision {
    Observe
    Retest(Int)
    Treat(Float)
    Escalate(String)
}

when utility LabDecision {
    case LabDecision.Escalate("critical") when risk >= 0.9 score 100
    case LabDecision.Treat(2.5) when risk >= 0.6 score 80
    case LabDecision.Retest(3) when confidence < 0.7 score 70
    else LabDecision.Observe
}
```

Rules:

- The target after `utility` must resolve to an enum type. `Option<T>` is one: `when utility Option<Float> { ... }`.
- The expression type is exactly the target enum type; the target is authoritative and is not inferred from arms.
- Each `case` result and the required `else` fallback must be a qualified variant construction of the target enum. For `Option<T>` these are `Option.Some(value)` and `Option.None`, which take `T` from the target.
- Tag-only candidates use `Enum.Variant`; payload candidates use ordinary explicit construction such as `Enum.Variant(value)`.
- Payload candidates do not bind payloads and do not introduce pattern matching; use `match` to analyze the selected value later.
- Payload expressions are evaluated only after utility scoring selects their candidate, or after no case qualifies and the `else` fallback is selected. Losing candidate payloads are not evaluated.
- Unqualified variants are rejected even when the target enum is known.
- Not every enum variant must appear as a candidate; `else` is still required.
- Conditions must be `Bool`, scores must be `Int`, highest score wins, equal scores keep the earliest matching case, and `else` is selected only when no case qualifies.

`match` and judgment utility have opposite roles:

- `match` analyzes an enum value that has already been selected and must be exhaustive.
- `when utility EnumName` scores candidate variants and produces a selected enum value, with an explicit `else` fallback.

## Examples

Valid:

```oct
package Main

enum ParseResult {
    Ok(Int)
    Err(String)
}

fn Score(result: ParseResult) -> Int {
    return match result {
        case ParseResult.Ok(v) => v * 2
        case ParseResult.Err(msg) => -1
    }
}
```

Invalid:

```oct
package Main

enum ParseResult {
    Ok(Int)
    Err(String)
}

fn Broken(result: ParseResult) -> Int {
    return match result {
        case ParseResult.Ok => 1
        case ParseResult.Err(msg) => 0
    }
}
```
