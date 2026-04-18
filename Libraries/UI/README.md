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

---

## 🔷 UIIR vs MIR — Boundary Principle

Machina UI deliberately separates **UIIR (presentation)** from **MIR (computation)**, even though both operate over similar underlying data and state.

At a data level, frontend and backend are not fundamentally different — both are mostly state, records, and transformations. However, they **lower to different semantic targets**:

* **MIR (backend / core)**
  Handles computation, control flow, analysis, persistence, and general program logic.

* **UIIR (frontend / UI)**
  Handles presentation, layout, interaction surfaces, and event bindings.

This separation is intentional and must be preserved.

### Core rule

> **UIIR is a presentation IR, not a general execution IR.**

UIIR must not absorb responsibilities such as:

* arbitrary computation or service logic
* persistence or data orchestration
* backend-style control flow beyond UI interaction

Likewise, MIR should not take on UI layout or rendering responsibilities.

### Interface boundary

Even when both sides run in the same environment (e.g. Wasm):

* MIR and UIIR communicate through an **explicit interface boundary**
* typically via:

  * events (UI → core)
  * state or projections (core → UI)

This follows the principle of separation of concerns — keeping UI and logic independent improves maintainability, testability, and clarity

### Design intent

* Shared state models are encouraged
* Shared domain logic is allowed
* **Lowering remains separate**

This ensures:

* UIIR stays small, declarative, and LLM-friendly
* MIR remains the single place for general computation
* future targets (native, Wasm, etc.) stay composable

### Anti-goal

Do **not** merge UIIR and MIR into a single IR.

That leads to:

* bloated responsibilities
* unclear semantics
* “god runtime” anti-patterns
* loss of clarity in both UI and computation layers

---

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
- `AbsoluteBox` coordinates/sizes are `px`; `AnchoredBox` coordinates are normalized `ui`.
- `AbsoluteBox` / `AnchoredBox` support bounded optional z-order (`-5..5`, default `0`), and same-z overlap is rendered deterministically in stable node order.

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

## M99 Browser Host Renderer

M99 is the first host milestone with visible rendering using the real emitted Wasm artifact.

- browser host lives at `tools/machina-ui-host/`
- consumes the same locked M96 UIIR JSON ABI
- dispatches canonical event JSON back into Wasm
- keeps Wasm as the source of truth for state/transitions/UIIR generation

Reference docs: `docs/MACHINA_UI_HOST_RENDERER_M99.md`.

## M100 Native Host (webview-backed first slice)

M100 extends the same host/runtime contract into a desktop webview boundary.

- desktop webview layer lives at `tools/machina-ui-desktop/`
- reuses browser host runtime/renderer (`tools/machina-ui-host/host.js`) directly
- consumes the same emitted `.ui.wasm` artifact and `machina.uiir.v1` JSON ABI
- dispatches the same canonical event JSON contract

This milestone is architectural proof of “one UI runtime, multiple hosts,” not native platform productization.

Reference docs: `docs/MACHINA_UI_NATIVE_HOST_M100.md`.
