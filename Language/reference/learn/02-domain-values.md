# 2. Values with meaning

[Previous](01-first-program.md) · [Reference home](../README.md) · [Next](03-functions-and-alternatives.md)

Put meaning in a type once, then let the compiler carry it through the program.
Here a journey owns a distance and a duration; its speed must have the right
dimension, and a zero duration is an explicit error.

<!-- verified-example: ../../ReferenceExamples/valid/domain_values.octest -->
```oct
package ReferenceExamples

concept Distance = Float<m>
concept Duration = Float<s>
concept Speed = Float<m/s>

concept Journey {
    Length: Distance
    Elapsed: Duration
}

fn JourneySpeed(journey: Journey) -> Speed ! Error {
    if journey.Elapsed <= 0.0s {
        return error("elapsed time must be positive")
    }
    return journey.Length / journey.Elapsed
}

[Fact]
fn DomainValuesKeepTheirMeaning() -> Void ! Error {
    let walk = Journey { Length: 12.0m Elapsed: 3.0s }
    let longer = walk with { Length: 20.0m }
    Assert.Equal(4.0m/s, JourneySpeed(walk)?, "dimensions compose")
    Assert.Equal(12.0m, walk.Length, "with preserves the original")
    Assert.Equal(20.0m, longer.Length, "with constructs an updated value")
    Assert.Error(JourneySpeed(walk with { Elapsed: 0.0s }), "bad input is explicit")
}
```

Run the [source](../../ReferenceExamples/valid/domain_values.octest) with
`oct test <source> --execution compiled`.

`concept Distance = Float<m>` is a transparent name for a dimensioned number.
`concept Journey { ... }` defines a nominal product value: another declaration
with the same fields is a different type. `record` remains supported for the
same product-value machinery. Prefer `concept` for a domain value description;
use `record table` for a validated table.

`with` constructs an updated value and preserves the original. Ordinary record
fields are not mutated in place. A `var journey` may be rebound to the new
value. FLOW boards are the separate state-owned mutation surface.

## Recoverable failure belongs in the signature

`Speed ! Error` means the call may fail. `?` returns an error to the enclosing
fallible caller; it does not turn a bad value into a speed. The test itself
declares `Void ! Error`, so it can propagate unexpected failure.

Use statement `match Call() { ok(value) => { ... } err(problem) => { ... } }`
when recovery needs different behavior. Use postfix `Call()!` only at a
deliberate fatal-unwrap boundary. Prefix `!condition` is not Boolean negation;
write `not condition`.

## Validate a value once

A refined concept adds admissibility requirements to a scalar or array:

```oct
concept PositiveCount = Int {
    Require(Self > 0, "count must be positive")
}
```

A known `let count: PositiveCount = 3` is proved during checking. An unknown
`Int` crosses the explicit fallible constructor `PositiveCount(raw)` and must
be handled. Accepted values retain their identity without repeated checks.
Arithmetic returns the base numeric type, so its result needs fresh admission.

`Require` is a bounded compile-time proof facility, not a runtime precondition.
For runtime input validation, follow the ordinary `if` plus `Error` pattern
above. See [concepts and refinements](../language/concepts-and-refinements.md) for the proof
subset and [units](../language/units.md) for conversions and unit stripping.
