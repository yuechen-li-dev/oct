# ATTR-MAKE4-DESIGN — Make purity judgments recon

## Status

Design/recon only. This note does **not** implement `[Pure]` enforcement, does not change Make execution semantics, and does not add a language-wide purity system.

## Scope and goal

`[Pure]` in `Make.oct` should mean:

> this function is intended to construct deterministic Make data and should not directly touch host capabilities or observable effects.

That is deliberately narrower than formal referential transparency. Make purity is a pragmatic contract for Make plan/data construction, not a general Oct effect system.

`[Pure]` in `Make.oct` should **not** mean:

- no fallible returns;
- no `error(...)`;
- no record, array, enum, command-target, or C ABI metadata construction;
- no command strings as data;
- no local helper calls;
- no `match`, `switch`, `if`, `for`, or allocation;
- no ordinary validation logic.

## Part 1 — Enum, `match`, `switch`, and judgment documentation audit

### What enum forms Oct currently supports

The reference documents nominal enum sum types with qualified variants. Variants may be tag-only or single-payload:

```oct
enum ParseResult {
    Ok(Int)
    Err(String)
}
```

Supported construction forms are `Enum.Variant` for tag-only variants and `Enum.Variant(value)` for payload variants. Same-enum values support `==` and `!=`; cross-enum equality and enum ordering are rejected.

Out of scope in the current reference are multi-field payloads, tuple/record destructuring patterns, nested pattern matching, and guards.

### Are payload/tagged enums implemented or only designed?

They are implemented, not only designed.

Evidence from the implementation:

- The parser accepts `Variant(Type)` in enum declarations and stores an optional payload type.
- The AST has `EnumVariantDecl.Payload`, `EnumValueExpr`, and call-shaped enum variant construction through normal call expressions.
- The typechecker registers payload types, rejects payload omissions/additions, and checks payload expression types.
- The interpreter represents enum values with `EnumValue{TypeName, Variant, Payload}`.
- The compiled backend lowers enums to generated Go tag/payload representations and lowers enum `switch`/`match`.
- Typechecker tests cover enum-targeted utility with both tag-only and payload candidates.

One notable historical inconsistency: `docs/internal/judgment_enums_j1.md` says J1 was design-only and that payload candidates were deferred for M0. The current language reference and tests have moved beyond that: payload candidates in enum-targeted `when utility` are documented and typechecked.

### How `match` works today

There are two `match` surfaces in the codebase:

1. Legacy statement `match` for fallible results, with `ok(value)` and `err(e)` arms.
2. Expression `match` for enum analysis.

The user-facing reference now describes `match` as expression-only enum analysis with optional payload binding. Enum `match` takes an already-selected enum value and must be exhaustive. Payload variants require binding syntax; tag-only variants must not bind a payload. All result arms must have one result type.

Example shape:

```oct
return match result {
    case ParseResult.Ok(v) => v * 2
    case ParseResult.Err(msg) => -1
}
```

The implementation typechecks exhaustiveness and payload binding against the registered enum variant table.

### How `switch` differs from `match`

`switch` is expression-oriented dispatch, not payload analysis. It has two forms:

- subject switch: `switch value { case literalOrEnum => result else => fallback }`;
- condition switch: `switch { case condition => result else => fallback }`.

For enum subjects, `switch` compares qualified variants and is exhaustive if all variants are listed; otherwise it requires `else`. It does not bind payloads. Use `match` when payload data must be inspected.

Role split from the reference:

- `switch`: literal/value dispatch and tag-only enum branching;
- `match`: associated-data enum payload binding;
- `when`: utility/policy and Octomata decision surfaces.

### What `when utility` is

`when utility` is an expression form for deterministic utility-scored selection. Ordinary standalone utility selection scores candidate result expressions using `when` conditions and `score` expressions, with an `else` fallback. Scores are `Int`; conditions are `Bool`; highest score wins; equal scores keep the earliest matching case; `else` is used only when no case qualifies.

The enum-targeted form is:

```oct
when utility Decision {
    case Decision.Run when ready score 80
    case Decision.Fault("missing input") when bad score 100
    else Decision.Hold
}
```

For enum-targeted utility, the target after `utility` must be an enum type; every `case` result and `else` fallback must be a qualified variant construction of that enum. Payload variants are constructed explicitly, but payloads are not pattern matching and do not bind names.

### Is `when utility` implemented in Oct code, Go layer, docs only, or tests?

It is implemented in the Go language implementation layer and covered by tests. It is also documented in the language reference. It is not implemented in Oct user code and should not be; Oct is not self-hosted.

Implementation locations include parser support for `ast.UtilityWhenExpr`, typechecker support for standalone and enum-targeted utility, interpreter evaluation, and compiled lowering in `internal/build/compiler.go`.

### How judgment enums are represented in Go

There is no special AST node or declaration kind for a “judgment enum.” A judgment enum is an ordinary enum used as the target of `when utility EnumName`.

Go representation is therefore the ordinary enum representation:

- `ast.EnumDecl` with `[]ast.EnumVariantDecl`;
- optional payload type on each variant;
- `ast.UtilityWhenExpr.EnumTarget` to mark enum-targeted utility at the expression site;
- interpreter `ValueEnum`/`EnumValue` for runtime values;
- generated Go enum tag/payload structs in the compiled backend.

Separately, `internal/judgment` is a Go-only utility-scoring package for compiler/tooling decisions. It is not the runtime representation of Oct enum values.

### Are judgment utilities available to ordinary Oct code, compiler internals, or both?

Both concepts exist, but at different layers:

- Ordinary Oct code can use language-level `when utility`, including enum-targeted utility, as a normal expression.
- Compiler/tooling internals can use `internal/judgment`, a domain-neutral Go package that scores bounded candidates and returns an inspectable trace.

The two are conceptually aligned, but they are not the same implementation. The Go package exists specifically because Oct is implemented in Go and compiler/tooling internals cannot depend on Oct-layer code.

## Part 2 — Go implementation audit

### Parser and AST

Parser support exists for:

- enum declarations with optional payload type;
- enum variant references through field access and enum value expressions;
- payload variant construction via call expressions;
- `switch` expressions;
- legacy fallible `match` statements;
- enum `match` expressions;
- standalone and enum-targeted `when utility` expressions.

AST support includes `EnumDecl`, `EnumVariantDecl`, `SwitchExpr`, `MatchExpr`, `UtilityWhenPolicy`, `UtilityWhenCase`, `UtilityWhenExpr`, and `EnumValueExpr`.

### Typechecker

The typechecker registers enum declarations into an `enumInfo` table whose variants carry optional payload `Type`. It validates:

- duplicate enum/type declarations and variants;
- qualified enum values;
- payload presence/absence and payload type;
- same-enum equality and inequality;
- enum `switch` case type consistency, duplicate cases, and exhaustiveness/no-else errors;
- enum `match` exhaustiveness and payload binding requirements;
- standalone utility condition/score/result rules;
- enum-targeted utility target enum, qualified target variants, payload correctness, and required `else`.

The current Make attribute enforcement also lives in the Go implementation layer. Direct Make host primitive calls requiring `[RequiresAuthority]` are already a hard semantic rule, not a fuzzy judgment.

### Interpreter/runtime

The interpreter has `ValueKind` `ValueEnum` and `EnumValue{TypeName, Variant, Payload}`. It evaluates:

- tag-only enum values;
- payload enum construction;
- enum equality;
- `switch` by matching runtime values;
- enum `match` by comparing the selected variant and binding payloads;
- standalone `when utility`, including policy state for flow instances;
- enum-targeted `when utility`, evaluating only selected candidate/fallback values after scoring.

Octagon data loading/emission supports tag-only enum data in constrained contexts and rejects payload enum data literals.

### Compiled backend

Compiled lowering lives in `internal/build/compiler.go`. It contains MIR nodes and lowering/emission for enum values, payload variant construction, `switch`, `match`, and utility selection. The backend has explicit handling for enum-targeted utility and notes an unsupported path for delayed payload lowering in one compiled flow path, so Make purity diagnostics should not assume every utility payload lowering path is equally mature.

### Judgment/utility implementation

There are two layers:

1. Language-level `when utility`: parser/typechecker/interpreter/compiler support for Oct programs.
2. Go-layer `internal/judgment`: reusable deterministic candidate scoring with candidates, eligibility, considerations, totals, priorities, and traces.

`internal/judgment` is domain-neutral and intentionally does not own language semantics. Domain packages should generate candidates and considerations; the package only scores/selects/traces.

### Existing tests around judgment enums

Typechecker tests cover enum-targeted utility for tag-only variants, enum types that also have payload variants, payload candidate construction, payload fallback construction, wrong target types, wrong enum candidates, unqualified variants, missing `else`, non-`Bool` conditions, non-`Int` scores, and payload mismatch diagnostics.

Parser tests cover enum-targeted utility parsing with payload candidate/fallback constructors. Command-level enum switch tests cover enum-aware `switch` and cross-package enum switching.

## Structures that could represent Make purity judgment results

Existing structures that can represent the result shape include:

- simple Go enums/string constants in the Make checker/doctor code;
- `internal/judgment.Result` if the checker needs scored candidate selection and trace output;
- ordinary Oct enum concepts in documentation, if the public API is later exposed;
- Oct enum-targeted `when utility` for user programs, but not for compiler implementation logic.

For ATTR-MAKE4-H1, Make purity is mostly evidence classification plus severity mapping, not one-winner ambiguity resolution. Direct host authority is an authoritative hard rule. Unknown calls are a warning class. Pure data is an allow class. That suggests a small Go-only internal evidence enum is cleaner than forcing `internal/judgment` into a non-selection problem.

## Is there a reusable judgment/utility framework?

Yes, `internal/judgment` is reusable for bounded one-winner Go tooling decisions. It is not specific to one feature. However, its documented boundary says not to move language semantics into judgment scoring.

Make purity can reuse the **shape** and vocabulary of judgment utilities without necessarily reusing `internal/judgment.Decide()` in H1. If a future doctor report must choose one “best explanation” among multiple plausible diagnostics for the same expression, `internal/judgment` would be a good fit. For first enforcement, direct evidence collection is simpler and less coupled.

## Could Make purity diagnostics reuse that framework?

They could, but H1 should not start there. Purity evidence is naturally multi-label: a function may contain pure data construction, an unknown helper call, and a direct authority call. The result is not exactly one winner. Mapping each evidence item to severity is a rule table, not a scored selection.

A clean future use would be explanation ranking, for example choosing the most helpful primary diagnostic among several warnings at one call site. That is optional and should remain domain-owned.

## Recommendation: Go-only internal enums first

Use Option C in concept, with implementation starting as Go-only:

- Go typechecker/doctor uses internal Make-specific evidence/severity enums.
- Documentation mirrors Oct enum/judgment concepts so the model is understandable and future-proof.
- Do not expose `Make.PurityJudgment` or `Make.JudgmentUtility` in `Libraries/Make` yet.
- Later, if diagnostics become stable and user-facing validation APIs are desired, expose a Make-level enum deliberately.

Reasons:

- avoids depending on Oct-level judgment maturity for compiler diagnostics;
- avoids exposing unstable diagnostic categories as public Make API;
- preserves separation of concerns: Go implements Make attribute checks; Oct expresses Make files and tests;
- keeps `[Pure]` Make-specific and avoids a language-wide purity system;
- leaves room to use `internal/judgment` only where a real one-winner selection appears.

## Part 3 — Make purity problem statement

`[Pure]` in `Make.oct` means:

> The function is intended to build deterministic Make plan/data values and should not directly touch host capabilities or observable effects.

It is a Make planning/data-construction marker. It should help authors and `oct make doctor` distinguish pure plan builders from host authority actions without making normal Oct programming unpleasant.

The boundary should be high-confidence and direct-call focused in H1. No transitive call graph purity is required. No broad language-wide effect system is introduced.

## Part 4 — Proposed Make purity evidence categories

Use a small closed Go-only evidence model:

```text
MakePurityEvidence
  PureData
  HostAuthority
  ObservableEffect
  UnknownCall
  DeterministicFailure
  ControlFlow
```

Meanings:

- `PureData`: literals, records, arrays, enum values, target construction, command target records, C ABI metadata records, string commands as data.
- `HostAuthority`: direct calls to Make host primitives such as `Make.Tool`, `Make.Env`, `Make.Exec`, `Make.Remove`, `Make.ReadText`, and related authority-bearing host calls.
- `ObservableEffect`: direct user-visible output/effect builtins such as `Print` if present in the current Make context.
- `UnknownCall`: local, imported, wrapper, or otherwise unresolved function call whose purity is not known in H1.
- `DeterministicFailure`: `error(...)`, validation failure construction, and fallible propagation from known-pure/fallible helpers. This is allowed.
- `ControlFlow`: `if`, `switch`, `match`, `for`, `while`, and utility selection. This is allowed by `[Pure]` in H1; `[NoWhile]` remains the specific while-loop policy.

Severity/utility model:

```text
MakePurityUtility
  Allow
  Info
  Warning
  Error
```

Default mapping for `[Pure]` functions:

| Evidence | Default utility | Rationale |
| --- | --- | --- |
| `PureData` | `Allow` | This is the motivating case. |
| `DeterministicFailure` | `Allow` | Validation and `error(...)` are deterministic data/control results, not host effects. |
| `ControlFlow` | `Allow` | Branching/looping is not impurity by itself; `[NoWhile]` owns while style. |
| `HostAuthority` | `Error` | Direct host capability calls contradict Make purity and already require `[RequiresAuthority]`. |
| `ObservableEffect` | `Warning` in first doctor pass; candidate `Error` in strict mode | Printing is an observable effect, but current call surfaces should be audited before hard-failing all output. |
| `UnknownCall` | `Warning` in doctor | H1 has no transitive call graph purity, so warnings provide useful pressure without monastic strictness. |

If `Print` or an equivalent output primitive is known to be a direct host primitive in the Make checker, it may be promoted to `Error` with the same confidence as `HostAuthority`. Otherwise warn first.

## Part 5 — Oct-level enums vs Go-only internal enums

### Option A — Go-only internal judgment enums

Pros:

- smallest and safest;
- no dependency on public Oct judgment API stability;
- easy to keep Make-specific;
- natural fit for typechecker and doctor diagnostics.

Cons:

- less dogfooding of Oct enum/judgment concepts;
- not directly reusable by Make user code.

### Option B — Oct-level judgment enums in `Libraries/Make`

Pros:

- visible and documentable as Make API;
- could support future `Make.Validate` or structured doctor output.

Cons:

- exposes implementation diagnostics too early;
- risks coupling public Make APIs to current checker heuristics;
- tempts self-hosting-style designs where Oct code defines language/tool behavior.

### Option C — Hybrid staged model

Start with Go-only internal enums, document the model using Oct judgment vocabulary, and reserve an intentional future API if/when categories stabilize.

Recommendation: **Option C, implemented as Go-only internal enums first**.

## Part 6 — Proposed staged enforcement policy

### ATTR-MAKE4-H1

Hard errors in `[Pure]` functions:

- direct calls to Make host primitives requiring `[RequiresAuthority]`;
- direct obvious observable effects only if the checker can identify them with high confidence.

Doctor warnings in `[Pure]` functions:

- call to unmarked local helper;
- call to imported/unknown function;
- direct `Print`/output call if not yet promoted to hard error;
- optional informational note when purity could not be fully established because H1 is direct-call-only.

Allowed in `[Pure]` functions:

- `error(...)`;
- fallible return types;
- `?` propagation from known pure/fallible helper calls;
- records, arrays, enums, literals, and allocation;
- `match`, `switch`, condition switch, `if`, and `for`;
- `while`, unless separately rejected/warned by `[NoWhile]` policy;
- command strings and `Make.CommandTarget` record construction;
- C ABI metadata construction.

Explicit decisions:

1. `error(...)` is allowed.
2. `Print(...)` should warn first unless it is already classified as host authority; strict mode can escalate.
3. Local helper calls should warn unless the direct callee is marked `[Pure]`.
4. H1 should not require every direct local callee to be `[Pure]` as a hard error.
5. `[Pure]` should not imply `[NoWhile]`.
6. `while` in `[Pure]` should not warn solely because it is `while`; `[NoWhile]` is explicit.
7. Command strings in `Make.CommandTarget` are pure data.

### Future strict mode

A future strict mode can escalate:

- unknown/unmarked direct calls from warning to error;
- observable output from warning to error;
- possibly imported calls without declared purity from warning to error.

Strict mode should still not become general language purity. It remains Make-specific.

## Part 7 — Interaction with `[RequiresAuthority]`

ATTR-MAKE3 already gives `[Pure]` a meaningful boundary:

- `[Pure] + [RequiresAuthority]` is invalid;
- direct Make host primitive calls require `[RequiresAuthority]`;
- therefore a `[Pure]` function that directly calls `Make.ReadText`, `Make.Exec`, `Make.Env`, etc. already fails because it cannot also be marked `[RequiresAuthority]`.

ATTR-MAKE4 should formalize this as purity evidence and improve the primary diagnostic.

Recommended diagnostic wording:

```text
function RustArtifact is marked [Pure] but calls Make.ReadText, which requires host authority; move the call to a [RequiresAuthority] helper or pass read data into the pure planner
```

If emitted from the existing authority checker, a secondary note can preserve the old rule:

```text
Make.ReadText is a Make host primitive and direct callers must be marked [RequiresAuthority]
```

This is clearer than only saying:

```text
function RustArtifact calls Make.ReadText and must be marked [RequiresAuthority]
```

because that old wording suggests adding `[RequiresAuthority]`, which is invalid on a `[Pure]` function.

## Part 8 — Doctor output design

Doctor should report direct-call-only purity findings without pretending to prove transitive purity.

Example:

```text
Pure diagnostics:
  Plan: ok
  RustArtifact: ok
  BuildTarget: ok
  FormatOutputPath: warning: calls helper NormalizePath without [Pure]; transitive purity is not checked in this release
  PrintBanner: warning: calls Print, an observable output operation inside a [Pure] function
```

If the file has a hard typecheck/attribute error, `oct make doctor` may fail before printing full doctor output. That is acceptable. Where possible, doctor can add a focused `[Pure]` explanation before returning failure.

Limitations to state in doctor/help text:

- H1 checks direct calls only;
- local helpers are trusted only when directly marked `[Pure]`;
- imported/wrapper calls are warnings unless specifically known;
- command strings and target records are data, not execution.

## Part 9 — Smallest implementation path

1. Add Make-specific Go evidence/severity constants near the existing Make attribute checker/doctor code.
2. Reuse the existing direct Make host primitive identification from ATTR-MAKE3.
3. During Make file attribute checking, when a direct host primitive call appears inside `[Pure]`, emit the clearer `[Pure]` contradiction diagnostic instead of suggesting `[RequiresAuthority]`.
4. In `oct make doctor`, walk direct calls in `[Pure]` functions and emit warnings for unmarked local/imported/unknown calls and output calls.
5. Add `.octfail` / `.octest` language contracts under `Language/` for user-visible behavior if checker behavior changes.
6. Run targeted typecheck/cmd tests and full Go tests only when implementation changes occur.

Do **not** add transitive call graph analysis in H1.

## Future work

- Strict mode for Make purity warnings.
- Structured doctor output that can expose evidence categories and severity.
- Optional diagnostic explanation ranking via `internal/judgment` when multiple explanations compete at one site.
- Possible public `Make.PurityJudgment` and `Make.JudgmentUtility` enums only after internal categories stabilize and a Make user API needs them.
- Audit compiled flow utility payload lowering before relying on enum-targeted utility as a compiled-path teaching example.

## Deferred/non-goals reaffirmed

This design does not implement:

- full purity checking;
- transitive call graph purity;
- Make execution semantic changes;
- new general-purpose attributes;
- user-defined attributes;
- macros/reflection/metaprogramming;
- language-wide purity;
- attribute requirements for normal Oct code.

## ATTR-MAKE4-H1 implementation note — Go-layer purity judgment diagnostics

ATTR-MAKE4-H1 implements the first Make purity diagnostic pass without adding a full purity effect system and without moving hard Make authority semantics into scoring.

### `internal/judgment` usage

The implementation uses `internal/judgment` only to select the primary explanation when a `[Pure]` Make function has multiple direct evidence items. Evidence extraction and semantic classification remain owned by the Make checker/doctor code. The judgment candidates are diagnostic evidence records, not language semantics. The hard rule that direct Make host primitives require `[RequiresAuthority]` remains direct checker logic.

The current scoring mirrors the design intent:

| Evidence | Utility | Default severity |
| --- | ---: | --- |
| `HostAuthority` | 100 | `Error` |
| `ObservableEffect` | 60 | `Warning` |
| `UnknownCall` | 40 | `Warning` |
| `PureData` | 0 | `Allow` |
| `DeterministicFailure` | 0 | `Allow` |
| `ControlFlow` | 0 | `Allow` |

This means that a function with both `Make.ReadText(...)` and an unmarked helper call reports the host-authority contradiction as the primary explanation instead of burying it behind the lower-confidence unknown-call warning.

### Evidence categories

The H1 implementation keeps the evidence categories internal to Go and does not expose `Make.PurityJudgment` or `Make.JudgmentUtility` in `Libraries/Make`.

- `PureData`: literals, arrays, records, enum values, target record construction, C ABI metadata construction, command strings as data, calls to directly marked `[Pure]` helpers, and non-host `Make` data helpers.
- `HostAuthority`: direct calls to Make host primitives such as `Make.Tool`, `Make.Env`, `Make.Exec`, `Make.ReadText`, `Make.WriteText`, `Make.Remove`, `Make.HashFile`, and related host operations.
- `ObservableEffect`: direct `Print(...)` calls in a `[Pure]` function.
- `UnknownCall`: direct local/imported/helper calls whose purity is not known in H1.
- `DeterministicFailure`: `error(...)` validation/failure construction; this remains allowed.
- `ControlFlow`: `if`, `switch`, `match`, `for`, `while`, and `when utility`; these remain allowed by `[Pure]` in H1. `[NoWhile]` remains the separate while-loop policy.

### Hard errors versus doctor warnings

Direct Make host primitive calls remain hard errors in `Make.oct` functions unless the enclosing function has `[RequiresAuthority]`. For `[Pure]` functions the diagnostic is now purity-specific because `[Pure]` and `[RequiresAuthority]` cannot be combined:

```text
function RustArtifact is marked [Pure] but calls Make.ReadText, which requires host authority; move the call to a [RequiresAuthority] helper or pass read data into the pure planner
```

For non-`[Pure]` functions, the existing ATTR-MAKE3 diagnostic remains:

```text
function CheckTools calls Make.Tool and must be marked [RequiresAuthority]
```

`oct make doctor` reports `[Pure]` functions as `ok` when no concerning direct evidence is found. It warns for direct calls to helpers without `[Pure]` and for direct `Print(...)` observable output. Unknown-call and observable-effect warnings are advisory and do not become hard failures in H1.

### Direct-call-only limitation

ATTR-MAKE4-H1 is intentionally direct-call-only. If `[Pure] Plan()` calls unmarked `BuildTarget()`, doctor may warn that `BuildTarget` is not marked `[Pure]`, but the checker does not recursively inspect `BuildTarget` as part of `Plan()`'s purity. If `BuildTarget()` itself directly calls `Make.Tool` without `[RequiresAuthority]`, ATTR-MAKE3 rejects `BuildTarget()` directly. If `BuildTarget()` is marked `[Pure]`, the direct call from `Plan()` is treated as OK.

### Public enum deferral

The implementation model intentionally mirrors Oct's enum/judgment idiom and the Go-layer `internal/judgment` primitive, but no public Make enums are exposed yet. Public `Make.PurityJudgment` or `Make.JudgmentUtility` should wait for a structured user-facing `Make.Validate` or doctor API where the categories are stable.

### Future strict/transitive mode

A future ATTR-MAKE4 follow-up can add a strict/transitive mode that walks the Make helper graph, distinguishes imported package purity metadata, and optionally escalates unknown/effect evidence. That future mode should still keep authoritative host-capability rules as hard checker logic and reserve `internal/judgment` for selecting/ranking diagnostic explanations.
