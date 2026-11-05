# Async functions

[Reference home](../README.md) · Prerequisite: [Octomata](flows-and-state-machines.md)

Use `async fn` when a sequential computation must wait for another FLOW and
resume afterward. For independent array work, use [batch](batch.md).


Prefix `async` declares a resumable function and prefix `await` suspends until
an async/FLOW computation completes:

```oct
async fn FetchThenProcess(A: Int) -> Int {
    let X = A + 1
    let Y = await Delayed(40)
    return X + Y
}
```

The written return type is the eventual result type. Calling an async function
constructs the existing FLOW-instance handle; it does not execute the body
synchronously. M0 permits `await` only as the complete initializer of a
`let`/`var`, directly on an `async fn` or `flow` call. Arbitrary values are not
awaitable.

The compiler lowers async source to ordinary Octomata states, persistent
fields, `Step`, `Complete`, `Result`, `goto`, `suspend`, and `return`. Generated
names are deterministic and visible in MIR dumps. See
[Octomata](flows-and-state-machines.md) and
[`docs/internal/async_m0.md`](../../../docs/internal/async_m0.md).

M0 rejects await in loops, stored-handle await, async recursion, fallible or
generic async functions, `return await`, and async generators. It provides no
scheduler, cancellation, timeout, stream, join/race, Future trait, or custom
awaiter protocol.


## How suspension works


`async fn` / `await` is a high-level spelling of an ordinary generated FLOW
machine. Each await has deterministic `AwaitN`, `SuspendAwaitN`, and
`ContinueAfterAwaitN` control states plus a persistent awaited-machine handle.
Only locals live across a suspension are lifted to persistent generated
fields. `return` completes the machine; `suspend` is temporary.

`await` is compiler-generated continuation control. `remember` / `resume` is
the separate explicit programmer-controlled resume-slot abstraction; async
lowering does not redefine or consume that slot.

ASYNC-M0 software execution uses the same interpreter and compiled FLOW
machinery described below. Its nested-machine `Step` pattern is not yet legal
under `profile Verilog`; the normal FLOW hardware-legality boundary rejects it
without adding scheduler semantics to RTL.


## Run a complete example

The [async contract](../../ControlFlow/AsyncM0/valid/async_m0.octest) includes
the child FLOW, async declarations, and tests that drive them with `Step`.
The short declaration above needs that child definition; it is not a complete
file on its own.

```text
oct test Language/ControlFlow/AsyncM0/valid --execution compiled
```

Calling an async function only constructs its FLOW. The caller drives it with
`Step`, observes completion with `Complete`, and handles `Result` explicitly.
An await is not a timer or a promise of operating-system parallelism.
