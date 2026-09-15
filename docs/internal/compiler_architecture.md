# Compiled backend organization

The ordinary compiled path remains:

```text
project.Load
    -> typecheck.CheckProgram
    -> lowerProgram
    -> MIRModule
    -> emitGo
    -> go build
```

The `internal/build` package keeps this pipeline deliberately direct. Its file
boundaries are:

- `compiler.go`: public/internal compile entry points, phase orchestration, and
  native test-harness orchestration.
- `mir.go`: ordinary mutable-local CFG MIR and supporting module declarations.
- `mir_flow.go`: FLOW state-machine control MIR. Ordinary FLOW value
  computation continues to use `MIRFlowSharedExpr` and ordinary MIR.
- `lower.go`: module/function/statement lowering orchestration and shared
  lowering context.
- `lower_expr.go`: ordinary expression and value lowering.
- `lower_flow.go`: FLOW control, persistence, and shared-expression adaptation.
- `mir_dump.go`: deterministic textual MIR inspection.
- `emit_go.go`: MIR-to-Go translation and Go-specific naming/type helpers.
- `emit_go_flow.go`: FLOW-specific Go emission.
- `emit_go_runtime.go`: generated Go runtime/support source templates.
- `artifact.go`: artifact naming and target concerns.

A future backend should start from `MIRModule` and live beside `emit_go.go`; it
should not enter compiler orchestration or FLOW lowering to obtain the MIR.
There is intentionally no backend registry or plugin abstraction yet.

## Known backend-neutrality debt

Ordinary MIR values, assignment targets, call arguments, and terminator values
are still strings. Some of those strings contain Go-shaped expression syntax
created during lowering. This representation is preserved for compatibility in
the current cleanup, including the exact MIR dump and generated Go behavior.

Before adding a SystemVerilog backend, introduce a bounded typed MIR
value/expression representation at that seam. Do not make a new control-flow IR
or convert the current mutable-local CFG to SSA merely to remove the strings.
