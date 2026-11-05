# Interoperability and specialized tooling

[Reference home](../README.md) · [Support boundaries](../support.md)

Start with ordinary Oct functions and libraries. Use one of these explicit
boundaries when an existing host, native library, or build tool owns work that
Oct should call or describe.

## Go companion contracts

OctGo allows an existing Go package to have a typed `*.contracts.oct` companion
and Octest contracts. A companion can declare a selected exported free function
with bodyless `go fn` syntax:

```oct
go fn StrictlyAbove(value: Int, threshold: Int) -> Bool
```

This is a declaration fragment in that companion file. It is not an ordinary
Oct implementation or a general FFI. The host checks the real exported Go
function and its admitted exact signature using Go type information. The
name remains the same in Go and Oct. Imported calls are compiled-only and are
not allowed in compile-time `Require` evaluation.

`oct check <go-package-directory>` checks the companion and any committed
bridge without running Octest. `--generate` explicitly writes the deterministic
bridge. The [OctGo contract and limits](../../../docs/experiments/OCTGO_M1.md)
and [working scalar dogfood](../../../docs/experiments/OCTGO_M2.md) describe the
admitted signatures, host-controlled generation, and real call path.

## Compiled Go source and FLOW hosts

The experimental `internal/build.EmitGoSource` seam performs ordinary load,
check, lower, and Go emission without building an executable. Its output can
live in an ordinary Go package. This internal API is distinct from `go fn`:
one embeds Oct computation in Go; the other declares a validated Go operation
for an Oct companion.

Generated FLOW facades expose typed construction, stepping, and the admitted
checkpoint/restore boundary. See [turns and checkpoints](../runtime/turns-and-checkpoints.md)
and the [source-emission experiment](../../../docs/experiments/OCT_COMPILED_GO_EMIT_M0.md).
Generated symbols and encodings are not a stable ABI. A known source-only
floating-literal helper omission is recorded in [FEEDBACK](../../../FEEDBACK.md);
it is not a missing feature of the ordinary executable compiler.

## Native wrappers and sidecars

A package wrapper declares a native function once through its manifest.
Wrapper source, transports, native build planning, and runtime discovery are
specified in [package tooling](packages-and-dependencies.md). Package sync does not silently
build native code. Native build permission and runtime sidecar discovery are
explicit.

First-party standard-library builtins using Octxiliary are a different
implementation boundary: ordinary Oct library function bodies still run in
both lanes. Their compiled builtins use the documented sidecars. See
[library execution](../language/standard-libraries.md#backend-support-builtins-implementation-detail).

## UI hosts

Machina UI authoring uses `Libraries/UI` values and functions. It is
experimental, not a prerequisite for ordinary Oct. The detailed
[UIBridge protocol](uibridge-protocol.md) owns envelopes, DTOs, units,
transport, and error fields; the [library guide](../language/standard-libraries.md#ui-machina-ui-authoring-surface)
owns the author-facing entry point. A UI value is not a browser object.

## Build and source-generation tools

`oct make` and `Make.oct` coordinate explicit build plans. The special
`[MakePlan]`, `[Pure]`, `[NoWhile]`, and `[RequiresAuthority]` markers have the
bounded rules in the [CLI reference](cli.md#makeoct-attributes). They are
not a general attribute or language-wide effect system. Current scope and
limits are in the [Make guide](../../../Libraries/Make/).

OctGen evaluates checked Oct models and lets a host validate and render
committed output in an explicit generation/check step. It is not an automatic
phase of `go build`, a token macro system, or runtime reflection. See
[OctGen](../../../docs/experiments/OCTGEN_M1.md) for the supported staging seam.

[Atlas](atlas.md) is opt-in semantic documentation data linked by ordinary
typed values. [Oct-XML](../language/typed-markup.md) is typed call syntax, not a
separate host or document runtime.
