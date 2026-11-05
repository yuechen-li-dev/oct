# Turns, yields, and checkpoints

[Reference home](../README.md) · Prerequisite: [Octomata](flows-and-state-machines.md)

Use a turn input for new information supplied by a caller, a board for private
state that must persist, and a yield for one observable result before completion.
A state local belongs only to its current activation.

A top-level yield resumes at the next state statement. A yield or suspend
nested inside a branch or loop resumes after the containing top-level state
statement, rather than retaining the nested execution position. See the
[FLOW rules](flows-and-state-machines.md#rules-and-contextual-keywords) for
the associated scope and unreachable-statement checks.


```oct
flow Counter(start: Int) accepts amount: Int yields Int -> String {
    board { Count: Int }

    state Start {
        board.Count = start
        goto Active
    }

    state Active {
        board.Count = board.Count + amount
        yield board.Count
        goto Active
    }
}

fn UseCounter() -> Void {
    let counter = Counter(0)
    Step(counter, 2)
    let first = Yielded(counter)!
    Step(counter, 5)
    let second = Yielded(counter)!
}
```

`yield` is a state-machine boundary, not a lazy collection operation. A yielding flow can model a generator, reactive controller, workflow, or decision machine without introducing a second coroutine/generator runtime. `suspend` remains a legal value-less scheduling boundary. `Result(flow)` remains unavailable until a final `return` completes the flow.

An iterator/generator is a yielding flow consumed repeatedly. This model has no
hidden lazy-list semantics and does not require a parallel generator runtime.
The bounded `query` authoring form is specified separately in
[FLOW-backed queries](queries.md). It lowers to exactly this yielding FLOW
model and does not redefine behavioral FLOW semantics.

### Logical checkpoints and generated Go host facade (FLOW-TURN-M1)

Embeddable compiled-Go emission exposes an experimental generated facade for
each flow: `NewFlow`, a typed `Step` method, a typed turn with `Yielded`, and,
for yielding flows, `Checkpoint` plus `RestoreFlow`. Record-shaped Concept
inputs use their generated concrete Go struct; refinement Concepts use their
lowered static representation; nominal enum inputs use the generated enum type
and variant constructors. External Go does not need compiler-private
`fn_...`/`__octStep` symbols.

A checkpoint is legal after a turn ended at `yield`. It represents the machine
after the yield: version and flow fingerprints, named state and opaque
continuation position, construction values, private board, resume slot,
feature-observed history, the commitment of each `when policy` site, and the
last yielded value needed to preserve `DidYield`/`Yielded` observability. The completed turn
input and state locals are absent. Restore rejects incompatible version, flow,
fingerprint, state/continuation, board, construction, utility-site, and yield
schemas with machine-readable reasons.

The interpreter and compiled materializer use this same logical ownership
model. They do not serialize ASTs, environments, Go pointers, or arbitrary
object graphs. Because locals may not survive `yield`, durable continuation
must be represented in construction state, the private board, resume slot, or
utility policy state. Generated checkpoint bytes are deterministic typed JSON
for the experimental host boundary; that byte encoding and generated naming
are not a permanent Oct 1.0 ABI and may change across compiler revisions.
Logical checkpoint schema version 3 adds deterministic typed recursive values
for records, nested records, plain and payload enums, arrays, vectors, and
matrices. Version 4 records the commitment of a `when policy` site as the arm
it is committed to and how long it has been held, in place of the value that
arm produced and the score it had; a site of any result type is therefore
checkpointable, in both lanes. Older logical
checkpoint versions are rejected rather than silently reinterpreted.

For the deterministic, effect-free subset, continuing an in-memory machine and
serializing, restoring, then continuing from the same boundary must produce the
same result, board, resume slot, utility commitment, and state history. The
contract fixture at
`Language/ControlFlow/OctomataCheckpointDeterminism/valid/` continuously checks
this property for state/goto, nested control, board mutation, suspend,
remember/resume, and deterministic utility selection. Ambient time, external
effects, `Entropy` draws, and random streams or draw indices that are not stored
in persistent flow data are outside this deterministic subset.



## Inspecting a running machine

`Complete` answers whether final return occurred; `Result` extracts that return
fallibly. `DidYield` and `Yielded` describe only the most recent turn.
`Active`, `ResumeTarget`, and `StateHistory` expose control state. `BoardSnapshot`
is a fallible typed copy of the declared board; later array mutation cannot
change an earlier snapshot. See [Octomata rules](flows-and-state-machines.md#rules-and-contextual-keywords)
for persistence types and default initialization.

The runnable [lesson FLOW](../../ReferenceExamples/valid/unusual_features.octest)
demonstrates two input-bearing turns. The
[turn contracts](../../ControlFlow/OctomataFlowTurnM0/valid/) and
[checkpoint contracts](../../ControlFlow/OctomataFlowTurnM1/valid/)
cover the larger surface. Host checkpointing is experimental interoperability;
it is not a general source-level serializer for arbitrary program state.
