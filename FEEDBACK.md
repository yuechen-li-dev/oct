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

Resolution:
Not fixed on 2026-10-03, and the suggestion as written is not safe. A call into another package strips that package's prefix from record and enum arguments, and the return adds it back to enums only. Adding it to records as well would rename a caller's own record that passes through a library and comes back. The fix is one canonical type identity at construction, which touches every place the interpreter keys on a type name.

---

Observation:
An `.octest` cannot assert that a call stops with a non-recoverable runtime error. `Assert.Error` covers fallible results only, and `.octfail` covers compile-time rejection only. Runtime preconditions of builtins (a negative index, reversed bounds) therefore have no home in the Oct test corpus. Random v2 uses a deliberately failing fixture under `testdata/` driven by a Go test in `cmd/oct`, as the older runtime-domain tests in `cmd/oct/main_test.go` do with embedded source.

Suggestion:
Add a test form for expected runtime failure, for example `Assert.Fails(<expression>, "<error substring>")` or an `.octfail` header such as `expect runtime error: "..."`, so these contracts can live beside the library in both lanes.

Status: Resolved

Resolution:
An `.octfail` may begin with `expect runtime error: "..."`. The source must compile and its `Main` must fail with that text, in both lanes under `--execution auto`. `Language/reference/tooling/31-octest.md`; fixtures under `testdata/octfail_runtime`, `Language/Expressions/ArrayScalarBroadcast/invalid` and `Libraries/RandomUsage`. The Random and Entropy precondition fixtures still use the older Go-driven form.

---

Observation:
An `.octfail` fixture is copied to a temporary directory before it is checked, so it cannot `import` a repository library. A compile-time contract for "calling library X incorrectly from another package" cannot be written as an `.octfail`. The Random v2 argument-type contracts are written as standalone `package Random` fixtures for that reason, and only the two cross-package cases that need no import live in `Libraries/RandomUsage`.

Suggestion:
Resolve `.octfail` imports against the repository's `Libraries` and `Packages` roots, as `.octest` files already are.

Status: Resolved

Resolution:
The copy resolves imports against the import roots of the fixture's own directory. `Libraries/RandomUsage/Random.Usage.invalid.ImportedArgumentType.octfail` imports `Random` and misuses it. Files and packages beside a fixture are still not part of it.

---

Observation:
`Language/reference` has no page for the Random library or for compiler-owned library builtins in general; Random is described only in `Libraries/Random/README.md` and `internal/random/`. Separately, `Libraries/Random/tests/README.md` says production `Libraries/Random/*.oct` files must not depend on `Assert`, while those files use `Assert.True` for runtime validation and `Language/reference/language/09-builtins.md` explicitly allows that.

Suggestion:
Document compiler-owned library builtins in the reference (which names are builtins, the qualified-only rule outside the owning package), and correct the Random tests README when the v1 library layer is replaced.

Status: Resolved

Resolution:
`Language/reference/language/17-standard-libraries.md` has `Random`, `Entropy` and "Compiler-owned namespaces" sections, and the tests README is corrected.

---

Observation:
`Language/reference/language/06-errors.md` documents fallible `match` as an expression with value arms, `return match ParseRetries(raw) { ok(v) => v  err(_) => 3 }`, in three "Valid" examples. The parser rejects all of them with `expected 'case' in match`: `parseMatchExpr` only parses the enum form. The form that works is the statement with block arms, `match ParseRetries(raw) { ok(v) => { return v } err(e) => { return 3 } }`, which is what the fixtures under `Language/` use. The Entropy contracts use the statement form.

Suggestion:
Either implement the expression form of fallible `match` or correct the reference examples to the statement form, and add the chosen form to the `Language/` corpus.

Status: Resolved

Resolution:
The reference is corrected: fallible `match` is a statement with block arms. Its examples are pinned in `Language/Errors/Fallible/valid/fallible_match_statement_forms.octest`, and `invalid/fallible_match_is_not_an_expression.octfail` pins the rejection. The expression form is not implemented.

---

Observation:
In the compiled lane, a fallible `match` whose `err` arm discards its binding, `err(_) => { ... }`, generates Go that fails to build: `cannot use _ as value or type`. The interpreted lane accepts it. `err(_)` is the spelling the reference uses in `06-errors.md`. Found while writing `Language/Builtins/Entropy/valid/entropy_builtins.octest`, which names the binding instead.

Suggestion:
Lower a discarded `ok`/`err` binding without assigning from it, and add a both-lanes fixture for `ok(_)` and `err(_)`.

Status: Resolved

Resolution:
A discarded binding binds nothing, in functions and in flow states. `Language/Errors/Fallible/valid/match_discarded_bindings_and_branching_arms.octest` and `Language/ControlFlow/OctomataFallibilityM0/valid/flow_match_discarded_bindings.octest`. The same lowering wrote an arm's closing jump to the arm's first block, so an `if` inside an arm was skipped in the compiled lane; that is fixed and covered by the first fixture.

---

Observation:
`Entropy` is a second compiler-owned library namespace after `Artifact` and, like `Random`, is not described in `Language/reference`. The reference's capability list in `18-concepts.md` names a future `Crypto.Random` capability family; `Entropy` is the surface that family would govern, and today it is guarded only by the interpreter's artifact and discovery checks.

Suggestion:
When the reference gains a page for compiler-owned library builtins, state there which namespaces need no import (`Array`, `Artifact`, `Entropy`) and tie `Entropy` to the `Crypto.Random` capability family.

Status: Deferred

Resolution:
The reference documents `Entropy` and the namespaces that need no import. It does not tie `Entropy` to `Crypto.Random`: ordinary execution allows `Entropy` with no grant, so the tie is a language decision.

---

Observation:
The repository's Oct sources are not in the formatter's style and nothing checks that they are. With the formatter as rewritten on 2026-10-03, 427 of 1,703 `.oct`/`.octest`/`.octfail` files would change under `oct fmt`; before the rewrite the figure was 1,251, because the formatter itself was wrong (`docs/internal/ocfmt_layout_rewrite.md`). The 427 are mostly experiments written one statement per line without spaces, unpadded record braces, and files that had been run through the old formatter. `Experiments/OrbitalDecay` was committed in the old formatter's output, which is how the fault was noticed.

Suggestion:
Decide whether the tree is meant to be formatted. If it is, run `oct fmt` over `Libraries`, `Language`, `Experiments` and `Examples` once, in a commit of its own, and add `oct fmt <root> --check` to CI. Note that `.octfail` expectations that quote a column would need their columns rechecked.

Status: Open

Resolution:
Remeasured on 2026-10-03, after the formatter stopped rewriting arrows: 303 of 1,723 files would change (114 under Experiments, 120 under Libraries, 58 under Language).

---

Observation:
Ten sources in the tree that are not `.octfail` do not parse, so `oct fmt` refuses them and the test sweeps report them as `test failed: parse ...`: `Language/ControlFlow/OctomataBoardIndexedAssignment/valid/manifest.oct`, `Language/ControlFlow/OctomataCoreA/runtime/valid/result_unwrap_after_completion.octest`, `Language/ControlFlow/OctomataCoreA/valid/flow_smoke_scalar_board_progression.octest`, `Language/ControlFlow/OctomataFlowRecordLiteral/valid/flow_return_record_literal_surface.octest`, `Language/ControlFlow/OctomataFlowRecordLiteral/valid/flow_when_return_record_literal_surface.octest`, `Language/Functions/Calls/valid/markdown_helpers_single_line_and_keyvalue_ok.octest`, `Language/Functions/Calls/valid/namespaced_calls_m0.octest`, `Language/Functions/Calls/valid/pow_builtin_float_exponentiation.octest`, `Libraries/IfErrNotEqualNil/IfErrNotEqualNil.Core.oct` and `testdata/m34a/CollectionIteration/collection_iteration.octest`. Eight of them sit in `valid/` directories.

Suggestion:
Repair or retire each one. A fixture under `valid/` that does not parse is not asserting anything.

Status: Resolved

Resolution:
The eight under `Language/` are repaired and run in both lanes. `Libraries/IfErrNotEqualNil` is rewritten (see its entry below). The `testdata/m34a` probe failed on a parameter named `matrix`: `matrix[` always began a matrix literal. The parser now reads a literal only for `matrix[[` and `matrix[]`, so a value named `matrix` can be indexed. The probe is rewritten with `Append` as `Language/ControlFlow/Loops/valid/counted_loop_array_traversal.octest`, and its report is kept as `docs/internal/collection_iteration_pressure_m34a.md`.

---

Observation:
`oct fmt` rewrites every `=>` as `->`, as `Language/reference/tooling/32-ocfmt.md` says it does. The reference's own examples in `06-errors.md` and `12-enums.md`, and most match and switch arms in the repository, are written with `=>`. Formatting the tree would change all of them.

Suggestion:
Either keep the arrow the author wrote, or change the reference examples to `->`, so that the reference and the formatter describe one style.

Status: Resolved

Resolution:
The formatter keeps each arrow as written. `--arrows thin` and `--arrows fat` are optional settings.

---

Observation:
`let width: Float<m>=xs[0]` does not parse (`expected '>' after dimension qualifier`): the lexer reads the `>` that closes a type argument list and the `=` after it as one `>=` token. A space is required. The formatter never writes the two together, but a person can.

Suggestion:
Have the parser split a `>=` token where a type argument list is being closed, or report the error as "write a space between '>' and '='".

Status: Resolved

Resolution:
The lexer is unchanged; the parse error now ends with "'>=' is one token, so write a space between '>' and '='". `Language/Types/UnitsM1/invalid/dimension_close_lexed_as_greater_equal.octfail`.

---

Observation:
In the compiled lane an `if` expression evaluated both branches before choosing one. `let x = if i > 0 { xs[i - 1] } else { 0.0 }` panicked with `index out of range [-1]` at `i == 0`, and a call in the untaken branch ran. The interpreted lane was correct. `lowerIfExpr` lowered both branch expressions into the block that held the condition and branched only to pick the result. This was the long-standing compiled failure of `Experiments/PrometheusMeasurementFilteringLab/M4` (0 of 7), and the existing contract `IfExpressionSkipsNonSelectedBranchEvaluation` failed in the compiled lane for the same reason.

Suggestion:
Lower each branch inside its own block.

Status: Resolved

Resolution:
`internal/build/lower_expr.go` lowers each branch in its own block. `Language/ControlFlow/IfExpression/valid/if_expression_evaluates_only_taken_branch.octest`.

---

Observation:
`oct artifact` rejects the artifact entry points of `Experiments/FmBrownNoiseKalman` M3, M4, M4b, M5 and M6 as committed: each one writes a file and then reads it back with an ordinary runtime call to check it, and artifact evaluation reports `artifact evaluation rejected ambient operation`. The recorded outputs in those directories therefore could not be regenerated by the documented command. M2 has that fault and a second one: two entry points publish `m2b_sweep_progress.json`, which is rejected as a duplicate output path.

Suggestion:
Keep read-back checks in `[Fact]` tests, not in `[Artifact]` entry points. For M2, give the progress file one owner.

Status: Open

Resolution:
The read-backs are removed from M3, M4, M4b, M5 and M6 and their outputs regenerate. M2 is unchanged and its recorded outputs are still the Random 0.1.0 ones.

---

Observation:
Oct has no builtin that adds the elements of an `Int[]` or `Float[]`. The Random v2 contract described the removed `RollDiceSum` as the one-liner `Sum(RollDice(...))`, which does not exist; the replacement is a loop, or `Algorithms.Fold` with a named reducer.

Suggestion:
Add `Sum` over `Int[]` and `Float[]` (dimension-preserving for `Float<u>[]`), or stop describing reductions as one-liners.

Status: Deferred

Resolution:
`Array.Sum` was started on 2026-10-03 and stopped. A global `Sum` would collide with functions named `Sum` in the repository. A namespaced one has to return a typed zero for an empty array, and the interpreter does not have the static element type at a call. That needs the typechecker to hand call-site types to the interpreter, which is its own change.

---

Observation:
A call into a package that is not imported gets one of two diagnostics. If the function is a compiler-owned builtin of that package, the message is ``unknown namespace/module 'Random'; did you forget `import Random`?``. If it is any other name, including a function the package declares in Oct, the message is `unknown package 'Random'`, with no hint. The two contracts are `Libraries/RandomUsage/Random.Usage.invalid.MissingImport.octfail` and `Random.Usage.invalid.RemovedV1Builtin.octfail`.

Suggestion:
Give the import hint whenever the qualifier names a package the resolver can find.

Status: Resolved

Resolution:
`unknown package 'X'` adds "did you forget `import X`?" when X is spelled like a package. `Language/Packages/CrossPackageM81/invalid/unimported_package_call_suggests_import.octfail`.

---

Observation:
Nothing ran the `Language` corpus as a whole. CI runs six of its files for lane parity and individual Go tests name some directories. Nineteen directories under `Language/` could not be run with `oct test <directory>`: ten no longer loaded (old manifest forms, pre-`[Fact]` syntax, three package names in one directory), and the rest were package sets, artifact fixtures or expected failures that only a specific Go test knows how to run.

Suggestion:
Run every `Language` directory in both lanes from one test, with an explicit, checked list of the directories that cannot be run that way.

Status: Resolved

Resolution:
`cmd/oct/language_corpus_test.go`, in the `integration` lane; about one minute. `Language/README.md` describes it.

---

Observation:
A compile-time `.octfail` passed when compilation failed with the expected text, and compilation includes building the generated Go. `length_one_array_not_scalar.octfail` and `nested_rank_broadcast.octfail` expected "operator + not defined" and passed on the Go compiler's message about two slices. The typechecker accepts both programs; they are run-time length mismatches.

Suggestion:
Never let a failure of the Go toolchain satisfy a contract.

Status: Resolved

Resolution:
`build.ErrGeneratedProgramDidNotBuild`; the tester reports it as "the source was accepted, and the generated program did not build". No other contract depended on it. The two fixtures are runtime contracts now, and the compiled lane implements element-wise array arithmetic.

---

Observation:
The lanes disagreed in four more places, each found by a fixture that passed in one lane only. Value assertions accepted an unhandled fallible operand: interpreted unwrapped it, compiled compared the wrapper. A `[Fact]` with no assertion failed interpreted and passed compiled. `[1, 2] + [3, 4]` ran interpreted and did not build compiled. A fallible `match` arm containing an `if` gave the wrong answer compiled.

Suggestion:
Fix each, and keep both lanes in the corpus test so the next one is found when it is introduced.

Status: Resolved

Resolution:
Fixed, each with a contract under `Language/`. See `docs/internal/language_corpus_cleanup_2026_10_03.md`.

---

Observation:
A fixture under `Language/` could not import a library. Import roots were taken from the nearest ancestor holding `Libraries/` or `Packages/`, and `Language/Packages` is a fixture domain, so the resolver treated `Language/` as the repository. `Language/Packages/String` was a stub added to make `import String` work. `Libraries/Markdown` also had no `manifest.oct`, so a package with a manifest could not import it, which is why `Libraries/ArtifactUsage` did not load.

Suggestion:
Do not let a nested `Packages/` hide the repository's `Libraries/`.

Status: Resolved

Resolution:
The walk continues to the nearest ancestor with `Libraries/`; a `Packages/` on the way is searched first. `Language/reference/language/13-packages.md`. The stub is removed and `Libraries/Markdown` has a manifest.

---

Observation:
In the compiled lane a test file does not see declarations made in a sibling test file, and in the interpreted lane it does. A directory's test files therefore can neither share a declaration nor repeat one. The compiled behavior is deliberate (`contributesSelectedPackageDeclarations` in `internal/project`), and the reference did not say so.

Suggestion:
Either load a directory's test files the same way in both lanes, or document the rule.

Status: Open

Resolution:
Documented in `Language/reference/tooling/31-octest.md`: shared declarations go in a `.oct` file or in a `.octest` with no test entry points. The lanes still differ.

---

Observation:
The compiled lane evaluates every candidate value of a `when utility` expression before selecting one, and refuses enum-targeted candidates with payloads ("delayed payload lowering"). Three facts in `Language/Expressions/UtilityWhen/valid` fail compiled for that reason. It is the same shape as the `if` expression defect: operands lowered before the branch.

Suggestion:
Select the candidate first and lower each value in its own block.

Status: Resolved

Resolution:
The defect was wider than the refusal. For every utility `when`, the compiled lane evaluated the value and the score of each case, and the `else` value, before it selected. A case whose condition was false, or an `else` that was not needed, could fail or propagate an error the interpreter never raised: three wrong answers in a function and two in a flow state, none covered by a fixture. A standalone `when utility` is now lowered to ordinary blocks in the order the reference gives, enum-targeted payloads included, in functions and in flow states; `when policy` gathers its candidates in source order and takes `else` as a thunk. Contracts: `Language/Expressions/UtilityWhen/valid/standalone_utility_evaluation_order.octest`, `Language/ControlFlow/OctomataUtilityWhen/valid/utility_evaluation_order.octest` and `.../runtime/invalid/policy_evaluates_every_value_whose_condition_holds.octfail`. The selection runs in the generated program, which cannot import `internal/judgment`, so that package was not used.

---

Observation:
`Libraries/IfErrNotEqualNil` declares `fn IfErrNotEqualNil(value: Int ! Error) -> Int ! Error`. Oct has no fallible parameter types, so the library has never parsed. It is listed in the canonical registry.

Suggestion:
Retire the library, or decide that fallible parameter types exist.

Status: Resolved

Resolution:
Kept, as the identity template `IfErrNotEqualNil<T>(value: T) -> T`. Oct does not let an unhandled error reach a parameter, so by the time the wrapper is called there is nothing left to check; the library says so in its doc comment and returns its argument. It has tests in both lanes and a contract that passing an unhandled fallible is rejected.

---

Observation:
Expected failures under `Language/` were `.octest` or `.oct` files that only a particular Go test knew to expect a failure from: fourteen artifact failures, four Concept capability failures, four wrapper manifest mismatches and one template provenance failure. `oct test <directory>` on any of them reported a failure, and the Go tests held the expected messages, which is semantics in Go.

Suggestion:
Make each an `.octfail`.

Status: Resolved

Resolution:
`.octfail` gains `expect artifact error:` for `[Artifact]` entry points that must fail and publish nothing, and may state several expectation lines that the one failure must all contain. A failure that needs a second file or a manifest is a package in `Packages/<Name>/` beside the fixture, which the fixture imports. The Go assertions are removed; `cmd/oct/language_corpus_test.go` lists one directory another test owns, down from twelve. `Language/Tooling/ConceptCapabilitiesM2/valid` remains: it holds two artifacts that are expected to be refused, and they need a manifest beside them and native approvals passed by the host, which an `.octfail` cannot state.

---

Observation:
Two wrapper fixture directories pass in one execution lane only, and nothing in the source said so. `Language/Testing/CompiledOctxiliary/valid` has stub bodies that only the compiled lane replaces with sidecar calls; `Language/Testing/InterpretedOctxiliary/valid` pins that the interpreted lane runs a source body the manifest also names.

Suggestion:
Let a test state the lane it belongs to, with a reason.

Status: Resolved

Resolution:
`[Interpreted("reason")]` and `[Compiled("reason")]` on a `[Fact]` or `[Theory]`. The reason is required. The other lane reports the test as skipped and does not build it, and under `--execution auto` a `[Compiled]` test does not fall back to the interpreter. `Language/reference/tooling/31-octest.md` says when not to use it: a feature one lane is missing is not a reason.

---

Observation:
A function that a wrapper manifest names and that also has a source body means two things. The interpreted lane runs the source body. The compiled lane replaces the body with the sidecar call. Eleven standard libraries are built this way (`Archive`, `Compression`, `Csv`, `Hash`, `IO`, `Image`, `Json`, `Pdf`, `Plot`, `Text`, `Time`): 45 functions, each with both. `Make` is the one wrapper library whose 15 functions are named by the manifest alone.

The two definitions are two implementations in Go. The source body calls a builtin that the interpreter runs in-process (`internal/interpret/wrapper_*.go`, which links `fpdf`, `gonum/plot` and `excelize` into `oct`). The manifest entry names a wire function of a first-party sidecar (`cmd/octxiliary-*`). The compiled lane has no implementation of most of those builtins: with the manifest entries ignored, `Archive`, `Compression`, `Hash`, `Image`, `Pdf`, `Plot`, `Text` and `Time` fail to compile ("does not yet support builtin HashSha256Text") and only `Csv` and `Json` still pass.

For 21 functions the builtin and the wire function take the same arguments. For 20 (`Image`, `Pdf`, `Plot`, and the workbook functions of `IO`) they do not: the builtin takes a bare `Int` handle and separate scalars, the wire function takes a typed handle and records. Four (`Csv.Read`, `Csv.Write`, `Json.Load`, `Json.Save`) forward to `IO`.

The two kinds of call are also governed differently during artifact evaluation: a builtin by the artifact effect rules, a manifest wrapper call by native grants (`--grant-native`).

So "a wrapper name has one definition" cannot be enforced by deleting one side. Removing the source bodies makes the interpreted lane need sidecars for every use of these libraries and makes those calls native operations in artifacts. Removing the manifest entries leaves the compiled lane without the libraries until it can run the builtins.

Suggestion:
Make the standard libraries ordinary source over builtins, as `Language/reference/language/17-standard-libraries.md` describes them, and teach the compiled lane to run those builtins through the first-party sidecars, as it already does for `CsvRead` and `FileReadText`. Keep manifest wrapper functions for native code outside the toolchain, with no source body, dispatched to the sidecar in both lanes. Then reject a name that has both. The 20 functions whose two signatures differ need the sidecar to accept the builtin's arguments, or an adapter in the compiled lane.

Status: Open

---

Observation:
The manifests of the eleven standard wrapper libraries declare `GoModuleDir: "octxiliary"`, and none of those directories exists; the sidecars are built from `cmd/octxiliary-*` by `tools/build_sidecars`. `oct pkg wrappers` in `Libraries/Hash` plans the module path `Libraries/Hash/octxiliary`.

Suggestion:
Settle this with the entry above. If the standard libraries stop declaring wrappers, the field goes with them.

Status: Open

---

Observation:
A package manifest that existed and did not parse or validate was dropped without a message whenever the program's root did not require manifests, which includes a single file that imports a library. The package then loaded with no wrapper declarations, and its stub bodies ran in place of the sidecar calls.

Suggestion:
Report the manifest error.

Status: Resolved

Resolution:
`internal/project` reports the manifest of an imported package whenever it exists and is wrong. The entry package's manifest keeps the old leniency where none is required: a file selected on its own is specified to run beside a wrong manifest (`cmd/oct` single-file target tests), and a milestone directory run on its own borrows a family manifest that names the family and not the milestone's package. `Language/Testing/CompiledOctxiliary/invalid/wrapper_undeclared_record_arg.octfail` is the contract.

---

Observation:
A value named `vector` cannot be indexed: `vector[i]` is a one-element vector literal. Unlike `matrix[[...]]`, the literal and the index have the same shape, so the parser cannot tell them apart by looking ahead. The result is a type error far from the cause, such as "Assert.Near supports only Float scalars".

Suggestion:
Either resolve it by scope (a `vector[...]` whose name is bound to a value is an index), or reject `vector` as a binding name with a diagnostic that says why.

Status: Resolved

Resolution:
Resolved by scope, in the parser. Where a parameter, a `let` or `var`, a `for` variable, a match binding, a `batch` item or a function value's capture named `vector` is in scope, `vector[...]` indexes it; everywhere else it is the literal. A binding is in scope from the statement after it to the end of its block, and a function value sees its parameters and captures only. `Language/Types/VectorsMatricesM92/valid/vector_as_a_value_name_m92.octest` and `invalid/vector_literal_is_shadowed_by_a_value_named_vector_m92.octfail` are the contracts.

---

Observation:
An ordinary program that reaches `Artifact.WriteText` is rejected at different times: the interpreted lane stops when the call runs, and the compiled lane refuses to build the program. The compiled message used to be "does not yet support builtin ArtifactWriteText", which was wrong twice: the name is internal and the feature is not pending.

Suggestion:
Reject it in the typechecker, in both lanes, when an `Artifact.*` call is reachable from `Main`.

Status: Open

Resolution:
The compiled message is now "Artifact.WriteText is available only during `oct artifact` evaluation; a compiled program cannot call it". The difference in timing remains; `Language/Tooling/Artifacts/invalid/artifact_write_outside_phase.octfail` holds the compiled half and `internal/tester/artifact_phase_test.go` the interpreted half.

---

Observation:
An array index out of bounds stops both lanes with different messages: interpreted "runtime error: index 9 out of bounds for array of length 1", compiled the Go runtime's "index out of range [9] with length 1". A runtime `.octfail` for it cannot be written with one expectation.

Suggestion:
Have the compiled lane report the interpreter's message.

Status: Open

---

Observation:
The two standalone forms of `when utility` evaluate values differently, and the reference specifies both: the plain form evaluates the value of every case whose condition holds, the enum-targeted form the selected value alone. Separately, the plain standalone form accepts `hysteresis` and `min_commit`, which have no effect without a controller.

Suggestion:
Evaluate only the selected value in both standalone forms, and reject policy fields on a standalone form.

Status: Resolved

Resolution:
Every utility `when` evaluates one value, the selected one. `hysteresis` and `min_commit` on a standalone `when utility`, plain or enum-targeted, are a parse error that names `when policy`. Contracts: `Language/Expressions/UtilityWhen/valid/standalone_utility_evaluation_order.octest` and `invalid/standalone_policy_fields_rejected.octfail`, `invalid/enum_utility_policy_fields_rejected.octfail`. One program used the fields: `Experiments/PrometheusSgemmAlgorithmLab/M4`, where they had no effect and are removed.

---

Observation:
`when policy` committed to a value. The site remembered the value it had selected and looked for an equal value among the next evaluation's candidates. Three consequences. An arm whose value changed between evaluations lost its commitment, so `min_commit` did not hold it. Two arms that produced equal values were one commitment. And to compare values the policy had to evaluate the value of every case whose condition held, where every other utility `when` evaluates one; a site whose values were not scalars could not be checkpointed.

Suggestion:
Commit to the arm.

Status: Resolved

Resolution:
A site records the committed arm (the case's position, or `else`), its score at commitment and the commit age. The committed arm is held while its condition holds and either `min_commit` has not elapsed or no other arm beats its recorded score by more than `hysteresis`; `else` is never held. Only the selected arm's value is evaluated. The interpreter, the generated Go and the Verilog profile agree, and every site can be checkpointed (interpreter checkpoint version 4, compiled payload version 2; older checkpoints are refused). `Language/ControlFlow/OctomataUtilityWhen/runtime/valid/commitment_is_to_the_arm.octest` is the contract. The 24 directories that use `when policy` give the same results as before in both lanes.

---

Observation:
Two compiled defects in utility `when` inside a flow, both found while changing the commitment rule. A standalone `when utility` in a flow state did not build under the Verilog profile ("Verilog M2 FLOW expressions must lower to one acyclic MIR block"); this was introduced by the change that made compiled utility `when` evaluate in order, and no Verilog fixture used the form. A `when policy` that was part of a larger expression generated Go that did not build.

Suggestion:
Fix both and add the missing fixture.

Status: Resolved

Resolution:
The Verilog profile keeps the structured single-block lowering for utility `when`; the Go backend uses blocks. `Language/Profiles/VerilogM2/valid/utility_standalone` is new and its testbench passes in Icarus Verilog, as does `utility_policy`. A `when policy` inside a larger expression is covered by `commitment_is_to_the_arm.octest`.

---

Observation:
`hysteresis` compares the leading arm's score with the score recorded for the committed arm, not with the committed arm's score at this evaluation. The record is refreshed only when the committed arm itself leads by more than `hysteresis` over it. A committed arm whose score has since fallen is therefore held until a rival beats the old score. With `hysteresis: 2`, an arm committed at 10 whose score drops to 1 is held against a rival at 5. This predates the change of commitment from value to arm and was kept; the reference says "its committed score". No contract pins when the record is refreshed: removing the refresh from either lane fails no test.

Suggestion:
Decide whether the comparison should use the committed arm's current score. If it should, the arm's score is already evaluated on every pass, so only the comparison changes.

Status: Open

---

Observation:
`Assert.Equal` does not accept arrays ("does not support type Int[] in M24a"), so a test that builds an array asserts its length and each element.

Suggestion:
Accept arrays of the types it already compares, and report the first differing index.

Status: Open

---

Observation:
A test run leaves files in the working tree. `Libraries/Pdf/Pdf.CompiledText.octest` writes `m21_pdf_compiled_styled.pdf` and `m21_pdf_compiled_text.pdf` to the repository root when a PDF sidecar is found, and an `IO` test writes `io_xlsx_m0.xlsx` there; none of the three is tracked or ignored. A `cmd/oct` test rewrites the tracked `cmd/oct/analysis_output.png`. `git add -A` after a full run therefore commits generated binaries, which happened twice in this work and was undone both times.

Suggestion:
Write those outputs to the test's artifact scope or a temporary directory.

Status: Open

---

Observation:
`oct fmt` writes a negative score in a utility `when` case as a subtraction: `case 1 when open score -5` becomes `case 1 when open score - 5`. `score` is an identifier to the lexer, so the formatter spaces the `-` after it as a binary operator. The tokens are unchanged and the program means the same.

Suggestion:
Treat `score` in a utility `when` case as the keyword it is there, so that what follows starts an expression.

Status: Open

---
