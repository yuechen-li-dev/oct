# SDSL-V graphics reconciliation

## Purpose

The canonical SDSL-V specification and conformance corpus now unify GoOct’s
modern compute language with the bounded graphics ideas developed historically
in Wyrmcoil and Aurelian. GoOct is the reference implementation. Wyrmcoil and
Aurelian remain independent, unchanged compiler stacks; a future reconciliation
targets source/manifest behavior, not GoOct internals.

The detailed machine-readable decision ledger is
`docs/SDSL_V_GRAPHICS_RECONCILIATION.json`.

## Canonical migrations

| Historical surface | Canonical SDSL-V | Migration meaning |
|---|---|---|
| graphics parser/emitter defines behavior | specification plus conformance corpus | implementation disagreement is a compiler defect |
| `interface`, `implements`, `override` | `concept`, `config`, `template`, `compile` | removed; `SDSL-V4101` identifies old declarations |
| fallible return, postfix `?`/`!`, `ok`/`err`, `error` | payload enum/status/resource | removed deliberately; `SDSL-V4100` identifies old syntax |
| mutable or uninitialized `let` | initialized immutable `let`; initialized mutable `var` | explicit binding intent and definite initialization |
| integer-only enum | unit or payload enum plus exhaustive `match` | unit cases remain directly expressible; payload cases are canonical |
| separate stage/resource/builtin constructs | one `stream` with validated role | role is explicit in semantic model and VD-MIR |
| graphics coordinate alias | `type T = Base @space(name)` | statically distinct, zero-cost, no implicit transformations; the shared nominal mechanism also admits dotted non-graphics semantic vector spaces |
| engine material parameter keys | `material` sugar over record + readonly uniform | deterministic binding/layout, no runtime reflection magic |
| raw graphics texture objects | `texture2d<T>`, `sampler`, `Sample` | backend-neutral typed resource surface |
| generic shader interface hierarchy | concept-constrained template materialization | only concrete `compile` results emit entry points |

## Stream and interface reconciliation

Canonical streams are compiler-owned typed boundaries with exactly one role:
stage-value, resource, or builtin. Vertex inputs, varyings, and pixel outputs use
locations/targets; textures, samplers, uniforms, and storage buffers use
bindings; stage invocation data uses backend-neutral builtins. Mixed roles are
invalid. Existing compute stream names and binding behavior are unchanged.

Vertex/pixel linkage is checked before backend emission. The paired contract
requires matching location, type (including coordinate space), and
interpolation. Exactly one vertex clip-position output maps to the position
builtin. The initial builtin set is vertex/instance ID and pixel
position/front-face, in addition to the unchanged compute set.

## Materials and resources

Historical material authoring is retained only as typed sugar. A material block
creates an immutable record-shaped constant buffer with deterministic offsets,
size, alignment, and binding in the graphics bundle. Textures and samplers are
explicit resources; they do not hide in an engine key registry. Sampling uses
the closed `Sample(texture, sampler, float2)` operation.

## Static abstraction

Concepts and configs express closed requirements/facts. Templates reuse shader
program structure. `compile Template<Config> as Program` validates constraints,
monomorphizes the program, and establishes deterministic entry names. There is
no parallel interface generic system and no entry point for an unmaterialized
template.

Comptime remains bounded immutable planning. It does not grow into arbitrary
I/O, reflection, mutable compile-time state, or a hidden runtime.

## Independent compiler contract

A future Wyrmcoil or Aurelian adapter should consume
`examples/SDSL-V/conformance/manifest.json` and proceed through the six tiers:

1. acceptance/rejection parity;
2. semantic manifest parity;
3. diagnostic code and span parity;
4. normalized structural SPIR-V parity;
5. runtime behavior parity when a runtime exists;
6. exact byte parity only for explicitly named golden artifacts.

It need not use GoOct’s AST, validator organization, VD-MIR, HLSL formatting, or
compiler implementation. Universal byte-identical SPIR-V is not required.

## Scope boundary

This reconciliation adds no graphics runtime, window, render pass, swapchain,
pipeline-state DSL, triangle example, or engine binding. It adds no graphics
stage beyond vertex and pixel, and no alternate backend. Wyrmcoil and Aurelian
source trees were not modified.
