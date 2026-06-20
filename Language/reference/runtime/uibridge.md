# UIBridge v0

UIBridge is the canonical frontend/backend contract for UI-facing Oct applications. UIBridge v0 is Octagon-first: Octagon record data is the schema authority and native message format. JSON is only a compatibility projection generated from the Octagon contract.

UIBridge is Octxiliary-inspired, but it is not ordinary Octxiliary. It is a DTO/message boundary between an explicit UI/app surface and a backend. It is not arbitrary MIR access, a remote object system, or a mechanism that exposes all public functions remotely.

`MachinaUIBridge` may name a Machina-specific client or host implementation. It is not the canonical contract name. React and web clients may consume generated JSON or TypeScript projections in future work, but those projections are not protocol authority.

WebSocket, HTTP, stdio, and local sockets are transports for UIBridge messages. They do not define the protocol.

## Protocol constants

Every canonical UIBridge v0 message uses these constants:

```text
Protocol: "ui.bridge"
Version: 0
```

The protocol string is deliberately short, product-neutral, and distinct from `machina.uiir.v1` and `octxiliary.v0`: UIBridge is an app/frontend backend contract, not a UIIR document ABI and not an Octxiliary sidecar protocol.

## Message categories

UIBridge v0 defines these message categories by `Kind`:

- `Handshake`
- `Capabilities`
- `InvokeRequest`
- `InvokeResponse`
- `StateSnapshot`
- `StatePatch`
- `Event`

`UIBridgeError` is a record used inside responses and diagnostics; it is not a separate transport authority.

## Envelope field rules

Every UIBridge message must contain:

```text
Protocol: String
Version: Int
Kind: String
```

Request and response messages must contain:

```text
RequestId: String
```

Response messages must contain:

```text
Ok: Bool
```

If `Ok: false`, the response must contain:

```text
Error: UIBridgeError
```

If `Ok: true`, the response may contain:

```text
Result: <DTO record/value>
```

State and event messages use typed payload fields:

```text
State: <DTO record>
Patch: <DTO record or patch record>
Event: <event record>
```

Canonical field names are the Octagon schema fields shown in this document and the golden fixtures. Golden fixtures use unqualified record names because current Octagon emission rejects package-qualified record and enum names.

## Canonical Octagon examples

Handshake:

```octagon
UIBridgeMessage {
    Protocol: "ui.bridge"
    Version: 0
    Kind: "Handshake"
    Client: "MachinaUIBridge"
}
```

Capabilities:

```octagon
UIBridgeMessage {
    Protocol: "ui.bridge"
    Version: 0
    Kind: "Capabilities"
    Encodings: ["octagon-document", "octagon-frame", "json-projection"]
    SupportsStatePatch: true
    SupportsEvents: true
}
```

Invoke request:

```octagon
UIBridgeMessage {
    Protocol: "ui.bridge"
    Version: 0
    Kind: "InvokeRequest"
    RequestId: "req-001"
    Target: "Counter.Increment"
    Args: CounterIncrementArgs {
        Amount: 1
    }
}
```

Invoke response success:

```octagon
UIBridgeMessage {
    Protocol: "ui.bridge"
    Version: 0
    Kind: "InvokeResponse"
    RequestId: "req-001"
    Ok: true
    Result: CounterValue {
        Value: 42
    }
}
```

Invoke response error:

```octagon
UIBridgeMessage {
    Protocol: "ui.bridge"
    Version: 0
    Kind: "InvokeResponse"
    RequestId: "req-002"
    Ok: false
    Error: UIBridgeError {
        Code: "ValidationError"
        Message: "Amount must be positive"
        Source: "Backend"
        Retryable: false
    }
}
```

State snapshot:

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

State patch:

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

Event:

```octagon
UIBridgeMessage {
    Protocol: "ui.bridge"
    Version: 0
    Kind: "Event"
    Event: CounterChanged {
        Value: 43
    }
}
```

The parse-validated golden fixtures live under `Language/Runtime/UIBridge/golden/`.

## DTO values

Allowed UIBridge DTO values in v0 are:

- `Int`
- `Float`
- `Bool`
- `String`
- records
- arrays
- simple enums or tagged values where Octagon supports them
- dimensioned numeric scalars where Octagon supports them
- errors through the `UIBridgeError` envelope

Excluded UIBridge DTO values in v0 are:

- functions
- closures
- raw handles
- raw pointers
- arbitrary object identity
- mutable references
- cyclic graphs
- direct MIR internal values
- untyped `Any` or dynamic JSON graph payloads
- matrices, vectors, and tensors unless explicitly represented as DTO records or arrays

Octagon is Oct data. UIBridge must not invent foreign runtime values.

Matrices, vectors, and tensors are not primitive bridge DTOs in v0. Applications that expose them must use explicit DTO records, for example:

```octagon
MatrixDTO {
    Rows: 2
    Cols: 2
    Values: [1.0, 2.0, 3.0, 4.0]
}
```

## Error envelope

`UIBridgeError` has these required v0 fields:

```text
Code: String
Message: String
Source: String
Retryable: Bool
```

Canonical example:

```octagon
UIBridgeError {
    Code: "ValidationError"
    Message: "Amount must be positive"
    Source: "Backend"
    Retryable: false
}
```

Optional future fields may include `Trace: String[]` and `Details: <record>`. UIBridge v0 fixtures do not use JSON `null`; if optional semantics are needed, use explicit fields rather than null-like sentinels.

## Dimensioned values

Dimensioned scalar literals are valid DTO values where Octagon supports them. A UIBridge state payload can carry native dimensioned Oct data:

```octagon
UIBridgeMessage {
    Protocol: "ui.bridge"
    Version: 0
    Kind: "StateSnapshot"
    State: MotionState {
        Speed: 3.2m/s
        Acceleration: 9.81m/s^2
    }
}
```

The corresponding golden fixture is `Language/Runtime/UIBridge/golden/state_snapshot_dimensioned.octagon`.

If a future projection cannot preserve native dimension semantics, it must use an explicit wrapper such as:

```octagon
DimensionedFloat {
    Value: 3.2
    Dimension: "m/s"
}
```

## Explicit backend exposure model

Backends expose UI-visible surfaces explicitly: actions, queries, state, events, and capabilities. UIBridge does not expose every public function or MIR value.

Future schema metadata may use an Octagon-shaped declaration like this:

```octagon
UIBridgeSchema {
    Protocol: "ui.bridge"
    Version: 0
    Actions: [
        UIBridgeAction {
            Target: "Counter.Increment"
            Args: "CounterIncrementArgs"
            Result: "CounterValue"
        }
    ]
    States: [
        UIBridgeState {
            Name: "Counter"
            Snapshot: "CounterState"
            Patch: "CounterPatch"
        }
    ]
    Events: [
        UIBridgeEventSchema {
            Name: "Counter.Changed"
            Payload: "CounterChanged"
        }
    ]
}
```

This is a design-only example in UIB2, not new Oct syntax and not runtime behavior.

## JSON compatibility projection

The following JSON examples are compatibility projections, not canonical protocol definitions. Canonical field names are the Octagon schema fields. The JSON naming policy is provisional in UIB2, TypeScript generation is future work, and JSON cannot carry all Octagon semantics without explicit wrappers.

Invoke request projection:

```json
{
  "protocol": "ui.bridge",
  "version": 0,
  "kind": "InvokeRequest",
  "requestId": "req-001",
  "target": "Counter.Increment",
  "args": {
    "amount": 1
  }
}
```

Invoke response error projection:

```json
{
  "protocol": "ui.bridge",
  "version": 0,
  "kind": "InvokeResponse",
  "requestId": "req-002",
  "ok": false,
  "error": {
    "code": "ValidationError",
    "message": "Amount must be positive",
    "source": "Backend",
    "retryable": false
  }
}
```

State patch projection:

```json
{
  "protocol": "ui.bridge",
  "version": 0,
  "kind": "StatePatch",
  "patch": {
    "value": 43
  }
}
```

## UIB2 scope

UIB2 defines the contract document and golden fixtures only. It does not implement Machina UI, `MachinaUIBridge`, ReactBridge/WebBridge, servers, HTTP, WebSocket, stdio or local socket frame transports, UIIR compiler changes, MIR backend generation, Octxiliary changes, JSON canonicalization, Octagon parser/writer changes, language syntax, or UI runtime behavior.
