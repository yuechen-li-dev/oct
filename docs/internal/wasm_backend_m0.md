# Direct WebAssembly backend M0

## Outcome

Oct now has a bounded direct path:

```text
Oct source -> parser/typechecker -> current structured MIR -> internal/wasm -> .wasm
```

`oct build <path> --target wasm` never generates Go and never invokes `go
build`, `GOOS=js`, `GOARCH=wasm`, TinyGo, a WAT assembler, or a second frontend.
Binary `.wasm` is the canonical artifact; there is no WAT intermediate or dump
in M0.

## Phase 0 inventory

| Component | Purpose and age | Referenced/tested | Input and output | Runtime/host assumption | M0 classification |
| --- | --- | --- | --- | --- | --- |
| `internal/interpret/wasm_module_builder.go` | April 2026 module header, section framing, and ULEB encoder for Machina UI M98 | Used by the UI emitter; unit tested | Explicit section bytes -> binary `.wasm` | None | Usable design evidence; too package-specific to import |
| `internal/interpret/ui_wasm_lowering.go` | April 2026 hard-coded counter/routing UI runtime and direct function/data emission | Used by `EmitMachinaUIWasmArtifact`; Node tests | Machina UI render templates, not MIR -> binary `.wasm` | One-page linear memory, scratch JSON, no imports | Current for its product slice, stale/inapplicable as a compiler backend |
| `internal/interpret/ui_wasm_artifact.go` | File-writing API for the fixed Machina UI module | Referenced by UI/desktop-host tests | Fixed lowering -> named `.wasm` | Custom host boundary exports | Retained unchanged |
| `internal/interpret/ui_wasm_runtime.go` | Go-side model of the earlier M97 UI boundary | Unit tested; not module codegen | UI state/events -> JSON buffer | Custom Machina host ABI | Separate runtime oracle, not reusable compiler infrastructure |
| `docs/MACHINA_UI_WASM_RUNTIME_M97.md` / `MACHINA_UI_WASM_EMISSION_M98.md` | Locked UI ABI and emitter record | Current documentation for that bounded subsystem | Documents binary direct emission | Browser-neutral custom host; Node qualification | Accurate; explicitly not a general backend |
| Browser/desktop hosts under `tools/machina-ui-*` | Instantiate the fixed UI module and render UIIR | Referenced by Machina UI tests/docs | UI-specific module | JavaScript/native desktop host | Out of ordinary compiler scope |
| Compiler/CLI before M0 | Go and Verilog selection only | Broadly tested | Current MIR -> Go or SystemVerilog | Go toolchain for native | No old ordinary WASM hook existed |

Repository history dates the surviving direct emitter to 2026-04-17 (M98d/e)
with follow-up fixes through 2026-05-24. There is no old ordinary MIR-to-WASM
backend, WAT generator, WASI layer, general heap, string ABI, or array ABI to
reconnect. The viable part was the architectural proof that Oct can emit
standards-valid binary modules in-process.

## Integration and module ABI

`build.LoadMIR` is the narrow frontend seam. It performs ordinary package load,
type checking, and `lowerProgram`, returning the exact `MIRModule` consumed by
Go/SystemVerilog. `internal/wasm` imports those current typed MIR definitions;
it rejects `MIRBackendValue` rather than parsing historical Go-shaped strings.

The M0 module has:

- no imports, memory, tables, globals, start function, WASI dependency, or host callbacks;
- one core function per supported MIR function;
- stable type/function/export/code ordering and one deterministic provenance custom section;
- entry-package source functions exported by their Oct names (the bounded M0 convention in the absence of language-level visibility syntax);
- JavaScript `BigInt` at the host boundary for `Int`/`i64`.

There is no stack ABI beyond the WebAssembly operand stack and locals. There is
no heap. Panic/failure is a core `unreachable` trap for `MIRFail`; fallible
function/result representation is rejected before emission. I/O and native
calls are rejected. The host contract for the qualified subset is therefore an
empty import object plus ordinary calls to exported functions.

## Type and value mapping

| Oct/MIR | WebAssembly M0 |
| --- | --- |
| `Bool` | canonical `i32` 0/1 |
| `Int` | signed `i64`; preserves the frontend/runtime's parsed 64-bit integer domain |
| `Float` | IEEE-754 `f64` |
| `Void` | no result |
| payload-free enum | deterministic zero-based `i32` tag in declaration order |
| local/parameter | mutable WebAssembly local of mapped type |
| `MIRLiteral`, `MIRLocal`, `MIRUnary`, `MIRBinary`, scalar `MIRConvert` | direct numeric/local instructions |
| `MIREnumValue`, `enum-is` | tag constant / `i32.eq` |
| `MIRClone` of scalar | identity |

Integer addition, subtraction, and multiplication use signed core values;
comparisons use signed instructions. Division and modulo arrive as semantic
MIR intrinsics and remain fail-closed until their edge cases are qualified.
Float arithmetic and comparisons use `f64` instructions. Int/Float conversion uses signed
`i64`/`f64` conversion instructions. Advanced conversion corner-case parity is
outside M0 and should be qualified before expanding the accepted corpus.

## CFG and function lowering

MIR remains mutable-local CFG; no SSA is introduced. Each function receives an
internal `i32` program-counter local. An outer `block`/`loop` is the structured
WebAssembly control shell. Each MIR block is a deterministic guarded case:

```text
if pc == block-index:
    emit statements
    return, or set pc from jump/branch and br to dispatch loop
```

`MIRBranch` uses core `select` to choose the next block index, then branches to
the dispatch loop. This is the smallest principled arbitrary-CFG adaptation
and mirrors the established Go backend's program-counter dispatch rather than
attempting a new Relooper. Ordinary statically resolved `MIRCall` maps to a
direct function index with preserved parameter order and single return value.

## MIR support matrix

| MIR construct | M0 status | Mapping / reason |
| --- | --- | --- |
| `MIRFunction`, `MIRBlock` | Supported | core function plus dispatch-loop CFG |
| `MIRAssign` | Supported for scalar/enum values | `local.set` |
| `MIRCall` | Supported for pure static calls | direct `call`; builtins and indirect calls rejected |
| `MIRReturn` | Supported | `return` |
| `MIRJump`, `MIRBranch` | Supported | set `pc`, `br` dispatch; branch uses `select` |
| `MIRFail` | Supported as bounded policy | `unreachable` trap |
| `MIRConstructRecord` | Straightforward but not implemented | needs an explicit deterministic aggregate ABI |
| `MIRConstructArray`, `MIRIndexAssign`, `MIRRowAssign`, `MIRIndex` | Needs runtime support | linear memory, allocation, length, bounds and copy semantics |
| `MIRGenericOctxiliaryCall`, native/builtin calls | Unsupported for M0 | needs explicit import/runtime ABI |
| `MIRDestructureCall`, function values/captures | Needs WASM-specific lowering | multi-value/indirect-call environment design |
| `MIRBatchMap`, FLOW MIR | Unsupported for M0 | separate execution/runtime contracts |
| strings, bytes, records, payload enums, results | Needs runtime support | no heap/object/string/result ABI exists |
| `MIRBackendValue` | Hard rejected | direct backend accepts structured MIR only |

Records and arrays were not forced into M0: the old UI emitter's scratch JSON
memory is product-specific and is not a valid ordinary-Oct heap or value ABI.

## Proof and determinism

`Examples/WasmCompute` is the canonical specimen. It covers `Add`, signed
integer arithmetic, `SumTo`, Boolean logic, a payload-free enum/match
(`ModeCode`), `FloatKernel`, static calls, and `Main`.
`internal/wasm/wasm_test.go` compiles that source through current MIR twice,
requires byte identity, validates/instantiates it with Node's standards
WebAssembly runtime, and calls every representative export. A separate backend
fixture proves unsupported `String` fails before a malformed artifact can be emitted.

The same example's `Main` result is suitable for three-lane comparison:

```text
interpreter -> 104
Go backend  -> 104
WASM/Node   -> 104
```

Stable module bytes follow from sorted frontend MIR declarations, first-use
type interning, sorted exports, fixed section order, canonical LEB encodings,
and absence of timestamps/random identifiers.

## M1 recommendations

1. Add a deliberate flattened or linear-memory record ABI, beginning with immutable scalar records and field access.
2. Define allocator/length/bounds/value-copy rules before enabling arrays or strings; do not borrow the Machina scratch-buffer ABI.
3. Add an explicit source visibility/export concept before widening beyond the M0 entry-package export convention.
4. Qualify fallible results and imported host calls with a versioned `oct.*` ABI; add WASI only when an actual capability requires it.
5. Improve CFG structure only if measurements justify it; the dispatch loop is correct, deterministic, and handles current arbitrary mutable-local MIR.
