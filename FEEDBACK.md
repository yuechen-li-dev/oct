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

Status: Resolved 2026-10-03. `Language/reference/language/17-standard-libraries.md` has `Random`, `Entropy` and "Compiler-owned namespaces" sections, and the tests README is corrected.

---

Observation:
`Language/reference/language/06-errors.md` documents fallible `match` as an expression with value arms, `return match ParseRetries(raw) { ok(v) => v  err(_) => 3 }`, in three "Valid" examples. The parser rejects all of them with `expected 'case' in match`: `parseMatchExpr` only parses the enum form. The form that works is the statement with block arms, `match ParseRetries(raw) { ok(v) => { return v } err(e) => { return 3 } }`, which is what the fixtures under `Language/` use. The Entropy contracts use the statement form.

Suggestion:
Either implement the expression form of fallible `match` or correct the reference examples to the statement form, and add the chosen form to the `Language/` corpus.

Status: Open

---

Observation:
In the compiled lane, a fallible `match` whose `err` arm discards its binding, `err(_) => { ... }`, generates Go that fails to build: `cannot use _ as value or type`. The interpreted lane accepts it. `err(_)` is the spelling the reference uses in `06-errors.md`. Found while writing `Language/Builtins/Entropy/valid/entropy_builtins.octest`, which names the binding instead.

Suggestion:
Lower a discarded `ok`/`err` binding without assigning from it, and add a both-lanes fixture for `ok(_)` and `err(_)`.

Status: Open

---

Observation:
`Entropy` is a second compiler-owned library namespace after `Artifact` and, like `Random`, is not described in `Language/reference`. The reference's capability list in `18-concepts.md` names a future `Crypto.Random` capability family; `Entropy` is the surface that family would govern, and today it is guarded only by the interpreter's artifact and discovery checks.

Suggestion:
When the reference gains a page for compiler-owned library builtins, state there which namespaces need no import (`Array`, `Artifact`, `Entropy`) and tie `Entropy` to the `Crypto.Random` capability family.

Status: Partly resolved 2026-10-03. The reference now documents `Entropy` and the namespaces that need no import. It does not tie `Entropy` to `Crypto.Random`: ordinary execution allows `Entropy` with no grant, so the tie would be a language decision, not a documentation fix. Open.

---

Observation:
The repository's Oct sources are not in the formatter's style and nothing checks that they are. With the formatter as rewritten on 2026-10-03, 427 of 1,703 `.oct`/`.octest`/`.octfail` files would change under `oct fmt`; before the rewrite the figure was 1,251, because the formatter itself was wrong (`docs/internal/ocfmt_layout_rewrite.md`). The 427 are mostly experiments written one statement per line without spaces, unpadded record braces, and files that had been run through the old formatter. `Experiments/OrbitalDecay` was committed in the old formatter's output, which is how the fault was noticed.

Suggestion:
Decide whether the tree is meant to be formatted. If it is, run `oct fmt` over `Libraries`, `Language`, `Experiments` and `Examples` once, in a commit of its own, and add `oct fmt <root> --check` to CI. Note that `.octfail` expectations that quote a column would need their columns rechecked.

Status: Open

---

Observation:
Ten sources in the tree that are not `.octfail` do not parse, so `oct fmt` refuses them and the test sweeps report them as `test failed: parse ...`: `Language/ControlFlow/OctomataBoardIndexedAssignment/valid/manifest.oct`, `Language/ControlFlow/OctomataCoreA/runtime/valid/result_unwrap_after_completion.octest`, `Language/ControlFlow/OctomataCoreA/valid/flow_smoke_scalar_board_progression.octest`, `Language/ControlFlow/OctomataFlowRecordLiteral/valid/flow_return_record_literal_surface.octest`, `Language/ControlFlow/OctomataFlowRecordLiteral/valid/flow_when_return_record_literal_surface.octest`, `Language/Functions/Calls/valid/markdown_helpers_single_line_and_keyvalue_ok.octest`, `Language/Functions/Calls/valid/namespaced_calls_m0.octest`, `Language/Functions/Calls/valid/pow_builtin_float_exponentiation.octest`, `Libraries/IfErrNotEqualNil/IfErrNotEqualNil.Core.oct` and `testdata/m34a/CollectionIteration/collection_iteration.octest`. Eight of them sit in `valid/` directories.

Suggestion:
Repair or retire each one. A fixture under `valid/` that does not parse is not asserting anything.

Status: Open

---

Observation:
`oct fmt` rewrites every `=>` as `->`, as `Language/reference/tooling/32-ocfmt.md` says it does. The reference's own examples in `06-errors.md` and `12-enums.md`, and most match and switch arms in the repository, are written with `=>`. Formatting the tree would change all of them.

Suggestion:
Either keep the arrow the author wrote, or change the reference examples to `->`, so that the reference and the formatter describe one style.

Status: Open

---

Observation:
`let width: Float<m>=xs[0]` does not parse (`expected '>' after dimension qualifier`): the lexer reads the `>` that closes a type argument list and the `=` after it as one `>=` token. A space is required. The formatter never writes the two together, but a person can.

Suggestion:
Have the parser split a `>=` token where a type argument list is being closed, or report the error as "write a space between '>' and '='".

Status: Open

---

Observation:
In the compiled lane an `if` expression evaluated both branches before choosing one. `let x = if i > 0 { xs[i - 1] } else { 0.0 }` panicked with `index out of range [-1]` at `i == 0`, and a call in the untaken branch ran. The interpreted lane was correct. `lowerIfExpr` lowered both branch expressions into the block that held the condition and branched only to pick the result. This was the long-standing compiled failure of `Experiments/PrometheusMeasurementFilteringLab/M4` (0 of 7), and the existing contract `IfExpressionSkipsNonSelectedBranchEvaluation` failed in the compiled lane for the same reason.

Suggestion:
Lower each branch inside its own block.

Status: Resolved 2026-10-03. `internal/build/lower_expr.go` does that; `Language/ControlFlow/IfExpression/valid/if_expression_evaluates_only_taken_branch.octest` adds four facts. That directory went from 4 pass / 5 fail to 9 / 0 in the compiled lane.

---

Observation:
`oct artifact` rejects the artifact entry points of `Experiments/FmBrownNoiseKalman` M3, M4, M4b, M5 and M6 as committed: each one writes a file and then reads it back with an ordinary runtime call to check it, and artifact evaluation reports `artifact evaluation rejected ambient operation`. The recorded outputs in those directories therefore could not be regenerated by the documented command. M2 has that fault and a second one: two entry points publish `m2b_sweep_progress.json`, which is rejected as a duplicate output path.

Suggestion:
Keep read-back checks in `[Fact]` tests, not in `[Artifact]` entry points. For M2, give the progress file one owner.

Status: Partly resolved 2026-10-03. The read-backs are removed from M3, M4, M4b, M5 and M6 and their outputs regenerate. M2 is unchanged and its recorded outputs are still the Random 0.1.0 ones. Open for M2.

---

Observation:
Oct has no builtin that adds the elements of an `Int[]` or `Float[]`. The Random v2 contract described the removed `RollDiceSum` as the one-liner `Sum(RollDice(...))`, which does not exist; the replacement is a loop, or `Algorithms.Fold` with a named reducer.

Suggestion:
Add `Sum` over `Int[]` and `Float[]` (dimension-preserving for `Float<u>[]`), or stop describing reductions as one-liners.

Status: Open

---

Observation:
A call into a package that is not imported gets one of two diagnostics. If the function is a compiler-owned builtin of that package, the message is ``unknown namespace/module 'Random'; did you forget `import Random`?``. If it is any other name, including a function the package declares in Oct, the message is `unknown package 'Random'`, with no hint. The two contracts are `Libraries/RandomUsage/Random.Usage.invalid.MissingImport.octfail` and `Random.Usage.invalid.RemovedV1Builtin.octfail`.

Suggestion:
Give the import hint whenever the qualifier names a package the resolver can find.

Status: Open

---
