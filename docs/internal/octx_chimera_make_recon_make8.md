# MAKE8-DESIGN-RECON — Rust Octxiliary/Octagon Chimera sidecar

Status: design reconnaissance only. This report does **not** implement a Rust sidecar, a Go client, a new example, an Octxiliary protocol change, a Go runtime change, a Rust Octagon parser, a Rust Octxiliary SDK, a C layer, default Cargo/Go build gates, `oct make` executor behavior, UIBridge, Machina, or JSON canonicalization.

## Executive recommendation

MAKE9 should add a separate opt-in `Examples/ChimeraOctxHello` proof where a Go final executable speaks the existing Octxiliary `octxiliary.v0` stream protocol to a Rust sidecar process. The experiment should reuse the existing `OCTWRAP\0` handshake, `uint32` little-endian frames, `OctxiliaryRequest`/`OctxiliaryResponse` envelope, `family`/`function` dispatch, request IDs, and typed `OctxiliaryValue` record transport. It should **not** invent a smaller Chimera-specific protocol.

The M0 DTO should be transported as an existing Octxiliary generic typed record value rather than as a bare top-level `ChimeraRequest`/`ChimeraResponse` frame:

```text
OctxiliaryRequest { id: 1 family: "ChimeraOctx" function: "ChimeraHello" args: [ OctxiliaryValue { kind: "Record" recordType: "ChimeraRequest" fields: [ OctxiliaryField { name: "GoValue" value: OctxiliaryValue { kind: "Int" int: 7 } } ] } ] }
```

```text
OctxiliaryResponse { id: 1 ok: true value: OctxiliaryValue { kind: "Record" recordType: "ChimeraResponse" fields: [ OctxiliaryField { name: "GoValue" value: OctxiliaryValue { kind: "Int" int: 7 } } OctxiliaryField { name: "RustValue" value: OctxiliaryValue { kind: "Int" int: 35 } } OctxiliaryField { name: "Total" value: OctxiliaryValue { kind: "Int" int: 42 } } ] } }
```

Rust can mimic this frame format with only standard library IO and a tiny exact parser for one `GoValue` integer. Full Octagon parsing, full Octxiliary SDK ergonomics, C integration, and default CI execution should remain deferred.

## Inputs inspected

Required prior notes:

- `docs/internal/cabi_chimera_make_recon_make6.md`
- `docs/internal/oct_make_status_make5c.md`

Repo surfaces inspected:

- `README.md`
- `docs/internal/octxiliary.md`
- `pkg/octxiliary`
- `internal/octxiliary`
- `internal/build/compiler.go` generated-runtime templates
- `cmd/octxiliary-*` sidecar command pattern by targeted search
- `Libraries/*/manifest.oct` wrapper package metadata by targeted sampling
- `tools/build_sidecars`
- `cmd/oct/generic_octxiliary_test.go`
- `Language/Testing/CompiledOctxiliary`
- `Language/reference/tooling/34-octagon.md`
- `Language/reference/runtime/uibridge.md`
- `Language/Data/Octagon/...` fixtures
- `internal/octagon`

## Part 1 — Current Octxiliary protocol audit

### Exact frame format

Current Octxiliary transport is stream oriented over stdin/stdout.

Handshake:

- client writes magic bytes `OCTWRAP\0`;
- client writes ABI major `uint16` little-endian;
- client writes ABI minor `uint16` little-endian;
- sidecar validates the same tuple and writes the same tuple back;
- only ABI major is enforced for compatibility in the current reader.

Frame format:

- every message after handshake is one frame;
- prefix is a `uint32` little-endian payload byte length;
- payload is that many bytes of textual Oct-like protocol data.

Request envelope shapes currently accepted:

```text
OctxiliaryRequest { id: <Int> family: "..." function: "..." args: [ <OctxiliaryValue>* ] }
OctxiliaryRequest { id: <Int> family: "..." function: "..." path: "..." bytes: { <byte-int>* } }
OctxiliaryRequest { id: <Int> family: "..." function: "..." path: "..." lines: { <quoted-string>* } }
OctxiliaryRequest { id: <Int> family: "..." function: "..." path: "..." text: "..." }
OctxiliaryRequest { id: <Int> family: "..." function: "..." path: "..." }
```

Generic wrapper work should use the `args: [ ... ]` shape. The `path`/`text`/`lines`/`bytes` forms are legacy/narrow IO-family forms that still coexist with the generic path.

Response envelope shapes currently accepted:

```text
OctxiliaryResponse { id: <Int> ok: true value: <OctxiliaryValue> }
OctxiliaryResponse { id: <Int> ok: true bytes: { <byte-int>* } }
OctxiliaryResponse { id: <Int> ok: true lines: { <quoted-string>* } }
OctxiliaryResponse { id: <Int> ok: true text: "..." }
OctxiliaryResponse { id: <Int> ok: true exists: <Bool> }
OctxiliaryResponse { id: <Int> ok: false error: "..." }
```

Generic wrapper work should use the `ok: true value: <OctxiliaryValue>` shape and the existing `ok: false error: "..."` shape for sidecar errors.

Request IDs:

- requests carry integer `id`;
- compiled generic client increments a per-sidecar request ID;
- sidecars echo the same ID in the response;
- current generated clients serialize calls per sidecar process with a mutex, so M0 does not need multiplexed concurrent response routing.

OK/error envelope:

- success is `ok: true` and one payload field;
- failure is `ok: false error: "..."`;
- sidecar operational failures should become the error envelope when the frame can be parsed, or process/protocol errors otherwise.

Family/function dispatch:

- every request names a `family` and a `function`;
- existing Go sidecars dispatch first on family, then wire function;
- wrapper manifests declare `Family`, `SidecarCommand`, `Protocol`, and function `WireName` entries.

Typed value representation:

- typed values use `OctxiliaryValue { kind: "..." ... }` envelopes;
- supported kinds in the current protocol implementation include `Void`, `Int`, `Float`, `Bool`, `String`, `String[]`, `String[][]`, `Float[]`, `Bytes`, `Record`, and `Handle`;
- records use `kind: "Record" recordType: "..." fields: [ OctxiliaryField { name: "..." value: <OctxiliaryValue> } ... ]`;
- handles use family/type/positive ID fields.

### Payload format classification

Despite historical docs calling it framed Octagon, the actual current wire payload is a custom textual Oct-like protocol. It is not JSON and it is not a raw `.octagon` document. It borrows Oct record-literal aesthetics, but its generic typed values are protocol records named `OctxiliaryRequest`, `OctxiliaryResponse`, `OctxiliaryValue`, and `OctxiliaryField` with lower-case field labels and quoted kind/type strings. Therefore the Rust sidecar should mimic the current textual protocol exactly, not read or write arbitrary `.octagon` files.

### Minimal Rust sidecar subset for one request

A Rust M0 sidecar must implement only:

1. read exactly 12 handshake bytes: 8 magic bytes plus two little-endian `u16` values;
2. validate magic and major `0`, preferably tolerate minor `>= 1` the same way the Go reader currently does;
3. echo the handshake;
4. read one little-endian `u32` frame length;
5. read exactly that many UTF-8/text bytes;
6. verify the frame is the expected `OctxiliaryRequest` with `family: "ChimeraOctx"`, `function: "ChimeraHello"`, and one `Record` arg with `recordType: "ChimeraRequest"` and integer field `GoValue`;
7. compute `rust_value = 35` and `total = go_value + rust_value`;
8. write one `OctxiliaryResponse` typed `Record` value for `ChimeraResponse` with matching request ID;
9. on recognized request-level failures, write `OctxiliaryResponse { id: <id-or-0> ok: false error: "..." }` and exit non-zero only for protocol/IO failures.

### Reuse from Go client code

`pkg/octxiliary` is a public sidecar SDK focused on Go sidecar authors. It exports value types, response constructors, `Serve`, `Main`, dispatcher helpers, argument helpers, and aliases to the internal request/response types. It does not expose a high-level client that spawns a sidecar process. The generated compiled-runtime client templates currently import `internal/octxiliary`, not `pkg/octxiliary`, and hand-roll process spawning/discovery in generated Go.

For a standalone example Go client, two choices are viable:

- preferred if located inside this module: import `github.com/yuechen-li-dev/oct/internal/octxiliary` or add a small package-internal helper only inside the example module path if module boundaries permit;
- more portable for an external-style example: implement the 12-byte handshake and frame IO directly, or use `pkg/octxiliary` only for public types/constructors while duplicating encode/write helpers that are not public.

Because `pkg/octxiliary` currently lacks public `EncodeRequest`, `ParseResponse`, `WriteFrame`, `ReadFrame`, and client spawn helpers, MAKE9 should either keep the Go example in the main repo module and use `internal/octxiliary`, or write the tiny client framing directly. It should not expand the public SDK in M0 unless that is made the explicit scope.

### Portability of protocol

The protocol is portable enough for Rust to mimic: byte order, magic, length prefix, and textual envelope are deterministic. The portability hazard is not the framing; it is that the textual grammar is custom and order-sensitive in the current Go parser/encoder. The Rust sidecar should emit exactly the Go encoder's canonical field order and spacing for M0.

### Current generic wrapper tests

Current generic tests exercise both the public SDK and generated compiled wrapper path:

- protocol round-trip tests cover typed values, arrays, bytes, matrices, floats, records, handles, malformed values, and non-finite float rejection;
- `pkg/octxiliary` tests exercise handshake, serving one and multiple frames, clean EOF, malformed frame errors, constructors, argument helpers, dispatcher routing, and SDK/internal parser compatibility;
- `cmd/oct/generic_octxiliary_test.go` exercises compiled calls through a built `octxiliary-test-wrapper`, missing-sidecar diagnostics, manifest return/fallible mismatch diagnostics, record returns, record arg mismatch, and interpreted generic wrapper behavior.

## Part 2 — Current Octagon data subset audit

### Accepted record literal syntax

Current `.octagon` accepts a single top-level data value. Valid data includes scalars, arrays, record literals, and enum values. Valid record literals look like:

```octagon
SimulationConfig {
    Name: "Cantilever"
    Dt: 0.001s
    Steps: 1000
}
```

Commas are not required in the fixtures. Fields use `Name: Value` syntax. Nested arrays, dimensioned numeric literals, and enum values such as `SolverMode.Explicit` are represented in fixtures.

### Top-level typed records

Top-level typed records like this are accepted by current Octagon fixtures and docs:

```octagon
ChimeraRequest {
    GoValue: 7
}
```

A bare `ChimeraResponse { ... }` shape is valid `.octagon` data as long as the consuming Oct program declares a matching record type and the fields match at load/materialization time.

### Scalars and arrays

Observed/current conventions:

- strings use double-quoted Oct string literals;
- integers use decimal integer literals;
- booleans use `true` / `false`;
- arrays use `[a, b, c]` in examples and fixtures, including arrays of scalars and dimensioned values;
- records use field labels with colons;
- enum values use qualified variant syntax such as `SolverMode.Explicit`.

### Package-qualified record names

UIBridge reference notes a current emission constraint: golden fixtures use unqualified record names because current Octagon emission rejects package-qualified record and enum names. Current wrapper transport records may carry `RecordType` strings such as `Plot.Size` or `Main.TestOptions` inside the Octxiliary protocol, but those are strings in `OctxiliaryValue`, not top-level Octagon record constructor names.

For Chimera M0, use unqualified record names in DTO text examples (`ChimeraRequest`, `ChimeraResponse`). If the wrapper manifest path is used, the transport type name may be `ChimeraOctx.ChimeraRequest` or similar in manifest metadata, but the wire `recordType` should be chosen deliberately and matched by the client. To minimize ambiguity, prefer `recordType: "ChimeraRequest"` / `"ChimeraResponse"` in the direct example protocol, or document any manifest-qualified choice if the generated wrapper route requires it.

### Minimal Chimera subset

M0 needs only:

- one request record type;
- one response record type;
- field names `GoValue`, `RustValue`, and `Total`;
- integer values only;
- deterministic field order;
- no strings in DTO payload, no arrays, no floats, no bools, no dimensions, no enums, no package-qualified record constructors.

Rust can avoid full Octagon parsing by recognizing the exact current Octxiliary typed-record envelope and extracting one decimal integer after the `GoValue` field. The report should not present that parser as a general Octagon parser.

## Part 3 — M0 DTO contract

### Validity as current Octagon

The preferred bare DTO shapes are valid current `.octagon` data in syntax:

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

The future C variant is also syntactically valid if the consuming record type includes `CValue`:

```octagon
ChimeraResponse {
    GoValue: 7
    RustValue: 30
    CValue: 5
    Total: 42
}
```

### Fit with current Octxiliary typed value transport

These DTOs fit current Octxiliary transport as `OctxiliaryValue { kind: "Record" ... }` values. They should not be sent as bare `ChimeraRequest { ... }` frames because current `ParseRequest` expects an `OctxiliaryRequest` envelope.

Recommended request value:

```text
OctxiliaryValue { kind: "Record" recordType: "ChimeraRequest" fields: [ OctxiliaryField { name: "GoValue" value: OctxiliaryValue { kind: "Int" int: 7 } } ] }
```

Recommended response value:

```text
OctxiliaryValue { kind: "Record" recordType: "ChimeraResponse" fields: [ OctxiliaryField { name: "GoValue" value: OctxiliaryValue { kind: "Int" int: 7 } } OctxiliaryField { name: "RustValue" value: OctxiliaryValue { kind: "Int" int: 35 } } OctxiliaryField { name: "Total" value: OctxiliaryValue { kind: "Int" int: 42 } } ] }
```

### Envelope recommendation

M0 should use the existing Octxiliary request/response envelope. This experiment is valuable only if Rust proves it can speak the existing protocol. A smaller Chimera-specific envelope would be easier but would prove less.

Use:

- `family: "ChimeraOctx"`
- `function: "ChimeraHello"`
- one request arg, record type `ChimeraRequest`
- response value, record type `ChimeraResponse`
- existing `ok: false error: "..."` for errors.

## Part 4 — Example layout recommendation

Create a new, separate example:

```text
Examples/ChimeraOctxHello/
  manifest.oct
  README.md
  Make.oct
  Make.octest
  go/
    main.go
  rust-sidecar/
    Cargo.toml
    src/main.rs
```

Do **not** put this under `Examples/ChimeraHello/octx/` for M0. The C ABI Chimera and Octxiliary Chimera prove different concepts:

- `Examples/ChimeraHello`: in-process native linking through C ABI;
- `Examples/ChimeraOctxHello`: process-boundary typed interop through Octxiliary/Octagon-like DTOs.

Separate examples keep the conceptual boundary visible, avoid accidental C ABI reuse for the Go/Rust boundary, and let each README explain its own opt-in gate.

Suggested package metadata for `manifest.oct` should include ordered `Authors: String[]` and ISO `Date: String` per repo convention. The example manifest should preserve the explicit author/date policy used elsewhere.

## Part 5 — Go client design

### Reuse options

A Go final executable can reuse the existing internal protocol implementation if it is built inside the root module and imports `internal/octxiliary`. However, a standalone example should be careful not to imply external users can import internal packages. `pkg/octxiliary` is sidecar-author oriented and does not currently expose client framing/spawn helpers.

Recommended M0 design:

- keep the Go client tiny and explicit;
- either use `internal/octxiliary` if the example is intentionally repo-internal, or direct frame IO if the example should be educational outside the module;
- do not change `pkg/octxiliary` API in MAKE9 unless that becomes an explicit subgoal.

### Minimal declaration/manifest if using wrapper metadata

If the Go client is generated from Oct wrapper metadata later, the wrapper metadata would need something like:

```oct
Wrapper {
    Name: "chimera-octx"
    Family: "ChimeraOctx"
    Protocol: "octxiliary.v0"
    SidecarCommand: "chimera-octx-sidecar"
    GoModuleDir: "rust-sidecar" // not semantically right for current Go wrapper builder
    TransportTypes: [
        WrapperTransportType { Name: "ChimeraRequest" Kind: "record" Fields: [WrapperTransportField { Name: "GoValue" Type: "Int" }] },
        WrapperTransportType { Name: "ChimeraResponse" Kind: "record" Fields: [
            WrapperTransportField { Name: "GoValue" Type: "Int" },
            WrapperTransportField { Name: "RustValue" Type: "Int" },
            WrapperTransportField { Name: "Total" Type: "Int" }
        ] }
    ]
    Functions: [WrapperFunction { OctName: "Hello" WireName: "ChimeraHello" Args: ["ChimeraRequest"] Return: "ChimeraResponse" Fallible: true }]
}
```

But current wrapper build planning assumes `GoModuleDir` for Go sidecar builds, so M0 should not try to push the Rust sidecar through package-manager wrapper build machinery.

### Sidecar location

The Go client should locate the Rust sidecar binary in this order:

1. explicit CLI argument, e.g. `go-client <path-to-rust-sidecar>`;
2. environment variable `OCT_CHIMERA_OCTX_SIDECAR`;
3. sibling binary next to the Go executable, if useful for `oct make` output layout.

Use the opt-in gate `OCT_CHIMERA_OCTX_HELLO=1` for real execution. Keep `OCT_WRAPPER_PATH` out of the example unless deliberately comparing with compiled Octxiliary discovery; this is a standalone Go final executable, not a compiled Oct program.

### Stdout

M0 stdout should be exactly:

```text
chimera octx hello: go=7 rust=35 total=42
```

If C is added later, stdout may become:

```text
chimera octx hello: go=7 rust=30 c=5 total=42
```

All protocol debugging should go to stderr or trace artifacts, not stdout, to keep the golden assertion simple.

## Part 6 — Rust sidecar design

### Feasibility

Rust can implement the frame format trivially with `std::io::{Read, Write}` and little-endian byte conversion:

- `read_exact(&mut [u8; 12])` for handshake;
- `u16::from_le_bytes` for ABI fields;
- `write_all` to echo handshake;
- `read_exact(&mut [u8; 4])` for frame length;
- `u32::from_le_bytes` for length;
- `read_exact` for payload;
- `write_all(&len.to_le_bytes())` plus response string bytes.

No crates are needed for M0.

### Parser

Use a tiny hand-rolled exact parser for M0:

- extract request ID after `OctxiliaryRequest { id: `;
- require `family: "ChimeraOctx"`;
- require `function: "ChimeraHello"`;
- require one `OctxiliaryValue { kind: "Record" recordType: "ChimeraRequest" ... }` arg;
- find an `OctxiliaryField` named `GoValue` whose value is `OctxiliaryValue { kind: "Int" int: <n> }`;
- reject missing, duplicate, negative-if-not-supported, or malformed integer fields with an `ok: false` response when possible.

Do not parse arbitrary strings, arrays, floats, handles, comments, dimensions, package-qualified record constructors, or full Octagon.

### Response shape

Rust should output the exact existing `OctxiliaryResponse` envelope, not a bare `ChimeraResponse`. Use a typed record `OctxiliaryValue` as the `value` payload.

### Error handling

Recommended M0 behavior:

- malformed handshake or impossible frame IO: write a clear stderr message and exit non-zero;
- parsed request ID but unsupported family/function/shape: return `OctxiliaryResponse { id: <id> ok: false error: "..." }` and exit after writing;
- request ID cannot be parsed: return ID `0` only if a frame was read and an error response can still be safely formed;
- all error strings must be quoted/escaped using a minimal Rust string quoting helper compatible with Go `%q`/strconv-style output for ordinary ASCII messages.

### Self-test mode

Add a `--self-test` mode in MAKE9 only if it materially helps without broadening scope. It can run the parser/encoder against one in-memory canonical request and print nothing on success. However, default `Make.octest` must not execute the Rust binary or require Cargo. A self-test is useful for manual opt-in debugging after `cargo build`.

## Part 7 — Optional C layer

Defer C from M0.

C does not add useful missing information to the first Octxiliary/Octagon proof. The question for M0 is whether Go and Rust can communicate across a process boundary through the existing Octxiliary-style protocol and typed DTOs. Adding C would distract from that by reintroducing C ABI concerns already proven separately by `Examples/ChimeraHello`.

Future C extension:

- keep Go/Rust boundary unchanged as Octxiliary frames;
- have the Rust sidecar call a tiny C function internally through C ABI;
- response becomes `GoValue`, `RustValue`, `CValue`, and `Total`;
- gate it separately, e.g. `OCT_CHIMERA_OCTX_C=1`, and keep it out of default CI.

## Part 8 — `oct make` plan recommendation

Suggested M0 targets:

| Target | Kind | Purpose | Default? |
| --- | --- | --- | --- |
| `CheckTools` | `FunctionTarget` | Report whether `go`/`cargo` are discoverable; no builds. | yes/dry-run safe |
| `BuildRustSidecar` | `CommandTarget` | `cargo build --release` in `rust-sidecar`. | opt-in only |
| `BuildGoClient` | `CommandTarget` | `go build -o out/<platform>/chimera-octx-hello ./go`. | opt-in only |
| `RunChimeraOctx` | `CommandTarget` | Run Go client with sidecar path/env. | opt-in only |
| `TestChimeraOctx` | `PhonyTarget` | Depends on build/run targets or invokes the run assertion. | opt-in only |
| `Clean` | `FunctionTarget` | Remove example-local generated outputs. | manual |

M0 does not need `FlowTarget`. C ABI Chimera already demonstrates raw command orchestration. Unless there is a meaningful state machine to inspect, a `PhonyTarget` grouping command targets is simpler and more legible. If future work wants an Octomata trace, a later `FlowTarget` could model states such as `CheckTools -> BuildRust -> BuildGo -> Run -> Verify`, but that should not be part of the first proof.

Generated outputs should remain example-local and uncommitted, for example:

```text
Examples/ChimeraOctxHello/out/<goos>-<goarch>/chimera-octx-hello[.exe]
Examples/ChimeraOctxHello/out/<goos>-<goarch>/chimera-octx-sidecar[.exe]
Examples/ChimeraOctxHello/.octmake/
```

## Part 9 — `Make.octest` recommendation

`Make.octest` should be pure plan/config validation with no host side effects and no native toolchain execution.

Assert:

- default target is safe and does not build/run;
- config uses an example-local `StateDir`, e.g. `.octmake/chimera-octx`;
- trace/state behavior is configured consistently with existing make examples;
- target names include exactly or at least `CheckTools`, `BuildRustSidecar`, `BuildGoClient`, `RunChimeraOctx`, `TestChimeraOctx`, `Clean`;
- dependencies are declared as expected, e.g. `BuildGoClient` and `BuildRustSidecar` before `RunChimeraOctx`, and `TestChimeraOctx` after `RunChimeraOctx`;
- `CommandTarget` program/args are structured and deterministic, not opaque shell blobs when avoidable;
- no target requires `OCT_MAKE_AUTHORITY` for pure plan inspection;
- no test executes `cargo`, `go build`, the Rust sidecar, or the Go client.

## Part 10 — Build/test gating

The real example must be opt-in:

```text
OCT_CHIMERA_OCTX_HELLO=1
```

Default/CI-safe checks may run:

```sh
oct test Examples/ChimeraOctxHello/Make.octest
oct make --file Examples/ChimeraOctxHello/Make.oct --list
oct make --file Examples/ChimeraOctxHello/Make.oct --dry-run --trace
```

Default/CI-safe checks must not run:

- `cargo build`;
- `go build` for the example;
- the Rust sidecar;
- the Go client;
- any C compiler path.

Real run only when the gate is explicitly set:

```sh
OCT_CHIMERA_OCTX_HELLO=1 oct make --file Examples/ChimeraOctxHello/Make.oct TestChimeraOctx --trace
```

If Cargo or Go is missing under the opt-in gate, fail with a clear tool-missing diagnostic. Do not silently skip after the user explicitly requested the real run.

## Part 11 — Relationship to UIBridge

Do not mix UIBridge into this example.

UIBridge is a frontend/backend application contract with Octagon-first state, events, commands, and rendering concepts. Octx Chimera is component/tool sidecar interop: a Go executable sends one typed request to a Rust process and receives one typed response. Both are Octagon-first design ideas, but they are not the same protocol and should not share names, target layouts, or tests in M0.

## Part 12 — Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Protocol drift from existing Octxiliary | Use exact `OCTWRAP` handshake, `uint32` LE frames, `OctxiliaryRequest`/`OctxiliaryResponse`, `family`/`function`, IDs, and typed `OctxiliaryValue` record envelope. Add golden frame fixtures in MAKE9 if useful. |
| Hand-rolled Rust parser fragility | Parse only one exact record subset; reject everything else clearly; document that it is not a full Octagon parser. |
| Platform line ending/path issues | Use binary stdin/stdout for frames; keep sidecar path explicit through CLI/env; generate outputs under example-local `out/<goos>-<goarch>`. |
| Sidecar process management | M0 reads one request and writes one response, so no long-lived pool, restart policy, or multiplexing is needed. |
| stdout/stderr deadlock | Go client should pipe stdin/stdout and either inherit or drain stderr; Rust sidecar should keep stderr small. One-request M0 reduces risk. |
| Frame length mismatch | Compute lengths from UTF-8 bytes, not character counts; add one golden frame length test in MAKE9. |
| No full Octagon parser | Treat this as intentional; use `OctxiliaryValue` typed record envelope and exact subset parser. |
| Cargo toolchain absent | Keep real run behind `OCT_CHIMERA_OCTX_HELLO=1`; default tests inspect plans only. |
| Default CI accidental toolchain requirement | Do not add default Go/Cargo build tests; Make.octest, `--list`, and `--dry-run` only. |
| Confusion with C ABI Chimera | Separate example directory and README; no C ABI on Go/Rust boundary. |
| Confusion with JSON | State clearly that payload is custom Oct-like Octxiliary text, not JSON and not canonical `.octagon`. |

## Part 13 — MAKE9 implementation prompt

Recommended next milestone name:

```text
MAKE9-CHIMERA-OCTX-M0
```

Exact next prompt:

```text
You are working in the Oct repository.

Task: MAKE9-CHIMERA-OCTX-M0 — Rust Octxiliary/Octagon Chimera M0 example

Read first:
- docs/internal/octx_chimera_make_recon_make8.md
- docs/internal/cabi_chimera_make_recon_make6.md
- docs/internal/oct_make_status_make5c.md
- docs/internal/octxiliary.md
- Language/reference/tooling/34-octagon.md

Implement the first opt-in Rust Octxiliary Chimera proof only.

Create:
- Examples/ChimeraOctxHello/manifest.oct
- Examples/ChimeraOctxHello/README.md
- Examples/ChimeraOctxHello/Make.oct
- Examples/ChimeraOctxHello/Make.octest
- Examples/ChimeraOctxHello/go/main.go
- Examples/ChimeraOctxHello/rust-sidecar/Cargo.toml
- Examples/ChimeraOctxHello/rust-sidecar/src/main.rs

M0 behavior:
- Go final executable starts a Rust sidecar process.
- Boundary uses existing Octxiliary `octxiliary.v0` protocol: `OCTWRAP\0` handshake, little-endian ABI fields, little-endian `uint32` frames, `OctxiliaryRequest` and `OctxiliaryResponse` textual envelopes, request IDs, `family`/`function`, and typed `OctxiliaryValue` records.
- Request family/function: `ChimeraOctx` / `ChimeraHello`.
- Request arg: `Record` `ChimeraRequest` with `GoValue: Int`.
- Response value: `Record` `ChimeraResponse` with `GoValue`, `RustValue`, and `Total` integer fields.
- Expected stdout: `chimera octx hello: go=7 rust=35 total=42`.
- Rust sidecar should be std-only and parse only this exact subset.
- No C layer in M0.
- No changes to Octxiliary protocol, Go runtime behavior, `oct make` executor behavior, UIBridge, Machina, JSON canonicalization, or `Examples/ChimeraHello`.
- Do not add default CI gates requiring Cargo or building the example.

Make plan:
- Use separate example directory `Examples/ChimeraOctxHello`.
- Real run is opt-in behind `OCT_CHIMERA_OCTX_HELLO=1`.
- Default-safe tests may run only Make.octest, `oct make --list`, and `oct make --dry-run --trace`.
- Default tests must not run `cargo build`, `go build` for the example, the Rust sidecar, or the Go client.

Tests:
- Run the pure `Make.octest` lane.
- Run `oct make --list` and `oct make --dry-run --trace` for the example with sidecars available as needed.
- Do not run Cargo/Go build unless `OCT_CHIMERA_OCTX_HELLO=1` is explicitly set.
```

## Convergence state

This reconnaissance reaches **meaningful progression**: the current protocol shape, Octagon subset, M0 DTO envelope, layout, gating, and next implementation prompt are isolated. No implementation has been made.
