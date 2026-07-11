# SDSL-V M31a - flow stack syntax and static semantics

M31a adds compiler-owned static control metadata for ordered SDSL-V flow
states. It does not emit a dispatcher, physical return stack, HLSL stack array,
stack pointer, stack bounds check, state switch, inlining, or flattening
optimization. Those remain M31b work.

## Correct pre-M31 history

Pre-M31 SDSL-V flows were ordered, function-local phase blocks. They flattened
directly into structured VD-MIR and HLSL in declaration order. There was no
SDSL-V flow graph, no prior SDSL-V `goto` surface, and no old SDSL-V goto
compatibility obligation.

For source without M31a transitions, this legacy lowering path is unchanged:
states still flatten to ordinary structured statements, and `MaxStackDepth` is
zero.

## Source model

States execute by declaration-order fallthrough unless the final top-level
statement of the state is an explicit transition. The final state falls through
to flow completion.

| Source | Stack effect | Next state |
| --- | --- | --- |
| Fallthrough | none | declared successor |
| `push S` | push successor | `S` |
| `pop` | pop | saved successor |
| `goto S` | none | `S` |
| `finish` | terminate | complete |

`push S` saves the ordinary declaration-order successor of the pushing state
and enters `S`. If the pushing state is the final declared state, the saved
successor is `FlowComplete`. `pop` resumes the most recent saved successor.
`goto S` is a non-returning transfer and does not alter the return stack.
`finish` completes the whole flow at any stack depth.

All new transitions are unconditional state terminators. They are forbidden in
nested `if`, runtime `when`, loops, comptime blocks, and helper functions. A
top-level statement after a transition is an unreachable-statement error.

## Static analysis

The validator resolves state identifiers to declaration-order state IDs and
constructs one explicit terminator for every state:

- `Fallthrough`, with a resolved successor or `FlowComplete`;
- `Push`, with resolved target and resolved return successor;
- `Pop`;
- `Goto`, with resolved target;
- `Finish`.

Push recursion is rejected before stack analysis. Diagnostics include the
cycle path where practical, for example `A -> B -> C -> A`.

Stack analysis explores exact configurations `(state, return-stack shape)`.
It proves that every executed `pop` has a frame, computes deterministic
`MaxStackDepth`, permits shared pop-bearing subflows with different
caller-specific return successors at the same depth, and rejects mixed-depth
state reachability.

Reachability is computed from the first declared state. The analysis follows
fallthrough, push targets, pop return successors, and goto edges; `finish` and
`FlowComplete` terminate exploration. Unreachable states are diagnosed.

## Pushed-region rule

M31a keeps pushed subflows structured. Any state executing with nonzero
flow-stack depth may use only:

- fallthrough;
- `push`;
- `pop`;
- `finish`.

`goto` at nonzero depth is rejected. A pushed path that falls through to
`FlowComplete` without `pop` or `finish` is rejected. A state reached with
different stack depths is rejected as an ambiguous merge.

## Barrier policy

Barrier-bearing states are marked using the existing workgroup barrier builtin
set:

- `WorkgroupBarrier`;
- `WorkgroupMemoryBarrier`;
- `WorkgroupMemoryBarrierWithSync`.

The scan covers ordinary nested statements, not just direct expression
statements. Top-level fallthrough, `push`, `pop`, `goto`, and `finish`
terminators are structurally uniform. Nested transitions are forbidden, so
M31a does not add conditional stack transitions.

A barrier-bearing state cannot be reached through multiple exact stack shapes.
This is intentionally conservative. Divergent data operations inside a state
remain valid; M31a does not attempt a complete divergence or uniformity theorem
prover.

## M31b handoff

The compiler now exposes a resolved `validate.ValidatedFlow` model and mirrors
legacy-flow metadata into VD-MIR as `vdmir.Flow`. The model contains:

- entry state ID;
- declaration-ordered states;
- explicit terminator per state;
- resolved `push` target and return successor;
- resolved `goto` target;
- `FlowComplete` sentinel where needed;
- maximum stack depth;
- `HasPushPop` and `HasGoto`;
- barrier metadata;
- reachability and reachable stack depths;
- exact state, state-name, terminator, and target spans.

M31b can consume this contract without reparsing source or resolving AST state
names. M31a still rejects explicit transitions at the current HLSL lowering
boundary with a clear M31b-not-implemented error.

## Non-goals

M31a does not add computed state targets, first-class state values, cross-flow
push, state parameters, recursive push calls, dynamic stack allocation, stack
inspection, peek/depth operations, exceptions, coroutines, event scheduling,
parallel states, conditional transition syntax, runtime dispatcher generation,
HLSL stack arrays, or stack inlining/flattening optimization.
