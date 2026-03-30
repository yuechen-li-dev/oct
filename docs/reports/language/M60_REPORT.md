# M60 Report — MIR Skeleton + Typed Lowering + Tiny Compiled Subset

M60 introduces the first honest compiled bring-up path for Oct.

## Added in M60

- Concrete MIR data model in `internal/build`:
  - module, records, enums, functions
  - blocks, statements, terminators
  - explicit locals and temporaries
- Typed lowering from checked Oct AST into MIR for a tiny subset.
- MIR inspection path via `OCT_MIR_DUMP=1` during `oct build`.
- First Go backend emission from MIR with `go build` artifact generation.

## Compiled subset supported now

Compiled mode currently supports:

- plain functions
- integer/float/bool/string literals
- arithmetic/comparison/logical operations
- `if` / `else` (statement form)
- locals and assignment (`let`, `var`, `=`)
- arrays (`[]` literals, indexing, index assignment)
- records (literals and field access)
- enums (value references)
- ordinary function calls (same package and imported packages)
- explicit returns
- package imports and qualified calls
- direct lowering for builtins: `Len`, `Append`, `Print`

## MIR currently supports

- literal value usage
- assignments and temporary values
- call statements
- record and array construction statements
- branch/jump/return terminators

## Unsupported compiled features (intentional for M60)

Compiled mode currently fails clearly on:

- `batch`
- Octomata features (`flow`, `state`, `goto`, `suspend`, `remember`, `resume`, `when`)
- `switch` expressions
- fallible propagation/unwrap (`?`, `!` paths)
- `.octagon` runtime paths
- advanced benchmark/artifact specialized compilation behavior

Diagnostic shape is explicit, e.g.:

- `compiled mode does not yet support batch`

## Why this is intentionally narrow

M60 is bring-up, not full-language compilation. The implementation favors:

- clear phase boundaries
- deterministic and inspectable MIR
- deterministic and readable generated Go
- honest failure for unsupported features

## Intentionally deferred after M60

- optimizer passes
- SSA
- batch lowering
- Octomata lowering/runtime integration in compiled mode
- broad builtin/runtime parity
- wasm backend polish
