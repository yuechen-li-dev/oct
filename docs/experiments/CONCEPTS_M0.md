# Concepts M0

## Verdict

Concepts M0 supports named and record-shaped value concepts and compiler-owned compile-time requirements. Behavioral concepts are deferred because Oct does not yet have a user-defined conformance relation, generic specialization, or a real behavioral consumer.

The adopted definition is:

> A concept is Oct's user-facing compile-time description of a valid value shape and its compile-time requirements.

The decisive specimen passes: Oct can name a domain value shape, construct and use a record-shaped concept normally, reject an invalid model before Go generation, and emit ordinary concrete Go with no concept registry or delayed `Require` failure.

## Grammar and identity

```oct
concept Length = Float<m>

concept Position {
    X: Length
    Y: Length
}
```

Named value concepts are transparent aliases. They expand to existing scalar, unit, array, vector, matrix, function, record, or enum types before ordinary checking and emission. They introduce no conversion, wrapper, allocation, Go declaration, or runtime object.

Record-shaped concepts are nominal product values. The parser records their concept origin, then lowers them through the existing record checker, interpreter, equality, update, and Go-struct paths. This mixed identity is deliberate: unit vocabulary benefits from zero-cost transparency, while record construction already relies on stable nominal names.

`record` remains accepted for compatibility, record tables are unchanged, and `enum` remains the sum declaration. M0 does not claim that products, sums, scalars, dimensions, and future behavioral descriptions share one internal compiler representation.

Alias cycles and direct by-value record-concept cycles are rejected. Array indirection is not treated as a direct by-value cycle. Alias concepts are package-local in M0; package-qualified aliases need an exported alias symbol model before they can be supported honestly. Record-shaped concepts retain existing package-qualified record behavior.

## Compile-time `Require`

The actual pre-existing spelling was singular `Require`, not `Requires`. M0 accepts:

```oct
Require(condition)
Require(condition, "explanation")
```

The condition must be `Bool`; the explanation, when present, must be a compile-time `String`. A true requirement erases before interpretation and MIR lowering. A false requirement reports the Oct path, line, column, enclosing function, source expression, and explanation. A condition that cannot be evaluated during checking is rejected with no runtime fallback.

The bounded proof evaluator supports Boolean, integer, string, and array literals; immutable `let` bindings in that subset; Boolean and integer unary/binary operations; integer comparisons; string concatenation/equality; and `Len` of a statically shaped array. Parameters, mutable state, indexing, field inspection, arbitrary calls, loops, I/O, process/environment access, time, randomness, and networking are not evaluable. This pass reuses the existing typed AST and scope; it is not an interpreter, macro system, or second VM.

The repository audit found 110 previous `Require` uses, not the three initially visible in UI:

- 101 production-library uses: 98 dynamic preconditions and three UI checks;
- nine language-contract uses, including the old runtime behavior and arity contracts.

All production uses were migrated. The 98 non-recoverable compatibility preconditions now use explicit runtime `Assert.True`; the three UI checks use fallible validation with deliberate unwraps. `Assert.True` is therefore the explicit runtime assertion path in ordinary source, while other `Assert.*` helpers remain test-only. The old language fixture is now a compile-time requirement contract, one-argument `Require` is valid, and zero or more than two arguments are rejected. No plural alias was added.

## Specimen A: unit vocabulary

`Libraries/Units/Units.Core.oct` now names `Length`, `Duration`, `Speed`, `Acceleration`, and `Temperature` as transparent concepts. `AverageSpeed` and `Accelerate` prove that dimensional arithmetic remains the authority. The Units suite passes 43 interpreted and 43 compiled tests; the Concepts corpus separately rejects assigning a length to a speed.

The unit specimen grew from 40 to 51 nonblank source lines and its focused test from 17 to 24. This does not redesign dimensional arithmetic or change non-SI record wrappers.

## Specimen B: compiled model lock

`tools/compiled_model_lock/audit_stages.oct` now declares `StageSpec`, `AuditStage`, and `GeneratedAuditStages` as record-shaped concepts. Two real invariants assert exactly 29 noise stages and 16 context stages before rendering.

The valid check leaves `audit_stages.generated.go` byte-identical. A dedicated invalid OctGen fixture reduces the operations array and fails during Oct type checking with source provenance before renderer decoding or output replacement. The host keeps its external-boundary validation, but it is no longer the sole authority for these counts.

Measured with the same nonblank-line convention as the earlier OctGen notes, the generator changed from 157 to 159 lines and emitted Go stayed 454 lines. A warm M1 check completed in about 425 ms on this checkout, compared with the previously recorded 565 ms warm generation measurement; no material concept overhead was observed, although this is not a controlled before/after benchmark. Host decoding is unchanged.

## Erasure and Go representation

Named concepts erase to their concrete existing Go types. Record-shaped concepts emit the same ordinary structs as records. The compiler drops accepted `Require` expression statements before MIR construction, and the old Go `panic` emission case was removed.

The integration test inspects generated source and MIR: it finds the concrete `Main_Motion` struct and finds no `concept`, `Require`, explanation text, requirement helper, conditional, or panic. The committed M0 Time and M1 audit outputs both remain byte-identical to `HEAD`; the M1 generated file contains none of `Require`, `requirement`, `concept`, or `panic`.

## Diagnostics and contracts

The Concepts corpus has one 31-line valid contract and 12 invalid contracts covering duplicate and unknown concepts, malformed right-hand shapes, alias and record cycles, missing/extra/wrong record fields, incompatible units, false requirements, non-Boolean conditions, and non-evaluable conditions. Parser tests cover both declaration shapes and malformed recovery. The invalid OctGen fixture covers early rejection and Oct source provenance.

Focused diagnostics avoid Go implementation terms. The false-requirement form is:

```text
path:line:column: compile-time requirement failed in Function: explanation (condition: expression)
```

No unsupported concept form reaches the Go backend in M0; unsupported aliases are rejected before lowering.

## Compatibility evidence

- Existing records and record updates: RecordTables and RecordUpdateM9 pass interpreted and compiled.
- Units and dimensional checking: UnitsM1 and the 43-test Units library pass interpreted and compiled.
- Enums and exhaustive matching: EnumsAssociated passes 10 interpreted and 10 compiled contracts, including non-exhaustive rejection.
- Runtime-precondition migration: Cooking 40, Mechanics 67, Octomata 92, Random 22, and Statistics 44 tests pass in both modes with zero fallback (invalid contracts are not compiled).
- OctGen: internal/public tests pass; M0 Time and M1 audit checks pass; the compiled model lock hash remains valid.
- The normal `cmd/oct` build and the focused integration backend test pass.
- No local TSPack tree, `concepts.oct`, or OctGen M2 document was present, so that bounded generator check could not be run. `experimental/octgen` public tests were run instead.
- The standalone `Language/ControlFlow/EnumPayloadMatchCompiled` root remains locally unwired (`unknown package 'Main'`); the working EnumsAssociated exhaustive-match lane passed and no files in the unwired fixture were changed.

## Measurements

Production implementation change, counted from the diff:

- parser/AST/lexer production code: 84 added, 7 removed lines;
- concept expansion and project binding: 442 added lines;
- type checker, including requirement checking: 235 added, 16 removed lines;
- bounded compile-time evaluator: 169 added lines within the type checker;
- interpreter: 23 added, 23 removed lines, including record/array equality parity and `Require` erasure;
- Go backend: 10 added, 7 removed lines; one runtime `Require` emission route removed;
- compatibility migration: 101 production `Require` sites removed across 16 library files; 98 became explicit runtime assertions and three UI sites became fallible validation;
- real invariants moved to compile time: two;
- semantic contracts added: one valid and 12 invalid Concepts cases, one updated `Require` contract, focused lexer/parser/backend tests, a Units fact, and an invalid OctGen fixture;
- generated Go: 454 to 454 nonblank lines, byte-identical;
- host decoding: unchanged.

The 421-line concept expansion pass is intentionally separate from the record implementation: it recursively rewrites type references once, then converges on existing typed representations instead of duplicating record, unit, interpreter, or backend logic.

## Files changed

- language implementation: `internal/ast/program.go`, `internal/lex`, `internal/parse`, `internal/concept/expand.go`, `internal/project/project.go`, `internal/typecheck/typecheck.go`, `internal/interpret/interpret.go`, and `internal/build/compiler.go`;
- implementation integration tests: lexer/parser tests, `internal/build/compiler_test.go`, `internal/octgen/octgen_test.go`, and `internal/octgen/testdata/invalid_requirement/generator.oct`;
- semantic contracts: `Language/Types/ConceptsM0`, the `Language/Functions/Calls` requirement fixtures, and the Units fact;
- specimens: `Libraries/Units/Units.Core.oct`, `tools/compiled_model_lock/audit_stages.oct`, and `Examples/ConceptsM0/main.oct`;
- compatibility migrations: Cooking, Mechanics Endurance/Fatigue/Notch/Shafts/Stress, Octomata AntiWindup, Random Core/CoinToss/Dice/Distributions, Statistics Core/Regression/Summary, and UI Style;
- reference and evidence: `Language/reference/README.md`, types, units, builtins, records, the new concepts chapter, and this report.

No Prometheus files were modified and no GPU/model workloads were run.

## Ergonomics and remaining friction

M0 materially improves domain vocabulary for units, declaration vocabulary for record-shaped models, and source-located compile-time invariant declaration. It does not remove required record-literal field labels, add collection transformations, simplify OctGen host decoding, or eliminate provenance plumbing. Record construction remains ordinary but equally verbose.

Remaining friction is classified as:

- syntax: package-qualified alias concepts are absent;
- concept identity/compatibility: nominal aliases and refinements are undefined;
- record construction: repeated field labels remain;
- collection transformation: compile-time map/filter helpers remain absent;
- compile-time evaluation: intentionally bounded to a small constant subset;
- host decoding: unchanged;
- project embedding and tool distribution: unchanged from OctGen M2/public APIs;
- unrelated consumer API design: runtime validation remains the responsibility of each library API.

## Deferred behavior and recommendation

M0 inspected functions and signatures but found no existing user-defined conformance, specialization, or overload-search seam sufficient for behavioral concepts. Adding string-named operations would not establish one. Templates, macros, reflection, quotation, type objects, and a separate comptime language were therefore not added.

The next milestone should **stabilize named and record concepts**, especially exported/package-qualified transparent aliases and formatter coverage. Concept refinements or one bounded behavioral specimen should wait for a real consumer that can justify conformance and specialization semantics.
