# Chapter 4 — Constant Folding and Constant Propagation

Start with the smallest possible optimization:

```oct
fn Example() -> Int {
    return 2 + 3
}
```

Why should the generated program add 2 and 3 at runtime when the compiler
already knows the answer?

```text
2 + 3
→ 5
```

This is **constant folding**: evaluate an operation during compilation when all
of its required operands are already known constants.

Now change the program slightly:

```oct
fn Example() -> Int {
    var x = 5
    return x + 1
}
```

Merely looking at the tree for `x + 1` is not enough. The compiler must first
prove:

```text
x == 5
```

Only then can it perform two steps:

```text
x + 1
→ 5 + 1       constant propagation
→ 6           constant folding
```

This chapter implements both transformations against Oct's production MIR. It
then composes them with constant-branch simplification:

```text
literal expression
→ local fold
→ propagate a known assignment
→ fold the newly exposed expression
→ simplify a constant branch
→ expose unreachable code for Chapter 5
```

The theme is more important than any individual rewrite:

> Analysis tells us what is true. Transformation uses those facts to change
> the program.

## 1. Constant folding is local

Oct's ordinary MIR represents expressions as structured `MIRValue` nodes, not
as snippets of Go or WebAssembly text. The variants include literals, locals,
unary and binary operators, conversions, indexing, field access, aggregate
helpers, enums, intrinsics, and a deliberately isolated backend compatibility
value.

That structure makes folding mechanical. The optimizer recursively folds the
children, then asks whether the parent operation is now computable. Thus:

```text
(2 + 3) * 4
→ 5 * 4
→ 20
```

The implementation is [`internal/build/optimize.go`](../../internal/build/optimize.go):

```go
func FoldValue(value MIRValue) (MIRValue, bool) {
    return foldValueWithConstants(value, nil)
}

func foldValueWithConstants(value MIRValue, constants LocalConstantMap) (MIRValue, bool) {
    // Recurse into structured children first.
    // Then call foldUnary, foldBinary, foldConversion, or foldIntrinsic.
}
```

The returned boolean says whether anything changed. A parent can therefore
fold immediately after a child does; no reparsing and no backend knowledge are
needed.

### The exact M0 folding surface

The implementation does not infer support from an idealized Oct. It handles
the scalar forms represented by current MIR:

| MIR form | Folded in Chapter 4 |
| --- | --- |
| `MIRLiteral` | finite `Bool`, signed 64-bit `Int`, and finite `Float` |
| `MIRUnary` | boolean `!`/`not`; integer and float negation |
| `MIRBinary` | integer and finite-float `+`, `-`, `*`; scalar comparisons; fully constant boolean `and`/`or` |
| `MIRConvert` | identity, `Int → Float`, and in-range finite `Float → Int` |
| `MIRIntrinsicValue` | nonzero constant safe division and Euclidean modulo, subject to the boundaries below |

Folding also walks scalar children inside other structured values. It does not
fold arrays, records, strings, ranges, memory accesses, payload enums, or
backend-shaped expressions into new constants. Payload-free enums already have
a scalar WASM representation, but the Chapter 4 analysis deliberately does not
add an enum-identity constant domain. Constant `match` simplification is left
for a later extension.

## 2. Constants are values, not strings

`MIRLiteral` retains source-shaped text because emitters need it. Analysis needs
typed equality and arithmetic. A small internal value in
[`internal/build/constants.go`](../../internal/build/constants.go) bridges the
two without becoming a parallel type system:

```go
type Constant struct {
    Kind  string
    Int   int64
    Float float64
    Bool  bool
}
```

Only the three existing scalar literal kinds enter this model. Float equality
inside the analysis compares IEEE-754 bit patterns, so `0.0` and `-0.0` are
not silently merged as identical compiler facts. Constants convert back to
ordinary `MIRLiteral` values before rewriting MIR.

## 3. Correctness comes before opportunity

An optimization is not correct because its output looks simpler. It is correct
because it preserves observable program semantics for every program in its
supported domain.

The interpreter stores Oct `Int` as signed 64-bit values, and direct WASM maps
it to `i64`. The optimizer uses that same domain. Addition, subtraction, and
multiplication retain their wraparound bit behavior, and comparisons remain
signed. Generated Go currently spells `Int` as platform-width `int`; the
amd64 proof matrix below agrees, while the 32-bit native ABI question is
recorded in `FEEDBACK.md` rather than hidden by a broader claim. The optimizer
does not replace a failing operation with an optimizer failure:

- division and modulo by zero remain unchanged;
- signed `MinInt / -1` remains unchanged because the direct WASM instruction
  traps at that edge;
- non-finite float inputs or results remain unchanged;
- float-to-int conversion folds only for finite values inside the signed
  64-bit range;
- float formatting retains negative zero;
- unsupported operators remain unchanged.

This is the fail-closed rule. If the optimizer cannot prove a rewrite safe, it
does nothing. A missed optimization can make a program slower. A wrong
optimization makes it wrong.

The same principle rules out tempting algebraic identities. Replacing
`f() * 0` with `0` could remove a call that performs I/O, mutates state, or
fails. Current ordinary MIR makes calls explicit statements, and Chapter 4
does not remove or evaluate them. It folds fully constant boolean operands but
does not use short-circuit identities to discard an unknown operand. No effect
analysis or interprocedural evaluation is smuggled into this milestone.

## 4. Why folding is not propagation

The expression `2 + 3` contains all the evidence needed to fold it. The
expression `x + 3` does not. To substitute `x`, the compiler must ask a
control-flow question.

Chapter 3 asked:

```text
Which definitions can reach this point?
```

Chapter 4 asks the next question:

```text
Do those definitions tell us the value itself?
```

For mutable-local MIR, the safe rule is:

> Replace a use of a local with a constant when every executable definition
> reaching that use gives the local the same constant value.

Notice that the rule does not say “there is one reaching definition.” Several
definitions can agree.

## 5. Three facts a compiler can know

For each tracked local, the analysis records one of three states:

```text
Unknown          the fixed-point solver has not learned a fact yet
Known(c)         every path examined so far gives the same constant c
Overdefined      no single constant describes the value
```

The corresponding implementation is:

```go
const (
    ConstantUnknown ConstantStateKind = iota
    ConstantKnown
    ConstantOverdefined
)

type ConstantState struct {
    Kind     ConstantStateKind
    Constant Constant
}
```

In lattice notation these are often written `⊥`, `Const(c)`, and `⊤`. The
plain-English meanings matter more than the symbols.

Merge is deterministic:

```text
Unknown  + Const(5) → Const(5)
Const(5) + Const(5) → Const(5)
Const(5) + Const(7) → Overdefined
Overdefined + anything known → Overdefined
```

Here `Unknown` is solver initialization, not permission to assume an
uninitialized runtime value. Function parameters and scalar captures enter as
`Overdefined`: they are real runtime values, simply not one compile-time
constant. Oct's frontend remains responsible for definite assignment.

This small domain is an example of **abstract interpretation**: execute a
program over facts such as “constant 5” instead of over every possible runtime
value.

## 6. A forward fixed-point analysis

There are two natural ways to build propagation on Chapter 3:

1. Query reaching definitions at each use, evaluate every defining value, and
   substitute only when every result is the same constant.
2. Run a direct forward dataflow analysis that carries one constant state per
   local through every block.

Chapter 4 chooses the second form because definitions such as `y = x + 1` can
be evaluated naturally from the state immediately before the statement. It is
still the value-level counterpart of reaching definitions: predecessor facts
are merged at the same CFG joins, assignments kill the old local fact, and the
worklist repeats until loop facts stop changing.

The production result is intentionally inspectable:

```go
type ConstantAnalysis struct {
    CFG             CFG
    In              map[string]LocalConstantMap
    Out             map[string]LocalConstantMap
    BeforeStatement map[string][]LocalConstantMap
    Unreachable     []string
    BlocksProcessed int
}
```

`AnalyzeConstants` never mutates MIR. It processes reachable blocks in stable
CFG order, keeps predecessor order stable, and records a fact map before every
statement plus one before the terminator. Only whole scalar `Bool`, `Int`, and
`Float` locals are tracked.

An `MIRAssign` evaluates its right-hand side in the incoming environment. A
literal or a supported expression of known locals produces `Known(c)`. Calls,
destructuring calls, wrapper calls, array construction, record construction,
and batch operations make scalar result targets `Overdefined`. Row and index
writes do not pretend to redefine a whole scalar. Aliasing, heap state,
compound writes, captures of mutable state, FLOW, and `MIRBackendValue` are
outside this analysis.

## 7. Joins can preserve a constant

Consider valid Oct shaped like this:

```oct
fn Same(flag: Bool) -> Int {
    var x = 0
    if flag {
        x = 5
    } else {
        x = 5
    }
    return x + 1
}
```

At the join there are two reaching assignments:

```text
left:  x = 5
right: x = 5
```

Their facts merge to `Known(5)`, even though their definition identities are
different. Propagation and folding can produce `return 6`.

Now change one assignment:

```oct
fn Different(flag: Bool) -> Int {
    var x = 0
    if flag {
        x = 5
    } else {
        x = 7
    }
    return x + 1
}
```

The merge is now:

```text
Const(5) + Const(7) → Overdefined
```

At runtime `x ∈ {5, 7}`. Neither value is a fact about every execution, so the
optimizer leaves `x + 1` alone.

## 8. Loops need fixed points too

Suppose a loop initializes and then mutates a counter:

```text
entry:  i = 0
loop:   i = i + 1
        branch again ? loop : done
```

The first trip suggests `Const(0)` at the header. The back edge contributes a
different value. Reprocessing the header merges those paths and makes `i`
overdefined. An analysis that looked only at entry facts would incorrectly
replace every loop use with zero.

The existing CFG worklist provides the right model. Analysis fixed points mean
“facts stop changing.” They are different from transformation fixed points,
where rewritten MIR stops changing.

## 9. Analysis and rewrite stay separate

The propagation rewrite consumes `BeforeStatement`; it does not discover facts
while editing instructions. At an eligible `MIRLocal`, it substitutes an
ordinary literal:

```text
y = x + 1       fact: x = Const(5)
→ y = 5 + 1
```

It does not delete the assignment that originally defined `x`. It recursively
rewrites value operands in assignments, calls, aggregate operations, returns,
failures, and branches while leaving statement targets alone.

Separating the phases provides two independently testable questions:

```text
AnalyzeConstants:       what is constant here?
rewriteFunctionConstants: use those facts to replace reads
```

The next folding pass then turns `5 + 1` into `6`.

## 10. Pass ordering and transformation fixed points

One pass enables another:

```text
x = 2
y = x + 3
if y == 5 ...

propagate:       y = 2 + 3
fold:            y = 5
propagate:       if 5 == 5
fold:            if true
branch fold:     jump known_target
```

The small driver in `OptimizeFunction` uses this explicit sequence:

```text
fold
analyze constants
propagate
fold again
fold constant branches
repeat until no phase changes MIR
```

The loop has a defensive limit of 32 iterations and reports an internal error
if it does not converge. These rewrites only replace local reads with literals,
replace constant operations with literals, and replace branches with jumps, so
they do not reverse one another. Real specimens converge in two iterations.

Running the optimizer again is idempotent:

```text
Optimize(Optimize(MIR)) == Optimize(MIR)
```

Tests check structural equality and byte-identical deterministic MIR/WASM
output. This is not a general pass manager and it introduces no SSA names or
phi nodes.

## 11. Constant branches change the CFG

Branch folding is the chapter's one CFG-changing rewrite:

```go
if literal.Value == "true" {
    block.Terminator = MIRJump{Target: branch.TrueTarget}
}
if literal.Value == "false" {
    block.Terminator = MIRJump{Target: branch.FalseTarget}
}
```

The transformation is deliberately small. Rebuilding the CFG makes the
untaken successor unreachable, and the next constant-analysis iteration ignores
that block. Chapter 4 does **not** delete it. Branch simplification establishes
the fact; unreachable-code elimination belongs to Chapter 5.

## 12. The canonical specimen

[`Examples/WasmCompute/WasmCompute.oct`](../../Examples/WasmCompute/WasmCompute.oct)
contains the real example:

```oct
fn ConstantDemo(flag: Bool) -> Int {
    var x = 2
    var y = x + 3

    if y == 5 {
        y = y + 2
    } else {
        y = 99
    }

    return y
}
```

The unused runtime parameter makes it easy to execute representative inputs
without affecting the constant proof. The checked-in snapshots are generated
from the production frontend and optimizer:

- [`constant-demo.before.mir`](snapshots/constant-demo.before.mir)
- [`constant-demo.after.mir`](snapshots/constant-demo.after.mir)

The focused MIR difference is:

```text
before                                      after
__oct_user_1 = 2                            __oct_user_1 = 2
tmp0 = (__oct_user_1 + 3)                   tmp0 = 5
__oct_user_2 = tmp0                          __oct_user_2 = 5
tmp1 = (__oct_user_2 == 5)                  tmp1 = true
branch tmp1 ? b1 : b2                       jump b1
b1: tmp2 = (__oct_user_2 + 2)               b1: tmp2 = 7
b2: __oct_user_2 = 99                       b2: __oct_user_2 = 99
b3: return __oct_user_2                     b3: return 7
```

The generated names are less important than the evidence. The optimizer
propagates, folds, simplifies the branch, rebuilds reachability, and then proves
the return. The false block and assignments to `x`, `y`, and temporaries remain
visible. We have not implemented dead-code elimination.

## 13. Backend-neutral integration and opt-in behavior

Optimization sits in the shared pipeline:

```text
source
→ parser and type checker
→ backend-neutral MIR
→ AnalyzeConstants / OptimizeMIR
→ Go or WebAssembly backend
```

It does not live secretly inside the WASM encoder. `OptimizeMIR` accepts and
returns `MIRModule`, and both production backends have an explicit optimized
entry point. The CLI exposes one conservative switch:

```text
go run ./cmd/oct build Examples/WasmCompute --target wasm --opt
go run ./cmd/oct build Examples/WasmCompute --target native --opt
```

There is no invented `-O0/-O1/-O2/-O3` hierarchy. Without `--opt`, compilation
keeps the previous behavior and byte output. That makes the book's before/after
experiment reproducible and avoids silently changing every existing artifact.

## 14. Real output measurements

The synchronized metrics snapshot is
[`constant-demo.metrics`](snapshots/constant-demo.metrics). It counts foldable
expression operations plus conditional branches in `ConstantDemo`, while WASM
sizes cover the containing `WasmCompute` module:

| Program | fold/branch MIR ops before | fold/branch MIR ops after | WASM bytes before | WASM bytes after |
| --- | ---: | ---: | ---: | ---: |
| `ConstantDemo` in `WasmCompute` | 4 | 0 | 1179 | 1165 |

The WASM code section shrinks from 960 to 946 bytes. The 14-byte improvement is
real but modest. The direct backend still emits a dispatch-loop case for every
retained block, including the newly unreachable one, and retained dead stores
still become instructions. MIR scalar optimization and backend control-flow
structuring are separate dimensions; a small result is more useful than a fake
marketing win.

## 15. Semantic proof, not visual confidence

`TestCurrentMIRProducesDeterministicExecutableModule` executes the canonical
module through:

```text
Oct interpreter
generated Go, unoptimized
generated Go, optimized
direct WASM, unoptimized
direct WASM, optimized
```

Both WASM modules execute `ConstantDemo(false)` and `ConstantDemo(true)` and
return `7`; all paths return the same `Main` result, `114`. The test also
encodes each optimized module twice and requires byte identity. Snapshot tests
regenerate the before/after MIR through production lowering and fail on drift.

Focused optimizer tests cover:

- integer, float, boolean, comparison, nested, conversion, division, and modulo
  folding;
- straight-line propagation;
- agreeing and disagreeing joins;
- loop-carried mutation becoming overdefined;
- propagation followed by folding;
- true and false branch folding;
- unsupported and effect-sensitive values remaining unchanged;
- division/modulo traps and the signed division edge remaining unchanged;
- optimizer idempotence and deterministic output.

## 16. What Chapter 4 intentionally does not do

The optimizer does not rename locals, create phi nodes, or convert MIR to SSA.
Mutable-local propagation is the lesson.

It also does not remove:

```text
x = 5
y = 6
return y
```

Even if propagation makes `x` unread, Chapter 4 has only proved a value fact.
It has not asked whether removing a definition is safe. Similarly, branch
folding can make a block unreachable without deleting it.

The following remain explicit later work:

```text
alias and heap analysis
record-field and array-element propagation
payload enums and constant match selection
interprocedural constant propagation
arbitrary compile-time function evaluation
effect analysis
FLOW optimization
backend-specific MIRBackendValue optimization
global value numbering and common-subexpression elimination
liveness and dead-code elimination
```

## 17. Exercises

1. Fold `(2 + 3) * 4` by showing each child-before-parent step.
2. For `x = 5; y = x + 1`, state the analysis fact, propagation rewrite, and
   folding rewrite separately.
3. At a join with `x = 5` on the left and `x = 5` on the right, what is `x`?
4. At a join with `x = 5` on the left and `x = 7` on the right, why must the
   compiler not substitute either value?
5. Why does `x = 0; loop: x = x + 1` stop being one constant at the loop
   header?
6. Why is `f() * 0 → 0` potentially unsafe?
7. Why does an optimized MIR snapshot still contain assignments whose values
   have already been propagated?
8. What additional proof would be required to fold a payload-free enum match?

## 18. Terminology cheat sheet

**Constant folding** evaluates a fully known operation at compile time.

**Constant propagation** replaces a local read with a proved constant.

**Constant state** is `Unknown`, `Known(c)`, or `Overdefined` for one local at
one program point.

**Merge** conservatively combines facts from predecessor paths.

**Rewrite / transformation** changes MIR while preserving semantics.

**Branch folding** replaces a branch with a jump when its condition is known.

**Optimization pipeline** is the ordered composition of analyses and rewrites.

**Idempotence** means optimizing an already optimized program changes nothing.

**Conservative optimization** declines a rewrite when proof is insufficient.

**Abstract interpretation** is the broader technique of executing over facts
such as constants rather than concrete runtime states.

## Representation as answered questions

Chapter 3 answered:

```text
Which definitions can reach this point?
```

Chapter 4 answered:

```text
Do those definitions tell us the value itself?
```

Still unanswered:

```text
Is this assignment ever used?
Is this block now unreachable?
Is this local live?
Can this expression be reused?
Can this loop computation move?
```

# Next: Dead Code and Dead Definitions

The optimized example can contain:

```text
x = 5
y = 6
return y
```

If `x` is never read anymore, why are we still computing it? And if branch
folding removed the last edge into a block, why does the block remain in the
module?

Chapter 5 will answer those questions with unreachable-block elimination and
dead-definition elimination, introducing liveness only where the proof needs
it. Chapter 4 stops here: it exposes dead code but does not remove it.
