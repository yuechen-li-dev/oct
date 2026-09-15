# Compilation profiles

## Verilog M2

`profile Verilog` is a compile-time, compilation-unit declaration. It selects
the SystemVerilog backend and its hardware legality rules; it is not a runtime
value and does not change supported Oct semantics. Profiled hardware sources
are compile-only; ordinary interpreted execution rejects them.

The declaration may precede `package`. A standalone profiled file may omit the
package declaration and then belongs to implicit package `Main`:

```oct
profile Verilog

fn Add(A: Int, B: Int) -> Int {
    return A + B
}
```

M2 recognizes exactly the case-sensitive name `Verilog`. An ordinary source
still requires `package` and continues to use the existing Go backend. One
declaration in the entry package is authoritative for the loaded compilation
unit. Duplicate, conflicting, and non-entry-only declarations are rejected.

The emitted target is synthesizable SystemVerilog (`.sv`). M1 combinational
support remains: `Bool`/`Int`, acyclic `if`, immutable records and `with`, finite
payload enums and exhaustive `match`, statically resolved pure calls,
transparent Concepts, compile-time `Int<D>` dimensions, and literal-bounded
straight-line range `for` loops (at most 1024 iterations).

M2 additionally admits Octomata `flow` as the only sequential source model.
Every FLOW module has active-high synchronous `Reset` and rising-edge `Clock`
ports. Source states use stable declaration-order binary encodings; a separate
instruction register preserves continuation after `suspend` and `yield`.
Construction parameters are captured while reset is asserted, turn input uses
a `Turn_<name>` port, board fields are persistent `Board_<Field>`
outputs/registers, and one clock edge corresponds to one `Step` turn.
Combinational dispatch follows ordinary `goto` transitions in the same turn
until `suspend`, `yield`, or `return`.

`yield` pulses `YieldValid` and retains `YieldValue`; `suspend` pulses
`Suspended`; final `return` sets sticky `Done` and stable `Result` until reset.
`remember`/`resume` use one explicit resume-state slot, successful resume clears
it, and empty resume sets sticky `Fault`. Ordered guard `when` retains the first
true case. Controller-bound `when policy` emits persistent selected value,
score, and commitment age and uses the source-order argmax, hysteresis,
minimum-commit, and fallback rules. `StateView`, `InstructionView`, board,
resume, utility-site, yield, and result outputs are the explicit capturable
state view; M2 does not add a restore/load protocol.

Arrays remain runtime-sized containers. Vector and Matrix are distinct
mathematical tensor categories, but their extents are not part of current type
identity. Arrays, Vector, Matrix, tensor notation, Float, String, fallibility,
closures, native calls, recursive within-turn FLOW control, and runtime loops
remain outside the hardware profile. FLOW value expressions currently require
one acyclic ordinary MIR block; statement-level `if`, ordered `when`, and
literal-bounded straight-line `for` remain admitted.

See `docs/internal/veril_oct_m1.md` for the combinational foundation and
`docs/internal/veril_oct_m2.md` for the sequential contract and evidence.
