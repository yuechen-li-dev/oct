# FEEDBACK.md

## Stabilization pass — 2026-10-09

This pass treats the entries below as the user-requested checklist. Statuses are updated only after focused verification. An `Open` entry with a stabilization note is active work; `Deferred` records a deliberate scope or language-design decision, not a claimed fix. Duplicate reports point to their later resolution.

Checklist closeout: **85 Resolved, 10 Deferred, 1 Superseded** (including resolutions already on main). Every entry has a recorded disposition; none remains active WIP. Deferred entries remain explicit design or packaging work, including the missing Make package-local module, and are not claimed as fixed.

Verification on Windows: full fast suite, full integration suite (including all Language directories in interpreted and strict compiled lanes), external-tool suite, and explicit slow wrapper suite pass. All four maintained source roots pass the formatter gate. The updated FFT temporary-output harness also passes its focused integration check. Kalman M2–M6 and a new O0 artifact run succeed; all twelve historical ledger pins match checkout and Git bytes. M19 passes all eight interpreted facts in 23.9 seconds.

The work is split into compiler/runtime stabilization and a separate mechanical formatting commit. Generated outputs and local test evidence remain outside commits.

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

Resolution (2026-10-09): Deferred to a compiler-wide integer ABI milestone. This pass preserves existing 64-bit interpreter/WASM and amd64 native behavior; choosing 32-bit native overflow semantics affects every numeric/helper/sidecar boundary and is not a local stabilization fix.

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

Status: Superseded

Resolution:
The later imported-record identity entry records the implementation and `Language/Packages/ImportedRecordIdentity` contracts. This duplicate is superseded; pass-through identity still needs a focused stabilization check before claiming that broader case.

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

Status: Resolved

Resolution:
Remeasured on 2026-10-03, after the formatter stopped rewriting arrows: 303 of 1,723 files would change (114 under Experiments, 120 under Libraries, 58 under Language).


Stabilization note (2026-10-09): WIP: formatter contextual-negative fix is verified. Normalize the four maintained source roots in a separate mechanical commit, then add a CI formatting gate.


Verification (2026-10-09): Resolved: normalized Libraries, Language, Experiments and Examples with the current formatter. CI now checks all four roots. The contextual negative-score fix preserves subtraction spacing; formatter golden and idempotence tests pass. Final whole-tree formatting check is part of closeout.

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

Status: Resolved

Resolution:
The read-backs are removed from M3, M4, M4b, M5 and M6 and their outputs regenerate. M2 is unchanged and its recorded outputs are still the Random 0.1.0 ones.


Stabilization note (2026-10-09): WIP: M2 now publishes nine distinct outputs successfully; repeated generation reports all unchanged. Verify the later milestones through their artifact entry points before closing this duplicate.


Verification (2026-10-09): Resolved: M2 now publishes nine distinct outputs and repeat generation reports all unchanged; M3, M4, M4b, M5 and M6 artifact entry points all succeeded through the real artifact command into isolated output roots. Their earlier ambient read-back issue had already been corrected on main.

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

Status: Resolved

Resolution:
Documented in `Language/reference/tooling/31-octest.md`: shared declarations go in a `.oct` file or in a `.octest` with no test entry points. The lanes still differ.

Stabilization resolution (2026-10-09): Retained the deliberate compiled selection rule already documented in 31-octest.md. Shared declarations belong in .oct or declaration-only support .octest files.

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

Status: Resolved

Resolution:
Done as suggested. The eleven libraries declare no wrappers; the compiled lane lowers 39 library builtins to sidecar calls from the table in `internal/builtin/sidecar.go`, which `TestSidecarBuiltinTableAgreesWithTypechecker` checks against the typechecker. A handle is an `Int` in the builtin and a typed handle on the wire. The `pdf` and `plot` sidecars take the builtins' flat arguments in place of records. A source function with the name of a manifest wrapper function is a compile error (`Language/Testing/CompiledOctxiliary/invalid/wrapper_function_defined_twice.octfail`), and the reference states the rule in `tooling/33-oct-pkg.md`. Each library gives the same results as before in both lanes with sidecars present.

Three defects had been hidden by the stub bodies, since no function with record arguments was ever defined by its manifest alone: the typechecker could not resolve a transport type that a manifest qualified with its own package's name; the interpreter refused such a record ("expects record Main.TestOptions, got TestOptions"); and the compiled lane could not resolve a call to an imported package's wrapper function. All three are fixed, and the generic wrapper fixture that was compiled-only runs in both lanes.

---

Observation:
The manifests of the eleven standard wrapper libraries declare `GoModuleDir: "octxiliary"`, and none of those directories exists; the sidecars are built from `cmd/octxiliary-*` by `tools/build_sidecars`. `oct pkg wrappers` in `Libraries/Hash` plans the module path `Libraries/Hash/octxiliary`.

Suggestion:
Settle this with the entry above. If the standard libraries stop declaring wrappers, the field goes with them.

Status: Resolved

Resolution:
The standard libraries declare no wrappers, so they declare no module directory. `Registry/registry.oct` lists them as `library`. `Make` is the one first-party wrapper package left, and it has the same defect: see the next entry.

---

Observation:
`Libraries/Make/manifest.oct` declares `GoModuleDir: "octxiliary"` and `Libraries/Make/octxiliary` does not exist. Its sidecar is built from `cmd/octxiliary-makehost`. `oct pkg wrappers` in `Libraries/Make` plans the missing path.

Suggestion:
Either let a first-party wrapper name its command package, or move the makehost sidecar's module under the library.

Status: Deferred


Stabilization note (2026-10-09): Confirmed: package-local Libraries/Make/octxiliary is absent; the working first-party build is go run ./tools/build_sidecars --out dist/sidecars, which builds cmd/octxiliary-makehost. The W8b package builder intentionally requires a package-local standalone go.mod. A command-package source selector and distributable Make wrapper module need an explicit packaging milestone; path traversal or a machine-local replace directive would break that boundary. This remains a known packaging defect, not a claimed fix.

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

Status: Deferred

Resolution:
The compiled message is now "Artifact.WriteText is available only during `oct artifact` evaluation; a compiled program cannot call it". The difference in timing remains; `Language/Tooling/Artifacts/invalid/artifact_write_outside_phase.octfail` holds the compiled half and `internal/tester/artifact_phase_test.go` the interpreted half.


Stabilization note (2026-10-09): Retain the documented phase boundary: artifact evaluation owns publication, interpreted ordinary execution rejects when reached, and compiled ordinary programs reject during lowering. A shared reachability/phase checker requires a bounded compiler milestone (including dead code, imported calls and function values). The existing compiled diagnostic explicitly identifies Artifact.WriteText and its phase; no execution fallback was added.

---

Observation:
An array index out of bounds stops both lanes with different messages: interpreted "runtime error: index 9 out of bounds for array of length 1", compiled the Go runtime's "index out of range [9] with length 1". A runtime `.octfail` for it cannot be written with one expectation.

Suggestion:
Have the compiled lane report the interpreter's message.

Status: Resolved

Resolution:
Compiled MIR indexing uses checked access and assignment helpers. `Language/Testing/FeedbackStabilization/invalid/array_read_bounds.octfail` and `array_write_bounds.octfail` pass through runtime checks in both lanes on 2026-10-09.

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
A site records the committed arm (the case's position, or `else`) and the commit age. The committed arm is held while its condition holds and either `min_commit` has not elapsed or no other arm beats it by more than `hysteresis`; `else` is never held. Only the selected arm's value is evaluated. The interpreter, the generated Go and the Verilog profile agree, and every site can be checkpointed (interpreter checkpoint version 4, compiled payload version 2; older checkpoints are refused). `Language/ControlFlow/OctomataUtilityWhen/runtime/valid/commitment_is_to_the_arm.octest` is the contract. The 24 directories that use `when policy` give the same results as before in both lanes.

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

Status: Resolved

Resolution:
The comparison uses the committed arm's score at this evaluation, in the interpreter, the generated Go and the Verilog profile. The recorded score is gone from the site, from checkpoints and from the Verilog module's ports (`UtilitySite<N>Score`). `Language/ControlFlow/OctomataUtilityWhen/runtime/valid/hysteresis_boundary.octest` holds the contract, with a committed arm whose score falls and one whose score rises. No existing program changed its result.

---

Observation:
`Assert.Equal` does not accept arrays ("does not support type Int[] in M24a"), so a test that builds an array asserts its length and each element.

Suggestion:
Accept arrays of the types it already compares, and report the first differing index.

Status: Resolved

Stabilization resolution (2026-10-09): Recursive array equality now uses the existing value-equality authority. ArraysAndEnumPayloadsCompareByValue passes interpreted and strict compiled execution.

---

Observation:
A test run leaves files in the working tree. `Libraries/Pdf/Pdf.CompiledText.octest` writes `m21_pdf_compiled_styled.pdf` and `m21_pdf_compiled_text.pdf` to the repository root when a PDF sidecar is found, and an `IO` test writes `io_xlsx_m0.xlsx` there; none of the three is tracked or ignored. A `cmd/oct` test rewrites the tracked `cmd/oct/analysis_output.png`. `git add -A` after a full run therefore commits generated binaries, which happened twice in this work and was undone both times.

Suggestion:
Write those outputs to the test's artifact scope or a temporary directory.

Status: Resolved


Stabilization note (2026-10-09): PDF and XLSX contracts delete their generated files after successful assertions. The XLSX contract verifies existence and non-empty saved bytes before deletion; the host wrapper harness verifies cleanup. The CLI analysis test rewrites a temporary copy of its fixture to a t.TempDir output, so it cannot overwrite the tracked PNG. The FFT artifact harness also runs in a temporary child-process working directory; it no longer deletes/replaces tracked out/ witnesses or changes the test process working directory. Verified the PDF and XLSX strict compiled lanes with sidecars and cmd/oct tests. Failure outputs remain available for diagnosis; no new global output-scope API was invented.

---

Observation:
`oct fmt` writes a negative score in a utility `when` case as a subtraction: `case 1 when open score -5` becomes `case 1 when open score - 5`. `score` is an identifier to the lexer, so the formatter spaces the `-` after it as a binary operator. The tokens are unchanged and the program means the same.

Suggestion:
Treat `score` in a utility `when` case as the keyword it is there, so that what follows starts an expression.

Status: Resolved

Stabilization resolution (2026-10-09): Parser-provided expression offsets identify contextual score operands. Golden and idempotence tests pass without changing ordinary score-variable subtraction.

---

Observation:
Every library builtin that the compiled lane sends to a sidecar is implemented twice in Go: once in the interpreter (`internal/interpret/wrapper_*.go`) and once in the sidecar (`cmd/octxiliary-*`). The two are written separately and can drift; only the library tests, run in both lanes with sidecars, compare them. Plotting already avoids this: both sides call `internal/plotrender`.

Suggestion:
Give each family one Go package that holds the work, as `internal/plotrender` does, and have the interpreter builtin and the sidecar both call it.

Status: Deferred


Stabilization note (2026-10-09): Keep shared plotrender as the pixel-size authority; both adapters now call it and actual PNG dimensions are checked. The remaining extraction is an architecture proposal, not a bounded stabilization patch: interpreter capabilities and process-local sidecar handles differ. Extract one family at a time with explicit handle/capability ownership and both-lane contracts; do not replace those boundaries with a parallel generic runtime.

---

Observation:
Eight library builtins have no compiled implementation, and a compiled program that reaches one is refused by name: `PdfDrawImage` and `PdfDrawImageSized` (a page and an image are handles of two different sidecars), `JsonLower`, `JsonLoadStructured`, `CsvWriteTable`, `CsvWriteMatrix`, `PlotLine` and `PlotScatter`. `Libraries/IO/IO.Json.octest` has 12 tests that fail compiled for `JsonLoadStructured`, and `Libraries/Pdf/Pdf.Core.octest` six for `PdfDrawImage`.

Suggestion:
Add the four data builtins and the two short plot forms to the sidecar table; they need wire functions and no new mechanism. Decide separately whether the image-handle form of `PdfDrawImage` should exist in the compiled lane or be retired in favour of `DrawImageBytes`.

Status: Resolved

Resolution:
Partly, 2026-10-07. `JsonLower` and `JsonLoadStructured` were removed with the first Json library, and `Libraries/IO/IO.Json.octest` with them (`internal/json/JSON_V2_M5.md`). The other six remain.

Stabilization resolution (2026-10-09): Pdf image-handle calls transfer encoded bytes through existing image/PDF sidecars; all six Pdf.Core tests pass strict compiled. CsvWriteMatrix and short PlotLine/PlotScatter pass both lanes in serialization/csv_and_plot.octest. CsvWriteTable retains its explicit not-implemented error in both lanes rather than implying a writer exists. Retired Json v1 names stay removed.

---

Observation:
`Language/Testing/CompiledOctxiliary` and `Language/Testing/InterpretedOctxiliary` are named after the lane each once belonged to. Both run in both lanes now.

Suggestion:
Rename them when the corpus is next reorganized; two Go test files name the paths.

Status: Resolved


Stabilization note (2026-10-09): WIP: rename the two corpora to describe contracts and dispatch rather than execution lanes, update executable references, and verify both lanes.


Verification (2026-10-09): Resolved: renamed the corpora OctxiliaryContracts and OctxiliaryDispatch; updated live code/docs and fixture paths. Historical internal milestone notes retain their original paths. OctxiliaryDispatch passes all four contracts interpreted and strict compiled with a current test sidecar; the full corpus checks both new names.

---

Observation:
A plot is larger than the size it is asked for. `Plot.Size { Width: 400px Height: 300px }` writes a PNG of 533 by 400 pixels. `internal/plotrender.PixelLength` turns a pixel count into the same number of points, and the image is rendered at 96 dots per inch, so every length grows by 4/3. Both lanes share the renderer, so both do it. No test checks the dimensions of a plot.

Suggestion:
Convert pixels to points with the renderer's resolution, so that the image has the pixels the `Int<px>` asked for. Recorded plots, `cmd/oct/analysis_output.png` among them, will change size.

Status: Resolved

Stabilization resolution (2026-10-09): PixelLength converts requested pixels at 96 DPI. The real PNG Render test verifies exactly 400 by 300 pixels; internal/plotrender passes.

---

Observation:
In the interpreted lane a record of an imported package does not equal the same record returned by that package. With `record Point { X: Int Y: Int }` and `fn Origin() -> Point` in package `Passer`, `Assert.Equal(Passer.Point { X: 0 Y: 0 }, Passer.Origin(), "...")` in another package fails interpreted and passes compiled. The literal is named `Passer.Point` and the returned value `Point`: `qualifyCrossPackageValue` qualifies the enums in a value that crosses a package boundary and not its records, and `valuesEqual` compares the names. Found while checking how a wrapper function's record should be named; wrapper results go through the same function and so behave as a source function's do.

Suggestion:
Qualifying records on the way out, as enums are, is not enough: a record of the caller's own package that passes through a library function would come back named for the library. Give a record its package when it is constructed and compare that.

Status: Resolved


Stabilization note (2026-10-09): The earlier main fix qualifies record results. This pass also qualifies caller-owned inputs before entering another package and restores the caller namespace on return, including nested values. Normalization operates on a clone so it cannot mutate caller-retained nested data. FeedbackStabilization/packages checks imported records, enums and caller-owned records in both lanes; interpreter package tests pass.

---

Observation:
An `Int` is accepted where a `Float` is declared, and the two lanes then disagree about the value. `let x: Float = 1` followed by `x / 2` is `0` interpreted and `0.5` compiled. The same holds for an `Int` variable bound as `Float`, an `Int` argument to a `Float` parameter, and an `Int` array literal used as a `Float[]` local, argument, return value or record field. The typechecker accepts all of them, the compiled lane converts, and the interpreter keeps the `Int`. `Language/reference/language/03-expressions.md` says "Implicit conversion is not allowed", and `docs/COMPILED_SUPPORT.md` (M28a) records the compiled conversion as a fix. Found while writing contracts for `[1 ... n]` in a `Float[]` context; no contract was written for it.

Suggestion:
Decide which of the three is the language. If an `Int` does not convert, reject it in the typechecker and say to write `1.0` or `Float(n)`. If it does, the interpreter has to convert at every place the typechecker accepts it, and the reference has to say so.

Status: Resolved

Resolution: The declaration decides: `let x: Float = 1` binds the `Float`. The interpreter now converts at every place the typechecker admits an `Int` for a `Float`, or an `Int` or `Float` for a `Complex` (`internal/interpret/conform.go`), and the compiled lane converts at the places where it did not build or panicked: flow sites, row and element assignment, record update fields, enum payloads, nested arrays, vectors and matrices. The rule is in "Declared types and numeric values" in `02-types.md`; contracts are under `Language/Types/DeclaredNumericTypes`, 30 facts in both lanes.

---

Observation:
Whole-row assignment to a board field is not checked in the compiled lane. With `board.Grid = [[0.5, 0.5], [0.5, 0.5]]`, `board.Grid[1] = [9.5]` fails interpreted with `row length mismatch: expected 2, got 1`; compiled, the program completes and the row has one element. A local `rows[i] = row` is checked in both lanes. The compiled statement is a bare Go slice assignment (`emitGoFlowFieldIndexAssign`), so an index out of range is also a Go panic with a stack trace in place of the array bounds error. `board.Grid[i] = [value ...]` is not affected: it reads the row's length and checks the index itself.

Suggestion:
Lower a one-index assignment to a two-dimensional board field through the helper that local rows use, and add an `.octfail` for the mismatch.

Status: Resolved

Resolution: The compiled statement goes through the row helper a local row uses, so the length is checked and the row is copied. Contracts: `Language/ControlFlow/OctomataBoardIndexedAssignment/invalid/board_row_length_mismatch.octfail` and `valid/board_row_assignment.octest`.

---

Observation:
A compiled runtime error whose message carries a code prints a Go stack trace. `runtime error [OCT-RTBL003]: record table ... columns have inconsistent lengths` and `[OCT-RTBL004]` arrive in the output of a compiled `.octfail` as `panic: runtime error [OCT-RTBL003]: ... [recovered, repanicked]` followed by goroutine frames, while `runtime error: ...` messages are one line. The interpreter reports both forms as one line.

Suggestion:
Have the generated program's top-level recovery accept `runtime error [` as it accepts `runtime error: `.

Status: Resolved

Stabilization resolution (2026-10-09): Generated top-level recovery recognizes coded Oct diagnostics. TestCodedRuntimeDiagnosticHasNoGoStack passes on a real compiled OCT-RTBL003 failure.

---

Observation:
`value ...` fills only where a table or a row assignment fixes a length, because Oct has no array type with a length in it. In Concept the form is most at home initializing a statically sized array. `let xs: Float[] = [0.0 ...]` is a compile error that says to write a count. Row fill is also limited to two-dimensional arrays, the only depth at which both lanes implement whole-row assignment.

Suggestion:
If Oct gains a sized array type, let its declaration fix the length for `value ...`; the check, the two evaluators and the diagnostic are already written around "a length fixed by the context".

Status: Deferred

Resolution (2026-10-09): Sized array types and arbitrary-depth whole-row fill are explicit language extensions. Retain the current count-required contract and two-dimensional row fill; no inferred length is invented when the type supplies none.

---

Observation:
The compiled test runner names its generated Go file after the package and the absolute path of the `.octest` file (`sanitizeHarnessName` in `internal/tester/tester.go`), so whether a test can run depends on where the repository is checked out. A fixture named `builtin_reached_only_from_a_flow_statement.octest` gave a 256-character file name in a worktree under a long scratch path and failed with "write generated go ...: file name too long"; it was renamed. The longest name in the tree is now 192 characters under `C:\Users\<name>\source\repos\oct`, before the temporary directory is added.

Suggestion:
Keep a readable prefix of the name and replace the rest with a short hash of the whole of it.

Status: Resolved

Stabilization resolution (2026-10-09): Names have a maximum 64-byte readable prefix and a 16-digit SHA-256 suffix. TestHarnessNamesAreBoundedAndDistinct verifies length, determinism and distinct paths; internal/tester passes.

---

Observation:
A state local read after a `suspend` is accepted, and the two lanes give different values. `state Run { var x = 7  suspend  return x }` returns `7` interpreted and `0` compiled. `Language/reference/runtime/21-octomata.md` says state locals "do not cross `goto`, `suspend`, `yield`, or turn boundaries". The typechecker enforces this for `yield` only, and only for the block the `yield` stands in: a local of an enclosing block stays in scope after a nested `yield`, and a loop that contains one is not treated as a boundary. `Experiments/RfAdaptiveLinkControllerProbe/M0` kept its index and its result in locals across a `suspend`; its test passed interpreted on a result of two elements where one per sample was meant, and failed compiled. It now keeps them on the board. No other source in the repository reads a local after a `suspend`.

Suggestion:
Treat `suspend` as `yield` is treated, and drop the locals of every enclosing block of the state body, not only the innermost. When a name fails to resolve for this reason, say that it was a state local before a turn boundary and belongs on the board; today the message is "undefined variable".

Status: Resolved

Stabilization resolution (2026-10-09): Possible boundaries expire all enclosing activation locals without mutating sibling scopes. Loops, branches, matches and suspend/yield actions are covered. suspend_local and nested_yield_local contracts pass with a board-oriented diagnostic.

---

Observation:
A `suspend` or `yield` inside an `if`, a loop or a `when` action resumes after the top-level statement of the state body that contains it, not after itself. In `if ready { yield 1  board.A = 50 }` the assignment never runs, in either lane, and `while i < n { ...  suspend }` leaves the loop for good at its first `suspend`. `Language/ControlFlow/OctomataFlowNestedTransferM1/valid/nested_transfer.octest` pins this for `suspend` ("should continue after the containing statement"). The reference does not state it, and for `yield` it says the opposite: "preserves the continuation immediately after the yield". Nothing warns about the statements that can no longer run.

Suggestion:
Decide which is the language. If a nested boundary really leaves the containing statement, say so in the reference and reject a statement that follows one in the same block as unreachable. If the continuation is meant to be the next statement, both lanes need it, and a loop with a `suspend` in it becomes the generator it reads as.

Status: Resolved

Stabilization resolution (2026-10-09): Preserved the existing containing-state-statement continuation contract and documented it explicitly in 21-octomata.md. Statements after a direct nested boundary are rejected; nested_boundary_unreachable.octfail passes. This is a visible correction of the previous reference inconsistency.

---

Observation:
A failed `Assert.Equal` reports its message and neither value: `assertion failed: length`. Finding out what the two sides were takes a second program that prints them.

Suggestion:
Append the expected and actual values to the failure, in both lanes, at least for scalars and strings.

Status: Resolved

Stabilization resolution (2026-10-09): Both lanes report expected and actual values. The deliberately failing Language/Testing/FeedbackStabilization/diagnostics/assertion_values.octest is exercised by TestAssertionDiagnosticReportsValuesInBothLanes; internal/tester passes.

---

Observation:
`Experiments/ContinuumComputabilityBoundary/M16` computed different numbers from the ones its report records, and its last test failed. An earlier version of this entry said the test asserted a verdict the probe did not reach. That was wrong. The probe was written when `var next = current` shared the array with `current`: its transport sweep wrote into `nextOx` while reading `ox`, and those were one array, so each pass ran in place. Arrays became values on 2026-06-15 (`value_copy_semantics_ofix1.octest`). The same source then ran each pass from a snapshot, the tangents that meet at the centre of the circle cancelled, and the interior cell had no orientation. With the toolchain of 2026-06-04 the test passes; with today's it reported an interior magnitude of 0 for path A where the report's results need 0.707.

Suggestion:
Experiments written before 2026-06-15 that write to a copy while reading the original now compute something else, and only a test strict enough to notice says so. M16 is the only directory under `Experiments/` with this shape (`var nextX = x` followed by writes to `nextX`). Recorded numbers of older experiments are otherwise unverified against the current toolchain.

Status: Resolved

Resolution: The sweep is written in place, which is what it did. Both lanes give the twelve values of the 2026-06-04 toolchain to the last digit, and the directory's five tests pass.

---

Observation:
An element assignment in the interpreter copies the whole array. `xs[i] = value` clones `xs` and stores the clone (`assignNestedArrayIndex`), so filling an array by index is quadratic. An interpreter value is 592 bytes (`unsafe.Sizeof(Value{})`), so one write to a `Float[]` of 8,000 elements copies 4.7 MB. Measured on 2 cores, interpreted: 2,000 elements 1.7 s, 4,000 elements 7.5 s, 8,000 elements 26 s. The compiled lane does the same loops in milliseconds. Eleven experiment tests took 39 to 100 s each interpreted where their whole directory takes 1 to 3 s compiled, and `PrometheusSgemmAlgorithmLab/M19` `M19RectangularStressHoldsIndexingInvariants`, whose largest matrix is 193 by 129, did not finish in 900 s interpreted and takes under a second compiled. Copying only the outer slice in place of a deep clone was tried and changes nothing: for scalars the two are the same copy.

Suggestion:
Write in place when the binding owns its array, or make a value small. Every other holder of an array would have to be shown to hold its own copy first: a record field, an element of an array of arrays, an enum payload, a flow parameter and a returned value all share storage with the variable they came from today, and are safe only because a write makes a new array.

Status: Resolved


Stabilization note (2026-10-09): assignNestedArrayIndex mutates the storage already owned by the target binding; existing copy boundaries still clone binding/assignment/capture/board/snapshot values and replacement elements. All 36 Array valid contracts pass interpreted. Host benchmark: 1,024 and 65,536 element writes take about 105 and 104 ns/op respectively with zero allocations, removing array-length-dependent copying. The real PrometheusSgemmAlgorithmLab/M19 directory now passes all eight facts interpreted in 23.9 seconds, including the formerly timing-out rectangular stress case (.tmp/feedback-m19-interpreted.json records the command result).

---

Observation:
In the compiled lane `Append` writes into storage its argument shares. A record field holds the array it was built from without copying it, and `Append(record.Field, value)` is Go's `append`, which writes into spare capacity. With `var xs = [1.0]`, two appends to `xs`, `let r = Pack { Values: xs }`, `xs = Append(xs, 4.0)` and then `let w = Append(r.Values, 9.0)`, `xs[3]` is `9.0` compiled and `4.0` interpreted. Found by reasoning about the board fixes in this pass.

Suggestion:
An `Append` whose result is not assigned back to its own first argument should never write into the array it was given: `append(xs[:len(xs):len(xs)], value)`. It is copied afterwards already, so the cost does not change.

Status: Resolved

Resolution: The cause was the record, not `Append`: a record field, a `with` replacement and an enum payload held the array or matrix they were built from without copying it, so a write to the variable also changed the record. They now hold a copy, as an element of an array of arrays already did. Contract: `Language/Types/Arrays/valid/value_copy_semantics_in_aggregates.octest`, six facts, five of which failed compiled before. `Append` is unchanged.

---

Observation:
The reference says "Implicit conversion is not allowed" (`03-expressions.md`), and mixed `Int` and `Float` arithmetic and comparison are accepted and give a `Float` result in both lanes: `3 + 1.5`, `3 * 1.5`, `3 > 1.5`, `[1.5, 2.5] * 3`. One contract pins it, `Language/Expressions/Arithmetic/valid/mixed_division_promotes_to_float.octest`, for `/` only. The reference does not state the promotion.

Suggestion:
Decide whether `1.5 + 1` is the language. If it is, the reference should say so beside the declared-type rule, and the other operators need contracts. If it is not, the typechecker should reject it and say to write `1.0`.

Status: Resolved

Stabilization resolution (2026-10-09): 03-expressions.md now states dimension-compatible Int/Float arithmetic and comparison promotion. Mixed operations pass both lanes.

---

Observation:
A whole-row assignment with the row index out of range reports different text in the two lanes. Interpreted: `runtime error: index 5 out of bounds for array of length 2`. Compiled: `runtime error: row index 5 out of bounds for array with 2 rows`. This holds for a local and, since this pass, for a board field. A runtime `.octfail` cannot state either.

Suggestion:
One message. The interpreter's is the one the other array bounds errors use.

Status: Resolved

Resolution: The compiled row helper reports the interpreter's text. Contracts: `Language/Types/Arrays/invalid/whole_row_assignment_index_out_of_bounds.octfail` and `Language/ControlFlow/OctomataBoardIndexedAssignment/invalid/board_row_index_out_of_bounds.octfail`, each checked in both lanes.

---

Observation:
A state local cannot be assigned by index in a compiled flow. `var held = board.Trace` followed by `held[1] = 3.5` in a state body fails with "compiled mode does not yet support flow statement ast.IndexAssignStmt". The interpreted lane runs it.

Suggestion:
Lower it as the same statement is lowered in a function.

Status: Resolved

Stabilization resolution (2026-10-09): LocalWritesAndIndependentSnapshots passes both lanes; indexed writes reuse ordinary MIR lowering.

---

Observation:
In the interpreted lane a matrix assigned to a board field shares storage with the variable it came from. In a state body, `var m = matrix[[1.5, 2.5] [3.5, 4.5]]`, `board.Grid = m`, `m[0, 0] = 9.5` leaves `board.Grid[0, 0]` at `9.5`. A matrix element is the one thing the interpreter writes in place, and board field assignment is the one place a value is stored without being copied. Arrays are not affected: an element write makes a new array. No contract can be written for both lanes yet, because the compiled lane does not lower `m[0, 0] = value` on a state local (the entry above).

Suggestion:
Copy a value that holds a matrix when it is assigned to a board field. Lowering index assignment on a state local in the compiled lane comes first, so that the fix can have a contract.

Status: Resolved

Stabilization resolution (2026-10-09): Field assignments clone persistent values. LocalWritesAndIndependentSnapshots proves detached board storage in both lanes.

---

Observation:
An array read out of bounds is a Go panic in the compiled lane. `let ns = [1, 1]`, `let k = 5`, `return ns[k]` stops interpreted with `runtime error: index 5 out of bounds for array of length 2`, and compiled with `panic: runtime error: index out of range [5] with length 2 [recovered, repanicked]` followed by goroutine frames. The same holds for an element assignment `xs[k] = value`. Row assignment and filled rows go through helpers and report the interpreter's text. A runtime `.octfail` cannot state the ordinary array bounds error for both lanes.

Suggestion:
Read and write elements through a checked helper, or recover the Go bounds panic in the generated `main` and report it in Oct's words.

Status: Resolved

Resolution:
The compiled backend uses checked MIR index reads and writes. `Language/Testing/FeedbackStabilization/invalid/array_read_bounds.octfail` and `array_write_bounds.octfail` pass in both runtime lanes with the interpreter's diagnostic on 2026-10-09. True Go implementation panics remain distinguishable from Oct diagnostics.

---

Observation:
In the compiled lane an enum value written directly as the payload of another enum value, and bound to a variable, was not the payload. `let full = Crate.Holding(Parcel.Weighed(2.5))` built a `Crate` whose payload was the whole `Crate` again, and the `match` that read it panicked with "interface conversion: interface {} is main.EnumsAssociated_Crate, not main.EnumsAssociated_Parcel". The same expression passed as an argument worked. The adapter that turns lowered Go text into MIR (`lowerGoExprNode`) kept an inner expression it did not understand as Go text, and took the text of the whole expression for it.

Suggestion:
Take the text of the inner node.

Status: Resolved

Resolution: `goNodeText` in `internal/build/lower_value.go`. Contract: `Language/Types/EnumsAssociated/valid/enum_as_enum_payload.octest`, both lanes. Found while adding `Option<T>` (Json v2 M1), whose nested and enum payloads take this path.

---

Observation:
In the interpreted lane a record returned by a function of another package kept its unqualified type name, while an enum was given its package. A `Geometry.Point` built in package Main and the same record returned by `Geometry.Origin()` were therefore unequal under `Assert.Equal`, an array literal holding one of each was "mixed element kinds Point and Geometry.Point", and the two printed differently (`Point{X: 0, Y: 0}` and `Geometry.Point{X: 0, Y: 0}`). The compiled lane had one type for both.

Suggestion:
Name a record that leaves its package as an enum is named.

Status: Resolved

Resolution: `qualifyCrossPackageValue` qualifies the record's own type name. An imported record that its own package built now prints with the package, as it already did when the caller built it; `TestM18PackageCoexistenceWithMutableLocalReassignment` pinned the other form and now expects `Geometry.Point{X: 3, Y: 4}`. The interpreter's Octagon writer wrote a qualified enum name as an error ("is not representable in .octagon output"); it now writes the name without the package, which is what the compiled writer writes and what both loaders accept. Contract: `Language/Packages/ImportedRecordIdentity`, both lanes.

---

Observation:
The interpreted Octagon loader resolved the field types of a record in the package that called `LoadOctagon`, not in the package that declares the record. `LoadOctagon<Sensors.Sample>(...)` from package Main failed on any field whose type is a record or an enum of `Sensors` ("record field Mode mismatch"). The compiled loader loaded it.

Suggestion:
Resolve a field's type, and an enum payload's, where it was written.

Status: Resolved

Resolution: `materializeOctagonValueOf` in `internal/interpret/octagon_load.go`. Contract: `AnImportedRecordWithOptionsLoadsFromOctagon` in `Language/Types/Option/packages/option_across_packages.octest`, both lanes. Json v2 M3 loads through these materialisers.

---

Observation:
`WriteOctagon` followed by `LoadOctagon` does not give the value back when it holds a `Float`.
(1) Both writers write a `Float` with a whole value without a decimal point: `[1.0m, 2.0m]` is written `[1m, 2m]`, and both loaders then refuse it ("expected Float<m>, got ast.IntegerLiteral"; the Go type name in that message is a second defect).
(2) The compiled writer writes no dimensions at all: `Dt: 0.5s` is written `Dt: (0.5)`, and loading it as `Float<s>` fails with `expected Float<s> dimension "s", got ""`. The interpreted writer writes `Dt: (0.5s)`.
Measured with a record `{ Dt: Float<s>, Samples: Float<m>[] }` written and loaded in each lane.

Suggestion:
Write a `Float` so that it reads as one (`1.0m`), and give the compiled serializer the declared type of each value, as the compiled loader already has (`FieldTypes`, `PayloadNames`), so that it writes the dimension.

Status: Resolved

Stabilization resolution (2026-10-09): Writers preserve Float literal kind and numeric dimensions. Compiled serialization receives declared types for fields, elements, and payloads. WholeFloatsDimensionsAndContainersRoundTrip passes both lanes.

---

Observation:
A `match` case label names its variant, and the enum name written before it is not checked: `case Anything.Some(v) =>` is accepted on any enum that has a variant `Some`. The parser keeps only the variant name (`ast.MatchCase`).

Suggestion:
Keep the written enum name and check it against the subject's type.

Status: Resolved

Stabilization resolution (2026-10-09): The AST preserves the written enum label and typechecking verifies subject identity. match_wrong_enum.octfail passes; parser and typechecker Go tests pass.

---

Observation:
Two packages of one program cannot each declare an enum of the same name in the compiled lane. The generated tag constants are named `<Enum>_<Variant>_tag` without the package, so `Lib.Mode { Slow Fast }` and `Main.Mode { Fast Slow }` give "Mode_Fast_tag redeclared in this block". The interpreted lane runs the program.

Suggestion:
Name the constants with the package, as the enum's Go type is named.

Status: Resolved

Stabilization resolution (2026-10-09): Generated tag names include package identity. PackageQualifiedMetadataDoesNotCollide passes both lanes.

---

Observation:
In the compiled lane `==` on two values of an enum whose payload holds an array panics: `Held.Items([1, 2]) == Held.Items([1, 2])` stops with "runtime error: comparing uncomparable type []int". The typechecker accepts the comparison and the interpreted lane answers `true`. Enum equality compiles to Go's `==` on a struct with an `any` payload. `Option<T>` does not have the defect: its `==` compiles to a comparison by value (`__octValueEqual`).

Suggestion:
Compile `==` on any enum that has a payload variant to the same comparison by value.

Status: Resolved

Stabilization resolution (2026-10-09): Nominal enum equality compares recursive payload values with reflect.DeepEqual, preserving distinct qualified enum types. ArraysAndEnumPayloadsCompareByValue passes both lanes.

---

Observation:
A variable or a parameter may still be named `Option`, but `Option.<name>` after it is always read as a variant of the builtin enum, so a record held in a variable named `Option` cannot have its fields read. Declarations (record, enum, concept, function, flow, package) named `Option` are refused.

Suggestion:
Refuse `Option` as the name of a binding and a parameter too, or reserve the word in the lexer.

Status: Resolved

Resolution: Neither. The parser resolves the name by scope, as it does `vector`: where a parameter or a local named `Option` is in scope, `Option.<name>` reads a field of it. Contract: `AValueNamedOptionIsThatValue` in `Language/Types/Option/valid/option_values.octest`, both lanes.

---

Observation:
In the compiled lane `BoardSnapshot(machine)` does not build when two flows of one package have the same result type: "compiled BoardSnapshot requires unambiguous flow identity for return type Int". The failure is in lowering, so every test of the file fails, not only the one that takes the snapshot. The interpreted lane runs it. Met while writing `Language/Types/Option/valid`, where two flows returned `Int`; one was given another result type.

Suggestion:
Carry the flow's identity in the type of the instance, which the typechecker already has (`FlowIdentity`), instead of finding the flow by its result type.

Status: Resolved

Stabilization resolution (2026-10-09): Concrete FLOW instance metadata preserves package-qualified flow identity. LocalWritesAndIndependentSnapshots exercises two Int-result flows with different board shapes and passes both lanes.

---

Observation:
The two `LoadOctagon` materialisers differed in three ways, met while wiring `Json.Load<T>` to them. A field declared `Vector<T>` or `Matrix<T>`: the compiled lane loaded an array into it and the interpreter refused the type ("unsupported expected type Vector<...>"); the compiled lane also read a dimensioned element as the vector's own type and refused it. A matrix whose rows differ in length: the compiled lane loaded it. A field declared as a refined array concept (`concept Weights = Float[] { ... }`): the interpreter admitted the array by its concept; the compiled lane did not admit it, and checked each element against the array's concept, which panicked on the first element ("interface {} is string, not []string").

Suggestion:
One rule in both lanes: an array loads as what the declared type says it is.

Status: Resolved

Resolution: Both materialisers load an array as the declared vector, matrix (rows of one length) or refined array (admitted whole). Contracts: `Language/Data/Octagon/Load/valid/load_octagon_arrays_as_declared.octest` and `Language/Data/Octagon/Load/invalid/load_octagon_refined_array_refused.octfail`, both lanes. Reference: `34-octagon.md`.

---

Observation:
`LoadOctagon<Vector<Int>>(path)` is refused by the typechecker ("type argument expects .octagon-representable type"), while `LoadOctagon<R>(path)` for a record `R` with a `Vector<Int>` field is accepted and loads. The check (`isOctagonRepresentableType`) looks at the type argument itself and accepts any named type without looking inside it, so a record with a `Complex` field passes it too and fails at run time ("record field Phase mismatch: unsupported expected type Complex").

Suggestion:
Check the whole type, as the Json builtins do (`internal/jsontype` with `octjson.Check`), and accept a vector or a matrix wherever a field of one is accepted.

Status: Resolved

Stabilization resolution (2026-10-09): Recursive representability checks accept supported containers at any depth and reject nested unsupported types. Vector-root roundtrip passes both lanes; nested_complex.octfail passes.

---

Observation:
`WriteOctagon` treats a vector differently by lane. For a record with a `Vector<Float>` field the interpreter stops with "WriteOctagon cannot serialize value: value kind Vector is not representable in .octagon output". The compiled lane writes the vector as an array, with whole Floats as integers (`Levels: [1, 2]`), which neither lane then loads as a `Vector<Float>`.

Suggestion:
Write a vector as an array and a matrix as an array of rows in both lanes, once a `Float` is written so that it reads as one (the entry above on the Octagon writer).

Status: Resolved

Stabilization resolution (2026-10-09): Both writers serialize vectors as arrays and matrices as rows, preserving Float kind. WholeFloatsDimensionsAndContainersRoundTrip passes both lanes.

---

Observation:
Indexing a value of a refined array concept gives the concept's type and not the element's. With `concept Tags = String[] { Require(Len(Self) > 0, "...") }` and `tags: Tags`, `let t: Int = tags[1]` reports "expected Int, got Tags", and `Assert.Equal("a", tags[1], "...")` reports that the arguments differ in type. `tags[1] == "a"` typechecks, and then the compiled lane does not build: "cannot use ...[1] (variable of type string) as Main_Tags value". `Len(tags)` works in both lanes.

Suggestion:
Type an index of a refined array as an element of its base array, as the reference's "Refined scalar to underlying representation is permitted" reads for scalars.

Status: Resolved

Stabilization resolution (2026-10-09): Typechecking and lowering erase the container refinement before selecting its element type. RefinedArrayIndexHasElementType passes both lanes.

---

Observation:
In the compiled lane `v + v` and `v - v` on two `Vector<Float>` values do not build unless the program also uses another linear-algebra operation: "undefined: __octVecAddVV". The helpers are emitted when `usesLinearAlgebraHelpers` finds one of a list of builtins, and `VecBinaryVV:+` and `VecBinaryVV:-` are not on it. A function `fn Twice(v: Vector<Float>) -> Vector<Float> { return v + v }` called from a fact is enough to see it. The interpreted lane runs it.

Suggestion:
Add the two names to the list, or have the emitter record a helper as needed where it emits the call to it.

Status: Resolved

Stabilization resolution (2026-10-09): VecBinary helpers participate in helper discovery. VectorAdditionNeedsNoOtherBuiltin passes both lanes.

---

Observation:
Float literal arithmetic differs by lane. `0.1 + 0.2 != 0.3` is true in the interpreted lane and false in the compiled lane: the generated Go adds two untyped constants exactly and rounds once, so `0.1 + 0.2` is `0.3` there and `0.30000000000000004` in the interpreter. Met while stating that `Json.Parse<Float>("0.30000000000000004")` keeps every digit.

Suggestion:
Emit Float literals as typed values (`float64(0.1)`), so that the compiled lane rounds each literal before it adds, as the interpreter does.

Status: Resolved

Stabilization resolution (2026-10-09): Go emits Float literals through a typed identity call, ensuring each operand rounds before arithmetic while preserving backend-neutral MIR and WASM optimizer specimens. FloatLiteralsRoundBeforeArithmetic passes both lanes; a Go constant conversion alone would still fold exactly.

---

Observation:
The compiled lane does not build a program that loads Octagon or JSON data and has two refined concepts of one name in two packages (`Main.Port` and `Net.Port`): "duplicate case \"Port\" in expression switch". The generated `__octValidateRefinement` matches each concept by `Package.Name` and by its bare name, and the bare names collide. The table of refinement bases added for refined arrays has the same two keys and the same limit. The interpreted lane runs the program. This is the collision the entry on enum tag constants describes, in another generated table.

Suggestion:
Name a refined type by `Package.Name` everywhere in the generated metadata, and drop the bare name.

Status: Resolved

Stabilization resolution (2026-10-09): Refinement metadata has only package-qualified keys. The Main.Port/Net.Port contract passes both lanes with independent requirements.

---

Observation:
A file can hold a value whose type is declared in a package the file does not import, and then cannot read its fields: with `Config.Service { Listen: Net.Address }` and only `import Config`, `service.Listen.Host` reports "type 'Net.Address' has no field 'Host' ... 'service.Listen' has type Net.Address, which has no fields". The typechecker of a package knows the declarations of the packages it imports and no others. The message does not say that an import is what is missing. Relatedly, a transparent alias (`concept Count = Int`) cannot be named from another package at all ("package 'Net' has no type 'Count'"), which the reference does not say.

Suggestion:
Say "import Net to read the fields of Net.Address" in the diagnostic; and state in `18-concepts.md` that a transparent alias is local to its package, or let it be named.

Status: Resolved


Stabilization note (2026-10-09): WIP: missing-owner-import diagnostic now names the import needed for field access. The reference explicitly documents transparent aliases as package-local. Add and run the focused missing-import contract before closing.


Verification (2026-10-09): Resolved: the missing-owner-import diagnostic names the import needed for field access. The reference documents transparent aliases as package-local. FeedbackStabilization/missing_import/missing_import.octfail passes through the real negative-contract runner with fixture-only FeedbackFieldConfig and FeedbackFieldNet packages.

---

Observation:
`oct run` prints `<invalid>` after a program whose `Main` is `fn Main() -> Void ! Error` and returns normally. A program whose `Main` returns `Int ! Error` prints its value. The binary that `oct build` makes of the same program prints nothing extra.

Suggestion:
Print nothing for a `Void` result, fallible or not.

Status: Resolved


Stabilization note (2026-10-09): The run adapter omits the zero/void Value result. internal/run/run_test.go invokes the real CLI execution path with fallible Void Main and verifies no invalid marker; internal/run tests pass.

---

Observation:
A fallible call could stand as a statement with `!` and not with `?`: `Save(path, value)?` was refused as a "standalone expression", so a `Void ! Error` call, which has no value to bind (`let _ = Save(...)?` is refused too), could be propagated only through a `match`.

Suggestion:
Permit a propagated call as a statement, as an unwrapped one is.

Status: Resolved

Resolution: The typechecker permits it; both lanes already ran it. Contract: `Language/Errors/Fallible/valid/propagation_as_a_statement.octest`, both lanes. Reference: `06-errors.md`.

---

Observation:
A function declared `-> Void ! Error` cannot fail on its own: `return error("cannot write")` in it is refused with "Void function cannot return a value". It can only fail by propagating the error of a call it makes. The check for a returned value in a `Void` function runs before the function's fallibility is looked at.

Suggestion:
Accept `return error(...)`, and the return of an `Error` value, in a `Void ! Error` function.

Status: Resolved

Stabilization resolution (2026-10-09): Fallible Void may return an Error value; infallible Void remains value-less. VoidFunctionsCanReturnErrors passes both lanes.

---

Observation:
`Sqrt` and `Ln` of a value outside their domain differ by lane when the value is not a literal. `Sqrt(0.0 - 1.0)` and `Ln(0.0)` stop the interpreted lane with "Sqrt expects non-negative input, got -1" and "Ln expects positive input, got 0"; the compiled lane computes a NaN and an infinity and goes on. Met while looking for a NaN to give `Json.Text`.

Suggestion:
Give the compiled lane the same preconditions, as the Random builtins have theirs in both lanes.

Status: Resolved

Stabilization resolution (2026-10-09): Compiled scalar helpers enforce the interpreter domain preconditions. sqrt_domain/ln_domain runtime contracts pass both lanes.

---

Observation:
The compiled Octagon materialiser refused a value of a refined concept over a dimensioned base (`concept Depth = Float<m>`): "expected Main.Depth dimension \"\", got \"m\"". It read the declared dimension out of the expected type's name, which for a concept is the concept's. The interpreter loaded the value.

Suggestion:
Read the dimension of the concept's base.

Status: Resolved

Resolution: `__octCheckNumericDimension` resolves a refined concept to its base. Contract: `Language/Data/Octagon/Load/valid/load_octagon_refined_dimension.octest`, both lanes.

---

Observation:
During `oct artifact` evaluation, `IO.ReadText`, `IO.ReadLines`, `IO.ReadBytes` and `IO.Exists` read an output the phase has already published, where it is staged, and refuse any other path. `Json.Load<T>` refused every path, as the first library's `JsonLoad` did, so an `[Artifact]` function could check its text outputs and not its JSON. Seven recorded experiments read their JSON summary back and could not be evaluated at all (`Experiments/PrometheusShadowAuthorityRakeLab` M1 to M5, `Experiments/PrometheusNumericalHeterogeneityLab` M0 and M1). The rule for reading an output back was also not in the reference.

Suggestion:
Give `Json.Load<T>` the rule the text readers have, and document it.

Status: Resolved

Resolution: `Json.Load<T>` goes through `prepareArtifactRead`. Contracts: `Language/Builtins/Json/artifact/json_artifact.octest` and `Language/Builtins/Json/invalid/artifact_evaluation_refuses_load.octfail`. Documented in `Language/reference/tooling/31-octest.md`.

---

Observation:
The CSV readers are still refused outright during `oct artifact` evaluation: `IO.Read`, `Csv.Read`, `Csv.ReadRows`, `Csv.ReadTable` and `Csv.ReadMatrix` of an output the phase has just published give "artifact evaluation rejected ambient operation CsvRead", while `IO.ReadLines` of the same file is allowed. `Experiments/FmBrownNoiseKalman/M2` reads its `metrics.csv` back this way and cannot be evaluated.

Suggestion:
Route the CSV readers through the same staged read as the text readers and `Json.Load<T>`, so one rule covers every reader: an output of the phase may be read back, nothing else.

Status: Resolved

Stabilization resolution (2026-10-09): Staged CSV reads use the existing artifact capability. TestStagedCsvArtifactReadback and ambient_csv.octfail pass; ambient reads remain refused.

---

Observation:
An `.octfail` expectation cannot contain a double quote. `expect artifact error: "... read \"data/tickets.json\"; only ..."` is matched with the backslashes in it, so it never matches a message that quotes a path. The expectation for `Language/Builtins/Json/invalid/artifact_evaluation_refuses_load.octfail` had to stop before the quoted path. `31-octest.md` does not say how the substring is read.

Suggestion:
Read the substring as an Oct string literal, with `\"` and `\\`, or say in the reference that it is taken verbatim up to the last quote.

Status: Resolved

Stabilization resolution (2026-10-09): Shared expectation parsing decodes escaped quotes/backslashes and preserves legacy raw quotes. Header tests and the quoted-path ambient_csv contract pass; 31-octest.md documents the rule.

---

Observation:
`Experiments/FmBrownNoiseKalman/M2` cannot be evaluated by `oct artifact`, for three reasons: `M2ArtifactFilesWrite` reads a CSV back (entry above); `M2Artifacts` is a second `[Artifact]` function that publishes the same files, a duplicate output path; and `M2bArtifactWriteAll` publishes `m2b_sweep_progress.json` once per sweep case, also a duplicate output path. Its recorded `metrics.csv` and `m2a_report.md` still hold the numbers of the Random v1 noise. Found while regenerating its JSON, which was done in a scratch copy with the three set aside.

Suggestion:
Remove `M2Artifacts`, report sweep progress with `Artifact.Progress` instead of a file, and regenerate the recorded outputs.

Status: Resolved

Stabilization resolution (2026-10-09): Removed duplicate M2Artifacts; progress goes through Artifact.Progress and the summary file is published once. Both generators pass, publishing nine outputs; a repeat reports all unchanged. Refreshed tracked metrics.csv and m2a_report.md for Random v2.

---

Observation:
A recorded artifact can be pinned by its SHA-256 somewhere else, and nothing connects the two. `internal/prometheus/DevelopmentReport/artifacts/Evt2OctOracle/experiment_ledger.json` records the hash of `o0_structural_witness.json`, which `Experiments/ZImageTurboNoiseRefiner0/M0` publishes. The Json rewrite changes the layout of every JSON file it writes, so evaluating that artifact now produces other bytes than the ledger names. It was found by searching for the file's name; a hash recorded without the name would not have been found.

Suggestion:
Decide whether the witness is regenerated and the ledger re-pinned, or kept as the bytes of the completed campaign. Longer term, let a ledger name the artifact it pins in a form `oct artifact` can check, so a change of bytes is reported where it happens.

Status: Resolved


Stabilization note (2026-10-09): Keep the completed Evt2OctOracle ledger and all archived witness bytes historical. New O0 runs default to Experiments/ZImageTurboNoiseRefiner0/M0/artifacts/current. A real new artifact run succeeds in a scratch root. oracle_ledger_test verifies all twelve named SHA-256 pins against archived bytes; .gitattributes disables text conversion for that archive. internal/prometheus tests pass.

---

Observation:
The compiled lane could not load a record with a field whose name does not begin with a capital letter. `record Doc { name: Int }` and `Json.Parse<Doc>("{\"name\": 2}")`, or the same through `LoadOctagon`, stopped with "panic: reflect: reflect.Value.Set using value obtained using unexported field". An Oct field is a Go field of the same name in the generated program, and reflection sets only an exported one. The interpreter loaded it. Declaring, constructing, reading and writing such a record all worked compiled; only the materialiser failed. `token_0`, `名前` and `ß` failed the same way.

Suggestion:
Set the field through its address.

Status: Resolved

Resolution: `__octMaterialize` sets a field it cannot set directly through `reflect.NewAt`. Contracts, both lanes: `Language/Builtins/Json/valid/json_field_names.octest`, `Language/Data/Octagon/Load/valid/load_octagon_lower_case_fields.octest`.

---

Observation:
A JSON key that no field name can match cannot be read into a record. Json matches a key to a field with `_`, `-`, `.`, spaces and case ignored, and a field name is letters and digits, so `$schema`, `@type`, `$ref`, `x/y` and `3d` match no field. Unknown members are an error, so a document with such a key cannot be loaded as a record at all: `Json.Parse<Doc>("{\"$schema\": \"x\", \"name\": \"y\"}")` gives `unknown member "$schema"; Doc has Schema, Name`. It loads only as a keyed table, which needs every value to have one type. JSON Schema, JSON-LD and OpenAPI documents all have such keys. `oct json infer` reports the object as having no declaration.

Suggestion:
Decide how a record names such a key. The smallest answer is to let the match ignore any character that cannot be in an identifier, so `$schema` matches `Schema`; that keeps "the declared type is the intent" and needs no new syntax, but writing would give `Schema`, not `$schema`. A field attribute that states the key (`[Key("$schema")]`) covers writing too.

Status: Deferred


Stabilization note (2026-10-09): Confirmed limitation, retained explicitly rather than silently discarding punctuation or unknown members. A reversible JSON key annotation/mapping requires a language and Json milestone, with collision checks, schema inference and parse/write round trips. Existing keyed-table support and inference refusal remain honest; stripping $, @ or / would conflate distinct keys.

---

Observation:
`record Range { ... }` is accepted, and the name still means the builtin `Range`: a field declared `Held: Range` has the builtin type, and `value.Held.X` is refused with "'r.Held' has type Range, which has no fields". Every other builtin type name is refused at the declaration ("duplicate type: String"), or by the parser (`Option`, `Vector`, `Matrix`).

Suggestion:
Add `Range` to the builtin type names a declaration cannot take.

Status: Resolved

Stabilization resolution (2026-10-09): Range is registered with the other reserved builtin types. range_name.octfail passes.

---

Observation:
`oct json infer` cannot tell an object of names from a record. `{"alice": 3, "bob": 5, "carol": 2}` has keys that read as field names and values of one type, as `{"width": 3, "height": 5, "depth": 2}` has, and both are printed as records. Nine names with a count each are still a record of nine fields. Keys that are visibly data (`user.created`, `u-100`, `2024`) do make a keyed table.

Suggestion:
None for the inference: no signal in the document separates the two, and a record is the reading that loses nothing. A `--table <path>` option that says "this object is a keyed table" would let a person state it without editing the output.

Status: Deferred


Stabilization note (2026-10-09): The record reading is deliberately retained because it preserves all data and the document supplies no authoritative table signal. A --table path override is an optional inference feature, requiring path validation and nested schema contracts; it is not a correctness fix and is deferred to a bounded JSON inference milestone.

---

Observation:
`oct test <directory>` of a directory with no `.octest` or `.octfail` file fails with "test failed: unknown package 'Main'". Seen on `Experiments/JsonIntentRecoveryLab/M0` after its only test was removed, and on an empty directory. The message names a package the user did not write and does not say that there is nothing to run.

Suggestion:
Say that the directory has no tests, and name it.

Status: Resolved

Stabilization resolution (2026-10-09): Empty directories report the missing .octest/.octfail tests and directory. TestEmptyDirectoryReportsMissingTests passes; SkipTest validation remains intact.

---
Observation:
`oct json infer` printed `record` for an object, the Json section of `Language/reference/language/17-standard-libraries.md` said "a JSON object is a `record`", and every Json contract declared records, while `Language/reference/language/18-concepts.md` says `concept` is preferred when a declaration names a valid domain value shape. A record-shaped concept already read and wrote as a record in both lanes, with no contract that said so. The tree has about 1860 `record` declarations and 30 record-shaped concepts, so the habit and the reference disagree well beyond Json.

Suggestion:
Print `concept`, state in the Json section that a concept with fields is the declaration of a document, and hold it with a contract.

Status: Resolved

Resolution:
`oct json infer` prints a record-shaped `concept` for an object and still a `record table` for a table. `Language/Builtins/Json/valid/json_concepts.octest` is the contract, in both lanes. The reference says it in 17 (Json) and 35 (CLI). The `record` declarations already in the tree, including those written when the experiments were migrated to Json v2, were not rewritten: they are valid, and rewriting them was not asked for.

---

Observation:
A record-shaped concept cannot state a requirement over its fields. `concept Window { Low: Int  High: Int  Require(Low <= High, "...") }` is a parse error ("expected ':' after concept field name"), and 18 Concepts lists record-field inspection as outside the requirement boundary and nominal records as unsupported refinement bases. A concept that is the declaration of a JSON document can therefore bound each field through a refined field type, but cannot say that two fields agree, which a JSON Schema says with `dependentRequired` or `if`/`then`.

Suggestion:
A Concepts milestone, not a Json change: requirements in a record-shaped concept. The open question is construction, since a record-shaped concept has no unrefined base to pass to a checked constructor: a literal whose requirements are not proved would have to be fallible. Json needs nothing new for it; a refusal would be reported at the object's place, as a refined field's is at its own.

Status: Deferred


Stabilization note (2026-10-09): Keep the explicit Concepts reference boundary. Cross-field requirements need a Concepts milestone defining construction failure, validation order, field access and JSON decode propagation; adding parser acceptance alone would create a false contract. Refined scalar fields remain supported; no workaround runtime in Oct was added.

---

