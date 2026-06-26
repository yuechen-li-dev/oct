# Chimera Octxiliary Rust SDK M1 API design

## Scope

This document defines the Rust SDK API shape for Rust sidecars that speak the
existing Octxiliary typed DTO frame protocol. It is the process-boundary sibling
of the C ABI Chimera Rust SDK described in
`docs/internal/chimera_rust_sdk_design_m1.md`.

The task-supplied recommendation file was referenced from the user's IDE rather
than present as a repository path in this checkout, so this synthesis evaluates
the supplied recommendation text and the current repository implementation.

This is not UIBridge, Machina, a JSON transport, a full procedural macro SDK,
or a change to Make execution semantics.

## Recommendation synthesis

### Belongs in Rust Octxiliary SDK M1

- PascalCase public Rust APIs that match Oct, Go, and C# naming conventions.
- Record field extraction helpers for required and optional fields.
- Response and field builder helpers for the existing typed transport values.
- Closure-based handler registration so handlers can capture sidecar state.
- A fallible handler shape, where handlers return `Result<Response, OctxError>`
  and dispatch converts errors into protocol error responses.
- A single-family `Dispatcher` plus a `CompositeDispatcher` for multi-family
  sidecars.
- A `MainLoop` convenience that owns the standard-input/standard-output
  Octxiliary sidecar loop.
- Protocol metadata constants for the existing handshake name and ABI version.
- Handler panic containment around dispatch, provided it does not complicate the
  public handler trait bounds.

These pieces remove the hand-rolled business/protocol boilerplate in
`Examples/ChimeraOctxHello/rust-sidecar` without changing the wire protocol.

### Belongs in Octxiliary compiler/runtime follow-up

`OCTX-RECORD-RETURN` should allow wrapper manifests to declare arbitrary Record
return types and teach compiled Oct wrapper lowering to reconstruct declared
record values from Octxiliary Record responses.

The protocol already carries Record values in requests and responses, and the
Rust SDK can encode and decode those values. The blocker is compiled Oct wrapper
manifest/type lowering, not the sidecar protocol.

### Belongs in Go client follow-up

`OCTX-GO-CLIENT` should add a convenient `pkg/octxiliary` client for tests and
examples that need spawn/handshake/request/response orchestration. The current
Go package has good sidecar-serving and value-construction helpers, but no
small client wrapper for process-spawned sidecar tests. That gap should remain
separate from the Rust SDK.

### Belongs in docs only

- The C ABI vs Octxiliary rule of thumb.
- The naming convention rationale.
- The fact that Octxiliary is not UIBridge/Machina and must not adopt JSON as
  its canonical transport.
- The absence of async runtime, thread-local state APIs, and procedural macros
  in M1.

## Naming convention

Rust Octxiliary SDK public APIs use PascalCase to match Oct, Go, and C# naming
conventions. Rust snake_case is not used for public Chimera/Octxiliary SDK APIs.

The crate deliberately uses:

```rust
#![allow(non_snake_case)]
```

This is Chimera/Octxiliary API policy, not an accidental Rust style lapse.
Rust crate names, file names, Cargo metadata, private helper functions, and
local implementation variables may remain Rust-compatible.

Representative public shape:

```rust
let GoValue = Request.FieldInt("GoValue")?;
let Name = Request.FieldString("Name")?;
let Response = Response::OkRecord("ChimeraResponse", vec![
    Field::Int("GoValue", GoValue),
    Field::Int("RustValue", 35),
    Field::Int("Total", GoValue + 35),
]);
let Dispatcher = Dispatcher::New("ChimeraOctx")
    .Handle("ChimeraHello", move |Request| Ok(Response))?;
MainLoop(Dispatcher)?;
```

Because the sidecar code was still early/internal and hand-rolled, M1 chooses a
clean PascalCase API without compatibility aliases. Compatibility aliases can be
added later only if an external SDK consumer appears before the API stabilizes.

## SDK crate home

M1 uses:

```text
internal/octxiliary/rust-sdk
```

This path emphasizes that the crate is a Rust SDK for the generic Octxiliary
protocol rather than a one-off Chimera example helper. Chimera remains the
interop story and naming family, but the sidecar transport is Octxiliary and can
serve non-Chimera wrappers too.

`internal/chimera/octx-rust-sdk` would make the Chimera family relationship more
obvious, but it would incorrectly imply the transport helpers are Chimera-only.
`internal/octxiliary/rust-sdk` is therefore the clearer ownership boundary.

## Request and Record field extraction

M1 provides helpers on both `Request` and `RecordRef`:

```rust
Request.FieldInt("GoValue")?;
Request.FieldFloat("Scale")?;
Request.FieldString("Name")?;
Request.FieldBool("Enabled")?;
Request.FieldRecord("Nested")?;

Request.OptionalInt("Count")?;
Request.OptionalFloat("Scale")?;
Request.OptionalString("Name")?;
Request.OptionalBool("Enabled")?;
```

The `Request` helpers are intentionally ergonomic for the common Octxiliary DTO
shape: the first argument is a typed Record. `RecordRef` helpers support nested
records and future lower-level code.

M1 decisions:

- Required helpers return `Result<T, OctxError>`.
- Optional helpers return `Result<Option<T>, OctxError>`.
- `FieldInt` returns `i64`, matching the Rust SDK protocol representation.
- `FieldFloat` returns `f64`.
- `FieldString` returns owned `String` for simple closure ergonomics in M1.
- Wrong-type errors include the field name, expected kind, and actual kind.
- Missing required fields are handler errors that dispatch converts to protocol
  error responses.

Borrowed string helpers and smaller integer conversion helpers can be added
later if real consumers need them.

## Fallible closure handlers

M1 handlers use:

```rust
Box<dyn Fn(&Request) -> Result<Response, OctxError> + Send + Sync>
```

This makes field helpers ergonomic:

```rust
Dispatcher::New("ChimeraOctx").Handle("ChimeraHello", |Request| {
    let GoValue = Request.FieldInt("GoValue")?;
    Ok(Response::OkRecord("ChimeraResponse", vec![
        Field::Int("GoValue", GoValue),
        Field::Int("RustValue", 35),
        Field::Int("Total", GoValue + 35),
    ]))
})?;
```

The dispatcher converts `Err(OctxError)` to `Response::Err`, keeping business
handlers free of repetitive error-response construction.

## Closure-based dispatcher and state injection

`Dispatcher` stores closures instead of bare function pointers. This allows
stateful sidecars to capture state explicitly:

```rust
let State = Arc::new(Mutex::new(MyState::new()));
let Dispatcher = Dispatcher::New("Admin").Handle("Create", {
    let State = State.clone();
    move |Request| {
        let mut State = State.lock().unwrap();
        // use State
        Ok(Response::OkVoid())
    }
})?;
```

M1 decisions:

- Handlers are `Send + Sync` so dispatchers can be moved into future serving
  arrangements without changing the public API.
- Duplicate function registration returns an immediate `OctxError`.
- Registration consumes and returns `self` for builder-style setup.
- The dispatcher itself is ordinary owned Rust data; additional concurrency
  wrappers can be added by callers when needed.

## Dispatcher family model and CompositeDispatcher

M1 keeps `Dispatcher` single-family and adds `CompositeDispatcher` for
multi-family processes:

```rust
let Chimera = Dispatcher::New("ChimeraOctx")
    .Handle("ChimeraHello", HandleChimera)?;
let Admin = Dispatcher::New("Admin")
    .Handle("Ping", HandlePing)?;
let Composite = CompositeDispatcher::New()
    .Add(Chimera)?
    .Add(Admin)?;
MainLoop(Composite)?;
```

Duplicate function names within one family fail during handler registration.
Duplicate families fail during composition. Unknown families/functions return
protocol error responses rather than process errors.

Composition preserves a simple mental model: one dispatcher owns one family,
and a composite owns routing across families.

## Response and Field builders

M1 exposes PascalCase value builders for the common scalar and record cases:

```rust
Response::OkVoid()
Response::OkInt(value)
Response::OkFloat(value)
Response::OkString(value)
Response::OkBool(value)
Response::OkRecord("RecordType", fields)
Response::Err(message)

Field::Int("GoValue", value)
Field::Float("Score", value)
Field::String("Name", value)
Field::Bool("Enabled", value)
Field::Record("Nested", "NestedType", fields)
```

The current H1 implementation covers the value kinds needed by
`ChimeraOctxHello` plus obvious scalar symmetry. Arrays, bytes, handles, and
borrowed string variants are deferred until there is a motivating sidecar.

## Panic policy

M1 catches panics around handler invocation with `catch_unwind` and converts the
panic into `Response::Err("handler panic")`. This improves sidecar robustness for
business-handler failures without changing handshake/framing behavior.

The main loop does not broadly hide protocol or IO bugs: handshake, frame IO,
and malformed output still return `OctxError` to the sidecar `main` function.
The C ABI SDK's `InstallQuietPanicHook` remains separate because the Octxiliary
SDK runs across a process boundary and does not need the same cgo stderr policy
in M1.

## Main loop

M1 exposes the top-level convenience:

```rust
MainLoop(Dispatcher)?;
```

The name stays PascalCase and mirrors the existing sidecar convenience concept.
A future `Dispatcher.Run()` method can be added if examples show that method
style is clearer, but M1 keeps the top-level function small and explicit.

## Protocol metadata constants

M1 exposes constants for existing protocol metadata only:

```rust
OCTXILIARY_PROTOCOL_NAME
OCTXILIARY_ABI_MAJOR
OCTXILIARY_ABI_MINOR
```

These are aliases for the current handshake name and ABI version. They do not
change the wire protocol.

## Record return compiler gap

`OCTX-RECORD-RETURN` is explicitly not a Rust SDK task.

The current protocol supports Record values both directions. The Rust SDK can
encode them, and the Go client path in `ChimeraOctxHello` can receive them. The
remaining gap is compiled Oct wrapper declaration and type-lowering support for
arbitrary Record return values.

## Go client SDK gap

`OCTX-GO-CLIENT` is explicitly not a Rust SDK task.

A future Go client wrapper should provide spawn/handshake/write-request/read-
response utilities for tests. It should layer on top of `pkg/octxiliary` and the
internal protocol helpers without changing sidecar-serving APIs.

## C ABI vs Octxiliary rule of thumb

Use C ABI if the call is inside a loop or wrapping a C library. Use Octxiliary
for task-scale sidecar/tool/service calls.

More specifically:

- C ABI is appropriate for tight loops, kernels, SIMD/GPU dispatch, large
  in-place buffers, and existing C libraries.
- Octxiliary is appropriate for files, documents, requests, tool calls,
  process-isolated services, typed DTO transport, handles, and tests.
- Safety discriminator: use Octxiliary when a Rust crash should not kill the
  Oct host process.
- Data discriminator: use C ABI when huge buffers cross repeatedly; use
  Octxiliary when data crosses once per task and is small or medium sized.

## Implemented H1

H1 implemented the reusable SDK crate at `internal/octxiliary/rust-sdk` and
refactored `Examples/ChimeraOctxHello/rust-sidecar` to depend on it by path.

Implemented:

- PascalCase public SDK APIs and deliberate `#![allow(non_snake_case)]`.
- `Request`/`RecordRef` field helpers for Int, Float, String, Bool, and nested
  Record.
- Optional field helpers for scalar field kinds.
- `Response` and `Field` builders for scalar and record values.
- Closure-based `Dispatcher` with fallible handlers.
- `CompositeDispatcher`.
- Per-handler panic conversion to protocol errors.
- `MainLoop`, handshake, frame read/write, request parsing, and response
  encoding for the M1 typed value subset.
- Unit tests for field extraction, dispatcher routing, composite unknown-family
  behavior, and parsing Go-encoded record requests.

Deferred:

- Full parity with every Go protocol value kind: arrays, bytes, and handles.
- A complete standalone Octagon parser. The SDK parses the existing
  Octxiliary value grammar subset it emits/consumes.
- Async runtime support.
- Procedural macros.
- Thread-local state/error APIs.
- Go client SDK.
- Compiled Oct Record wrapper returns.
- Make helper additions such as `Make.RustStaticLib` or `Make.GoCgoBinary`.
