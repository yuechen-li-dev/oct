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

## Refined concepts (M1)

A refined concept names the admissible subset of one existing scalar or array representation. It has distinct static identity but the same runtime data representation:

```oct
concept ColorChannel = Int {
    Require(Self >= 0, "a color channel must be at least 0")
    Require(Self <= 255, "a color channel must be at most 255")
}
```

`Self` denotes the candidate value and is in scope only inside the refinement requirements. Each requirement must be pure, non-fallible, and Boolean. M1 accepts the bounded literal/operator subset used by compile-time `Require`, plus floating-point literals and `Len(Self)` for array refinements. Calls other than `Len`, mutation, I/O, process/environment access, time, randomness, reflection, compiler objects, and host authority are rejected.

Known construction uses expected-type context:

```oct
let high: ColorChannel = 200 + 55 // proved while checking; no runtime branch
```

A known value that fails a requirement is rejected at that binding, argument, return, record field, array element, or assignment. An unknown base value is not admitted implicitly:

```oct
fn Admit(raw: Int) -> ColorChannel ! Error {
    return ColorChannel(raw)?
}
```

`Name(value)` is the explicit checked constructor for a package-local refined concept. It is fallible and therefore composes with `?`, `!`, and `match ok/err`. The compiler generates one ordinary fallible boundary function from the same typed requirement expressions used for static proof. On success its value has the refined identity; subsequent binding, parameter passing, returning unchanged, record/array storage, and indexing do not check again.

Refined scalar to underlying representation is permitted and zero cost. Underlying representation to refined requires either bounded compile-time proof or explicit checked construction. Arrays remain invariant because mutable base-array access could otherwise insert an unchecked element. Equality and comparison consume the underlying values. Arithmetic and unary numeric computation conservatively return the underlying type, so a result must be proved or checked before it regains refinement.

Transparent M0 aliases of a refined concept preserve the refinement identity. A refinement base may be a transparent alias that resolves to a supported representation. Refining a refined concept is rejected in M1; cycles are diagnosed. Record, enum, function, flow, vector, matrix, error, void, range, UI, bytes, and nominal-record bases are not supported refinement bases. Dimensioned `Int<D>` and `Float<D>` bases retain their unit dimension.

The interpreter executes explicit checked construction through the synthesized ordinary fallible function. Go lowering emits a type alias to the concrete representation and only the constructor's requirement branches. Statically proved construction, parameters, fields, arrays, and downstream uses emit no descriptor, registry, reflection metadata, wrapper allocation, or repeated validation.

Package-qualified refined types and statically proved arguments through imported function signatures are supported. Package-qualified checked-constructor spelling is not yet supported; checked construction currently occurs inside the declaring package or through a package API. General range inference, symbolic execution, dependent types, operation-preservation inference, behavioral conformance, templates, and specialization are not part of M1.

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
