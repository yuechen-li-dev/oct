# Builds, targets, and profiles

[Reference home](../README.md) · [CLI](cli.md) · [Support boundaries](../support.md)

## Native software

`oct build App/main.oct` checks and lowers ordinary Oct, emits Go, and invokes
the Go toolchain. The output is a standalone native executable. On Windows,
`Main.oct` produces `Main.oct.exe`; on Linux and macOS it has no suffix.
`.octbin` is reserved and is not the current native output format.

`oct run` interprets source. To verify the native path use a built executable
or `oct test ... --execution compiled`; a source run is not native evidence.
Wrapper-backed operations still need their runtime sidecars.

`--opt` requests Oct's opt-in scalar constant pipeline for native or WASM
builds. Omitting it skips that Oct pass; the native Go compiler still performs
its usual optimization. Ordinary native control-flow emission and scalar-loop
dispatch elimination do not require `--opt`.

## Direct WebAssembly

```text
oct build Examples/WasmCompute --target wasm
oct build Examples/WasmCompute --target wasm --opt
```

The direct backend emits a deterministic core `.wasm` module from current MIR.
It does not invoke Go, TinyGo, WAT tooling, or WASI. The bounded subset covers
scalar `Bool`, `Int`/`i64`, `Float`/`f64`, `Void`, tag-only enums, ordinary static
calls, and the admitted scalar control flow/operators. Entry-package functions
are exports; JavaScript receives `Int` values as `BigInt`.

The current module has no imports, linear memory, heap, host I/O, or general
container ABI. Strings, bytes, records, arrays, payload enums, `Option`,
fallible results, indirect calls/captures, batch, and FLOW are outside this
subset. Unsupported operators and MIR forms are rejected explicitly;
software support elsewhere does not imply WASM support. See the
[backend contract](../../../docs/internal/wasm_backend_m0.md) and
[runnable example](../../../Examples/WasmCompute/).

## Hardware profile

`profile Verilog` is a compile-time source declaration selecting hardware
legality and SystemVerilog output. It is distinct from the `--target wasm`
software build option. Start with the [profile reference](../language/compilation-profiles.md)
for the exact combinational and sequential FLOW subset, clock/reset behavior,
and compile-only restrictions. Neither runtime containers nor floating-point
software become hardware simply by adding the declaration.

## Inspectability

Generated Go, MIR, specialization names, and internal build directories are
compiler diagnostics and experimental integration seams. They are useful for
investigation but are not a public source or binary compatibility promise.
See [interoperability](interoperability.md) for the explicit source-only seam
and [the compiler guide](../../../docs/internal/compiler_architecture.md) for
implementation ownership.
