# Machina UI ABI (M96): Canonical UIIR + Event JSON

This document defines the **serialized truth surface** for Machina UI as of M96.

It is intentionally narrow and stable:

- No Wasm lowering in this milestone
- No host renderer in this milestone
- No patch/diff stream protocol in this milestone
- No binary protocol in this milestone

The goal is to lock a deterministic cross-boundary representation for future Wasm and host implementations.

## Contract boundary

Machina UI now has two explicit layers:

- **Control model (M94):** state/events/transitions in Oct
- **Presentation model (M95):** declarative UIIR tree (`Text`, `Button`, `AbsoluteBox`, `AnchorBox`, `Row`, `Column`, `Grid`, `Spacer`)

M96 defines how that presentation layer and event values are serialized at the boundary.

This contract is separate from procedural MIR and separate from any HTML/CSS/DOM model.

## ABI format

Current serialized ABI format: **canonical JSON**.

ABI identifier:

```json
"machina.uiir.v1"
```

## Canonical UIIR document shape

Top-level UIIR JSON document:

```json
{
  "abi": "machina.uiir.v1",
  "root": { "...node..." : "..." }
}
```

Node shape:

```json
{
  "id": "0.1",
  "kind": "Button",
  "key": null,
  "text": null,
  "label": "Increment",
  "enabled": true,
  "event": { "token": "counter.increment", "payload": null },
  "box": null,
  "layout": { "x": 10, "y": 20, "width": 100, "height": 30 },
  "children": []
}
```

Notes:

- `id` is deterministic (`0`, `0.0`, `0.1`, ...), assigned by child index order.
- `children` preserves authored/projected ordering.
- `kind` is explicit for all node families.
- `event` is explicit when present (currently on `Button`).
- `box` is explicit for placement nodes:
  - absolute: `{ "kind":"absolute", "absolute":{x,y,width,height}, "anchored":null }`
  - anchored: `{ "kind":"anchored", "absolute":null, "anchored":{left,top,right,bottom} }`
- `layout` carries resolved box data when available.

## Canonical event shape

Event JSON shape:

```json
{
  "token": "counter.increment",
  "payload": null
}
```

`payload` intentionally exists from the start; `null` is canonical when there is no payload body.

## Determinism rules

- Same UIIR tree => same canonical JSON bytes.
- Different child ordering => different canonical JSON.
- Event serialization is stable and always includes both `token` and `payload`.

## Validation/roundtrip behavior

The runtime provides strict decode checks for:

- ABI identifier
- required node fields (`id`, `kind`, node-kind payload fields)
- required event token
- explicit box kind payload presence

UIIR and event values support canonical JSON roundtrip tests in Go runtime tests.

## Forward-compatibility stance

This is a bounded first ABI surface. Future milestones may add fields or alternate formats with versioning, but this shape is now the reference surface for:

- future Wasm exports
- future native/web host consumption
- deterministic snapshots/debugging
- ABI discussions
