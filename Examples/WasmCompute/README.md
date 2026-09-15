# WasmCompute

This example exercises Oct's bounded direct WebAssembly backend: integer and
floating-point arithmetic, mutable-local CFG loops, ordinary function calls,
conditionals, a branch/join reaching-definitions specimen, and a payload-free
enum match.

```text
go run ./cmd/oct build Examples/WasmCompute --target wasm
```

The result is `Examples/WasmCompute/WasmCompute.wasm`. It has no imports and
can be instantiated by any core WebAssembly host. JavaScript hosts pass and
receive Oct `Int` values as `BigInt` because the ABI maps them to `i64`.
