# SDSL-V M22: Bounded Shader-Local `flow` / `state`

M22 adds a bounded shader-local subset of Oct `flow` / `state` around the M21 board noun.

This milestone follows Oct reference syntax where practical, but it does not implement full Octomata persistent control. Mutable board state remains deliberately deferred to M23.

Reference files used:

- `Language/reference/runtime/21-octomata.md`
- `Language/reference/language/04-control-flow.md`
- `Language/reference/language/11-records.md`
- `Language/Expressions/UtilityWhen/M80_REPORT.md`

## Syntax

M22 adds function-local phase blocks:

```sdslv
flow TileLoad {
    state Exact {
        when {
            case fullTile -> {
                ...
            }
            else -> {
                ...
            }
        }
    }

    state Tail {
        ...
    }
}
```

`flow` is a statement/block form inside ordinary SDSL-V function or stage bodies. A flow contains one or more `state` blocks, and state names must be unique within that flow.

## Execution Model

M22 uses execution model A: sequential phase blocks.

- states execute once in source order;
- no implicit loop over states exists;
- no cross-invocation or cross-dispatch persistence exists;
- no scheduler-visible machine object is created;
- lowering desugars `flow` / `state` to ordinary structured statement blocks before HLSL emission.

This gives shader code a clean phase-grouping surface without pretending to implement Octomata transitions.

## Supported State Bodies

State bodies validate like ordinary bounded SDSL-V blocks. They may contain:

- `let`
- ordinary assignment
- runtime guard `when`
- guarded `read ... when ... else ...`
- guarded `write ... when ...`
- `if`
- `for`
- `comptime let`
- `comptime if`
- `comptime match`
- `comptime when utility`
- `comptime for`
- helper calls
- `return`

Comptime expansion still happens before VD-MIR lowering, so `comptime for` inside a state body expands normally and does not reach HLSL.

## Boards

M22 allows immutable M21 board values inside `flow` / `state` bodies:

- board declarations
- board literals
- board field access
- helper returns of board values

M22 does not change M21 immutability:

- ordinary board field assignment is rejected;
- whole-board mutation is rejected;
- M23 later adds flow-owned board instances as the only mutable board surface.

## Unsupported Octomata Actions

M22 rejects these forms explicitly:

- `goto`
- `remember`
- `resume`
- `suspend`
- `when policy`
- persistent board mutation
- state-machine loops

Representative diagnostics:

```text
SDSL-V M23 does not support goto transitions
SDSL-V M23 does not support remember/resume/suspend
SDSL-V M23 does not support when policy; hysteresis/min_commit require persistent policy state
board field assignment is not supported in M22; mutable board state is reserved for flow-bound board mutation in M23
```

## Lowering

M22 lowers through VD-MIR by desugaring `flow` / `state` into an ordinary statement block that preserves source order across states. HLSL emission then produces ordinary structured statements and barriers.

Generated HLSL must not contain source-level `flow` or `state` spelling.

## Relationship To Other Milestones

- M19 adds runtime guard `when`.
- M21 adds immutable shader-local board values.
- M22 adds bounded shader-local `flow` / `state` grouping.
- M23 adds flow-bound mutable board state inside flow/state only.

## Examples

- `examples/SDSL-V/M22/FlowStateBasic.sdslv`
- `examples/SDSL-V/M22/FlowStateBoardLoadCoord.sdslv`
- `examples/SDSL-V/M22/FlowStateGuardWhenTileLoad.sdslv`

These examples demonstrate:

- multiple sequential states;
- immutable board values inside state bodies;
- runtime guard `when` inside states;
- guarded read/write inside states;
- comptime expansion inside states;
- semantic boolean operators;
- no mutable board state;
- no `goto` / `remember` / `resume` / `suspend`.

## M24 Follow-Up

M24 uses the bounded sequential execution model in a real Prometheus SGEMM kernel:

- one `flow TileLoad` inside the runtime K-tile loop;
- ordered `LoadLanes`, `SyncAfterLoad`, `Accumulate`, and `SyncBeforeNextTile` states;
- one `flow StoreOutput` for the final writeback phase.

This confirms the M22 execution model is viable for real GPU phase grouping without adding a scheduler illusion. Generated HLSL contains ordinary structured statements only; source-level `flow` / `state` spelling does not survive emission.
