# Chapter 1 — From Source to Executable WebAssembly

Put this function in front of the Oct compiler:

```oct
fn Add(a: Int, b: Int) -> Int {
    return a + b
}
```

The repository's real example builds it with:

```text
go run ./cmd/oct build Examples/WasmCompute --target wasm
```

That command produces a standards-valid binary module. A real WebAssembly runtime can instantiate it and report `Add(20, 22) = 42`.

We are going to follow this function all the way from source text to machine-executable WebAssembly. No compiler-history detour, and no educational compiler hiding behind the curtain: every important representation in this chapter comes from the production path in this repository.

## 1. What a compiler actually does

A useful first model of a compiler is **a sequence of representations and decisions**:

```text
Oct source text
    |
    v
syntax / AST
    |
    v
semantic analysis and type checking
    |
    v
Oct MIR
    |
    v
WebAssembly lowering
    |
    v
binary encoding
    |
    v
WebAssembly runtime
```

A **frontend** is the part that understands the source language. In Oct it includes lexical analysis, parsing, project elaboration, name resolution, and type checking. The parser turns tokens into an **abstract syntax tree (AST)**: an in-memory model of source structure. [`internal/ast/program.go`](../../internal/ast/program.go) has nodes for function declarations and binary expressions, while [`internal/parse/parse.go`](../../internal/parse/parse.go) constructs those nodes.

The AST says that `a + b` is a binary expression inside a return statement. That is source structure, not yet its complete meaning. **Semantic analysis** answers which declarations `a` and `b` refer to, whether their types are suitable, what `+` means for those types, and whether the result matches `Int`. Oct performs this work through project elaboration and [`typecheck.CheckProgram`](../../internal/typecheck/typecheck.go).

An **intermediate representation (IR)** is a program representation chosen for later compiler work rather than for source authors. Oct's ordinary **MIR**—the repository's name for its middle-level IR—retains executable operations and control flow while discarding much source spelling and sugar.

A **backend** consumes that shared meaning and chooses a target representation. The WebAssembly backend maps Oct types and MIR operations to WebAssembly types, locals, instructions, function indices, and structured control constructs. **Lowering** means translating to a lower-level representation while preserving meaning. **Code generation** is the broader backend work of choosing and emitting target operations.

Finally, **binary encoding** serializes the chosen target program into the WebAssembly byte format. A **runtime** loads, validates, instantiates, and executes that module. Node is the runtime used by the focused repository test; it is not involved in compilation.

This division is Oct's current design, not a law requiring every compiler to use exactly these phases. At each stage, some questions have already been answered and others remain deliberately unanswered. That idea is our compass for the rest of the book.

## 2. Why have an IR at all?

If each backend compiled directly from the parser's AST, the Go, WebAssembly, and SystemVerilog backends would each need to understand name lookup, types, source control forms, and every piece of language sugar. Three backends would become three partial implementations of the language.

Oct instead has this shape:

```text
                    +--> Go
Oct --> frontend --> MIR --> WebAssembly
                    +--> SystemVerilog for legal Veril-Oct programs
```

The frontend converts many source forms into a smaller semantic vocabulary. Backends share frontend decisions, tests can compare different executions of the same meaning, and a future optimization over suitable MIR can benefit more than one target.

This also isolates syntax from targets. Oct-XML can contain markup-looking syntax such as `<Document.P>...</Document.P>`. The parser initially represents it explicitly, but project elaboration in [`internal/project/parametric.go`](../../internal/project/parametric.go) rewrites it into ordinary calls, arrays, and literals before type checking. There is no markup MIR. The WebAssembly, Go, interpreter, and profile backends never receive markup nodes.

That is one concrete answer to “why IR?”: source syntax can change without requiring every backend to change.

## 3. Meet the real Oct MIR

The real MIR is a collection of Go values, not a magic textual language. Here is the structural center, trimmed from [`internal/build/mir.go`](../../internal/build/mir.go):

```go
type MIRFunction struct {
    Package         string
    Name            string
    Params          []MIRField
    CaptureEnv      []MIRCapture
    Return          string
    IsFallible      bool
    ErrorType       string
    Locals          []MIRField
    Blocks          []MIRBlock
    UsesUtilityWhen bool
}

type MIRBlock struct {
    Label      string
    Statements []MIRStmt
    Terminator MIRTerminator
}

type MIRAssign struct {
    Target string
    Value  MIRValue
}

type MIRCall struct {
    Target        string
    Callee        string
    Args          []MIRValue
    ArgTypes      []string
    Builtin       bool
    RetType       string
    FunctionValue bool
}
```

A module contains functions; a function contains parameters, locals, and blocks; a block contains statements followed by one terminator. The other fields retain facts needed by particular semantic and backend paths without changing that core shape.

The terminators are deliberately separate:

```go
type MIRReturn struct{ Value MIRValue }
type MIRJump struct{ Target string }
type MIRBranch struct {
    Cond                    MIRValue
    TrueTarget, FalseTarget string
}
type MIRFail struct{ Value MIRValue }
```

A **basic block** is a sequence of instructions with one entry and one control-flow decision at the end. That decision is the **terminator**. It returns, jumps, branches, or fails. Keeping it separate makes an incomplete or ambiguous ending harder to hide among ordinary assignments.

Values are structured too. The smallest relevant subset from [`internal/build/mir_value.go`](../../internal/build/mir_value.go) is:

```go
type MIRLiteral struct {
    Type  string
    Value string
}

type MIRLocal struct {
    Name string
    Type string
}

type MIRBinary struct {
    Op          string
    Left, Right MIRValue
    Type        string
}

type MIRUnary struct {
    Op    string
    Value MIRValue
    Type  string
}

type MIRConvert struct {
    TargetType string
    Value      MIRValue
}
```

Names and type identities are still strings, but expression structure is typed Go data. A backend switches on `MIRLiteral`, `MIRLocal`, or `MIRBinary`; it does not reparse an expression string.

## 4. Lower `Add` into MIR

The backend entry in [`internal/build/compiler.go`](../../internal/build/compiler.go) is short enough to show in full:

```go
func LoadMIR(path string) (MIRModule, string, error) {
    program, err := project.Load(path)
    if err != nil {
        return MIRModule{}, "", err
    }
    if err := typecheck.CheckProgram(program); err != nil {
        return MIRModule{}, "", err
    }
    module, err := lowerProgram(program, compileOptions{})
    if err != nil {
        return MIRModule{}, "", err
    }
    return module, program.EntrySource, nil
}
```

Here is the exact output of the current diagnostic MIR dumper for `Add`.

**Actual compiler dump**

```text
fn WasmCompute.Add(__oct_user_0:Int, __oct_user_1:Int) -> Int
  entry:
    __oct_internal_tmp_0 = (__oct_user_0 + __oct_user_1)
    return __oct_internal_tmp_0
```

This is a deterministic diagnostic view from [`internal/build/mir_dump.go`](../../internal/build/mir_dump.go). It is not the in-memory representation and is not parsed back into the compiler. The checked-in [snapshot](snapshots/add.mir) is synchronized by a test.

**Simplified explanatory form**

```text
function Add(a: Int, b: Int) -> Int
entry:
    temp = a + b
    return temp
```

The simplified form is ours, not Oct syntax and not an alternative MIR format.

By typed frontend/MIR time, the compiler knows that `a` and `b` refer to these parameters, both have type `Int`, the operator means integer addition, the function returns `Int`, and name lookup is complete.

MIR deliberately has not yet decided which WebAssembly value type represents `Int`, which local index each value receives, which function index identifies `Add`, how instructions are binary encoded, or how exports are serialized. Those are target questions.

## 5. Values do not have to be SSA values

Oct's ordinary MIR is currently a **mutable-local CFG IR**, not static single assignment form (SSA). A local may be assigned more than once:

```text
total = 0
...
total = total + i
```

`SumTo` does exactly this on each loop iteration. That is a perfectly valid IR design. Mutable locals correspond naturally to source mutation, Go variables, and WebAssembly locals.

SSA is another representation in which each SSA value is defined once. It can make some analyses and optimizations easier, but “not SSA” does not mean “not a real IR,” nor does it make clean code generation impossible. We will introduce SSA later when a concrete problem gives us a reason to want it—not as an initiation ritual.

## 6. From MIR types to WebAssembly types

The M0 mapping is explicit:

| Oct/MIR type | WebAssembly value type |
| --- | --- |
| `Bool` | `i32`, canonically 0 or 1 |
| `Int` | signed `i64` |
| `Float` | IEEE-754 `f64` |

The implementation in [`internal/wasm/wasm.go`](../../internal/wasm/wasm.go) is a small switch:

```go
switch t {
case "Bool":
    return i32, nil
case "Int":
    return i64, nil
case "Float":
    return f64, nil
case "Void":
    return 0x40, nil
}
```

The frontend answers “this value is an Oct `Int`.” The WebAssembly backend answers “on this target, materialize it as `i64`.” A different backend can make different choices while preserving language semantics. Veril-Oct, for example, has hardware-oriented legality and representation concerns.

## 7. Lower `Add` to WebAssembly

The function builder assigns indices. Parameters come first, then MIR locals, then one backend-owned program-counter local:

```go
for i, p := range fn.Params {
    locals[p.Name] = uint32(i)
    localTypes[p.Name] = p.Type
}
for _, l := range fn.Locals {
    t, err := e.valueType(l.Type)
    // check err and encode the declaration
    locals[l.Name] = uint32(len(locals))
    localTypes[l.Name] = l.Type
}
pc := uint32(len(locals))
```

`MIRLocal` emits WebAssembly's `local.get` instruction (`0x20`) plus the local index:

```go
case build.MIRLocal:
    idx, ok := locals[v.Name]
    if !ok {
        return nil, fmt.Errorf("local %s does not exist", v.Name)
    }
    out = append(out, 0x20)
    return append(out, u32(idx)...), nil
```

For `MIRBinary`, the backend recursively emits the left operand, the right operand, and a selected opcode. Its integer table maps `+` to `0x7c`, WebAssembly's `i64.add`. `MIRReturn` emits its value and `return` (`0x0f`). A human-readable WAT-style equivalent is:

```wat
(func (export "Add") (param i64 i64) (result i64)
    local.get 0
    local.get 1
    i64.add
)
```

Oct does **not** compile through WAT. The real backend emits binary WebAssembly directly. WAT is shown only because humans can read it. The actual function also sits inside the backend's dispatch shell; this one-block case returns immediately.

## 8. Binary WebAssembly

A `.wasm` file is a structured binary module. After its magic bytes and version, this backend emits:

```text
type      function signatures
function  each function's signature index
export    exported name to function-index mappings
code      encoded function bodies
custom    deterministic Oct backend provenance
```

There are no imports, memory, tables, globals, start function, host callbacks, or WASI dependency in M0. The section writer is almost disarmingly small:

```go
func (b *moduleBuilder) section(id byte, p []byte) {
    b.bytes = append(b.bytes, id)
    b.bytes = append(b.bytes, u32(uint32(len(p)))...)
    b.bytes = append(b.bytes, p...)
}
```

`u32` performs WebAssembly's variable-length integer encoding, while helpers build section payloads. We will postpone the byte-level tour. WAT and binary WASM can present the same target program, but the current Oct path constructs and serializes binary directly.

## 9. The first control-flow example

The canonical example also contains:

```oct
fn Max(a: Int, b: Int) -> Int {
    if a > b {
        return a
    }
    return b
}
```

Its [actual synchronized MIR snapshot](snapshots/max.mir) is:

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

The frontend retains an explicit join block (`b3`) even though this small case could be compressed. We are inspecting, not optimizing.

The blocks and possible transfers form a **control-flow graph (CFG)**:

```text
             entry
             /   \
          true   false
           /       \
          b1       b2
          |         |
      return a      v
                   b3
                    |
                return b
```

A node is a basic block. An edge is a possible control transfer. The graph of nodes and edges is the CFG. `entry` has two outgoing edges because its terminator is a branch; `b2` has one because its terminator is a jump; return blocks have none.

## 10. Why WebAssembly control flow is interesting

Oct MIR can express an arbitrary graph of jumps and branches. Core WebAssembly control flow is structured around nested `block`, `loop`, and branch instructions. Those shapes do not line up automatically.

The current M0 backend uses a **structured dispatch loop**. Every MIR block receives an integer index. A backend-owned `pc` local holds the index of the next MIR block. The emitted function has an outer `block`, an inner `loop`, and one guarded case per MIR block:

```text
block exit
    loop dispatch
        if pc == 0: execute MIR block 0
        if pc == 1: execute MIR block 1
        ...
```

A jump stores its target index in `pc` and branches back to the dispatch loop. A conditional branch uses WebAssembly `select` to choose its true or false target index, stores it, and also returns to dispatch. A return exits the function directly.

This strategy is simple, accepts arbitrary current CFGs, preserves mutable-local semantics, and is easy to validate. It is not the only strategy. A compiler could restructure the CFG, use a Relooper-like transformation, or reconstruct nested structured control directly. Those alternatives have different complexity and performance tradeoffs. M0 chooses correctness and coverage first; later chapters can measure the consequences.

## 11. Follow `SumTo` through control flow

Here is the current source:

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

Here is the [actual MIR snapshot](snapshots/sum-to.mir), with generated names intact:

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

Its CFG is the first loop in the book:

```text
        entry
          |
          v
     b1: test i <= n --------false------> b3: return total
          |
         true
          |
          v
     b2: update total and i
          |
          +-----------------------------> b1
```

At runtime the dispatch loop begins with `pc = 0`, so it executes `entry`. The jump selects `b1`. The test in `b1` selects `b2` while `i <= n`; `b2` mutates the WebAssembly locals for `total` and `i`, then selects `b1` again. When the condition becomes false, `b3` returns `total`.

Nothing here requires SSA. MIR assignments become `local.set`; reads become `local.get`; graph edges become updates to `pc` followed by structured branches.

## 12. How do we know the compiler is correct?

The focused proof in [`internal/wasm/wasm_test.go`](../../internal/wasm/wasm_test.go) uses **differential testing**: it runs the same program through independent execution paths and compares results.

```text
Interpreter  -> 104
Go backend   -> 104
WASM / Node  -> 104
```

The Node lane also validates and calls individual exports. Its current result vector is:

```text
Add, Max, SumTo, ModeCode, FloatKernel, BoolKernel, SignedKernel, Main
42,  22,  55,    7,        4.5,         1,          -14,          104
```

Agreement is not mathematical proof: two implementations can share a bug, and a finite suite cannot cover every program. It is nevertheless strong practical evidence, especially when the interpreter, generated Go, and direct WebAssembly paths perform materially different work. It is also an excellent regression alarm.

The same test calls `Encode` twice for the same MIR and requires byte-for-byte equality. That proves **determinism** for the specimen. Determinism and semantic correctness are different properties: identical wrong bytes would be deterministic, while two different but equivalent modules might both be correct. We want stable output and correct behavior.

## 13. What the backend does not support yet

M0 intentionally accepts a bounded subset:

| Feature | Why it is deferred |
| --- | --- |
| records | needs an aggregate representation and ABI |
| arrays | needs linear memory, allocation, length, bounds, and copy rules |
| strings and bytes | needs a memory and host representation |
| payload-carrying enums | needs payload layout and tag/payload rules |
| fallible results | needs a result ABI |
| FLOW MIR | has a separate execution and runtime contract |
| indirect calls/captures | needs table and environment design |

Payload-free enums are supported as deterministic `i32` tags, and `MIRFail` maps to WebAssembly's `unreachable` trap. Those facts do not imply that richer enums or fallible functions already have an ABI.

The negative test loads a program containing `String` through the real frontend and requires encoding to fail with `type String is unsupported in M0`. An honest compiler backend should reject unsupported semantics clearly rather than silently emitting wrong code. “Not implemented” is inconvenient; “compiled successfully into a lie” is much worse.

## 14. Representation as answered questions

The pipeline is easier to understand when every representation is treated as a set of answered questions:

| Stage | Already knows | Still defers |
| --- | --- | --- |
| Source | programmer intent and written syntax | resolved name and type meaning |
| AST | parsed source structure | complete semantic meaning and target choices |
| Typed frontend | declarations, names, types, and operator meaning | backend representation |
| MIR | executable operations, locals, calls, and CFG | target machine details |
| WASM lowering | value types, locals, indices, instructions, control strategy | exact binary bytes |
| Encoder | complete serialized target module | nothing relevant to compilation |

An IR is not mysterious compiler jargon. It is a useful representation of the program after some questions have been answered and before others need to be.

## 15. Run it yourself

Use Go 1.25 or newer and Node.js, then run from the repository root:

```text
go run ./cmd/oct build Examples/WasmCompute --target wasm
```

For the current sources, the key output is:

```text
build succeeded: .../Examples/WasmCompute/WasmCompute.wasm
target: wasm
sha256: f207a7374466b6762877def709a62eedd59d52828cf34b8dacd91df48c4f87a7
```

On PowerShell, validate and execute the binary with Node:

```powershell
node -e 'const fs=require("fs");const b=fs.readFileSync("Examples/WasmCompute/WasmCompute.wasm");console.log("valid:",WebAssembly.validate(b));WebAssembly.instantiate(b,{}).then(({instance:{exports:e}})=>console.log("Add(20, 22) =",String(e.Add(20n,22n)),"Main() =",String(e.Main())))'
```

Expected:

```text
valid: true
Add(20, 22) = 42 Main() = 104
```

Run the complete execution and determinism proof:

```text
go test ./internal/wasm -run TestCurrentMIRProducesDeterministicExecutableModule -count=1
```

Run the book synchronization check. It reloads `Examples/WasmCompute` through `LoadMIR`, invokes the production diagnostic dumper, and compares `Add`, `Max`, and `SumTo` with the checked-in snapshots:

```text
go test ./internal/build -run TestCompilerOptimizationBookMIRSnapshots -count=1
```

The native compiler can also emit the full diagnostic MIR when `OCT_MIR_DUMP=1` is set. The book test is cleaner because it does not leave a native executable or dump beside the example.

## 16. Where we go next

We now know what the program looks like when it reaches MIR. Before optimizing it, we need to understand the graph formed by MIR blocks.

Chapter 2 is **Basic Blocks and Control-Flow Graphs**. It will ask:

```text
What blocks can reach this block?
What blocks can this block reach?
Where do loops appear?
What does "predecessor" mean?
What is a back edge?
Why do optimizers care?
```

We have seen enough of `Max` and `SumTo` to make those questions concrete. We will answer them there, with the compiler still open in front of us.
