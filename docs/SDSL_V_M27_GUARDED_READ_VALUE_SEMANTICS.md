# SDSL-V M27: Guarded-read value semantics

`read target when guard else fallback` is a value expression, represented by `ast.GuardedReadExpr` and `vdmir.GuardedReadExpr`. It is supported only as a direct let initializer, assignment RHS, or return value.

The compiler invariant is:

1. evaluate `fallback` once into a deterministic, collision-safe temporary;
2. evaluate `guard` once; if true, read `target` into that temporary;
3. perform the surrounding destination assignment exactly once, after value materialization.

Consequently a false guard never evaluates the target read, but it still writes the fallback for an assignment RHS. Guarded writes use the separate `GuardedWriteStmt` path and remain conditional stores: false means no store.

The stale-tile defect was solely in HLSL emission. `emitGuardedReadAssign` previously emitted the destination store inside the conditional load block, accidentally giving an assignment-RHS guarded read guarded-write behavior. AST parsing, validation, VD-MIR, flow/derive expansion, and matrix/tile indexing already preserved the distinct `GuardedReadExpr` representation.

M27 emits a temporary-plus-`if`, rather than a ternary, so DXC cannot speculatively evaluate an invalid resource operand. The order is deterministic: fallback, guard, conditional target read, then destination assignment. Temporary names use the emitter's monotonic `__sdslv_guarded_read_N` counter, including after comptime expansion and inside nested blocks.

Regression coverage includes direct workgroup-tile assignment, let, assignment, return, runtime `when`, flow/state, derive, and comptime-expanded contexts. The M20/M24/M26 tail shaders now use direct guarded-read assignments again; their exact paths, barriers, geometry, bindings, metadata, and selector behavior are unchanged.
