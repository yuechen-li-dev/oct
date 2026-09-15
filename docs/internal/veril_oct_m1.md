# Veril-Oct M1: structured combinational SystemVerilog

## Executive verdict

**Accepted, with fixed-shape collections explicitly deferred.** `profile Verilog`
is now a practical structured combinational subset rather than a straight-line
expression emitter. Ordinary Oct `if`, exhaustive enum `match`, payload enums,
immutable records, `with`, pure calls, integer SI dimensions, transparent
Concept aliases, and statically bounded range loops reach synthesizable RTL.

The architecture remains:

```text
Oct source
  -> parser / AST
  -> ordinary typechecker
  -> lowerProgram
  -> backend-neutral MIRModule
  -> CheckSystemVerilogLegal
  -> direct SystemVerilog emitter
```

No RTL LIR is needed. M1 adds no MIR node: the existing CFG, aggregate, enum,
field-access, intrinsic, and call nodes carry the required semantics. The
backend performs bounded layout, call-graph, CFG, and static-loop analysis over
those nodes. It never reparses source or `MIRBackendValue` text.

## Admitted semantics

| Construct | M1 status | RTL shape |
|---|---:|---|
| `Bool`, `Int` | Yes | `logic`, signed 64-bit `logic` |
| arithmetic/comparison | Yes | explicitly sized operands and locals |
| `if` / `else` | Yes | acyclic combinational CFG activation and procedural choice |
| immutable Record / nested Record | Yes | deterministic packed vector |
| `with` | Yes | complete repacking; no mutation or retained state |
| Enum / payload Enum | Yes | tag plus zero-filled maximum payload storage |
| exhaustive `match` | Yes | deterministic tag comparisons and payload slices |
| pure calls | Yes | statically resolved `function automatic` helpers |
| Concept | Compile-time | transparent aliases erase; runtime refinements reject |
| `Int<D>` SI dimensions | Compile-time | dimension erases, stored integer payload remains |
| constant range `for` | Yes | synthesizable `for`; maximum 1024 iterations |
| fixed array | Deferred | ordinary `T[]` has runtime extent |
| Vector / Matrix | Deferred | distinct mathematical types, but extent is not in type identity |
| Tensor notation | Deferred | separate indexed mathematical surface, not an array alias |
| runtime loop / `while` | No | would require unbounded/cyclic combinational feedback |
| recursion | No | finite call graph required |
| `Float`, String, fallibility, FLOW, closure, native, allocation | No | no admitted M1 hardware semantics |
| clocked state, reset, register, pipeline | No | never inferred |
| Template | Incidental only | pre-MIR specialization may work; not an M1 promise |

Static `for` admission is deliberately narrow: bounds and positive stride must
be integer literals after ordinary lowering, and the loop body must remain one
straight-line MIR block. Runtime bounds and other cycles fail at the legality
gate with a hardware-specific diagnostic.

## Deterministic layouts

Records use declaration order. The first field occupies the least-significant
bits, subsequent fields follow toward the most-significant end, and nesting
recursively applies the same rule. `Bool` is one bit; `Int` and `Int<D>` are
signed 64-bit values. Aggregate ports are plain packed vectors for conservative
tool compatibility. Field extraction first lands in an explicitly typed local,
preventing an unsigned part-select from becoming arithmetic authority.

Enums use the smallest stable tag width, with a minimum of one bit. Variants are
numbered in declaration order. The tag occupies the least-significant bits. The
payload region follows it and has the width of the largest variant payload;
smaller and absent payloads are zero-extended. Payload signedness is restored by
the typed local receiving the slice. Unused tag encodings select the default
zero result already assigned by `always_comb`; they cannot arise from a valid
Oct enum value.

## Units and integer semantics

Oct's current unit suffixes carry a dimension, not an implicit display-unit
conversion. An `Int<m>` literal therefore reaches RTL as its stored signed
integer payload, and the dimension is erased only after ordinary typechecking.
Scale factors are not invented in the backend. `Float<D>` remains illegal, so
M1 emits no floating-point or hidden fixed-point hardware. Fractional SI values
are consequently outside M1.

All integer expressions remain signed 64-bit two's-complement. Sized literals,
typed signed locals, and typed payload extraction prevent SystemVerilog's
implicit unsigned promotion from owning Oct arithmetic. Addition,
subtraction, multiplication, and negation therefore wrap at 64 bits as the M0
policy requires.

## Calls and combinational completeness

The legality pass resolves every ordinary call and rejects builtins, native
calls, function values, fallibility, and direct or mutual recursion with a call
path. Reachable helpers are emitted in deterministic dependency order as local
`function automatic` declarations. Entry-package functions remain modules;
imported library functions become helpers rather than extra modules.

Each `always_comb` and automatic function assigns its result, locals, and block
activity defaults before evaluating CFG blocks. This makes assignment complete
by construction and prevents latch inference. The ordinary typechecker still
rejects a non-`Void` function with a missing return before hardware legality.

## Octest and one source of semantics

Octest remains the reference runtime. SystemVerilog is only an alternate build
artifact. The flagship demonstrates the intended split:

- `Language/Profiles/VerilogM1/flagship/Hardware/hardware.oct` is an ordinary,
  unprofiled library containing the algorithm.
- `Hardware/semantic.octest` executes that exact library through compiled
  Go/reference semantics.
- `Main/profile.oct` is the entry-package hardware selection and thin exported
  module boundary; it imports the same library.
- RTL simulation consumes deterministic vectors only as backend validation.

There is no Verilog test runtime and no handwritten Verilog implementation.

## Flagship evidence

The motor-command selector combines immutable records, a payload enum,
exhaustive `match`, conditions, pure cross-package calls, `with`, SI-dimensioned
integers, Boolean short-circuiting, and a bounded reduction. Current measured
size is 49 lines for the ordinary Oct library and 313 lines for generated RTL.
The checked-in `.golden.sv` is copied directly from compiler output.

Local qualification on 2026-09-14:

- Octest compiled reference semantics: 2 passed, 0 failed, no fallback.
- Icarus Verilog 13.0, `-g2012`: parsed/elaborated M0, M1 corpus, and flagship.
- Icarus RTL vectors: Hold, Offset, and Scale all matched Octest expectations.
- Yosys 0.66: `read_verilog -sv; hierarchy -check; proc; opt; check` reported
  zero problems. Statistics contain combinational arithmetic and mux cells and
  no latch or flip-flop cells.

## Deferred collection boundary

M1 explicitly investigated all three existing collection/mathematical surfaces:

- Arrays (`T[]`) are general runtime-sized containers.
- `Vector<T>` and `Matrix<T>` are separate mathematical value categories, not
  array aliases, but current type identities still do not carry their extents.
- Tensor notation is an index-aware expression surface over mathematical values;
  it is not a storage type that supplies a fixed hardware layout.

Literal-local extent inference alone would not make module ports or helper ABIs
statically shaped, and backend-only extent types would duplicate Oct typing.
Accordingly M1 rejects these values with a direct diagnostic. A future language
milestone can admit a canonical statically shaped type, after which each surface
can receive its own hardware policy rather than being conflated with arrays.

## Still rejected

Runtime loops, `while`, recursion, dynamic allocation, array/vector/matrix/tensor
values, String, Float, division/remainder with failure semantics, fallibility,
FLOW, closures, function values, utility decisions, batch, reflection,
Octxiliary/native calls, and backend-shaped MIR values remain illegal. M1 does
not infer clocks, registers, reset, pipelines, latency, scheduling, FSMs, or
memories.
