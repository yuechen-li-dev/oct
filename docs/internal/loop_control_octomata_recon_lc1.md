# LC1-DESIGN-RECON: loop control, Octomata, and resumable loop helpers

Date: 2026-06-19

## Scope and conclusion

This is a reconnaissance report only. It does **not** implement `continue`, `break`, generator syntax, iterator protocols, Python-style `yield`, named arguments, overloads, generics, Octomata semantic changes, `for`/`while` semantic changes, hidden loop state machines, labeled control flow, async/coroutine behavior, or persistent checkpointing.

Recommended LC1 outcome: land this report, then do a very small diagnostic-only follow-up for unsupported `continue` / `break`. Recommended LC2 outcome: prototype a first-party explicit `Loop` helper package before considering loop-control keywords.

The strongest finding is that ordinary loops have no local loop-control signal today. Function returns and fallible propagation exit loops through the existing statement-result path, while Octomata has a separate explicit `flowSignal` path for `goto`, `suspend`, `remember`, `resume`, and state `return`. Adding `continue` / `break` directly would require introducing a new non-return control signal for ordinary statements and mirroring it in typecheck, interpreter, MIR lowering, generated Go, and fixtures. A helper library avoids hidden control flow and aligns better with Oct's explicit-state thesis.

## Part 1: current loop semantic surface

### Current syntax

`for` syntax is:

```oct
for i in start..end {
    ...
}

for i in start..end step k {
    ...
}
```

The reference says `for i in start..end` has inclusive start and exclusive end bounds; `for` requires closed ranges; bounds must be `Int`; `step` is optional; and `step` must be a positive `Int`.

`while` syntax is:

```oct
while condition {
    ...
}
```

The reference requires a `Bool` condition.

### Ranges

Ranges are first-class expressions with optional endpoints in expression positions: `start..end`, `start..`, `..end`, and `..`. Stepped open-ended ranges are rejected in M0. `for` loops narrow this surface by requiring both start and end.

The interpreter defaults missing `step` to `1`, rejects non-`Int` endpoints/steps at runtime if an invariant is violated, rejects non-positive steps, and rejects closed ranges whose start exceeds end. Typechecking rejects non-`Int` endpoints/steps and static non-positive steps.

### Tokens and parser behavior for `break` / `continue`

`break` and `continue` are **not** tokens or keywords today. The lexer keyword table does not reserve either spelling, so both lex as ordinary identifiers.

A standalone `continue` or `break` statement currently parses as an expression statement containing an identifier expression. Typechecking then rejects it with the generic expression-statement diagnostic because standalone expressions must be side-effecting calls, assignments, or returns. If the name is first resolved first in a particular code path, it may also surface as an unknown-name style diagnostic, but the intended current architecture has no dedicated unsupported-loop-control diagnostic.

Expected current user experience is therefore generic and not loop-aware, approximately:

```text
function Main: This standalone expression cannot be ran. In Oct, a statement like this must be a function call (for side effects), an assignment, or a return. If you meant to keep the value, assign it to a variable; if you meant to return it, use return.
```

### Parser / AST / typechecker structure

The AST has `ForStmt` with loop variable, range expression, and body, and `WhileStmt` with condition and body. It does not have `BreakStmt`, `ContinueStmt`, or a general ordinary-loop control statement node.

The parser dispatches `for` and `while` from statement position, parses `for <identifier-like> in <expression> <block>`, and parses `while <expression> <block>`. `flow`, `state`, and `step` are accepted as identifier-like names in relevant positions, which is why `step` can remain contextual.

Typechecking checks `for` by requiring the range expression to have `Range` type, requiring closed endpoints when the syntactic range is directly visible, defining the loop variable as immutable `Int` in a child loop scope, and checking the body. Typechecking checks `while` by requiring a non-fallible `Bool` condition and checking the body in the current scope.

### Interpreter structure

Ordinary statement execution returns a `stmtResult` with a `returned` bit and value. This is the only structured non-local exit signal for ordinary functions today.

For ordinary `for`, the interpreter evaluates the range once, then runs a Go loop from `Start` to `End` exclusive by `Step`. Each iteration creates a child environment and defines the loop variable. If executing the loop body returns a `stmtResult` with `returned=true`, the loop immediately returns that result to the enclosing function.

For ordinary `while`, the interpreter repeatedly evaluates the condition in the same environment, exits when false, and returns the body result immediately if the body returned.

Inside flow state bodies there is a separate `flowSignal` path. Flow `while` and `for` mirror ordinary loops structurally, but they propagate `flowSignal` values such as `goto`, `suspend`, `return`, and resume-derived goto. This path is Octomata-specific and not used by ordinary function loops.

### Compiled lowering structure

The compiled path lowers ordinary `while` to MIR basic blocks: preheader jump to condition block, branch to body or exit, lower body, and back-edge to condition if the body falls through.

The compiled path lowers `for` only when the range expression is syntactically an `ast.RangeExpr`; it does not currently lower arbitrary first-class `Range` values in `for`. Lowering evaluates start/end/step into locals once, emits a positive-step check, emits a start/end order check, initializes the loop variable, branches on `i < end`, lowers the body, emits an increment block, and jumps back to the condition.

This is a noteworthy interpreted/compiled shape difference: typechecking accepts any expression of type `Range` for a `for` loop, while compiled lowering currently supports direct range expressions only.

### Return and error propagation out of loops

`return` exits loops because it sets the ordinary `stmtResult.returned` path in the interpreter and terminators in MIR lowering. Fallible propagation via `?` is expression-level: typechecking allows it only in fallible contexts, and lowering/evaluation convert an error result into an early return/error path. Force unwrap `!` asserts success and fails if the value is an error. Loop bodies do not need special error-control mechanics beyond the existing return/fallible expression machinery.

### Is there a natural ordinary-loop control signal path?

No. Ordinary loops have only the normal statement-result/return path. Octomata flow execution has a signal path, but it is scoped to state execution and carries flow-specific transitions. Reusing it directly for ordinary loops would blur the Go-implements-language / Oct-expresses-programs separation and would also risk making ordinary loops secretly Octomata-like without an explicit design.

## Part 2: current Octomata semantic surface

### What `remember` means today

`remember` is valid only inside flow state bodies. It stores the current state name in a single resume slot on the flow instance. A later `remember` overwrites the slot.

### What `resume` means today

`resume` is valid only inside flow state bodies. It jumps to the state stored in the resume slot, clears the slot on successful resume, and raises a runtime error if the slot is empty.

### Flow/state syntax and transitions

The reference describes `flow Name(params) -> ReturnType { ... }`, at least one `state`, optional `board`, `goto StateName`, `suspend`, state `return`, guard `when`, and controller `when policy`. Guard `when` actions are `goto`, `suspend`, `return`, or action blocks constrained by state-body rules.

The interpreter represents flow instances with current state, completion status, result, board values, state history, and a single resume target slot. Stepping a flow executes one scheduling step; `suspend` yields without completion; `return` completes; `goto` changes state.

The compiled path has explicit generated flow machinery: state IDs, an instruction counter, state history, resume target fields, and generated code snippets for `remember` / `resume` that update `hasResumeTarget`, `resumeTarget`, current state, instruction, and history.

### Resumable local loops

Octomata does not make ordinary local loops resumable today. A `while` or `for` may appear inside a state body, and if it emits a flow signal such as `goto`, `suspend`, `return`, or `resume`, that signal exits the loop. But loop iteration position itself is not captured as an independent resumable continuation. To model resumable iteration today, users must encode the loop index/progress as board state and transition among states explicitly.

### Can Octomata encode skip/stop/resume iteration patterns?

Yes, but at state-machine granularity. Skip/stop/resume patterns can be represented with board fields such as current index, active/done flags, and states such as `Check`, `Work`, `Skip`, `Done`, plus `goto`, `suspend`, `remember`, and `resume`. That is explicit and inspectable, but too heavy for simple local loops.

### Interaction with ordinary loops

Ordinary loops are not lowered to Octomata. Octomata flows can contain ordinary loop syntax in state bodies, but the flow executor wraps statements in a separate flow-signal interpreter. There is no current bridge where a normal function loop becomes a flow, and no iterator/generator hidden state machine behind loops.

### Terminology that should influence helper names

Existing terminology emphasizes `flow`, `state`, `board`, `goto`, `suspend`, `remember`, `resume`, `Step`, `Active`, `Complete`, `Result`, `ResumeTarget`, `StateHistory`, and `BoardSnapshot`. A loop helper should avoid implying hidden `Iterator` / `Generator` protocols. It may reuse `Active`, `Step`, or `Resume` concepts carefully, but should not imply it is the same as Octomata `resume` unless that relation is documented.

## Part 3: current package/library constraints and helper feasibility

### Package/library rules

First-party libraries under `Libraries/<Name>` can define public records, enums, and functions used by ordinary Oct code after `import Name`. Imported members are qualified as `Name.Symbol()` for functions and `Name.Type.Member` for enum/type-style access. The M0 call namespace is exactly two segments, so `Loop.Range(...)` is supported but nested package calls such as `Loop.Range.Current(...)` are not.

Records are nominal immutable values. A helper state record can be returned, rebound, and updated with `with`. Individual record fields are not assigned in place; whole-value rebinding is the intended style. This makes a `RangeState` helper naturally copy-oriented and avoids accidental aliasing for scalar fields.

Library functions can be fallible with `! Error` and callers can use `?`, `!`, or `match`. Interpreted and compiled package calls are supported for ordinary first-party Oct libraries, subject to the compiled path supporting the syntax used inside the library.

### Evaluation of proposed helper shape

The proposed surface:

```oct
package Loop

record RangeState {
    Current: Int
    End: Int
    Step: Int
    Active: Bool
}

fn Range(Start: Int, End: Int) -> RangeState
fn RangeStep(Start: Int, End: Int, Step: Int) ! Error -> RangeState
fn IsActive(State: RangeState) -> Bool
fn Current(State: RangeState) -> Int
fn Resume(State: RangeState) -> RangeState
fn Stop(State: RangeState) -> RangeState
```

The use site:

```oct
import Loop

var loop = Loop.Range(0, 10)

while Loop.IsActive(loop) {
    let i = Loop.Current(loop)

    if ShouldStop(i) {
        loop = Loop.Stop(loop)
    } else {
        if !ShouldSkip(i) {
            Work(i)
        }

        loop = Loop.Resume(loop)
    }
}
```

Assessment:

1. It should parse today after adding an `import Loop`; package-qualified calls with two segments are supported. The sample's prefix `if !ShouldSkip(i)` likely needs Oct's documented logical spelling: either `if not ShouldSkip(i)` or equivalent supported unary form, because `!` is postfix fallible unwrap, not prefix logical not.
2. It should typecheck if `Loop` exists, functions are declared exactly, `ShouldStop`, `ShouldSkip`, and `Work` are available with matching signatures, and any fallible calls are handled. `Loop.RangeStep(...)` callers must handle fallibility.
3. It should interpret today if implemented as ordinary Oct records/functions.
4. It should compile today if the helper avoids unsupported constructs. Use record `with` updates and ordinary arithmetic/conditionals; avoid relying on arbitrary `for Range` compiled lowering inside helper internals.
5. Main language gaps: no nested namespace calls, no named arguments, no generics/overloads, no methods, no mutation of record fields, and no negative-step closed-range semantics in current built-in ranges/for loops. These are manageable for an M0 helper.
6. Record copying/mutation semantics are correct for scalar `RangeState`. `Resume` and `Stop` should return new records. Users should rebind: `loop = Loop.Resume(loop)`.
7. Scalar record state should not alias. If future helpers include arrays or nested records, copy semantics and compiled clone behavior need review.
8. The helper fits existing style if it is explicit, package-qualified, record-returning, and avoids `Iterator` / `Generator` terms.

### Pure Oct helper feasibility for M0

A pure Oct `Loop` helper library is feasible for scalar range-style state. It should be designed as explicit state transformation functions, not as a protocol. The lowest-risk LC2 slice is a single `Libraries/Loop` package with `RangeState` plus positive-step increasing ranges only, interpreted/compiled tests, and docs that show guard-loop alternatives.

## Part 4: diagnostic design for unsupported `continue` / `break`

### Recommended diagnostic text

Adapted to current concise error style:

For `continue`:

```text
error: `continue` is not a loop-control keyword in Oct.
hint: For simple loops, use a guard condition.
hint: For resumable or generator-like loops, use explicit Loop state helpers or Octomata remember/resume.
```

For `break`:

```text
error: `break` is not a loop-control keyword in Oct.
hint: Put the stop condition in the loop condition, or use explicit Loop state helpers for resumable loop flows.
```

Until a `Loop` package exists, the first diagnostic should say "future Loop state helpers" or omit the exact package name. Once LC2 lands, mention `Loop` directly.

### Where it should live

Best LC1 diagnostic-only implementation point: parser statement dispatch. Because `break` and `continue` currently lex as identifiers, `parseStatement` can detect an identifier with lexeme `break` or `continue` in statement position before falling through to identifier-leading assignments or expression statements. This avoids reserving new keywords and should not affect legitimate uses such as a variable named `continue` in expression context.

Alternative: reserve `KeywordBreak` and `KeywordContinue` in the lexer and reject them in `parseStatement`. This gives a broader reservation but risks breaking existing code using `break` or `continue` as identifiers.

### Should they become reserved words now?

Not yet. Reserving them only for diagnostics would be disproportionate before the language decides whether these words will ever become supported. Statement-position detection is lower risk and preserves compatibility with existing code that may use those names as variables or parameters.

### Compatibility risk

Statement-position rejection changes behavior only for source lines where `break` or `continue` appears as a bare statement. Such code does not have meaningful supported behavior today. Reserving the words globally could break function parameters, locals, or fields named `break` / `continue`; that should wait for a formal reservation policy.

## Part 5: design options

### Option A: do nothing except docs

- Semantic clarity: medium. The language remains explicit but users hit generic errors.
- Implementation cost: very low.
- Risk of Python/C leakage: low.
- Compatibility with Octomata: high.
- Ergonomics: low for users who naturally try `continue` / `break`.
- v0.1 suitability: acceptable but misses a cheap diagnostic win.
- Areas touched: docs/reference and internal notes only.

### Option B: diagnostic-only

- Semantic clarity: high. Oct explicitly says these are not loop-control keywords.
- Implementation cost: low if handled in parser statement dispatch without new tokens.
- Risk of Python/C leakage: low, provided wording points to guard conditions and explicit helpers rather than promising future C-like semantics.
- Compatibility with Octomata: high.
- Ergonomics: medium; users get immediate guidance.
- v0.1 suitability: strong.
- Areas touched: parser, `.octfail` fixtures, possibly reference diagnostics docs.

### Option C: first-party `Loop` helper library

- Semantic clarity: high. Iteration state is a visible record and state transitions are explicit functions.
- Implementation cost: low-to-medium. Requires library package, manifest, docs, interpreted/compiled fixtures.
- Risk of Python/C leakage: low if names avoid `Iterator`/`Generator` and examples emphasize state records.
- Compatibility with Octomata: high. It complements Octomata by covering local/state-record loops without pretending to be a flow.
- Ergonomics: medium. More verbose than `continue` / `break`, but explicit and teachable.
- v0.1 suitability: good if constrained to increasing scalar ranges and no protocol machinery.
- Areas touched: `Libraries/Loop`, `Registry/registry.oct` if first-party registry exposure is desired, `Language/...` tests, reference docs.

### Option D: local loop-control keywords

- Semantic clarity: medium. Could be framed as local loop-flow transitions, but most users will import C/Python expectations.
- Implementation cost: medium-to-high. Requires AST nodes, parser, typechecker context validation, interpreter loop signal, flow-state interactions, MIR lowering, generated Go lowering, tests.
- Risk of Python/C leakage: high unless carefully documented and restricted.
- Compatibility with Octomata: mixed. It may duplicate a small local subset of transition semantics without Octomata's explicit state model.
- Ergonomics: high for familiar loops.
- v0.1 suitability: poor until explicit helper model and diagnostics exist.
- Areas touched: lexer/parser/AST/typecheck/interpreter/compiler/reference/fixtures.

### Option E: eventual Octomata-backed loop sugar

- Semantic clarity: potentially high if sugar lowers to visible/inspectable explicit state machines.
- Implementation cost: high.
- Risk of Python/C leakage: medium; lower than D if syntax is not `yield`/generators and lowering is explicit.
- Compatibility with Octomata: high if designed as Octomata syntax, not ordinary loop mutation.
- Ergonomics: potentially high for resumable workflows.
- v0.1 suitability: out of scope.
- Areas touched: language design, parser/AST, typechecker, interpreter, compiled backend, docs, Octomata runtime, observability tooling.

## Part 6: recommended staged plan

### LC1

- Land this design reconnaissance report.
- Optionally follow with a tiny parser diagnostic-only patch for bare-statement `continue` / `break`.
- Do not add helper library in LC1 unless explicitly requested as a separate task.
- Confirm no loop semantics changed.

### LC2

- Implement first-party `Loop` helper library with scalar increasing `RangeState` only.
- Include docs and examples that contrast simple guard conditions, `Loop` state helpers, and Octomata flows.
- Add interpreted and compiled parity tests for basic range iteration, stop, resume/advance, invalid zero step, and fallible construction.

### LC3

- Decide whether `continue` / `break` should exist as sugar over local loop-flow transitions.
- Only proceed after users have an explicit `Loop` model and after diagnostics collect enough friction evidence.
- If implemented, define semantics independently from C/Python labels and prohibit labeled forms.

### LC4

- Explore special resumable loop syntax backed by explicit Octomata state machines if real workloads need it.
- Keep ordinary loops ordinary.
- Preserve inspectability through state history, board snapshots, or equivalent tooling.

## Part 7: naming audit

### Package names

- `Loop`: recommended for LC2. Short, explicit, and package-qualified usage reads well: `Loop.Range`, `Loop.Resume`, `Loop.Stop`.
- `FlowLoop`: acceptable but sounds tied to Octomata `flow`; may imply ordinary helpers require flow semantics.
- `Stepper`: accurate for advancement but less discoverable for stop/active/current helpers.
- `Sequence`: too data-structure-oriented and may imply collection protocols.
- `RangeLoop`: too narrow if future helpers include non-range loop state.
- `Iterator`: avoid. It implies hidden protocol machinery and external language expectations.
- `Generator`: avoid. It implies hidden resumable frames and `yield`-style behavior.
- `OctomataLoop`: too heavy for ordinary local helper state and could blur helper vs flow semantics.

### Function names

- `Range`: good constructor name for simple increasing ranges.
- `RangeStep`: acceptable M0 substitute for overloading/default args; explicit fallible zero-step validation.
- `IsActive`: good predicate; avoids collision/confusion with Octomata `Active(flow)` builtin.
- `Current`: good accessor.
- `Resume`: conceptually aligned with explicit progression after a pause/skip, but potentially confusing because Octomata `resume` jumps to a remembered state. If used, docs must say `Loop.Resume` means "advance this loop state to its next position" and is not Octomata `resume`.
- `Stop`: good early-termination transition.
- `Skip`: ambiguous. It may mean "skip work but advance" or "mark skipped without advancing". Avoid in M0 unless semantics are unmistakable.
- `Next`: familiar but iterator-flavored and may imply hidden mutation. Use cautiously.
- `Advance`: strong alternative to `Resume`; clearer as pure state transform and less Octomata-conflicting.
- `Done`: good predicate alternative to `not IsActive`, but negative condition can be less ergonomic in `while`.

Recommended LC2 naming: `Loop.Range`, `Loop.RangeStep`, `Loop.IsActive`, `Loop.Current`, `Loop.Advance`, `Loop.Stop`. Consider adding `Loop.Resume` only if the design intentionally wants to teach loop-state helpers as resumable flow transitions; otherwise prefer `Advance` now and reserve `Resume` for Octomata.

## Part 8: proposed future tests

Diagnostic tests:

- Unsupported `continue` bare statement emits the dedicated diagnostic and guard/helper hints.
- Unsupported `break` bare statement emits the dedicated diagnostic and stop-condition/helper hints.

Documentation/fixture examples:

- Simple guard-loop alternative to `continue`.
- `while` condition alternative to `break`.

Loop helper tests:

- `Loop.Range` basic while iteration sums expected values.
- `Loop.Stop` terminates early.
- `Loop.Advance` / possible `Loop.Resume` supports skip-body pattern.
- `Loop.RangeStep` rejects step zero with fallible error.
- Negative step rejected in M0, unless a later design explicitly supports decreasing ranges.
- Interpreted and compiled parity for all helper cases.
- Interaction with `return` from inside a helper-driven `while` body.
- Interaction with `?` propagation when constructing a fallible range state.

Octomata relationship tests, if later sugar is considered:

- Explicit board-index loop can suspend/resume by state transitions.
- Ordinary loop remains non-resumable across flow `suspend` unless state is explicitly stored.

## Part 9: exact recommended next Codex task prompt for LC2

```text
You are working in the Oct repository.

Task: LC2-LOOP-HELPERS-M0 — implement explicit first-party Loop range-state helpers.

Read docs/internal/loop_control_octomata_recon_lc1.md first. Do not implement continue, break, generators, iterator protocols, yield, named arguments, overloads, generics, or Octomata semantic changes.

Implement a small pure-Oct first-party `Loop` library for scalar increasing range loop state only:
- `record RangeState { Current: Int End: Int Step: Int Active: Bool }`
- `fn Range(Start: Int, End: Int) -> RangeState`
- `fn RangeStep(Start: Int, End: Int, Step: Int) ! Error -> RangeState`
- `fn IsActive(State: RangeState) -> Bool`
- `fn Current(State: RangeState) -> Int`
- `fn Advance(State: RangeState) -> RangeState`
- `fn Stop(State: RangeState) -> RangeState`

Prefer `Advance` over `Resume` in M0 unless the docs explicitly justify `Resume` relative to Octomata `resume`.

Add docs and Language fixtures for interpreted and compiled parity:
- basic range iteration;
- early stop;
- skip-body pattern using guards plus Advance;
- invalid zero step;
- negative step rejected unless explicitly designed otherwise;
- fallible construction with `?` or `match`.

Confirm no loop semantics changed and ordinary loops are not lowered to Octomata.
```

## Appendix: files inspected

Primary files inspected in this pass:

- `README.md`
- `AGENTS.md`
- `Language/reference/language/04-control-flow.md`
- `Language/reference/runtime/21-octomata.md`
- `Language/reference/language/11-records.md`
- `Language/reference/language/13-packages.md`
- `internal/lex/lex.go`
- `internal/parse/parse.go`
- `internal/ast/program.go`
- `internal/typecheck/typecheck.go`
- `internal/interpret/interpret.go`
- `internal/build/compiler.go`
- `Language/ControlFlow/Loops/...`
- `Language/ControlFlow/OctomataCoreA/...`
- `Language/ControlFlow/OctomataResumeM57/...`
- `Libraries/Octomata/...`
