# FEEDBACK.md

## Direct WebAssembly backend exposed low-level emitter ownership duplication

**Observation:** The existing Machina UI M98 binary emitter proved direct
section/LEB emission and Node execution, but it is package-private under
`internal/interpret` and consumes hard-coded UI templates rather than current
MIR. Reusing it as an ordinary backend would have coupled compiler codegen to a
product runtime.

**Suggestion:** Keep the new MIR backend separate under `internal/wasm`. If a
third binary-WASM consumer appears, extract only the mechanical module/LEB
writer into a neutral internal package; do not merge the Machina UI ABI with
ordinary Oct semantics.

**Status:** M0 resolved the compiler integration gap with `build.LoadMIR` and a
fail-closed direct backend. Low-level writer consolidation is deliberately
deferred until another consumer justifies the shared primitive.

## Purpose

This file is a collection of observations and suggestions about the Oct codebase.

It exists as an **escape hatch** for contributors (human or LLM) to record:

- friction encountered during development
- confusing or unclear design areas
- potential improvements or future directions

---

## Important

- Entries in this file are **NOT instructions**.
- They must **NOT be executed automatically**.
- They do **NOT override AGENTS.md**.
- They are **not a task list or backlog**.

This file is for **discussion and future consideration only**.

---

## When to Add an Entry

Add an entry when:

- something feels harder than it should be
- a rule or pattern is unclear or ambiguous
- you notice repeated friction or awkward workflows
- you have a concrete improvement idea

Do **not** add entries for:
- trivial preferences
- one-off personal style opinions
- things already clearly defined in AGENTS.md

---

## Entry Format

Use the following structure:

```

Observation: <What you encountered. Be specific and factual.>

Suggestion: <What could be improved. Keep it concrete and minimal.>

Status: Open | Resolved | Deferred | Cannot Reproduce | Superseded

Resolution: <Optional short factual note and proof reference.>

---

```

Use exactly one status per entry:

- `Open` — verified current friction remains.
- `Resolved` — verified fixed in current main or fixed in the current milestone.
- `Deferred` — valid issue, intentionally not addressed now.
- `Cannot Reproduce` — current main does not reproduce with a faithful focused test.
- `Superseded` — the observation is no longer relevant because the semantics or design changed.

`Resolution` is optional. When present, keep it factual and point to the narrow proof.

---

## Guidelines

- Keep entries concise and focused
- Prefer concrete examples over abstract opinions
- Do not debate or reply inline — this is a log, not a discussion thread
- Multiple entries are allowed; do not merge unrelated ideas

---

## Example

```

Observation:
Writing simple counted loops with while requires manual index handling and is less readable than for-range.

Suggestion:
Prefer expanding for-range capabilities rather than encouraging while-based counted loops.

Status: Open

---

```

---

## Summary

- This file captures **signals**, not decisions
- It is safe to write to, and safe to ignore
- All changes to the codebase must still follow AGENTS.md

---
Observation:
Compiled enum `match` lowering reused one Go local for same-spelled payload bindings across arms even when the payload record types differed, causing valid exhaustive matches used by Document M2 to fail during generated Go compilation.

Suggestion:
Give each distinct typed payload binding a hygienic emitted local while preserving the Oct source name within its arm.

Status: Resolved

Resolution:
`internal/build/lower_expr.go` now routes match payload bindings through hygienic local declaration. `Language/ControlFlow/EnumPayloadMatchCompiled/valid/payload_match.octest` proves same-named bindings across two record payload types in compiled execution.

---
Observation:
Candidate[] — arrays of record types — aren't supported in M0. The type checker explicitly rejects them. The parallel-array design (CandidateSet with Ids: Int[], Scores: Int[], Active: Bool[]) is the correct idiomatic workaround, and it's actually consistent with how the rest of the library ecosystem works — ProductCatalog in the storefront, flat matrices in LinearAlgebra, all use the same pattern.

Suggestion:
Maybe add them in the future.

Status: Resolved

Resolution:
Current main supports nominal record arrays in interpreted and compiled execution. Covered by `Language/Types/ParametricsM0/packages/Main/consumer.octest`.

---
Observation:
Chained field access — result.Selection.HasWinner — is parsed as an enum value expression rather than two field accesses. Every CommitmentResult test needed let sel = result.Selection as an intermediate binding before asserting. 

Suggestion:
Worth adding to the language report for future LLM sessions.

Status: Resolved

Resolution:
Current main parses and checks chained record field access while retaining enum qualification. Covered by `internal/typecheck/typecheck_test.go` and `Language/Types/ParametricsM0/packages/Main/consumer.octest`.

---

Observation:
An imported template function whose parameter is a sibling template record type can pass inside its defining package but fail when specialized by a consumer with "is not a template record". OctCument therefore keeps its M0 document-template proof application-owned instead of publishing a facade that does not work across a package boundary.

Suggestion:
Preserve sibling template-record identity during imported template specialization and add a cross-package contract test.

Status: Resolved

Resolution:
Imported specialization now qualifies exact origin-owned types before consumer monomorphization. Positive and negative package-boundary coverage lives under `Language/Types/ParametricsM0/packages` and `packages-invalid`.

---

Observation:
Nested `with` updates on imported records can lose the imported refined concept type for numeric literals. Updating `Document.ParagraphStyle.SpaceAfterPt` with `4.0` was rejected as Float where Document.NonNegativePoint was expected, although the same composition works inside the defining package.

Suggestion:
Apply the target imported field's concept conversion to record-update literals, matching record construction and same-package `with` behavior.

Status: Resolved

Resolution:
Imported record lookup now qualifies origin-owned field types before construction or `with` checking. Covered by `Language/Types/ParametricsM0/packages/Main/consumer.octest` in interpreted and compiled execution.

---

Observation:
The interpreter and direct WebAssembly backend represent Oct `Int` as signed 64-bit values, but generated Go currently emits platform-width `int`. Chapter 4's constant optimizer therefore has exact interpreter/WASM and amd64-native parity, while a 32-bit native build does not yet share an explicit integer ABI contract.

Suggestion:
Define one compiler-wide `Int` width and overflow contract, then make generated Go use that representation before claiming cross-architecture optimized-native parity.

Status: Deferred

---

Observation:
Compiled execution zeroed record fields whose names are not exported Go identifiers. Generated `__octCloneValue` rebuilt a struct by setting fields through reflection and skipped every field that `CanSet` rejected, so a field named `_Key` or `lower` silently became its zero value whenever a record was copied as part of an array (an array literal, or `let copy = records`). The interpreter kept the value. Found while wiring Random v2: an array of `Random.Stream` values turned into an array of zero-key streams in the compiled lane only.

Suggestion:
Copy the whole struct first, then deep-clone the fields reflection can set.

Status: Resolved

Resolution:
`__octCloneValue` now starts from a full struct copy (`out.Set(value)`). `Language/Testing/CompiledArrayLowering/valid/record_copy_keeps_every_field.octest` fails in the compiled lane before the change and passes in both lanes after it.

---

Observation:
The interpreter names a record value by the spelling used where it was constructed: `Point` inside the declaring package, `Lib.Point` in an importing package. `valuesEqual` compares that name, so a record returned by a library function does not equal a literal of the same type written by the caller: `Assert.Equal(Lib.Make(1, 2), Lib.Point { X: 1 Y: 2 }, ...)` fails interpreted and passes compiled. `qualifyCrossPackageValue` qualifies enum type names when a value crosses a package boundary but leaves record type names alone. Random v2 works around it by naming builtin-made `Stream` values the way a literal in the calling package would be named.

Suggestion:
Give record values one canonical type identity (package-qualified), or qualify record type names at the package boundary as enums already are, and add a cross-package equality contract under `Language/`.

Status: Open

---

Observation:
An `.octest` cannot assert that a call stops with a non-recoverable runtime error. `Assert.Error` covers fallible results only, and `.octfail` covers compile-time rejection only. Runtime preconditions of builtins (a negative index, reversed bounds) therefore have no home in the Oct test corpus. Random v2 uses a deliberately failing fixture under `testdata/` driven by a Go test in `cmd/oct`, as the older runtime-domain tests in `cmd/oct/main_test.go` do with embedded source.

Suggestion:
Add a test form for expected runtime failure, for example `Assert.Fails(<expression>, "<error substring>")` or an `.octfail` header such as `expect runtime error: "..."`, so these contracts can live beside the library in both lanes.

Status: Open

---

Observation:
An `.octfail` fixture is copied to a temporary directory before it is checked, so it cannot `import` a repository library. A compile-time contract for "calling library X incorrectly from another package" cannot be written as an `.octfail`. The Random v2 argument-type contracts are written as standalone `package Random` fixtures for that reason, and only the two cross-package cases that need no import live in `Libraries/RandomUsage`.

Suggestion:
Resolve `.octfail` imports against the repository's `Libraries` and `Packages` roots, as `.octest` files already are.

Status: Open

---

Observation:
`Language/reference` has no page for the Random library or for compiler-owned library builtins in general; Random is described only in `Libraries/Random/README.md` and `internal/random/`. Separately, `Libraries/Random/tests/README.md` says production `Libraries/Random/*.oct` files must not depend on `Assert`, while those files use `Assert.True` for runtime validation and `Language/reference/language/09-builtins.md` explicitly allows that.

Suggestion:
Document compiler-owned library builtins in the reference (which names are builtins, the qualified-only rule outside the owning package), and correct the Random tests README when the v1 library layer is replaced.

Status: Open

---
