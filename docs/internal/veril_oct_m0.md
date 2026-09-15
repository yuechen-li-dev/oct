# Veril-Oct M0: direct combinational SystemVerilog

## Status and architecture

Veril-Oct M0 proves a second direct Oct backend:

```text
Oct source
  -> parser and ordinary typechecker
  -> existing backend-neutral MIRModule
  -> SystemVerilog legality check
  -> SystemVerilog emitter
  -> .sv source artifact
```

The source declaration is `profile Verilog`; the emitted language is
SystemVerilog. The profile is compile-time metadata only. It selects the backend
in compiler orchestration, is rejected by ordinary interpreted execution, and
never enters MIR. Absence of a profile retains the existing Go emission and
native build path.

There is no RTL LIR. M0 consumes structured `MIRValue` nodes directly. In
particular, the emitter never parses `MIRBackendValue.Expression`; the legality
check rejects every `MIRBackendValue`.

## Compilation-unit ownership

Exactly one explicit `profile Verilog` declaration may occur, and it must be in
the entry package. Unprofiled sibling files and imported packages inherit that
single selection. A declaration in a non-entry package cannot switch only part
of the loaded graph. A profile-first standalone file may omit `package` and is
then package `Main`; ordinary unprofiled files still require a package.

## M0 hardware model

Every supported Oct function in the entry package becomes one SystemVerilog
module. The function name becomes the module name, parameters become `input`
ports, and the return value becomes output port `Result`. Names are preserved
when legal and are deterministically sanitized only when SystemVerilog requires
it. Local values are module-local `logic` declarations. Straight-line MIR
assignments and return are emitted in one `always_comb` block.

M0 has no implicit timing model: no clock, reset, latency, registers, pipeline,
or resource schedule is introduced.

Type policy:

- Oct `Bool` -> SystemVerilog `logic`.
- Oct `Int` -> `logic signed [63:0]`, with signed 64-bit two's-complement
  arithmetic. This policy is fixed and host-independent.

Supported structured MIR values are Boolean and integer literals, locals,
unary negation/Boolean not, integer addition/subtraction/multiplication, Boolean logic, and
comparisons. Oct currently has no source-level bitwise integer operators, so M0
does not invent any. The ordinary explicit numeric conversion surface targets
`Float`, which M0 does not support; consequently there is no currently admitted
source conversion. Division and remainder are also deferred because M0 has no
hardware failure channel for preserving Oct's division-by-zero behavior.

## Explicit M0 legality boundary

The separate `CheckSystemVerilogLegal(MIRModule)` phase rejects unsupported
hardware constructs before emission. M0 rejects:

- `String`, `Float`, arrays, vectors, matrices, records, enums, and refinements;
- dynamic allocation and clone-dependent values;
- fallibility, failure terminators, panic/unwrap-dependent paths;
- ordinary and function-value calls, including imported module composition;
- builtins, native/Octxiliary calls, filesystem/network/process operations;
- closures, batch, artifacts, reflection/runtime-only values, and utility when;
- CFG branches, loops, FLOW/Octomata, and every `MIRBackendValue`.

Function calls use M0 strategy A: reject them. Branches are also rejected rather
than assigning clocked FSM meaning to an ordinary MIR CFG. These limits keep the
completed backend purely combinational and leave clear extension points at the
legality checker and emitter.

## Output and validation

`oct build source.oct` emits `source.sv`; building a directory emits
`<directory>/<directory>.sv`. The artifact is deterministic, readable source.
No FPGA or ASIC vendor tool is invoked. Structural golden tests are mandatory;
an available open-source SystemVerilog tool may provide an additional lane.

## Roadmap, not implemented

1. M1: combinational CFG and branch/mux lowering.
2. M2: explicit state and register support.
3. M3: FLOW/Octomata to FSM lowering.
4. M4: memories and bounded arrays.
5. M5: function/module composition.
6. M6: timing and latency annotations.
7. M7: scheduling and pipelining.
8. M8: resource sharing and PPA optimization.

None of these milestones commits Oct to an RTL LIR. A target-specific IR should
be introduced only if concrete lowering pressure demonstrates that MIR plus
bounded backend-local analysis is insufficient.
