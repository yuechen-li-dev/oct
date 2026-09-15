# Compiler Optimization by Example

*Building and Optimizing a Real WebAssembly Backend in Go*

This book teaches compiler construction and optimization by following the real Oct compiler and its direct Oct-to-WebAssembly backend. Oct is the working example, not required background.

## Chapters

1. [From Source to Executable WebAssembly](01-from-source-to-wasm.md)
2. [Basic Blocks and Control-Flow Graphs](02-basic-blocks-and-cfgs.md)
3. [Uses, Definitions, and Dataflow](03-uses-definitions-and-dataflow.md)

## Run the examples

From the repository root:

```text
go run ./cmd/oct build Examples/WasmCompute --target wasm
go test ./internal/wasm -run TestCurrentMIRProducesDeterministicExecutableModule -count=1
go test ./internal/build -run TestCompilerOptimizationBook -count=1
```
