# FLOW semantic parity M0

Outcome: **C — bounded parity improvement**.

FLOW's language contract is ordinary Oct computation plus explicit resumable
control and explicit persistent machine state. M0 removes several user-visible
gaps and retires the AST whitelist as a language definition. The compiled
backend still owns a parallel `MIRFlowExpr` lowering/emission path, so this
milestone does not claim architectural parity (A) or complete semantic parity
with localized duplication (B).

## Architecture audit

| Stage | Ordinary path | FLOW path | M0 finding |
| --- | --- | --- | --- |
| parse / AST | `internal/parse`, shared `ast.Expr` and `ast.Stmt` | shared AST plus `FlowDecl`, state/control nodes | ordinary syntax is not parsed as a second language |
| template elaboration | project elaboration before typecheck/build | same early specialization, including template flows | no runtime generics or FLOW template runtime |
| typecheck | `checker.checkExpr` / `checkBlock` | same functions with `functionContext.inFlow/inState`, board/state bindings, and control restrictions | already follows the preferred context model |
| interpreted expressions | `interpreter.evalExpr` | same evaluator | shared |
| interpreted statements | `executeStmt` | `executeFlowStmt` handles machine control and delegates ordinary statements | shared ordinary semantics |
| ordinary compiled expressions | `lowerCtx.lowerExpr` to ordinary MIR | not used by FLOW | broad current stable surface |
| compiled FLOW expressions | `lowerFlowExpr` + `inferFlowExprType` to `MIRFlowExpr` | separate path | principal remaining architectural debt |
| generated Go | ordinary MIR emitter | `emitGoFlow` / `emitGoFlowExpr` | same Go value/function representations, separate formatter cases |
| fallback | CLI test runner | compiled mode fails on unsupported lowering; auto reports fallback counts | no subexpression bridge was found |

The answer to the architectural question is: FLOW needs a distinct binding and
control context, not a separate expression language. The typechecker and
interpreter already implement that answer. The compiled backend does not yet:
its FLOW MIR duplicates literals, calls, indexing, records, branching, and
matching. M0 extends and documents that path without pretending it is unified.

## Measured parity snapshot

| Construct | Ordinary interpreted | Ordinary compiled | FLOW interpreted | FLOW compiled | Difference | Semantic? |
| --- | --- | --- | --- | --- | --- | --- |
| literals, identifiers, arithmetic, comparison, logic, parens | yes | yes | yes | yes | none observed | no |
| `let`, `var`, assignment, bounded `for`, `while` | yes | yes | yes | yes in the verified ordinary-computation shapes | compiled FLOW has separate statement MIR; nested machine transfer in loops remains debt | accidental implementation boundary |
| ordinary synchronous user call | yes | yes | yes | yes | old builtin-only documentation was stale | no |
| function-valued parameter / returned captured callable | yes | yes | yes | yes | same generated Go `fn(...)` seam | no |
| early-specialized generic `Algorithms.Filter` consumer | yes | yes | yes | yes | same specialized declaration | no |
| records, nested field access, record literal, `with` | yes | yes | yes | yes | newly verified in compiled FLOW | no |
| plain/payload enums and enum match expression | yes | yes | yes | yes | board persistence newly admitted | no |
| arrays / nested array indexing | yes | yes | yes | yes | separate lowering remains | no |
| vector / matrix literals and indexing | yes | yes | yes | yes | newly added to compiled FLOW lowering | no |
| `if`, `switch`, enum `match` expression | yes | yes | yes | yes for current ordinary compiled surface | separate FLOW MIR | no |
| fallible call handled by statement `match` | yes | yes | yes | yes for direct calls | newly added | no |
| fatal unwrap `!` | yes | yes | yes | yes for direct calls | newly added | no |
| propagation `?` | yes in fallible function | yes in fallible function | rejected in FLOW | rejected in FLOW | FLOW has no declared failure terminal contract | **yes** |
| anonymous function created inside a compiled state activation | yes | yes | yes | not yet | FLOW lowerer cannot register the ordinary anonymous-function MIR helper | accidental debt |
| batch and some runtime-heavy expressions | yes | backend-dependent | yes | not exhaustively ported | duplicate lowering | accidental debt unless a lifetime restriction is identified |

## FLOW-specific semantics

Only these operations specialize the state-machine model: board persistence,
active-state identity, `goto`, `suspend`, `yield`, `remember`, `resume`, guard
`when`, one typed turn input, completion, utility-controller memory, and future
explicit failure semantics. `when policy` / `when utility` keeps static candidate
identity; M0 does not add dynamic case lists. `remember` / `resume` remains one
slot: remember overwrites, successful resume clears.

One state activation runs ordinary local Oct computation. `let`, `var`, arrays,
records, and function values are activation-local. They do not persist across
state or turn boundaries. Persistent state is the board, active state,
remember/resume slot, history, and policy-controller memory. A turn still has
one typed input; multiple related facts belong in a record, and variant-shaped
events belong in an enum.

## Board contract

Legal board types are deterministic persistent values recursively composed from
Bool, String, Int, Float (including dimensions), arrays, vectors, matrices,
records, and enums. This is type legality, not a claim that large values are
cheap. Function-valued fields remain excluded in M0 because closure environment
identity, checkpoint serialization, and history/debug representation need a
deliberate contract.

The interpreter's `cloneValue` recursively detaches arrays, vectors, matrices,
records, and enum payloads. Generated Go records/enums copy by value, while
slices retain Go's backing-array behavior. The M0 fixture verifies replacement
of vector/matrix board values does not change a prior snapshot. Indexed mutation
after a compiled snapshot can still alias a slice backing array; logical
checkpoint export/restore also still serializes only scalar/array values. Those
are explicit next implementation blockers, not reasons to make aggregate types
illegal.

## Fallibility

Fatal unwrap `!` and local statement `match` use ordinary Oct behavior inside a
state. Unhandled fallible expressions remain rejected. `?` is not implemented:
FLOW declarations cannot yet name an error type or a `Failed(Error)` terminal
state. The diagnostic is:

```text
error propagation with '?' requires a fallible FLOW result contract; handle the error with match or use '!'
```

M0 deliberately does not bolt propagation onto an infallible machine.

## Effects and resources

| Operation category | Ordinary Oct | FLOW M0 | Reason |
| --- | --- | --- | --- |
| pure library call | allowed | allowed | ordinary synchronous computation |
| synchronous user function | allowed | allowed | duration is performance, not type legality |
| fallible locally handled call | allowed | allowed | ordinary value/control semantics |
| synchronous IO / artifact operation | backend/capability dependent | no blanket FLOW ban; compiled availability is still operation-specific | M0 found no replay mechanism that would justify purity by name |
| OctGo | compiled-only | not widened by M0 | host-call support and lifetime must use the ordinary compiled contract |
| Octxiliary wrapper | manifest/sidecar dependent | not widened by M0 | native process and transport contract remains authoritative |
| Prometheus/native authority | explicit authority | unchanged | authority/lifetime semantics, not cheapness |
| Make authority | explicit Make context | unchanged | FLOW does not acquire Make authority implicitly |

No “small”, “cheap”, “scientific”, or builtin-name policy defines semantics.
Operations with scoped native resources may require future explicit lifetime
restrictions; M0 does not invent an effect system.

## Fallback policy and evidence

`--execution compiled` never falls back: an unsupported stable construct is a
compile failure naming the lowering gap. `--execution auto` retains visible,
counted fallback reporting. There is no generated-Go path that enters the
interpreter for one unsupported FLOW subexpression.

Focused August 30 evidence:

- board aggregate fixture: interpreted 1/1, compiled 1/1, auto 1/1; compiled
  and auto fallback count 0;
- fallibility fixture: interpreted 3/3 and compiled 3/3; fallback count 0;
- Algorithms package including FLOW escaped/captured callable dogfood:
  interpreted 19/19, compiled 19/19 (13 executable positive cases plus negative
  contracts), auto 19/19; fallback count 0;
- the prior `ordinary_oct_inside_state.octest` motivating case now compiles 1/1
  with fallback count 0.

The aggregate fixture is a bounded correctness probe, not a performance claim.
CLI timings include generated-Go compilation and are therefore not valid Step
microbenchmarks. A dedicated snapshot benchmark is deferred until compiled
deep-clone semantics are complete; benchmarking a known aliasing implementation
would reward the missing work.

## Remaining exclusions and debt

- compiled FLOW still has parallel expression/type inference and FLOW-only MIR;
- anonymous functions created directly inside a compiled activation are not yet
  registered through ordinary anonymous-function lowering;
- compiled snapshot deep cloning for slice-backed aggregates is incomplete;
- logical checkpoint serialization does not yet encode record/enum/vector/matrix;
- control transfer nested in compiled loops needs an outer state-machine label
  or a precise compile diagnostic before it can be claimed generally;
- ordinary compiled surfaces not represented in `MIRFlowExpr` can still require
  a second implementation change.

The next milestone should be **FLOW shared compiled-expression context M1**:
make ordinary `lowerCtx.lowerExpr` callable from a FLOW expression context and
delete duplicated value-expression MIR cases, while preserving FLOW control
terminators. Do not begin fallible FLOW design until that seam is shared.
