# UIB1-DESIGN-RECON — UIBridge Octagon transport contract reconnaissance

## Status and scope

UIB1 is a design reconnaissance report only. It does not implement Machina UI behavior, React/web behavior, UIIR compiler changes, MIR backend generation, servers, sockets, WebSocket/HTTP/stdout transports, package-manager behavior, Octxiliary behavior, or `.octagon` parser/writer changes.

The recommendation is:

> Octagon is the canonical schema and native message format for UIBridge. JSON is a compatibility projection generated from the Octagon schema.

`UIBridge` should name the canonical frontend/backend contract. `MachinaUIBridge` may name a Machina-specific host/client implementation of that contract. Avoid using `Oct UI Bridge` as the product/API name because the contract is not tied to one UI product surface and should also support non-Machina clients through projections.

## 1. Current survey findings

### 1.1 `.octagon` support today

The language reference defines `.octagon` as Oct's typed data artifact format for value interchange. A `.octagon` payload is one top-level value and the allowed surface is data only: scalar literals, arrays, record literals, and enum values. Declarations, bindings, calls, control flow, and multiple top-level values are rejected. `WriteOctagon(path, value)` requires a `.octagon` suffix, only writes representable values, and returns an `Int` status. `LoadOctagon<T>(path)` is fallible, requires a `.octagon` suffix, requires a representable type argument, and performs runtime type materialization checks including top-level type, record shape, enum variant, array element, and dimension checks.

Current fixtures confirm these supported shapes:

- scalar literals;
- dimensioned scalar literals such as `9.81m/s^2`;
- arrays of scalars;
- arrays of dimensioned values;
- records;
- nested arrays in records;
- enum values.

The implementation parses `.octagon` through the normal lexer/parser restricted to data values (`parse.BuildDataValue`). The writer serializes interpreter values for `Int`, `Float`, `Bool`, `String`, arrays, records, and enums. It preserves dimensional suffixes for numeric values, rejects package-qualified record and enum type names on emission, and rejects non-data values.

### 1.2 Typed data in manifests, artifacts, registries, and lock-oriented designs

Oct already uses Oct-shaped typed records for package metadata and build-planning artifacts:

- `manifest.oct` files use record-like package metadata, including wrapper metadata. First-party manifests are expected to preserve ordered `Authors: String[]` and ISO `Date: String` metadata.
- The package-manager wrapper manifest contract requires wrapper packages to declare `Wrappers: Wrapper[]`, `WrapperFunction` metadata, and `Protocol: "octxiliary.v0"`.
- `oct pkg wrappers --registry-out <path>` can render an inert deterministic `.octagon` registry with `OctxiliaryRegistry`, `OctxiliarySidecar`, and `OctxiliaryFunction` records. The registry is explicitly metadata, not a sidecar build, lockfile, or runtime activation.
- Package lockfile design notes point future lock encode/decode work toward existing internal Octagon reader/writer conventions instead of adding source-Oct parsing.
- Test/artifact tooling prefers `Artifact.WriteOctagon` for structured artifact output when the payload is representable.

This suggests UIBridge should follow an explicit typed record schema with stable record names, deterministic field ordering, and version fields, rather than a JSON-first or ad hoc string protocol.

### 1.3 Octxiliary envelope, request, response, errors, and framing today

Octxiliary currently provides a framed sidecar protocol for Oct-to-external-wrapper calls. The public package documents it as a small framed protocol/runtime helper for wrapper sidecars. The current wire shape has:

- a binary handshake (`OCTWRAP\0`, ABI major/minor);
- length-prefixed frames (`uint32` little-endian byte count plus payload);
- textual Octagon-like request/response records such as `OctxiliaryRequest { id: ... family: ... function: ... args: [...] }` and `OctxiliaryResponse { id: ... ok: true value: ... }`;
- typed `OctxiliaryValue` values with `kind` strings;
- response errors as `ok: false error: "..."`;
- request IDs for correlation;
- explicit family/function dispatch rather than arbitrary function exposure.

The transport value set is intentionally bounded. It includes `Void`, `Int`, `Float`, `Bool`, `String`, `String[]`, `String[][]`, `Float[]`, `Bytes`, `Record`, and `Handle` in the Go protocol layer. Wrapper manifest docs still list a narrower source-level transport set (`Void`, `Int`, `Float`, `Bool`, `String`, `String[]`, `Bytes`), while implementation and tests have moved further for some generic transport paths. That is a surfaced documentation/coverage gap, not a UIBridge dependency.

Octxiliary is useful precedent for request IDs, explicit dispatch, typed values, `ok`/error envelopes, handshake/framing, deterministic text payloads, and the refusal to turn transport into untyped JSON. It should not be reused wholesale as the UIBridge contract because Octxiliary is primarily an Oct-to-wrapper/service sidecar protocol, while UIBridge is a frontend/client-to-compiled-Oct-app-backend contract with bidirectional events, state snapshots, patches, and UI/app capabilities.

### 1.4 JSON support today

JSON exists today as a wrapper-backed/lower-level compatibility surface:

- string helpers provide JSON string escaping/quoting compatibility functions;
- wrapper builtins normalize, parse, stringify, load, and save JSON as compact strings;
- a structured JSON compatibility path can lower JSON into a `JsonRawGraph` record with sorted object keys and explicit row fields;
- Machina UI/UIIR currently has canonical JSON for `machina.uiir.v1`, including serialized documents and events.

This JSON support is useful for compatibility projections but is not strict enough to be the source-of-truth UIBridge protocol. JSON has no native dimension type, no native enum type, weaker numeric distinctions, no typed record identity, and can only preserve some information through explicit wrappers. UIBridge should therefore derive JSON/TypeScript from the Octagon schema instead of letting web JSON define the schema.

### 1.5 Compiled MIR/binary and wrapper interfaces

Compiled wrapper sidecar design notes already recommend framed Octagon rather than JSON because Octagon is native Oct serialization and JSON is not type-faithful enough. They also note that compiled output already has textual Octagon encode/decode helpers for `WriteOctagon`/`LoadOctagon` lowering, and that broad dynamic `any` payloads remain deferred. Current compiled wrapper bridging is process-boundary, sidecar-oriented, and explicit about discovery/error handling.

This reinforces UIBridge's direction: use typed Octagon DTOs and explicit exported UI/app surfaces rather than arbitrary MIR internals, dynamic object graphs, or all-functions-are-remote lowering.

### 1.6 UIIR and Machina bridge concepts already present

Machina UI has a coherent internal UI spine:

`UI.* authoring -> semantic UIIR -> lowering.Result -> layout resolved geometry -> hittest -> render commands/snapshots -> headless session`

`internal/machina/uiir` owns a semantic document/event model and ABI contract named `machina.uiir.v1`. Current UIIR/Machina lanes use canonical JSON for UIIR documents/events, Wasm host dispatch via JSON bytes, and host renderer/deserializer paths. The docs call renderer backends consumers of the spine, not semantic authorities.

There is not yet a general frontend/backend UIBridge contract between a UIIR-compiled frontend and a MIR-compiled backend service. The current Machina/UIIR JSON ABI is a UI document/event ABI, not a canonical app backend transport. UIBridge should be defined above/beside these lanes as the app boundary contract, with Machina as one client/host.

### 1.7 Naming conventions in the repository

Observed naming uses:

- `Octxiliary` for wrapper/service sidecar transport;
- `Bridge` for specific native/runtime seams such as Prometheus bridge and wrapper bridge;
- `Transport` for typed value movement and sidecar ABI discussions;
- `Machina UI`, `UIIR`, and `machina.uiir.v1` for the native UI semantic/document ABI.

Recommendation:

- `UIBridge`: canonical transport contract between UI/frontends and compiled Oct backend app surfaces.
- `MachinaUIBridge`: Machina-specific implementation/client/host of UIBridge, if needed.
- `ReactBridge` or `WebBridge`: optional generated compatibility clients, not canonical protocol names.
- Avoid bare `Bridge` because it is too generic.
- Avoid `Transport` alone because it names mechanics, not the semantic contract.
- Avoid `Oct UI Bridge` because the boundary is not the Oct UI product name and should not imply JSON/web or Machina-only ownership.

## 2. Conceptual boundary

The intended boundary is:

```text
UIIR compiled frontend
  <-> UIBridge contract
  <-> MIR compiled backend binary/service
```

UIBridge is a DTO/message contract. It is not a remote object system, not a MIR inspector, and not an arbitrary function invocation mechanism.

Responsibilities:

- A UIIR-compiled frontend sends explicit UI/app requests, subscribes to or receives state/events, and applies snapshots/patches.
- A MIR-compiled backend exposes only explicitly exported UI-visible actions, queries, events, and state surfaces.
- Machina UI is one native client/host of this contract.
- React/web is another possible client through generated JSON/TypeScript projection.
- The backend validates all DTOs before invoking app logic.
- The contract never carries closures, functions, raw pointers, raw handles, mutable references, cyclic graphs, arbitrary object identity, or MIR internal values.

## 3. Native canonical format

UIBridge should be transport-neutral at the logical layer, Octagon-first at the schema/native encoding layer, and projection-friendly for compatibility.

Recommended split:

```text
Schema:
  Octagon record schema and examples

Native transport encoding:
  Octagon frame stream or single .octagon document/replay file

In-process:
  generated typed DTO structs, no text required

Compatibility:
  JSON projection and TypeScript DTOs generated from the same Octagon schema
```

Native messages should be logical typed DTOs. They may appear as `.octagon` files for fixtures/replay/artifacts, as length-prefixed Octagon frames for stdio/local sockets, or as generated in-process structs when both sides are linked into one runtime. The schema remains the authority in all modes.

Avoid line-delimited Octagon as the native default because `.octagon` documents can contain multiline records/arrays. Prefer length-prefixed frames for streams and complete `.octagon` documents for files.

## 4. UIBridge v0 message model proposal

Use stable envelope fields on every message:

- `Protocol: "ui.bridge"`
- `Version: 0`
- `Kind: String`
- `RequestId: String` when correlated request/response semantics apply
- `Trace: TraceInfo` or `Trace: String[]` optionally in diagnostics/development lanes
- `Capabilities` during handshake/capability exchange

Candidate v0 message categories:

- `Handshake`
- `Capabilities`
- `InvokeRequest`
- `InvokeResponse`
- `StateSnapshot`
- `StatePatch`
- `Event`
- `Error`

### Invoke request

```octagon
UIBridgeMessage {
    Protocol: "ui.bridge"
    Version: 0
    Kind: "InvokeRequest"
    RequestId: "req-001"
    Target: "Counter.Increment"
    Args: {
        Amount: 1
    }
}
```

### Invoke response success

```octagon
UIBridgeMessage {
    Protocol: "ui.bridge"
    Version: 0
    Kind: "InvokeResponse"
    RequestId: "req-001"
    Ok: true
    Result: {
        Value: 42
    }
}
```

### Invoke response error

```octagon
UIBridgeMessage {
    Protocol: "ui.bridge"
    Version: 0
    Kind: "InvokeResponse"
    RequestId: "req-001"
    Ok: false
    Error: UIBridgeError {
        Code: "ValidationError"
        Message: "Amount must be positive"
        Source: "Backend"
    }
}
```

### State snapshot

```octagon
UIBridgeMessage {
    Protocol: "ui.bridge"
    Version: 0
    Kind: "StateSnapshot"
    State: CounterState {
        Value: 42
    }
}
```

### State patch

Use explicit typed patch records rather than JSON Patch as the canonical form:

```octagon
UIBridgeMessage {
    Protocol: "ui.bridge"
    Version: 0
    Kind: "StatePatch"
    Patch: CounterPatch {
        Value: 43
    }
}
```

For UIB2, define whether patches are domain-specific records, a small typed operation set (`Replace`, `Append`, `Remove`), or both. Do not adopt JSON Patch as canonical.

### Event

```octagon
UIBridgeMessage {
    Protocol: "ui.bridge"
    Version: 0
    Kind: "Event"
    Event: UIBridgeEvent {
        Name: "Counter.Changed"
        Payload: CounterChanged {
            Value: 43
        }
    }
}
```

### Handshake/capabilities

```octagon
UIBridgeMessage {
    Protocol: "ui.bridge"
    Version: 0
    Kind: "Handshake"
    Capabilities: UIBridgeCapabilities {
        Encodings: ["octagon-frame", "octagon-document"]
        JsonProjection: true
        StatePatch: true
    }
}
```

## 5. DTO type rules

Allowed UIBridge DTO values for v0:

- `Int`, with explicit range policy in generated projections;
- `Float`, with finite-number validation;
- `Bool`;
- `String`;
- records with explicit schema names and fields;
- arrays of allowed DTO values;
- enums/tagged values represented in an Octagon-compatible way;
- errors only through the error envelope;
- dimensioned numeric scalars where native Octagon can represent them, otherwise explicit DTO records;
- bytes only if explicitly modeled later as an encoded DTO record or a named byte-array type; avoid making arbitrary bytes a default UI/app DTO.

Excluded from v0:

- functions;
- closures;
- raw handles;
- raw pointers;
- arbitrary object identity;
- cyclic graphs;
- mutable references;
- direct MIR internal values;
- tensors/matrices unless represented intentionally as DTO records/arrays with shape and element rules;
- untyped `Any`/dynamic JSON graph payloads as a general escape hatch.

Oct does not expose raw pointers or closures as normal DTO data, and Octagon is Oct data. UIBridge should stay aligned with that instead of inventing foreign runtime values.

## 6. Encoding rules for records, enums, errors, units, arrays, optionals, and tensors

### Records

Use named records with stable fields. Preserve schema field order for deterministic emission. The JSON projection may use lower camel case later, but the canonical Octagon schema should use Oct-style field names as written in the record schema.

### Arrays

Arrays are homogeneous by schema, including arrays of records and arrays of dimensioned values. For streams/replay, arrays must be bounded by transport/message size limits.

### Enums and tagged values

Current `.octagon` supports enum values such as `SolverMode.Explicit`. UIBridge v0 should use simple enum variants for closed tag sets. If associated enum payloads are required before Octagon has a documented native representation for them, model them as explicit records:

```octagon
SelectionChanged {
    Kind: "ItemSelected"
    ItemId: "sku-001"
}
```

Do not invent final associated-enum Octagon syntax in this report.

### Errors

Errors cross only as error envelope data:

```octagon
UIBridgeError {
    Code: "ValidationError"
    Message: "Amount must be positive"
    Source: "Backend"
    Retryable: false
}
```

Do not transport runtime exception objects, stack objects, or host errors directly. Development traces may be included as strings or structured diagnostic records when enabled.

### Optional/missing values

The surveyed `.octagon` reference does not establish a general null/optional literal. UIBridge v0 should avoid implicit missing/null fields. Use one of these explicit shapes until the language/reference defines a canonical optional data representation:

```octagon
OptionalString {
    Present: false
    Value: ""
}
```

or domain-specific records with explicit `HasX: Bool` fields. JSON `null` should only appear in compatibility projection when generated from such explicit schema.

### Dimensioned numeric values

Native `.octagon` supports dimensioned scalar literals and load-time dimension checks. Prefer preserving dimensional correctness in native UIBridge messages:

```octagon
MotionState {
    Speed: 3.2m/s
    Acceleration: 9.81m/s^2
}
```

For compatibility projections or schemas that cannot carry native units, generate explicit records:

```octagon
DimensionedFloat {
    Value: 3.2
    Dimension: "m/s"
}
```

The UIB2 schema stage should choose whether each dimensioned field projects as a wrapper record or as numeric plus generated metadata. JSON cannot preserve native Oct unit semantics by itself.

### Matrices, vectors, tensors

Do not treat matrices/vectors/tensors as primitive UIBridge DTOs in v0. If an app must expose such data, require an explicit DTO record with element array, shape, layout, and dimension metadata, for example `MatrixDTO { Rows, Cols, Values }`. This avoids leaking compiler/runtime internal representations.

## 7. JSON compatibility projection

JSON is a projection, not the canonical contract.

Projection rules to define in UIB2/UIB3:

1. Octagon record fields map to JSON object properties using a deterministic naming policy. Prefer preserving field names initially for lossless debugging, with optional generated TypeScript aliases later.
2. Records map to objects. Include an explicit type discriminator only when the static schema position is ambiguous.
3. Simple enums map to strings or `{ "type": "...", "variant": "..." }`; choose wrappers when round-trip type identity matters.
4. Tagged values with payloads map to explicit discriminator objects.
5. Dimensioned values map to `{ "value": number, "dimension": string }` unless the field schema carries the dimension and the projection documents the unit externally.
6. Errors map to an object with `code`, `message`, `source`, optional `retryable`, and optional trace fields.
7. Arrays map to arrays with schema-known element type.
8. `Int` projection must document safe range behavior for JavaScript. Large integers may require string or bigint-compatible projection.
9. Non-finite floats must be rejected or explicitly wrapped; JSON cannot represent NaN/Infinity portably.
10. Missing/optional values must derive from explicit Octagon DTO shapes, not implicit JSON `null` authority.

JSON Schema and TypeScript generation should be derived from the Octagon schema. React/web clients consume these generated compatibility artifacts; they do not define the canonical protocol.

Information JSON cannot preserve without wrappers includes record type identity at ambiguous positions, native enum identity if encoded as plain strings, dimensions/units, integer range, exact numeric type distinction, and absence-vs-null semantics.

## 8. Transport modes

Future transports should be categorized by authority:

Native:

- in-process generated DTO calls;
- stdio Octagon frame stream;
- local socket Octagon frame stream;
- `.octagon` replay/artifact files containing one message or an array/recorded stream of messages.

Compatibility:

- HTTP JSON projection;
- WebSocket JSON projection;
- generated TypeScript client;
- optional WebSocket Octagon frames only if the client stack can support Octagon directly.

Do not implement any transport in UIB1. For streams, prefer length-prefixed Octagon frames over line-delimited documents. For files, prefer complete `.octagon` replay documents.

## 9. Explicit backend exposure model

A compiled MIR backend must not expose every public function as remotely callable. That would make the UI boundary too broad, hard to secure, hard to version, and likely to leak internal APIs or incidental public helpers.

Future exposure options:

1. **Explicit language annotations** such as design-only examples `ui action Counter.Increment(...)`, `ui query Counter.Get(...)`, and `ui state CounterState`. This is expressive and close to source, but requires language design and syntax approval later.
2. **Manifest-based export declarations** listing UI-visible actions, queries, state records, event records, and versions. This avoids new syntax initially and aligns with manifest/package metadata traditions, but can drift from source without validation.
3. **Package metadata generated from annotated public functions** where source annotations or doc attributes feed bridge schema generation. This balances source ownership and metadata output.
4. **Generated bridge declarations from selected public functions** using an allowlist. This can bootstrap early work but must still be explicit.
5. **All public functions callable** should be rejected. It is unsafe, overexposes internals, complicates compatibility, and erodes the DTO boundary.

Recommended path: UIB2 should define manifest/schema examples for explicit actions/queries/state/events without adding syntax. Later milestones can decide whether language annotations are justified.

## 10. Relationship to Octxiliary

UIBridge should be Octxiliary-inspired but not ordinary Octxiliary.

Reuse or adapt these conventions:

- request/correlation IDs;
- explicit family/target naming rather than arbitrary invocation;
- handshake/capability negotiation;
- length-prefixed frames for streams;
- typed DTO/value validation;
- `ok` plus error envelope response convention;
- deterministic textual Octagon for native payloads;
- no untyped JSON/`Any` as the internal transport shortcut.

Differences:

- Octxiliary is primarily Oct -> external wrapper/service; UIBridge is frontend/client -> compiled Oct app backend plus backend -> frontend state/events.
- Octxiliary sidecars may own wrapper-family handles; UIBridge v0 should not transport raw handles or object identity.
- Octxiliary functions are wrapper-family operations; UIBridge targets are app/UI-visible actions, queries, events, and state surfaces.
- UIBridge needs bidirectional message categories, snapshots, patches, and capability negotiation suited to UI/app sessions.

Placement recommendation: future implementation should live under a new `pkg/uibridge`/`internal/uibridge` area, not inside `pkg/octxiliary`, while reusing small generic framing/Octagon helpers only if they are factored without importing wrapper semantics.

## 11. Naming recommendation

Use:

```text
UIBridge
  canonical transport contract

MachinaUIBridge
  Machina-specific host/client implementation of the contract

ReactBridge / WebBridge
  possible generated compatibility clients/projections later, not canonical
```

Do not use:

- `MachinaBridge` for the canonical contract, because React/web and other UI clients should not be second-class.
- `Bridge` alone, because the repo already has several bridge concepts.
- `Transport` alone, because the contract is more than the byte transport.
- `Oct UI Bridge`, because it implies a product/API name the task explicitly wants to avoid and blurs Machina UI, UIIR, and app backend transport.

## 12. Security and authority boundaries

Future UIBridge implementation should enforce:

- explicit exported actions/queries/state/events only;
- no arbitrary function invocation;
- no arbitrary filesystem/process access by bridge messages;
- request IDs for correlation and replay diagnostics;
- structured error envelopes with stable codes;
- capability handshake and version negotiation before app messages;
- DTO validation before invoking backend logic;
- message size limits and backpressure for stream transports;
- no direct MIR internal values, pointers, closures, raw handles, mutable references, or object graph identity;
- audit-friendly generated schema and target lists.

## 13. Staged plan recommendation

UIB1:

- land this design reconnaissance report only.
- confirm no UI runtime implementation and no JSON canonicalization.

UIB2:

- define canonical `.octagon` schema/examples for UIBridge v0 envelopes;
- add golden `.octagon` fixtures for handshake, capabilities, invoke success/error, state snapshot, state patch, and event;
- validate fixtures through existing `.octagon` parser/load lanes where practical;
- keep runtime transport unimplemented.

UIB3:

- implement schema validation/parser support if the current Octagon data surface needs helper validation;
- generate JSON projection examples from the same schema;
- document TypeScript DTO mapping without shipping a full client if still premature.

UIB4:

- implement a narrow stdio/local bridge runner for a compiled backend using Octagon frame streams;
- preserve explicit exported target allowlists;
- add no fallback to arbitrary function calls.

UIB5:

- integrate MachinaUIBridge as a Machina-specific client/host of UIBridge;
- keep `machina.uiir.v1` as UIIR document/event ABI unless intentionally migrated by a separate UIIR plan.

UIB6:

- generate React/web JSON/TypeScript compatibility projection;
- keep JSON generated from Octagon schema, never canonical.

## 14. UIB1 conclusion

The repo already has the essential pieces needed to define UIBridge as Octagon-first: `.octagon` is typed data interchange; manifests/registries already use typed deterministic Oct-shaped metadata; Octxiliary demonstrates framed, request/response, typed, explicit-dispatch IPC; and compiled wrapper design already rejects JSON as insufficiently type-faithful. Existing Machina/UIIR JSON ABI is a bounded UI document/event ABI and should not become the app backend protocol authority.

UIB1 therefore ends in **meaningful progression**: the canonical boundary, native format, message model, DTO rules, JSON projection posture, transport split, backend exposure model, Octxiliary relationship, naming, and staged implementation plan are defined without implementing runtime behavior.
