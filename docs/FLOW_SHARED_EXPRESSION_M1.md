# FLOW shared compiled-expression context M1

Outcome: **A — shared compiled expression architecture**.

Compiled FLOW state value computation enters `lowerCtx.lowerExpr`, produces
ordinary `MIRBlock` and `MIRStmt` nodes, and uses the ordinary Go statement
emitter. FLOW supplies typed binding expressions and retains state-machine
statements. The former literal/operator/call/index/record/match FLOW expression
variants, inference switch, resolver helpers, and Go emission cases have been
deleted.

## Architecture audit

| Expression feature | Pre-M1 ordinary implementation | Pre-M1 FLOW implementation | M1 authority | FLOW context / exception |
| --- | --- | --- | --- | --- |
| literals, operators, coercion | `lowerCtx.lowerExpr` to ordinary MIR | `lowerFlowExpr` tree and `emitGoFlowExpr` | ordinary MIR | typed parameter/input/board/local binding names |
| calls and function values | ordinary resolver plus `MIRCall` | `resolveFlowCall` plus `MIRFlowCallExpr` | ordinary resolver and `MIRCall` | ordinary calls allowed; actual backend capabilities still apply |
| anonymous function and capture | `lowerFunctionExpr`, typed helper and environment | unsupported | ordinary helper and environment | capture arguments map to current activation values |
| arrays, vectors, matrices, indexing | ordinary construction/call/assign MIR | FLOW-only literal/index nodes | ordinary MIR | board writes remain FLOW storage statements |
| records, enums, `with`, match/switch/if | ordinary blocks and nominal Go values | FLOW-only expression variants | ordinary MIR blocks | none for ordinary value semantics |
| fatal unwrap and handled fallibility | ordinary result MIR | FLOW-only unwrap/direct-call cases | ordinary MIR for expressions | statement `match` retains FLOW statement shell |
| `?` | ordinary fallible-function terminator | explicit rejection | contextual rejection | no fallible FLOW terminal contract |
| `when policy` | not an ordinary stateful expression | FLOW utility-site MIR | FLOW-specific | persistent controller commitment state |

`compiledExpressionContext` carries the program, package, flow identity,
anonymous helper identity, and emitted helper functions. Per-expression ordinary
contexts carry typed locals and exact Go binding expressions: activation locals
stay local, while construction parameters and turn input resolve through `f`,
and the synthetic board binding resolves through `f.board`. The context is
typed; there is no dynamic environment map in generated Go.

`MIRFlowSharedExpr` is the bridge container. It contains ordinary MIR blocks and
locals, not a second expression AST. Every ordinary state expression lowers to
this representation. `MIRFlowUtilityWhenExpr` is the only FLOW-specific
value-shaped variant because controller commitment is persistent machine state.
Its hysteresis, commitment, candidates, guards, scores, and fallback are all
`MIRFlowSharedExpr` computations. Fallible statement-match subjects also use
fallible ordinary MIR; they no longer retain a legacy call node.

## Callable and capture result

An anonymous captured predicate can be constructed directly inside a state and
passed to the early-specialized `Algorithms.Filter`. Capture expressions are
evaluated once by ordinary lowering, cloned using ordinary value semantics, and
fed to the ordinary typed helper. A board-derived local is therefore captured
by value; neither the machine nor board storage is captured by reference. Named,
parameter, returned/escaped, and activation-created callables share Go `func`
types. The Algorithms corpus reports 19 passed, 0 failed in compiled execution,
13 compiled positive cases, and zero fallback.

## Persistence

`cloneCompiledValueExpr` and the interpreter's `cloneValue` are the recursive
authorities for persistent snapshot values. Compiled `BoardSnapshot` applies the
clone only at snapshot construction; scalar fields copy directly. The aggregate
contract proves that a later indexed array mutation does not alter an earlier
snapshot. Capture construction uses the same ordinary clone boundary but does
not clone once per callback element.

Logical checkpoint schema version 3 recursively encodes scalars, dimensioned
scalars, arrays, nested arrays, vectors, matrices with shape, records with
ordered fields, and plain/payload enums. Restore validates nominal type, field
order, enum variant/payload shape, dimensions, and matrix shape. The Make
checkpoint representation embeds the deterministic JSON form of each typed
checkpoint value and retains version-2 scalar parsing only for diagnostics;
version validation prevents resumption under the new schema.

The aggregate roundtrip fixture runs to suspension, exports, restores, inspects
a nested record, record array, payload enum, vector, matrix, and nested array,
then continues to the expected result. Generated host checkpoints remain typed,
deterministic JSON; representative sizes are 286 bytes for a plain enum machine,
505 bytes for the durable controller, and 321 bytes for the generator specimen.

## Nested machine control

Generated state activation now has an explicit `__octFlowMachine` loop label.
`goto` and `resume` from nested ordinary loops/branches continue that labeled
machine loop instead of the nearest Go loop. Inline `if`, `for`, `while`, local
assignment, board assignment, and `remember` no longer advance the top-level
instruction accidentally. Nested goto, suspend, and remember+goto pass in
interpreted, compiled, and auto modes with zero fallback; code after terminal
transfer does not execute.

## Evidence and limits

- Structural MIR test: all ordinary expressions in the paired record/array/
  capture/call/index FLOW specimen are `MIRFlowSharedExpr` containing ordinary
  MIR; its capture helper uses `MIRCapture`.
- Generated Go inspection: the activation-created predicate is a typed
  `func(int) bool`; no dynamic environment or interpreter bridge is emitted.
- Algorithms: 19/19 compiled, 13 compiled positive cases, zero fallback.
- Entire `Language/ControlFlow` corpus: 82/82 interpreted, 82/82 compiled,
  82/82 auto; zero failures and zero reported fallback.
- Aggregate snapshot: interpreted 1/1, compiled 1/1, auto 1/1, zero fallback.
- Nested transfer: interpreted 1/1, compiled 1/1, auto 1/1, zero fallback.
- Host microbenchmarks on the recorded Ryzen 7 7700X: Step+Yield 12.93-13.31
  ns/op, 0 allocations; checkpoint 856.2-878.0 ns/op, 992 B and 3 allocations;
  restore 1.474-1.515 us/op, 392 B and 3 allocations. These measure the existing
  durable scalar/record host specimen, not every aggregate size class.

Deliberate exclusions are unchanged: `?` propagation, callable board fields,
stack semantics, async/await, dynamic `when` candidates, and template redesign.
No compiled interpreter bridge was introduced. The semicolon policy remains
unchanged: semicolons are optional separators and never required.

The truthful post-M1 description is: **ordinary Oct value computation through
shared compiled MIR, plus explicit FLOW machine control and persistence**.

The expression-architecture milestone is closed. Fallible FLOW propagation
remains a separate semantic milestone and is not started here.
