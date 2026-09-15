# ASYNC-M0: `async fn` / `await` as FLOW sugar

## Contract

`async fn` and prefix `await Expr` are source sugar for an explicit resumable
Octomata FLOW machine. Project loading erases each async declaration into an
ordinary `flow` before the existing type checker, interpreter, MIR lowering,
and Go backend execute it. There is no scheduler, event loop, thread, Future
trait, custom awaiter protocol, or second async IR/runtime.

```text
async source -> generated ast.FlowDecl -> ordinary FLOW MIR -> interpreter/Go
```

An `async fn Foo() -> Int` keeps the pleasant eventual source return type
`Int`. Calling it constructs the existing `FlowInstance<Int>` computation
handle and does not run it synchronously. A synchronous caller can store and
drive that handle with the existing `Step`, `Complete`, `Active`, and `Result`
builtins. ASYNC-M0 accepts `await` directly on either an async-function call or
an ordinary FLOW call. Arbitrary values are not awaitable, and awaiting a
stored handle is deferred.

## Deterministic lowering

For each source await, numbered in source order from zero, lowering emits:

- persistent handle field `AwaitN`;
- state `AwaitN`, which calls ordinary `Step` once and tests `Complete`;
- state `SuspendAwaitN`, which performs the value-less FLOW suspension;
- continuation `ContinueAfterAwaitN`, which extracts `Result(...)!` and runs
  the remainder of the source body.

The suspension helper loops back to `AwaitN` only when the outer machine is
stepped again. Thus `await` never blocks a FLOW turn. `return` remains final
FLOW completion; it is not `yield`.

Simple branches containing await lower to stable `IfNThen`, `IfNElse`, and
`ContinueAfterIfN` states. Loops containing await are rejected in M0.

## Local liveness

The lowering performs a bounded lexical liveness pass. A local is lifted to a
generated persistent field `Local_Name` only when an await lies between its
definition and a later use. Await result locals begin their lifetime in the
continuation, so they are not lifted merely because their initializer awaits;
they are lifted only if they cross a later await. Locals consumed before the
next suspension remain ordinary FLOW state locals.

For simple `if` lowering, M0's lexical pass is intentionally conservative at
arm boundaries: an await in one arm and a later lexical use in the other may
lift a local even though only one arm executes. It still does not hoist every
temporary, and the dump reports every lifted name. Graph-sensitive branch
liveness is deferred rather than hidden behind a different runtime model.

`OCT_MIR_DUMP=1 oct build ...` exposes `async-lowering`, lifted locals,
persistent handle fields, continuation names, and the ordinary FLOW states.
This is the supported inspection path; reading generated Go is unnecessary.

## `remember` / `resume`

`await` is a compiler-generated continuation with a fixed await/continue
shape. `remember` and `resume` remain explicit programmer-controlled capture
and transfer through FLOW's existing single resume slot. ASYNC-M0 does not
redefine or use that slot.

## Fallibility and restrictions

ASYNC-M0 rejects fallible async declarations and `?` within async bodies.
This preserves Oct's existing error semantics rather than inventing an async
exception channel. It also rejects async recursion, await in loops, `return
await`, await assignment, generic async declarations, shadowed async locals,
and async test/artifact/benchmark/Make entry attributes.

The software interpreter and compiled Go path support the generated FLOW
machines with no fallback. `profile Verilog` currently rejects ASYNC-M0's
nested FLOW handle and discarded `Step` operation through the existing FLOW
hardware-legality boundary. No scheduler-like RTL or async-specific
SystemVerilog backend was added. A later milestone may admit a hardware-static
handshake/inlining form, but M0 makes no Verilog support claim.

Checkpointing an async machine containing a live nested handle is also outside
M0. Ordinary FLOW checkpoint and `remember`/`resume` behavior is unchanged.

## Deferred roadmap (record only)

- ASYNC-M1: await in loops
- ASYNC-M2: fallible async composition
- ASYNC-M3: async streams
- ASYNC-M4: cancellation
- ASYNC-M5: select/race/join
- ASYNC-M6: scheduler/runtime policies

Cancellation, timeouts, task groups, custom awaiters, Future traits, async
traits, dynamic task graphs, work stealing, distributed tasks, and async
generators are explicit non-goals for M0.
