# Compiler Optimization by Example

*Building and Optimizing a Real WebAssembly Backend in Go*

This book teaches compiler construction and optimization by following the real Oct compiler and its direct Oct-to-WebAssembly backend. Oct is the working example, not required background.

## Chapters

1. [From Source to Executable WebAssembly](01-from-source-to-wasm.md)
2. Basic Blocks and Control-Flow Graphs *(planned)*

## Run the examples

From the repository root:

```text
go run ./cmd/oct build Examples/WasmCompute --target wasm
go test ./internal/wasm -run TestCurrentMIRProducesDeterministicExecutableModule -count=1
go test ./internal/build -run TestCompilerOptimizationBookMIRSnapshots -count=1
```
