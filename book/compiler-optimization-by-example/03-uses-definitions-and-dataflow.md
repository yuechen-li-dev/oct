# Chapter 3 — Uses, Definitions, and Dataflow

Consider this valid Oct function:

```oct
fn Choose(flag: Bool) -> Int {
    var x = 1

    if flag {
        x = 2
    }

    return x
}
```

At `return x`, which assignment to `x` might have produced its value?

```text
x = 1
x = 2
```

If `flag` is false, execution skips the second assignment. If `flag` is true,
the second assignment replaces the first value. Both answers are possible at
compile time.

This chapter teaches the compiler to compute that answer. It is our first real
analysis over the control-flow graph from Chapter 2.

The recurring idea for this book is:

> A compiler analysis is just the compiler asking a precise question about the
> program and computing enough facts to answer it everywhere.

Our question is:

> Which assignments could have produced the current value of this local when
> execution reaches this point?

The analysis that answers it is **reaching definitions**. We will begin with
code and execution paths. The notation comes later, after it has something
concrete to describe.

## 1. What is a definition?

In compiler terminology:

> A definition is a program operation that assigns a new value to a local or
> place we care about.

The initial `var x = 1` defines `x`. The later `x = 2` defines `x` again. An
immutable `let x = 1` also creates a definition: *definition* is about producing
a value, not about whether source code may later reassign the binding.

Oct's ordinary MIR currently has these statement forms:

```go
MIRAssign
MIRRowAssign
MIRIndexAssign
MIRCall
MIRGenericOctxiliaryCall
MIRDestructureCall
MIRConstructRecord
MIRConstructArray
MIRBatchMap
```

Chapter 3 tracks **whole scalar MIR locals**. In that scope:

- `MIRAssign` defines its target.
- `MIRCall` and `MIRGenericOctxiliaryCall` define their result target.
- `MIRDestructureCall` defines each result target, in result order.
- `MIRConstructRecord`, `MIRConstructArray`, and `MIRBatchMap` define their
  result target.
- A target named `_` discards a value and does not define a tracked local.
- Function parameters are synthetic definitions available at function entry.
  Captured inputs, when present, are modeled the same way.
- `MIRRowAssign` and `MIRIndexAssign` write a row or element inside an
  aggregate. They are explicitly classified as compound definitions, but they
  do not redefine the whole scalar local in this M0 analysis.

That last boundary matters. This chapter does not claim to analyze aliases,
heap memory, individual array elements, record fields, pointer-like references,
captures of mutable external state, or external state. Those require a memory
model, not a convenient fiction. The analysis records compound writes so a
future client cannot mistake “not tracked” for “does not exist.”

## 2. What is a use?

> A use is a read of a value needed to compute something else.

For example:

```text
x + y       uses x and y
if x > 0    uses x
return x    uses x
Foo(x)      uses x
```

In current MIR, uses live inside structured `MIRValue` nodes. A `MIRLocal` is a
local read. Literals and direct function references are not local reads. A
larger value can contain many smaller values, so one MIR expression can contain
several uses.

Uses also occur in terminators. `MIRBranch` uses its condition, a value-returning
`MIRReturn` uses its returned value, and `MIRFail` uses its error value. A void
return and `MIRJump` have no value use; the jump merely names a control-flow
target.

For an index or row write, the aggregate target is a use in this scope: the
operation needs the existing aggregate storage. Its indices and assigned value
are uses too. The sub-location write itself remains explicitly untracked as a
whole-local definition.

## 3. Extracting uses from structured MIR

Chapter 1 introduced Oct's structured MIR values. The current variants are:

```text
MIRLiteral             MIRLocal
MIRFunctionRef         MIRUnary
MIRBinary              MIRConvert
MIRIndex               MIRFieldAccess
MIRClone               MIRRangeValue
MIRArrayConvert        MIRLength
MIRMatrixColumnCount   MIRResultValue
MIREnumValue           MIREnumPayload
MIRIntrinsicValue      MIRBackendValue
```

Most use extraction is a recursive walk:

- a local contributes one use;
- a unary operation walks its operand;
- a binary operation walks left, then right;
- conversion and clone nodes walk their wrapped value;
- indexing walks both target and index;
- field access walks its target;
- ranges walk the present start, end, and step values;
- result and enum nodes walk the value or payload they actually contain;
- an intrinsic walks its arguments in order.

`MIRBackendValue` is the deliberate exception. It contains backend expression
text rather than a structured value tree. The analysis fails clearly if it
encounters one, because silently reporting zero uses would be false.

> If an expression were just `"x + y"`, reliable use analysis would require
> reparsing backend-shaped text. Structured MIR makes analysis mechanical.

This is one payoff from the MIR cleanup used by the WebAssembly backend. The
analysis does not know Go syntax or WebAssembly instructions. It walks the same
backend-neutral values both backends receive.

## 4. A reusable MIR use/def utility

The implementation lives in
[`internal/build/use_def.go`](../../internal/build/use_def.go). Its central
types are deliberately small:

```go
type DefinitionID struct {
    Kind      DefinitionKind
    Block     string
    Statement int
    Result    int
    Local     string
}

type UseSite struct {
    Block     string
    Statement int
    Operand   string
    Local     string
}

type UseDefInfo struct {
    Definitions         []DefinitionID
    InitialDefinitions  []DefinitionID
    DefinitionsByBlock  map[string][]DefinitionID
    Uses                []LocalUse
    UsesByBlock         map[string][]LocalUse
    CompoundDefinitions []CompoundDefinition
}
```

`ExtractUseDefInfo` does not mutate MIR. It walks function parameters, captures,
blocks, statements, terminators, and nested values. Slices follow MIR order.
Operand paths such as `value.left` or `condition` distinguish multiple reads in
one statement without random identifiers.

The utility handles every current ordinary MIR statement explicitly. Unknown
statement types, unknown value types, nil required values, empty targets,
malformed record construction, and opaque backend values return contextual
errors. Adding a new MIR variant therefore requires an intentional analysis
decision instead of silently weakening every future optimization.

## 5. Definition identity matters

These operations define the same local but are not the same definition:

```text
x = 1
...
x = 2
```

Reaching definitions must distinguish them. Oct derives identity from MIR
position:

```text
entry:stmt0/result0 defines x
then:stmt0/result0  defines x
```

`Result` matters for destructuring, where one statement can define several
locals. Parameters and captures use their declared index plus a distinct
definition kind. Nothing is random, and no global counter survives between
compiler runs.

For human-readable dumps, the deterministic function-local definition order is
also numbered `D0`, `D1`, and so on. The number is presentation; the structural
MIR position is the identity.

## 6. Uses and definitions in one block

Begin without branches:

```text
x = 1
y = x + 2
x = y * 3
return x
```

Read the block from top to bottom:

1. `x = 1` defines `x`.
2. `y = x + 2` uses the current `x` and defines `y`.
3. `x = y * 3` uses `y` and defines a new value for `x`.
4. `return x` uses the latest runtime value of `x`.

The first definition of `x` can reach the use in statement 1. It cannot reach
the return, because statement 2 redefines `x` on every path through this block.

> How does the compiler know which definitions can reach each point?

Inside one block, it can simulate statement order. Across blocks, it needs the
CFG.

## 7. Local reasoning is not enough

`Choose` has this shape:

```text
             entry: x = 1
                  |
                branch
               /      \
      then: x = 2      skip
               \      /
                  join
                   |
                return x
```

The join has two predecessors. The `then` predecessor sends the `x = 2`
definition. The skip predecessor sends the original `x = 1` definition.

> This is where the CFG from Chapter 2 becomes necessary.

The CFG tells us where facts may travel. It does not itself say what those
facts are.

## 8. Reaching definitions

> A definition reaches a program point if there is at least one control-flow
> path from that definition to the point along which the same local is not
> redefined.

The phrase “at least one” is crucial. Reaching definitions asks what **may**
happen, not what happens on every execution.

For `Choose`, let:

```text
D0 = the input definition of flag
D1 = x = 1
D2 = x = 2
```

Along the skip path, `D1` reaches the return. Along the taken path, `D2` reaches
the return and `D1` is replaced. Therefore both `D1` and `D2` reach the return
block.

## 9. GEN and KILL, informally first

Suppose one block contains:

```text
x = 2
y = 3
x = 4
```

After the block executes, its last `x` definition and its `y` definition are
new facts it contributes. These are the block's **GEN** facts.

The block also prevents older definitions of `x` and `y` from passing through.
These are its **KILL** facts. The earlier `x = 2` inside the block is superseded
before the block exits, so only `x = 4` is in `GEN`.

Once that behavior is intuitive, the standard compact form is useful:

```text
OUT = GEN ∪ (IN - KILL)
```

In words:

- begin with definitions that reach the block (`IN`);
- remove definitions replaced by assignments in the block (`IN - KILL`);
- add the last definitions produced by the block (`GEN ∪ ...`);
- the result is what can reach successor blocks (`OUT`).

The symbols are only shorthand. `∪` means combine the sets without duplicates,
and `-` means remove members of the right-hand set.

## 10. IN and OUT facts

For every reachable block `B`:

```text
IN[B]  = definitions that may reach B before B executes
OUT[B] = definitions that may reach a successor after B executes
```

If `B` has several predecessors:

```text
IN[B] = union of OUT[pred] for every predecessor pred
```

Why union?

> We care whether a definition can reach here along any possible path.

If one predecessor carries `D1` and another carries `D2`, both are possible at
the merge. Union is the first merge operator in this book.

Function parameters and captures seed `IN[entry]`. Oct models them as synthetic
definitions because a parameter's incoming value can feed a use even though no
ordinary MIR assignment created it inside the function.

## 11. Working `Choose` by hand

The real current MIR is generated into
[`snapshots/choose.mir`](snapshots/choose.mir):

```text
fn WasmCompute.Choose(__oct_user_0:Bool) -> Int
  entry:
    __oct_user_1 = 1
    branch __oct_user_0 ? b1 : b2
  b1:
    __oct_user_1 = 2
    jump b3
  b2:
    jump b3
  b3:
    return __oct_user_1
```

The backend-safe names are ordinary MIR local identities:

```text
D0 = parameter __oct_user_0, source name flag
D1 = entry:stmt0 defines __oct_user_1, source name x
D2 = b1:stmt0 defines __oct_user_1, source name x
```

Now solve each block:

```text
entry
  IN  = {D0}
  OUT = {D0, D1}

b1 (conditional arm)
  IN  = {D0, D1}
  OUT = {D0, D2}       D2 kills D1

b2 (skip arm)
  IN  = {D0, D1}
  OUT = {D0, D1}

b3 (join and return)
  IN  = {D0, D1, D2}   union of OUT[b1] and OUT[b2]
  OUT = {D0, D1, D2}
```

Filtering `IN[b3]` to definitions of `x` gives:

```text
{D1, D2}
```

That is the answer we wanted at the beginning of the chapter. The complete
generated block and use facts are in
[`snapshots/choose.reaching`](snapshots/choose.reaching).

## 12. Why loops change things

`SumTo` from Chapter 1 contains a `while` loop. Its CFG has a loop header `b1`
with two predecessors:

```text
entry -----> b1 -----> b2
              ^         |
              |         |
              +---------+
```

One predecessor initializes `total` and `i`. The other is the loop body, which
assigns their next values and jumps back.

Facts from the body therefore flow back into the header. If we visit the header
before the body, its first answer cannot yet include those body definitions.
A single pass is incomplete.

## 13. Fixed points without mysticism

The solution is operational:

> Keep propagating facts until another pass changes nothing.

That unchanged state is a **fixed point**.

Imagine a loop where the entry defines `x` as `D0` and the body defines it as
`D1`. The first header visit knows `{D0}`. After the body runs through the
analysis, the backedge brings `D1` to the header. The next header visit knows
`{D0, D1}`. Another trip adds nothing, so the answer is stable.

> A fixed point is not magic. It means the answer stopped changing.

This process terminates here because the function has a finite number of
definitions. The analysis only adds facts through predecessor union; it never
invents an unbounded new definition during iteration. Eventually there is
nothing left to add.

## 14. The worklist algorithm

Oct implements a purpose-built worklist in
[`internal/build/reaching_defs.go`](../../internal/build/reaching_defs.go). The
core loop uses the CFG's deterministic reachable-block order:

```go
worklist := append([]string(nil), cfg.Reachable...)
queued := make(map[string]bool, len(worklist))
for _, label := range worklist {
    queued[label] = true
}
for len(worklist) > 0 {
    label := worklist[0]
    worklist = worklist[1:]
    queued[label] = false

    newIn := make([]bool, len(info.Definitions))
    if label == cfg.Entry {
        unionBits(newIn, initialBits)
    }
    for _, predecessor := range cfg.Predecessors[label] {
        if reachable[predecessor] {
            unionBits(newIn, outBits[predecessor])
        }
    }
    newOut := transferDefinitionBits(newIn, genBits[label], killBits[label])
    inBits[label] = newIn
    if equalBits(newOut, outBits[label]) {
        continue
    }
    outBits[label] = newOut
    for _, successor := range cfg.Successors[label] {
        if reachable[successor] && !queued[successor] {
            worklist = append(worklist, successor)
            queued[successor] = true
        }
    }
}
```

Every reachable block starts on the worklist. Processing a block merges its
predecessors and applies its transfer behavior. If `OUT` changes, a successor's
next `IN` may change, so successors are enqueued. If `OUT` does not change,
successors would see exactly the same contribution and need no reconsideration
from this block.

`queued` avoids redundant duplicate entries. It is a scheduling aid, not part
of the analysis meaning.

## 15. The production reaching-definitions analysis

`AnalyzeReachingDefinitions(fn)` performs three steps:

1. `BuildCFG(fn)` validates and constructs the CFG.
2. `ExtractUseDefInfo(fn)` inventories definitions, uses, and compound writes.
3. The worklist computes block and statement-level facts.

The reusable result is:

```go
type ReachingDefinitions struct {
    CFG             CFG
    UseDefs         UseDefInfo
    Gen             map[string]DefinitionSet
    Kill            map[string]DefinitionSet
    In              map[string]DefinitionSet
    Out             map[string]DefinitionSet
    AtUse           map[UseSite]DefinitionSet
    DefUses         map[DefinitionID][]UseSite
    Unreachable     []string
    BlocksProcessed int
}
```

The analysis does not mutate MIR and performs no optimization. Reachable blocks
receive facts. Disconnected blocks are reported in `Unreachable` and receive no
invented `IN`, `OUT`, or per-use solution.

## 16. Why there is no generic dataflow framework yet

The worklist pattern will recur, but Oct does not yet hide it behind a large
generic lattice or solver framework. We have only one production dataflow
analysis in this part of the compiler.

A direct implementation lets us inspect the real merge, transfer, scheduling,
and termination rules. After a second analysis exists, we can compare actual
common behavior instead of predicting abstractions prematurely.

## 17. Statement-level precision

Block `IN` and `OUT` facts alone cannot answer which definition reaches an exact
use in the middle of a block. After solving blocks, Oct performs a simple local
walk:

```text
start with block IN
record facts for uses in statement 0
apply definitions made by statement 0
record facts for uses in statement 1
apply definitions made by statement 1
...
record facts for terminator uses
```

The right-hand side of an assignment is read before the assignment defines its
target. Thus `x = x + 1` observes the reaching definitions of the old `x`, then
replaces them with the new definition for later statements.

Each `UseSite` maps to a deterministic `DefinitionSet` in `AtUse`. In the
straight-line example, the use of `x` in `y = x + 2` sees the first definition,
while the return sees the later `x = y * 3` definition.

## 18. Use-def and def-use chains

These names describe two views of the same relation:

- **use-def chain:** for this use, which definitions might have produced the
  value?
- **def-use chain:** for this definition, which uses might observe it?

`AtUse` is the use-def view. `DefUses` is the reverse view generated from the
same solved facts. Oct does not add a second analysis or a complex indexed IR
to provide the terminology.

## 19. Real loop evidence: `SumTo`

The source is:

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

The relevant definitions in real MIR are:

```text
D0: parameter n
D1: entry initializes total
D2: entry initializes i
D5: loop body redefines total
D7: loop body redefines i
```

The loop header receives both entry and backedge facts:

```text
IN[b1] contains
  total: {D1, D5}
  i:     {D2, D7}
```

The condition's `i` use therefore has `{D2, D7}`. The body expression
`total + i` sees `{D1, D5}` for `total` and `{D2, D7}` for `i`. The final return
of `total` also sees `{D1, D5}`: the loop may execute zero times or one or more
times.

The full generated evidence is
[`snapshots/sum-to.reaching`](snapshots/sum-to.reaching). It includes compiler
temporaries as ordinary mutable MIR locals. Some previous-iteration temporary
definitions reach the header too, but no header use reads them before their
next definition. The per-use view makes that distinction visible.

This is the moment to internalize loop-carried data:

> The loop means yesterday's assignment can become tomorrow's input.

The implementation test also records that solving `SumTo` processes more
blocks than the number of reachable blocks. That is concrete evidence that the
backedge caused reconsideration before the fixed point.

## 20. A join is a loss of certainty

Return to:

```oct
var x = 1
if flag {
    x = 2
}
return x
```

At the return:

```text
possible definitions = {x = 1, x = 2}
```

The compiler has learned something precise: no other assignment can produce
this value. But it has not learned one exact value independent of the path. A
join often turns one fact per predecessor into several possibilities.

## 21. What if every reaching definition agrees?

Suppose both paths assign the same constant:

```oct
var x = 0
if flag {
    x = 5
} else {
    x = 5
}
return x
```

Two distinct definitions reach the return, but both produce `5`. A stronger
analysis could conclude:

```text
x == 5
```

That is constant propagation, and it is next. Chapter 3 deliberately does not
compute the values of definitions or rewrite the return.

## 22. Why this matters for optimization

Optimization needs evidence. Reaching definitions can support later work such
as:

- constant propagation;
- dead-store and dead-code elimination;
- copy propagation;
- SSA construction;
- diagnostics that explain where a value may have originated.

The important dependency is:

> Optimization needs facts; dataflow analysis computes those facts.

## 23. Mutable locals versus SSA

Oct's current ordinary MIR is mutable-local MIR:

```text
x = ...
x = ...
x = ...
```

We need reaching definitions to decide which assignment may feed a use. In
static single assignment form, each definition gets a distinct name:

```text
x0 = 1
x1 = 2
```

That makes some questions easier, but a control-flow join still needs a way to
select values from predecessor paths, commonly phi functions or block
arguments. We will not add those, rename locals, or construct SSA in this
chapter.

> We now understand the problem SSA is trying to simplify. Later, when we
> introduce SSA, it will have a reason to exist.

## 24. The recurring dataflow shape

Only after seeing a complete analysis is it useful to name the shared pieces:

```text
facts
+ transfer
+ merge
+ worklist
+ fixed point
```

Reaching definitions uses sets of definitions as facts, `GEN/KILL` as transfer,
union as merge, and a forward worklist to reach stability. Other analyses—such
as liveness, available expressions, and constant propagation—reuse much of this
shape but ask different questions and therefore use different facts or merge
rules.

## 25. Direction matters

Reaching definitions is a **forward analysis**. Assignments affect later program
points, so facts flow in execution direction from predecessors to successors.

Liveness, which we will meet later, is naturally backward. A future use tells
the compiler whether today's value must still be preserved, so information
flows from successors toward predecessors.

Direction is determined by the question, not by a preference in the worklist
implementation.

## 26. May versus must analysis

Reaching definitions asks:

> Which definitions **may** reach here?

One feasible predecessor path is enough, so merge uses union.

A different analysis might ask what is true on **all** paths. If one branch
checks a condition and the other does not, “the condition was checked” is not a
must-fact after their join. Such an analysis would need an all-path merge rather
than simply collecting every possibility.

For now, remember: the word *may* explains why two branch answers survive at a
join.

## 27. Complexity

The work depends on:

```text
number of blocks
number of CFG edges
number of definitions
number of reconsiderations before convergence
```

Each block merge inspects predecessor facts, and each transfer inspects the
finite definition domain. Loops may cause blocks to be processed again.

This first implementation prioritizes transparent correctness. It does not
promise a tighter bound than its actual slice-based bit operations and worklist
scheduling justify.

## 28. Representing definition sets

The solver assigns each structurally identified definition a deterministic
function-local index. Internally, a set is a `[]bool`: index `i` says whether
definition `i` is present. This makes union, removal, comparison, and transfer
small loops with no dependency on a bitset package.

At the API and dump boundary, those bits become `DefinitionSet`, an ordered
slice of `DefinitionID`. The order always follows the MIR definition inventory,
never Go map iteration.

This is efficient enough for the current compiler and easier to teach than a
packed machine-word bitset. If profiling later justifies packing bits, the
semantic API need not change.

## 29. Diagnostics and malformed MIR

Analysis must not lower compiler quality. `AnalyzeReachingDefinitions` first
calls the Chapter 2 CFG builder, so it rejects:

- missing or duplicate labels;
- missing terminators;
- invalid jump or branch targets;
- unsupported terminator forms.

Use/def extraction rejects malformed structured values and statements with the
function, block, and statement position in the error. Opaque backend values
fail because their uses are not structurally available. Compound assignments
are classified rather than ignored.

Failing closed is especially important for later optimization: an omitted use
could otherwise make a live assignment appear dead.

## 30. Deterministic dumps

`DumpReachingDefinitions` prints:

```text
function name
definition inventory
each block in MIR order
GEN, KILL, IN, and OUT
each local use and its reaching definitions
explicit unreachable markers
```

An excerpt for `Choose` is:

```text
block b3
  GEN:  {}
  KILL: {}
  IN:   {D0:__oct_user_0, D1:__oct_user_1, D2:__oct_user_1}
  OUT:  {D0:__oct_user_0, D1:__oct_user_1, D2:__oct_user_1}
  uses:
    term return __oct_user_1 <- {D1:__oct_user_1, D2:__oct_user_1}
```

Stable definition numbering and MIR block order keep diffs useful and make the
book executable documentation.

## 31. Book snapshots

The checked-in authoritative analysis snapshots are:

- [`snapshots/choose.reaching`](snapshots/choose.reaching)
- [`snapshots/sum-to.reaching`](snapshots/sum-to.reaching)

The supporting MIR and CFG snapshots for `Choose` are generated too. Tests in
[`internal/build/book_examples_test.go`](../../internal/build/book_examples_test.go)
regenerate the text through the production loaders and dumpers and compare it
byte-for-byte after newline normalization.

Maintainers can intentionally refresh these files with:

```powershell
$env:OCT_UPDATE_BOOK_SNAPSHOTS = "1"
go test ./internal/build -run TestCompilerOptimizationBook -count=1
Remove-Item Env:OCT_UPDATE_BOOK_SNAPSHOTS
```

Without that explicit environment variable, stale or missing snapshots fail
the test.

## 32. The canonical branch/join specimen

`Choose` now lives in
[`Examples/WasmCompute/WasmCompute.oct`](../../Examples/WasmCompute/WasmCompute.oct).
It is intentionally tiny:

- one Boolean parameter;
- one mutable scalar local;
- one conditional reassignment;
- one join;
- deterministic results `Choose(false) == 1` and `Choose(true) == 2`.

It is not an analysis-only invented graph. Oct lowers it through the production
front end, the Go backend, and the direct WebAssembly backend.

## 33. Runtime parity remains relevant

Reaching definitions is compile-time analysis, but the analyzed MIR must still
mean the same thing everywhere. The existing executable WebAssembly test now
checks both `Choose` paths. `Main` also calls both paths, so the complete example
returns `107` in the interpreter, generated Go executable, and WebAssembly.

This is more than a regression check. It reinforces the phase boundary:

> Analysis observes the program; transformation changes it.

Chapter 3's analysis is never consulted during execution and never changes the
result.

## 34. Analysis versus transformation

An **analysis**:

```text
reads MIR
computes facts
does not change the program
```

A **transformation**:

```text
reads MIR and usually analysis facts
rewrites the program
```

Chapter 3 adds only analysis. There is no constant propagation, dead assignment
removal, branch folding, dead-code elimination, copy propagation, SSA renaming,
or phi construction hidden in this implementation.

## 35. Exercises

### Exercise 1

For:

```oct
var x = 1
x = 2
return x
```

Which definition reaches the return? What happened to the first definition?

### Exercise 2

For a branch where only one arm redefines `x`, list the definitions of `x` that
reach the join.

### Exercise 3

Explain why predecessor `OUT` sets are unioned for reaching definitions.

### Exercise 4

Why can a loop require multiple worklist iterations? Name the CFG edge that
causes facts to return to an earlier block.

### Exercise 5

Given:

```text
IN   = {D0, D1, D2}
GEN  = {D3}
KILL = {D1}
```

Compute `OUT = GEN ∪ (IN - KILL)`.

### Exercise 6

Explain the difference between a use-def chain and a def-use chain. Which map in
`ReachingDefinitions` provides each view?

## 36. Terminology cheat sheet

| Term | Plain-language meaning |
|---|---|
| definition | An operation that produces a new value for a tracked local. |
| use | A read of a value needed to compute something else. |
| definition identity | The stable MIR position that distinguishes one assignment from another. |
| use site | The block, statement/terminator position, operand path, and local for one read. |
| reaching definition | A definition that can arrive without the same local being redefined on that path. |
| `GEN` | The last definitions a block contributes on exit. |
| `KILL` | Earlier same-local definitions a block prevents from passing through. |
| `IN` | Facts immediately before a block executes. |
| `OUT` | Facts immediately after a block executes. |
| transfer function | The rule that computes a block's `OUT` from its `IN` and local behavior. |
| merge | The operation that combines predecessor facts. |
| worklist | Blocks waiting to be processed or reconsidered. |
| fixed point | The state where another propagation step changes no answer. |
| forward analysis | Facts flow with execution, from predecessors to successors. |
| backward analysis | Facts flow against execution, from successors to predecessors. |
| may analysis | Asks what is possible on at least one path. |
| must analysis | Asks what is true on every path. |
| use-def chain | The definitions that may feed a particular use. |
| def-use chain | The uses that may observe a particular definition. |

## 37. Representation as answered questions

The book now has two related answers.

After Chapter 2, the CFG answered:

```text
Where can control go?
```

Chapter 3 answers:

```text
Which assignments can influence this point?
```

The compiler still cannot answer:

```text
What exact value does this local hold?
Is it constant?
Is this assignment ever observed?
Is this value live later?
Can this expression be reused?
```

Those questions motivate later analyses and transformations. Representation
grows useful when each new layer answers one precise question without claiming
answers it does not have.

The pattern we have implemented is:

```text
CFG
+ facts
+ transfer
+ merge
+ iteration
= dataflow analysis
```

We finally know not just where the program can go, but what assignments can
matter when it gets there.

## 38. Next: Constant Folding and Constant Propagation

We now know which definitions can reach a use. The next question is whether
those definitions tell us the value itself.

**Constant folding** evaluates an expression made entirely from constants:

```text
2 + 3  →  5
```

**Constant propagation** carries known values through locals:

```text
x = 5
y = x + 1
        ↓
y = 6
```

Chapter 4 will use the groundwork from this chapter to make those conclusions.
It will begin where analysis facts become useful to a transformation, without
confusing the two phases.
