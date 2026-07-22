# Concepts

## Definition

A concept is Oct's user-facing compile-time description of a valid value shape and its compile-time requirements. Concepts-M0 supports two declaration shapes:

```oct
concept Length = Float<m>

concept Position {
    X: Length
    Y: Length
}
```

A named value concept is a transparent alias for one existing concrete Oct type. A record-shaped concept is a nominal product value whose fields are checked by the existing record implementation. These two identity rules are intentionally mixed: unit vocabulary needs zero-cost transparency, while constructed record values need stable nominal identity.

## Named value concepts

Named concepts may describe primitive, dimensioned numeric, array, vector, matrix, function-signature, record, or enum types already accepted by the type grammar. Uses are expanded before ordinary type checking. Array use composes normally: if `Length` describes `Float<m>`, then `Length[]` describes `Float<m>[]`.

Aliases do not add implicit conversions. They emit no Go declaration, wrapper, allocation, descriptor, registry, or reflection object. Alias cycles are rejected.

Concept aliases are currently package-local in M0. A package may use its concepts across its own source files, but package-qualified concept alias references are deferred. Record-shaped concepts retain the existing package-qualified record behavior.

## Record-shaped concepts

`concept Name { fields }` lowers directly to an ordinary nominal record declaration. Construction, field access, immutable `with` updates, arrays, equality, argument passing, return values, interpreted execution, Octagon-compatible record behavior, and Go struct emission therefore use the established record paths. Missing, extra, duplicate, and incorrectly typed fields are rejected by the record checker.

Existing `record` syntax remains compatible. `concept` is preferred when a declaration names a valid domain value shape; `record` remains accepted and `record table` is not replaced. Enums remain nominal sums expressed with `enum`; Concepts-M0 does not invent a sum-form concept syntax.

Direct by-value cycles between record-shaped concepts are rejected. Array indirection is not a direct by-value cycle.

## Compile-time requirements

`Require` is a compiler-owned assertion statement:

```oct
fn Generate() -> Model {
    let stages = ["load", "check", "emit"]
    Require(Len(stages) == 3, "model must retain all stages")
    return BuildModel(stages)
}
```

The condition must have type `Bool`. The optional explanation must be a compile-time `String`. A true requirement erases before interpretation or Go lowering. A false requirement stops compilation at its Oct source location and reports the enclosing function, explanation, and condition. A non-evaluable requirement is rejected; there is no runtime fallback.

The M0 proof evaluator is deliberately bounded to:

- Boolean, integer, string, and array literals;
- immutable `let` bindings whose initializer is in this subset;
- `not`, integer unary minus, Boolean `and`/`or`;
- integer arithmetic and comparisons;
- Boolean and string equality/inequality, plus string concatenation;
- `Len` over a statically shaped array literal or immutable binding;
- parentheses.

Parameters, `var`, assignments, indexing, record-field inspection, arbitrary function calls, loops, file/process/environment access, clock, randomness, network access, and unrestricted interpreter execution are outside the M0 requirement boundary. This is a small compile-time proof pass in the existing checker, not a second interpreter or comptime VM.

Legacy dynamic `Require` preconditions were migrated to explicit runtime validation. Recoverable APIs should use fallible `Error` results; existing non-recoverable library boundaries use `Assert.True`, which remains runtime behavior and is not a concept requirement.

## Erasure and backends

The interpreter executes only programs whose requirements have already passed and treats accepted `Require` statements as erased. The Go backend does not lower them into MIR statements and emits no `Require` helper, conditional, panic, concept registry, or type descriptor. Named concepts lower to their concrete Go representation; record-shaped concepts lower to the same structs as records.

## Relationship to future behavior

Function signatures already describe parameter and return value shapes, and compiler-owned builtins already demonstrate bounded polymorphism. M0 does not have a user-defined conformance relation, specialization mechanism, overload search, or generic-function implementation, so behavioral concepts and concept-constrained functions are deferred. Future work should add one real behavioral consumer before choosing conformance or specialization syntax.

Concepts-M0 does not add `type`, `interface`, `trait`, `template`, `constraint`, or `comptime` declarations. It also adds no macros, quotation, compiler-object access, reflection, or runtime type enumeration.

## Rejected alternatives

- Keeping only `record` plus an unrelated alias declaration would preserve two neighboring public vocabularies instead of testing one concept boundary.
- Runtime records describing types would require reflection and residual metadata that the motivating specimens do not need.
- Immediately nominal aliases would force conversions or wrappers around unit-bearing values without evidence.
- String-named operations are not a behavioral concept model and were not added.

## Diagnostics

Focused diagnostics cover duplicate concepts, unknown referenced types, invalid right-hand shapes, alias cycles, unsupported direct record cycles, record construction fields, non-`Bool` conditions, false requirements, and non-evaluable requirements. Backend erasure is verified by compiled corpus execution and generated-source inspection.
