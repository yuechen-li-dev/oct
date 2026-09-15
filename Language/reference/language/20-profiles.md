# Compilation profiles

## Verilog M1

`profile Verilog` is a compile-time, compilation-unit declaration. It selects
the SystemVerilog backend and its hardware legality rules; it is not a runtime
value and does not change the meaning of supported Oct expressions. Profiled
hardware sources are compile-only; ordinary interpreted execution rejects them.

The declaration may precede `package`. A standalone profiled file may omit the
package declaration and then belongs to implicit package `Main`:

```oct
profile Verilog

fn Add(A: Int, B: Int) -> Int {
    return A + B
}
```

M1 recognizes exactly the case-sensitive name `Verilog`. An ordinary source
still requires `package` and continues to use the existing Go backend.

One declaration in the entry package is authoritative for the complete loaded
compilation unit. Other files in that package and imported packages omit the
declaration and inherit it. Duplicate declarations are rejected, and a profile
declared only by a non-entry package cannot select or split the backend.

The emitted target is synthesizable SystemVerilog (`.sv`), not legacy
Verilog-2001. M1 admits combinational `Bool`/`Int`, acyclic `if`, immutable
records and `with`, finite payload enums and exhaustive `match`, statically
resolved pure calls, transparent Concept aliases, compile-time `Int<D>` SI
dimensions, and literal-bounded range `for` loops with straight-line bodies
(at most 1024 iterations and an optional positive literal step). It never infers state,
clocks, resets, pipelines, or latency.

Arrays remain runtime-sized in ordinary Oct. Vector and Matrix are distinct
mathematical types rather than array aliases, but their extents are not part of
their current type identity. Arrays, Vector, Matrix, tensor notation, Float,
String, fallibility, FLOW, closures, native calls, recursion, and runtime loops
therefore remain outside the hardware profile.

See `docs/internal/veril_oct_m1.md` for layout, legality, workflow, and validated
toolchain details.
