# Chapter 2 — Basic Blocks and Control-Flow Graphs

Start with a question:

> How many possible execution paths does this function have?

```oct
fn Max(a: Int, b: Int) -> Int {
    if a > b {
        return a
    }

    return b
}
```

There are two source-level outcomes: return `a` when the comparison is true, or return `b` when it is false. By the time the current Oct compiler has produced MIR, it no longer needs to think about an `if` as source syntax. This is the real dump:

```text
fn WasmCompute.Max(__oct_user_0:Int, __oct_user_1:Int) -> Int
  entry:
    __oct_internal_tmp_0 = (__oct_user_0 > __oct_user_1)
    branch __oct_internal_tmp_0 ? b1 : b2
  b1:
    return __oct_user_0
  b2:
    jump b3
  b3:
    return __oct_user_1
```

The compiler now has:

```text
blocks
+
edges between blocks
```

That graph is the **control-flow graph**, usually abbreviated **CFG**. This chapter learns to read that graph directly. We will not optimize it, turn it into SSA, or calculate value-flow facts yet.

## 1. What is a basic block?

> A basic block is a sequence of executable operations entered from the top and executed straight through until one control-flow decision at the end.

Execution does not jump into the middle of a block. Once execution enters at the top, ordinary operations run in order. Execution does not leave early: the final **terminator** decides what happens next.

The relevant production definitions are in [`internal/build/mir.go`](../../internal/build/mir.go):

```go
type MIRFunction struct {
    Package string
    Name    string
    Params  []MIRField
    Return  string
    Locals  []MIRField
    Blocks  []MIRBlock
    // Other semantic fields omitted here.
}

type MIRBlock struct {
    Label      string
    Statements []MIRStmt
    Terminator MIRTerminator
}
```

`Blocks` is the function's ordered collection of executable regions. A block's `Label` gives control-flow edges a target. `Statements` holds work such as assignments and calls. `Terminator` describes the one transfer at the end.

The Go interface can technically contain `nil`, and the current Go and WASM emitters retain a defensive non-final fallthrough path for such input. Production lowering emits explicit terminators for the functions in this chapter. The CFG utility introduced below requires that completed MIR make the one-terminator invariant explicit rather than making later analysis guess.

## 2. Why terminators are separate

An assignment answers “what value changes?” A terminator answers a different question:

> Where can execution go after this block?

Oct's ordinary MIR currently has these terminators:

```go
type MIRReturn struct{ Value MIRValue }
type MIRJump struct{ Target string }
type MIRBranch struct {
    Cond                    MIRValue
    TrueTarget, FalseTarget string
}
type MIRFail struct{ Value MIRValue }
```

Their graph meanings are immediate:

| Terminator | Normal successors | Meaning |
| --- | ---: | --- |
| `MIRReturn` | 0 | Return to the caller. |
| `MIRJump` | 1 | Continue at one named block. |
| `MIRBranch` | 1 or 2 distinct blocks | Choose a target from a Boolean condition. |
| `MIRFail` | 0 | Leave normal control by failure or trap. |

A branch normally names two different successors. If both arms name the same block, the CFG has one distinct edge to that block. Separating terminators makes CFG construction almost mechanical: inspect the final value and copy out its target labels.

## 3. Build the `Max` CFG

The opening dump is authoritative compiler output, synchronized with [`snapshots/max.mir`](snapshots/max.mir). Read it one block at a time:

| Block | Statements | Terminator | Successors |
| --- | --- | --- | --- |
| `entry` | compare `a > b` into `tmp_0` | branch on `tmp_0` | `b1`, `b2` |
| `b1` | none | return `a` | none |
| `b2` | none | jump | `b3` |
| `b3` | none | return `b` | none |

The graph is:

```text
                     true
              +--------------> b1: return a
              |
entry: a > b ?
              |
              +--------------> b2
                     false       |
                                 v
                            b3: return b
```

There really is an intermediate `b2`; the graph does not simplify it away. This chapter describes current MIR rather than presenting a tidier imaginary representation.

## 4. Successors and predecessors

A block's **successor** is a block that may execute next. A block's **predecessor** is a block that may transfer control into it. The same edge has both descriptions: for `entry → b1`, `b1` is a successor of `entry`, and `entry` is a predecessor of `b1`.

For `Max`:

```text
Pred(entry) = {}
Pred(b1)    = {entry}
Pred(b2)    = {entry}
Pred(b3)    = {b2}
```

The successor sets come directly from terminators. Predecessors are the edges read backward. Many later analyses need both directions, so the compiler now derives both once and keeps their ordering deterministic. We are not doing those analyses yet.

## 5. Entry and exit

The **entry block** is where execution begins. For current `MIRFunction` values it is the first block; both examples label it `entry`. It has no normal predecessor inside its own function. A call arrives from outside the function, not over an intraprocedural CFG edge.

An **exit** is a block whose terminator has no normal successor. `b1` and `b3` are both exits from `Max` because both return. Oct does not force MIR into a synthetic single-exit shape. A function can have several return blocks. A block ending in `MIRFail` is also an exit from normal execution, though it exits by failure or trap rather than by returning a value.

## 6. Reachability

A block is **reachable** if some path from the entry leads to it. Every block in the real `Max` and `SumTo` dumps is reachable.

For contrast, consider this deliberately synthetic MIR graph:

```text
entry:
    return 0

orphan:
    fail "cannot arrive here"
```

There is no edge from `entry` to `orphan`, so `orphan` is unreachable. This is not a claim that Oct source currently lowers to that exact MIR; it is a hand-built graph used by the CFG unit test to isolate reachability behavior.

Unreachable blocks matter because a compiler must not assume that every block participates in a real execution. Later they can inform dead-code removal, diagnostics, and the validity of analysis assumptions. Chapter 2 only identifies them; it does not remove them.

## 7. Meet a loop: `SumTo`

The current example source is:

```oct
fn SumTo(n: Int) -> Int {
    var total = 0
    var i = 0

    while i <= n {
        total = total + i
        i = i + 1
    }

    return total
}
```

Here is its complete production dump, synchronized with [`snapshots/sum-to.mir`](snapshots/sum-to.mir):

```text
fn WasmCompute.SumTo(__oct_user_0:Int) -> Int
  entry:
    __oct_user_1 = 0
    __oct_user_2 = 0
    jump b1
  b1:
    __oct_internal_tmp_0 = (__oct_user_2 <= __oct_user_0)
    branch __oct_internal_tmp_0 ? b2 : b3
  b2:
    __oct_internal_tmp_1 = (__oct_user_1 + __oct_user_2)
    __oct_user_1 = __oct_internal_tmp_1
    __oct_internal_tmp_2 = (__oct_user_2 + 1)
    __oct_user_2 = __oct_internal_tmp_2
    jump b1
  b3:
    return __oct_user_1
```

We can give the blocks explanatory roles:

| Actual label | Explanatory role |
| --- | --- |
| `entry` | initialize `total` and `i` |
| `b1` | loop condition or header |
| `b2` | loop body |
| `b3` | function exit |

Those role names help humans; they are not fields stored on `MIRBlock`.

## 8. Draw the `SumTo` CFG

The real graph is:

```text
             entry: initialize
                    |
                    v
             b1: is i <= n? --------false--------> b3: return total
                    |
                   true
                    |
                    v
             b2: update total, i
                    |
                    +-----------------------------> b1
```

Its exact edges are:

```text
entry -> b1
b1    -> b2
b1    -> b3
b2    -> b1
```

The route `b1 → b2 → b1` is a cycle.

> A loop in source code becomes a cycle in the CFG.

Once that feels natural, a loop stops being a special source-language mystery. It is control revisiting graph nodes.

## 9. Back edges

Informally, a **back edge** is an edge that returns control to an earlier loop region or header and closes a cycle. In `SumTo`, the loop-closing edge is:

```text
b2 -> b1
```

“Back” does not mean “points to a lower-numbered block.” Labels and block order alone do not define loop structure. A later chapter can introduce the formal dominance-based definition. For now, `b2 → b1` is plainly the edge that sends another iteration to the condition.

## 10. Paths through the graph

A **control-flow path** is a sequence of blocks connected by edges. `Max` has short acyclic paths:

```text
entry -> b1
entry -> b2 -> b3
```

Loop paths can revisit blocks. Because `SumTo` initializes `i` to `0` and continues while `i <= n`, `SumTo(3)` executes the body for `i = 0, 1, 2, 3`:

```text
entry
-> b1 -> b2   // i = 0
-> b1 -> b2   // i = 1
-> b1 -> b2   // i = 2
-> b1 -> b2   // i = 3
-> b1 -> b3   // i = 4, condition is false
```

The static CFG contains four nodes. The dynamic trace can visit two of them many times. A graph describes possible transfers; an execution trace records the transfers chosen for one input.

## 11. Source structure is not CFG structure

Source code offers constructs such as:

```text
if
while
match
```

Ordinary MIR primarily presents control as:

```text
blocks
branches
jumps
returns
failures
```

An `if` commonly becomes a branch followed by returns or a join. A `while` becomes a branch plus a cycle. A `match` can lower into a decision structure with multiple tests and paths; the exact current shape depends on the matched form, so we will not pretend it is a single universal pattern.

Different source constructs can therefore produce similar graph shapes. The CFG is a more uniform representation of control flow than source syntax.

## 12. Joins

A **join** is a block with multiple predecessors: separate paths converge on one block. Consider this simplified, synthetic source-shaped example:

```oct
var x = 0

if condition {
    x = 1
}

return x
```

Its explanatory graph could be:

```text
              entry: x = 0; branch condition
                         /             \
                        v               |
                  then: x = 1           |
                         \             /
                          v           v
                         join: return x
```

Two paths enter `join`, so it has two predecessors. This diagram is explanatory pseudocode, not a claimed Oct MIR dump.

> We are going to leave the question “which value of `x` reaches this join?” for Chapter 3 and later SSA discussion.

## 13. Why joins are where compiler analysis gets interesting

At a join, facts from more than one incoming path meet. That creates questions such as:

```text
Is x definitely initialized?
Is x constant?
Which assignment to x can reach here?
Is this expression available on every path?
```

The CFG tells us which paths meet, but it does not answer those value questions by itself. That separation is useful: first establish where control can flow; then reason about what is true along those paths.

## 14. Construct a CFG from MIR

Before this chapter, backends independently interpreted MIR terminators, but ordinary MIR had no small reusable CFG analysis. [`internal/build/cfg.go`](../../internal/build/cfg.go) now provides one:

```go
type CFG struct {
    Entry        string
    BlockOrder   []string
    Successors   map[string][]string
    Predecessors map[string][]string
    Reachable    []string
}

func BuildCFG(fn MIRFunction) (CFG, error)
```

`BuildCFG` derives edges without mutating the function. `BlockOrder` follows MIR order. Successors preserve terminator order—true target before false target—and predecessor lists follow source-block order. `Reachable` uses deterministic breadth-first discovery order. The maps provide direct lookup; the ordered slices prevent diagnostics and future analysis code from depending on Go map iteration.

## 15. A real compiler utility, not book scaffolding

The utility lives beside MIR because its facts are backend-independent. It supports `MIRReturn`, `MIRJump`, `MIRBranch`, and `MIRFail`; rejects an empty function, empty or duplicate labels, missing terminators, unknown terminator types, and references to nonexistent labels; and does not optimize, introduce SSA, or depend on WebAssembly.

That narrow boundary is enough for later reachability, dominance, liveness, loop, and optimization work without prematurely implementing any of it. `DumpCFG` is a deterministic diagnostic formatter over the same analysis, not a second graph model.

## 16. Test CFG extraction

[`internal/build/cfg_test.go`](../../internal/build/cfg_test.go) loads `Examples/WasmCompute` through the production `LoadMIR` path. It asserts exact entry, block order, successors, predecessors, and reachable order for `Max` and `SumTo`.

The tests also isolate structural behavior with tiny hand-built MIR values: a disconnected block must remain absent from `Reachable`, and malformed graphs must fail with precise errors. These are implementation-boundary tests, not embedded Oct language specifications.

Run them with:

```text
go test ./internal/build -run TestBuildCFG -count=1
```

## 17. Deterministic graph dumps

The production formatter emits this for `Max`:

```text
function WasmCompute.Max entry=entry

block entry
  successors: b1, b2
  predecessors: -
  reachable: true

block b1
  successors: -
  predecessors: entry
  reachable: true

block b2
  successors: b3
  predecessors: entry
  reachable: true

block b3
  successors: -
  predecessors: b2
  reachable: true
```

The authoritative outputs are [`snapshots/max.cfg`](snapshots/max.cfg) and [`snapshots/sum-to.cfg`](snapshots/sum-to.cfg). `TestCompilerOptimizationBookCFGSnapshots` regenerates both from production MIR and fails if they become stale. ASCII drawings remain teaching aids; the MIR and CFG snapshots are executable evidence.

## 18. Arbitrary CFGs versus structured WebAssembly

Oct MIR can name explicit `jump` and `branch` targets, forming an arbitrary graph. Core WebAssembly does not expose the same free-standing label-and-`goto` machine. Its control is structured with nested constructs such as `block`, `loop`, and branches to enclosing labels.

That leaves the backend with a target-lowering question:

> How do I preserve this CFG using structured WASM control flow?

The challenge is not visible for a one-block `Add`, but `Max` already branches and `SumTo` already cycles.

## 19. The current dispatch-loop strategy

The production implementation is in [`internal/wasm/wasm.go`](../../internal/wasm/wasm.go). It assigns each MIR label its block index, creates a backend-owned `pc` local, initializes it to zero, and emits a structured outer block and inner loop:

```go
labels := map[string]int32{}
for i, block := range fn.Blocks {
    labels[block.Label] = int32(i)
}
body = append(body, 0x41)
body = append(body, s32(0)...)
body = append(body, 0x21)
body = append(body, u32(pc)...)
body = append(body, 0x02, 0x40, 0x03, 0x40) // block exit; loop dispatch
for i, block := range fn.Blocks {
    // Emit: if pc == i, execute this MIR block.
}
```

Conceptually:

```text
pc = entry block index

block exit
    loop dispatch
        if pc == 0: execute MIR block 0
        if pc == 1: execute MIR block 1
        ...
```

`emitTerminator` turns a jump target into a block index, stores it in `pc`, and branches back to the dispatch loop. A branch emits both candidate indices and the condition, uses WebAssembly `select`, stores the selected index, and redispatches. A return emits the result and WebAssembly `return`. `MIRFail` emits `unreachable`, which traps.

This preserves arbitrary current MIR CFGs. Jumps change dispatch state; branches choose it; returns leave; and loops naturally revisit block indices. For `SumTo`, block index 2 selects index 1 again, realizing `b2 → b1` inside structured WebAssembly.

## 20. What the dispatch loop costs

The strategy has clear advantages:

```text
simple
general
easy to validate
works for arbitrary CFGs
```

It also has plausible costs:

```text
an extra pc local
repeated dispatch tests and branches
less directly structured target code
potential performance and code-size overhead
```

Those are design tradeoffs, not benchmark results from this chapter. We will not optimize on intuition alone.

> Later chapters may teach us enough CFG structure to reconstruct cleaner WASM control flow.

## 21. Why CFG analysis belongs above the backend

Successors, predecessors, and reachability are properties of MIR, not of WebAssembly. The same graph can support:

```text
optimization
diagnostics
Go backend reasoning
WASM lowering
potential RTL legality and lowering
```

Putting `BuildCFG` in `internal/wasm` would make general compiler facts depend on one target. Keeping it near MIR lets every legitimate consumer share one definition.

## 22. Real Go implementation walkthrough

The implementation in [`internal/build/cfg.go`](../../internal/build/cfg.go) follows six small steps.

First, it walks blocks in MIR order, validates labels, initializes all map entries, and records `BlockOrder`. Second, it inspects each terminator:

```go
switch term := block.Terminator.(type) {
case MIRReturn, MIRFail:
    successors = []string{}
case MIRJump:
    successors = []string{term.Target}
case MIRBranch:
    successors = []string{term.TrueTarget}
    if term.FalseTarget != term.TrueTarget {
        successors = append(successors, term.FalseTarget)
    }
case nil:
    return CFG{}, fmt.Errorf("MIR block %s.%s:%s has no terminator", ...)
default:
    return CFG{}, fmt.Errorf("MIR block %s.%s:%s has unsupported terminator %T", ...)
}
```

Third, it checks every successor against the complete label set. This happens after collecting labels, so forward edges are valid. Fourth, it stores successors in true/false or jump order.

Fifth, predecessor construction simply inverts every edge:

```go
for _, label := range cfg.BlockOrder {
    for _, successor := range cfg.Successors[label] {
        cfg.Predecessors[successor] = append(cfg.Predecessors[successor], label)
    }
}
```

Finally, it traverses from `Entry` to calculate reachability. None of these steps changes MIR.

## 23. The reachability algorithm

Reachability uses a queue-based breadth-first traversal:

```go
seen := make(map[string]bool, len(successors))
worklist := []string{entry}
reachable := make([]string, 0, len(successors))
for len(worklist) > 0 {
    label := worklist[0]
    worklist = worklist[1:]
    if seen[label] {
        continue
    }
    seen[label] = true
    reachable = append(reachable, label)
    worklist = append(worklist, successors[label]...)
}
```

Start with the entry on a worklist. Repeatedly take a block. If it has already been seen, skip it; otherwise mark it reachable and add its successors. Cycles terminate because `seen` prevents `b1` and `b2` from being processed forever.

Depth-first search would identify the same reachable set. Breadth-first order is used here because the queue and ordered successor lists make the diagnostic result straightforward and stable.

## 24. Complexity, gently

CFG construction and reachability are both `O(V + E)`:

```text
V = number of blocks
E = number of control-flow edges
```

In ordinary language, each block and each edge is examined a constant number of times. Compiler CFGs are usually sparse: returns have zero outgoing edges, jumps one, and branches at most two distinct outgoing edges. Basic graph algorithms are therefore practical even for much larger functions than our examples.

## 25. Graph terminology cheat sheet

| Term | Meaning in this chapter |
| --- | --- |
| block | Straight-line operations ending in one control decision. |
| edge | A possible transfer from one block to another. |
| successor | A block that may execute next. |
| predecessor | A block that may transfer into this block. |
| entry | The first block executed; first in current `MIRFunction.Blocks`. |
| exit | A block with no normal successor, such as return or fail. |
| path | A sequence of blocks connected by edges. |
| reachable | Having some path from entry. |
| cycle | A path that can return to a block already on that path. |
| loop header | Explanatory name for the block that controls entry or repetition of a loop region; `b1` in `SumTo`. |
| back edge | Informally here, an edge returning to an earlier loop region/header and closing a cycle; formalized later with dominance. |
| join | A block with multiple predecessors. |

## 26. Exercises

### Exercise 1

Draw the CFG for:

```oct
fn Abs(x: Int) -> Int {
    if x < 0 {
        return -x
    }

    return x
}
```

Do not assume its block count. First obtain or imagine a MIR dump, then preserve every block shown.

### Exercise 2

Given this simplified MIR, list successor and predecessor sets:

```text
entry: branch cond ? yes : no
yes:   jump join
no:    jump join
join:  return value
```

Which block is the join?

### Exercise 3

List every edge in `SumTo`. Identify the cycle and the informal back edge.

### Exercise 4

Explain why a block ending in `return` has no normal successors even though execution continues in its caller.

### Exercise 5

Suppose `orphan` is present in `BlockOrder` but no reachable block targets it. Predict whether the worklist traversal visits it and what `DumpCFG` prints for its `reachable` field.

### Short solutions

1. The condition block has one edge to the negating return path and one toward the non-negating return path; preserve any explicit intermediate jump block in the MIR you use.
2. `Succ(entry)={yes,no}`, `Succ(yes)={join}`, `Succ(no)={join}`, `Succ(join)={}`; `Pred(entry)={}`, `Pred(yes)={entry}`, `Pred(no)={entry}`, `Pred(join)={yes,no}`. `join` is the join.
3. `entry→b1`, `b1→b2`, `b1→b3`, `b2→b1`; the cycle is `b1→b2→b1`, closed by informal back edge `b2→b1`.
4. Intraprocedural CFG edges remain inside the function. Return transfers to the caller, outside this CFG.
5. It is never added by a visited successor, so traversal does not visit it and the dump says `reachable: false`.

## 27. Representation as answered questions

Chapter 1 treated each compiler representation as a set of answered questions. At the CFG stage, the compiler has answered:

```text
what blocks exist
where each block can transfer control
which graph paths are possible
which blocks are reachable from entry
```

It has not yet answered:

```text
which definitions reach each use
which values are live
which expressions are constant
which code is dead
which loops are invariant
```

That boundary is deliberate. The graph gives later reasoning its roads; it does not yet tell us what facts travel on them.

## 28. Next: Uses, Definitions, and Dataflow

We can now look at the real MIR for `Max` and `SumTo` and identify blocks, edges, successors, predecessors, entries, exits, reachability, paths, cycles, the loop-closing edge, and joins. We also know why an arbitrary MIR graph does not directly match structured WebAssembly, and how Oct's current dispatch loop preserves that graph with a program-counter local.

The next question is:

> Now that we know where control can flow, what can we learn about the values flowing along those paths?

Chapter 3 will begin with uses, definitions, reaching facts, worklists, and dataflow. It will not need to rediscover the graph first.

> We know where control can flow. Now we can start asking what values flow with it.
