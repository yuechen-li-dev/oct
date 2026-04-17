# Machina UI (Machine Native User Interface) for Oct

Machina UI is the UI system first implemented in Oct. It is intentionally machine-native and explicit: source should read as authored layout and composition, not as hidden browser-style layout side effects.

The design goal is predictable rendering and deterministic behavior with clear boundaries between representation data, transition wiring, and procedural updates.

## Canonical Machina UI File Structure

- **Data**  
  Catalogs, label maps, and other pure data tables that represent canonical facts.

- **Placement**  
  Slot/grid tables and explicit geometry values used to place UI deterministically.

- **Dispatch**  
  Exact-key transition tables and small resolvers for direct key-to-result lookups.

- **Behavior**  
  Procedural update logic, event-family matching, and derived transitions that are not pure table lookup.

- **Composition**  
  Section builders, card builders, and local UI helpers that assemble UI and emit events.

- **Surface**  
  The final assembled `View` (or equivalent top-level UI value).

## Representation Rules

- Exact-key pure mappings should be tables.
- Exact-key simple transitions should be dispatch tables.
- Placement should remain explicit and deterministic.
- Procedural/dynamic logic should remain code.
- Composition emits events; behavior defines meaning.


## M94 Control Contract (Octomata-aligned)

Machina UI apps are modeled as control systems:

- **State**: explicit app state records (route + durable values)
- **Events**: `UIEvent { Token, Payload }` with deterministic token dispatch
- **Transitions**: Octomata `flow/state` control (no hidden callback loop)
- **View projection**: pure state -> `UI` projection suitable for signature/snapshot tests

Reference implementation: `UI.AppModel.oct` + `UI.M94.octest`.

## M95 Presentation Contract (UIIR)

Machina UI projection now lowers into **UIIR** (UI Intermediate Representation), a deterministic declarative tree that is distinct from Oct procedural MIR.

- **Control stays in Octomata** (`state` + `events` + `transitions`).
- **Presentation lowers to UIIR** (`Text`, `Button`, `AbsoluteBox`, `AnchorBox`, `Row`, `Column`, `Grid`, `Spacer`).
- **Stable ordering and identity** are encoded in signatures via deterministic node IDs.
- **Layout worldview** is led by `AbsoluteBox` and `AnchorBox`; row/column/grid/spacer remain helper composition nodes.

Current scope is representation only: no Wasm lowering, no host rendering ABI, and no effects runtime in this milestone.

## M96 Serialized ABI Truth Surface

M96 locks the canonical serialized cross-boundary representation for:

- **UIIR trees/nodes** (deterministic JSON)
- **event values** (`token` + `payload`)

The ABI surface is documented in:

- `docs/MACHINA_UI_UIIR_ABI.md`

This contract is the stable bridge for future Wasm exports and native/web hosts. It remains intentionally separate from procedural MIR and from any host rendering strategy.

## M97 Wasm Runtime Boundary Skeleton

M97 adds the first Machina UI Wasm runtime boundary skeleton in the Go runtime layer.

- explicit init/render/dispatch boundary functions
- pointer/length event input crossing discipline
- deterministic output buffer contract for canonical M96 UIIR JSON
- focused runtime-layer tests for boundary behavior

Reference docs: `docs/MACHINA_UI_WASM_RUNTIME_M97.md`.


## M98 Real Wasm Emission Slice

M98 is the first milestone where Machina UI is emitted as a real `.wasm` artifact and executed through real Wasm exports + linear memory.

- emitted artifact path is host-selected via `interpret.EmitMachinaUIWasmArtifact(outputPath)`
- exported boundary remains the M97 contract (`init`, `render`, `dispatch(ptr,len)`, `buffer ptr/len`)
- serialized UI surface remains canonical M96 JSON ABI (`machina.uiir.v1`)
- no host renderer exists in this milestone
- this remains a bounded Machina UI Wasm slice, not a full Oct-to-Wasm backend

Reference docs: `docs/MACHINA_UI_WASM_EMISSION_M98.md`.
