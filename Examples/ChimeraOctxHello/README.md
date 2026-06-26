# ChimeraOctxHello

`Examples/ChimeraOctxHello` is an experimental, opt-in process-boundary interop proof.
A Go final executable starts a Rust sidecar process and talks to it over the existing Octxiliary `octxiliary.v0` stream protocol.

This example is intentionally separate from `Examples/ChimeraHello`:

- `ChimeraHello` proves Go/Rust C ABI interop with a Rust static library linked into the Go executable.
- `ChimeraOctxHello` proves typed DTO interop across a process boundary with Octxiliary framing.

## What crosses the boundary

The stream uses the existing Octxiliary handshake and frame layout:

- `OCTWRAP\0`
- ABI major/minor as little-endian `uint16`
- one or more `uint32` little-endian length-prefixed UTF-8 protocol payloads

The request is an `OctxiliaryRequest` with:

- `family: "ChimeraOctx"`
- `function: "ChimeraHello"`
- one typed `OctxiliaryValue` record argument:
  - `recordType: "ChimeraRequest"`
  - `GoValue: Int`

The response is an `OctxiliaryResponse` whose success value is a typed record:

- `recordType: "ChimeraResponse"`
- `GoValue: Int`
- `RustValue: Int`
- `Total: Int`

The conceptual DTOs look like Oct/Octagon data:

```octagon
ChimeraRequest {
    GoValue: 7
}
```

```octagon
ChimeraResponse {
    GoValue: 7
    RustValue: 35
    Total: 42
}
```

On the wire they remain inside the existing Octxiliary typed-value envelopes. This is not raw JSON, and this example does not make JSON canonical.

## Boundaries

This is not UIBridge, Machina, or a C ABI example. M0 has no C layer and no callbacks.
The Rust sidecar now uses the repository-local H1 Rust Octxiliary SDK at `internal/octxiliary/rust-sdk`. The SDK is still intentionally small and is not a full Octagon parser. It recognizes the current typed Octxiliary value subset needed by the example:

- no arbitrary record parsing;
- no arrays;
- no handles;
- no comments;
- no dimensions;
- no package-qualified record constructors;
- no C layer;
- no callbacks.

The SDK public API deliberately uses PascalCase (`Request.FieldInt`, `Response::OkRecord`, `Dispatcher::New`, `Dispatcher.Handle`, `MainLoop`) to match Oct/Go/C#-style Chimera/Octxiliary conventions rather than Rust snake_case. The Go client uses the repository's existing internal Octxiliary encoder/parser helpers for this in-repo example, and uses direct process spawning for the Rust sidecar.

## Safe default validation

Default validation is pure plan/list/dry-run work only. These commands do not run Cargo, `go build`, the Rust sidecar, or the Go client:

```sh
go run ./cmd/oct test Examples/ChimeraOctxHello/Make.octest --execution interpreted
go run ./cmd/oct test Examples/ChimeraOctxHello/Make.octest --execution compiled

go run ./tools/build_sidecars --out dist/sidecars

OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
go run ./cmd/oct make --file Examples/ChimeraOctxHello/Make.oct --list

OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
go run ./cmd/oct make --file Examples/ChimeraOctxHello/Make.oct --dry-run --trace
```

## Opt-in real run

Real execution is gated by `OCT_CHIMERA_OCTX_HELLO=1`. If the gate is set and `cargo` or `go` is missing, the build fails clearly instead of silently skipping.

```sh
OCT_CHIMERA_OCTX_HELLO=1 \
OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
go run ./cmd/oct make --file Examples/ChimeraOctxHello/Make.oct TestChimeraOctx --trace
```

The expected client output is exactly:

```text
chimera octx hello: go=7 rust=35 total=42
```

## Current platform note

The M0 make plan uses explicit `out/linux-amd64/chimera-octx-hello` and `rust-sidecar/target/release/chimera-octx-sidecar` paths to keep command targets structured with `Program` and `Args`. Future work can add richer platform helpers without changing `oct make` executor behavior.
