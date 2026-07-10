# SDSL-V M23: Flow-Bound Mutable Boards

M23 adds mutable board instances, but only inside bounded shader-local `flow` / `state`.

This keeps the Oct distinction intact:

- `record` remains ordinary immutable structured data;
- `board` is the execution scratch/state surface for flow-owned phase logic.

Reference files used:

- `Language/reference/runtime/21-octomata.md`
- `Language/reference/language/04-control-flow.md`
- `Language/reference/language/11-records.md`
- `Language/reference/language/10-rules.md`

## Syntax

```sdslv
board LoadCoord {
    linear: u32;
    row: u32;
    col: u32;
    valid: bool;
}

flow TileLoad {
    board Load: LoadCoord = LoadCoord {
        linear: 0u;
        row: 0u;
        col: 0u;
        valid: false;
    };

    state Compute {
        Load.linear = 1u;
        Load.row = Load.linear;
        Load.valid = true;
    }
}
```

Current M23 rule: flow-owned board declarations must appear before `state` blocks in the flow body.

## Semantics

- flow-owned board instances are visible to every state in the owning flow;
- the initializer is required;
- the initializer must typecheck as the declared board type;
- board mutation is allowed only as `BoardName.field = expr;` inside state bodies of the owning flow;
- whole-board reassignment is rejected;
- immutable M21 board values remain immutable everywhere, including inside state bodies;
- no persistence exists beyond the current shader invocation.

This is bounded shader-local scratch state, not full Octomata persistence.

## Lowering

M23 continues to lower through VD-MIR.

- flow-owned board instances lower to ordinary local board storage;
- state execution still lowers as source-order sequential blocks;
- HLSL emits ordinary local struct values and field assignments;
- no host ABI, resource binding, or dispatch metadata changes occur.

## Unsupported

M23 still rejects:

- `goto`
- `remember`
- `resume`
- `suspend`
- `when policy`
- persistent Octomata state
- cross-invocation state

Representative diagnostics:

```text
SDSL-V M23 does not support goto transitions
SDSL-V M23 does not support persistent Octomata state
SDSL-V M23 does not support remember/resume/suspend
SDSL-V M23 does not support when policy; hysteresis/min_commit require persistent policy state
```

## Examples

- `examples/SDSL-V/M23/FlowBoardBasic.sdslv`
- `examples/SDSL-V/M23/FlowBoardComptimeFor.sdslv`
- `examples/SDSL-V/M23/FlowBoardGuardedTileLoad.sdslv`

## M24 Follow-Up

M24 is the first real Prometheus SGEMM kernel to rely on M23 flow-bound mutable boards:

- `LoadCoord` is mutated only inside `TileLoad` states;
- `StoreCoord` is mutated only inside `StoreOutput` states;
- no whole-board reassignment is used;
- no board state escapes the current invocation.

The detailed outcome is documented in `internal/prometheus/DevelopmentReport/SDSL_V_M24_FLOW_BOARD_SGEMM.md`.
