# Compilation profiles

## Verilog M0

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

M0 recognizes exactly the case-sensitive name `Verilog`. An ordinary source
still requires `package` and continues to use the existing Go backend.

One declaration in the entry package is authoritative for the complete loaded
compilation unit. Other files in that package and imported packages omit the
declaration and inherit it. Duplicate declarations are rejected, and a profile
declared only by a non-entry package cannot select or split the backend.

The emitted target is synthesizable SystemVerilog (`.sv`), not legacy
Verilog-2001. See `docs/internal/veril_oct_m0.md` for the bounded backend subset.
