# M102 — Machina UI production contract audit

## Scope

This is an audit and planning milestone only.

- No production behavior changes.
- No UIIR ABI changes.
- No renderer migration.
- No Gio integration.

## Status notes after planning milestones

- **M108 status (implemented):** `internal/machina/session` now provides a headless, backend-neutral native session loop contract that composes projection -> lowering -> layout -> hittest -> render commands -> deterministic snapshot, plus pointer dispatch and rebuild.
- **M109 status (implemented):** optional `machina_gio` backend spike now lives under `internal/machina/backend/gio`, consuming render commands and feeding pointer coordinates through `session.PointerDown` without semantic ownership changes.
- This preserves the M102 production direction while keeping Gio/window/pixel work out of scope for M108.
- **M111 status (implemented):** `Libraries/UI` now includes small pure dispatch helpers (`UI.EventValueDispatch`, `UI.ResolveEventValue`, `UI.MatchEventPrefix`) for explicit `Update` flows without reflection-like record mutation.
- **M112 status (implemented):** `Libraries/UI` now includes typed immutable style records (`UI.Color`, `UI.Insets`, `UI.TextStyle`, `UI.Style`) and deterministic helper constructors (`UI.Rgb/Rgba`, `UI.InsetsAll/InsetsXY`, default style constructors). This milestone is Oct-facing data only and does not alter lowering/render/runtime behavior.
- **M113 status (implemented):** `internal/machina/layout` now includes internal-only `GridArrange` + `CellFrame` resolution with fixed/fill tracks, gap/padding/span handling, and deterministic validation/error paths. M114 revised adds public nested-array `UI.GridRows(UI[][])` authoring lowered to internal grid/cell layout defaults (equal fill tracks, explicit rectangular rows).

Post-M101 architecture baseline:

- `internal/machina/uiir` owns UIIR model + canonical ABI/JSON/signatures/event codecs.
- `internal/machina/layout` owns current pure box resolution.
- `internal/interpret` is adapter/session runtime and builtin boundary.

## Production contract to preserve

Machina UI for Oct should be:

- Oct-native authoring through `Libraries/UI` (`UI.*` in user code).
- Native-first and renderer-independent.
- No CSS-style string/class system.
- State held in Oct records/app state; UI is a pure projection `View(state) -> UI`.
- Symbolic action/event strings.
- Styles as immutable Oct records (`with` updates), not hidden widget objects.
- Unit-aware layout (`Float<px>`, `Float<ui>`).
- Semantic UIIR lowered to flat layout rows + action/semantic/style metadata.
- Deterministic geometry resolution and pure hit testing.
- Renderers consume resolved render commands.
- Gio is a backend only (future native backend), never UI semantics owner.
- WASM stays useful, but long-term path is MIR-backend-first, browser target second.

### Explicit Oct constraints

- Octomata is not Dominatus.
- Octomata has one `remember`/`resume` slot and no implicit push/pop UI stack semantics.
- Screen/modal stack behavior must be explicit record/library state.
- Avoid reflection-like generic record mutation assumptions unless language/runtime support exists.

## 1) Current surface inventory (post-M101)

### A. UIIR node model + canonical ABI/JSON

- UIIR node and box model (`NodeKind`, `BoxKind`, `Node`, `BoxSpec`, `ResolvedRect`).
- Canonical ABI tag: `machina.uiir.v1`.
- Canonical JSON encode/decode for document and event payloads.
- Node ID assignment, cloning, signatures, event-token tree search.

Location:

- `internal/machina/uiir/uiir.go`

### B. Current layout/box resolver

- Pure resolver for absolute and anchored box specs.
- Deterministic bounds/range validation.
- Parent-relative recursive layout propagation.

Location:

- `internal/machina/layout/layout.go`

### C. Interpreter UI builtins + session adapter

- Builtins: `UIText`, `UIButton`, `UIColumn`, `UIRow`, `UICanvas`, `UIGrid`, `UISpacer`, `UIPlaceAbsolute`, `UIPlaceAnchored`, `UIMount`, `UIPatch`, `UIUnmount`, `UIEmit`, `UIDrainEvents`, `UISignature`.
- Adapter delegates core semantics to `internal/machina/uiir` + `internal/machina/layout`.
- `internal/interpret` currently owns mount/session/event queue behavior.

Location:

- `internal/interpret/interpret.go`
- `internal/interpret/ui_runtime.go`

### D. Current standard-library UI surface

There is already a `Libraries/UI` package that wraps builtins and introduces unit-aware box records:

- `UI.Text`, `UI.Button`
- `UI.Column`, `UI.Row`, `UI.Canvas`, `UI.Grid`, `UI.Spacer`
- `UI.UIBox` + `BoxKind` + placement helpers with `Float<px>` and `Float<ui>` fields
- mount/patch/unmount/emit/drain/signature wrappers

Location:

- `Libraries/UI/UI.Text.oct`
- `Libraries/UI/UI.Button.oct`
- `Libraries/UI/UI.Layout.oct`

### E. Current WebView/WASM host paths

- Desktop host boundary under `internal/machina/desktophost` (webview driver + shell tests).
- WASM runtime boundary exports:
  - `ExportMachinaUIInit`
  - `ExportMachinaUIRender`
  - `ExportMachinaUIDispatchEventJSON`
  - output buffer ptr/len
- WASM lowering/emission in `internal/interpret/ui_wasm_lowering.go`.

Location:

- `internal/machina/desktophost/*`
- `internal/interpret/ui_wasm_runtime.go`
- `internal/interpret/ui_wasm_lowering.go`

### F. Current tests

Core and adapter tests are present for UIIR/layout/runtime/WASM/desktop host paths.

Representative locations:

- `internal/interpret/ui_runtime_test.go`
- `internal/interpret/ui_wasm_runtime_test.go`
- `internal/interpret/ui_wasm_artifact_test.go`
- `internal/interpret/ui_wasm_host_renderer_test.go`
- `internal/interpret/ui_wasm_desktop_host_test.go`
- `internal/machina/desktophost/driver_webview_smoke_test.go`
- `internal/machina/desktophost/shell_test.go`

### G. Storefront wrappers/pattern evidence

Storefront milestones repeatedly define local wrappers for `Text`, `Button`, `Row`, `Column`, `Canvas`, box placement, and mount/patch/emit flows; this is direct evidence that `Libraries/UI` is the right authoring surface.

Location:

- `Experiments/Storefront/M0..M7/*.oct`

## 2) Gap analysis against production contract

| Contract area | Exists today? | Current location | Gap | Suggested milestone |
|---|---|---|---|---|
| `Libraries/UI` authoring surface | Partial | `Libraries/UI/*.oct` | Present but still thin wrappers over raw builtins; no production contract docs/tests for semantic layering | M103 |
| Immutable style records | Yes (M112 data-only) | `Libraries/UI/UI.Style.oct` | Renderer/lowering style application remains future work; no CSS/class/cascade semantics introduced | Follow-up milestone(s) |
| Semantic UIIR stability | Yes (v1 baseline) | `internal/machina/uiir/uiir.go` | Need explicit governance/compat policy tests for evolution | M103/M105 |
| Layout rows | No | N/A | Semantic tree not lowered to flat row IR | M104 |
| Layout document | No | N/A | No first-class `LayoutDocument` type | M104 |
| Resolved layout document | No | N/A | Current result is UIIR `Layout` rects, not dedicated resolved document model | M104 |
| Lowering result | No | N/A | No explicit lowering artifact bundling rows + metadata | M105 |
| Action metadata | Partial | UI button `Event` strings in UIIR | No normalized metadata table in lowering result | M105 |
| Semantics metadata | No | N/A | No semantic annotations map/stream for render/hit path | M105 |
| Style metadata | No | N/A | No style metadata pipeline | M105/M112 |
| Pure hit testing | No | N/A | No standalone pure hit-test over resolved geometry + metadata | M106 |
| Render command stream | No | N/A | Renderers consume UIIR JSON today, not explicit render command stream | M107 |
| Snapshot renderer | No | N/A | No deterministic snapshot backend for render command stream | M107 |
| Native session loop contract | Partial | `internal/interpret/ui_runtime.go` | Session loop exists but tied to interpreter adapters and current mount model | M108 |
| Gio backend | No | N/A | Not started (intentionally) | M109 |
| Dispatch helpers | Partial/experimental | `Experiments/Storefront/*` patterns | No standardized `Libraries/UI` dispatch helper contract yet | M111 |
| Storefront migration | Partial | `Experiments/Storefront/*` | Storefront still contains repeated local wrappers and experiment-specific scaffolding | M110 |
| WebView/WASM demotion | No | `internal/machina/desktophost`, `internal/interpret/ui_wasm_*` | Current paths are still primary in practical integration/testing | M114 |
| MIR→WASM future backend plan | No | N/A | No concrete plan artifact yet | M115 |

## 3) Risk map + mitigations

1. **Risk: Gio accidentally becomes semantic owner (layout/runtime semantics drift into backend).**
   - Mitigation: freeze semantic contract in core IR docs/tests before Gio spike; require Gio to consume command stream only.

2. **Risk: Dominatus stack assumptions leak into Octomata usage.**
   - Mitigation: mandate explicit screen/modal stacks as records in `UI` library contracts; cite Octomata single-slot resume constraints in each relevant milestone.

3. **Risk: stringly/reflection-like record mutation appears in dispatch helpers.**
   - Mitigation: scope M111 to constrained, explicit helper forms compatible with current records+`with`; prohibit generic field-name mutation unless language support lands first.

4. **Risk: raw builtins leak into user examples instead of `UI.*`.**
   - Mitigation: authoring docs/tests must import `UI` package; treat raw builtins as runtime-internal boundary only.

5. **Risk: `machina.uiir.v1` compatibility breaks.**
   - Mitigation: add ABI lock tests and compatibility fixtures before/alongside lowering refactors.

6. **Risk: CI depends on nondeterministic windowing/GPU behavior.**
   - Mitigation: promote pure-core and snapshot-command tests; isolate desktop/webview/gpu tests behind explicit integration lanes.

7. **Risk: over-porting MachinaLayout.JS too early.**
   - Mitigation: define Oct-first contract (`LayoutRow`/metadata/hit/render stream) before borrowing implementation details.

8. **Risk: style system regresses to CSS-like classes/strings.**
   - Mitigation: constrain M112 to typed immutable records (`with` updates), no class selectors/cascades/string style DSL.

## 4) Recommended milestone plan (starting M103)

### M103 — Libraries/UI M0 wrappers hardening

- **Goal:** lock `UI.*` as Oct authoring namespace and formalize current wrapper contract.
- **Expected files/packages:** `Libraries/UI/*`, language/reference or docs updates, UI wrapper tests.
- **Non-goals:** no IR redesign, no layout-row implementation.
- **Tests required:** UI wrapper behavior tests, compatibility tests with existing builtins.
- **Acceptance:** user examples/tests use `UI.*`; raw builtin usage is internal.
- **Surface:** user-facing.

### M104 — LayoutRow/LayoutDocument/ResolvedLayoutDocument core

- **Goal:** introduce flat layout-row core model and deterministic resolved document.
- **Expected files/packages:** likely new `internal/machina/layoutdoc` or expanded `internal/machina/layout` domain types.
- **Non-goals:** no renderer/Gio integration.
- **Tests required:** deterministic geometry resolution, stable ordering, bounds rules.
- **Acceptance:** semantic layout lowers to rows and resolves into explicit resolved layout docs.
- **Surface:** internal-only core.

### M105 — UIIR lowering result contract

- **Goal:** define lowering artifact combining layout rows + action/semantic/style metadata.
- **Expected files/packages:** `internal/machina/uiir` + lowering package boundaries.
- **Non-goals:** no hit testing/render backend yet.
- **Tests required:** artifact schema tests, deterministic metadata IDs.
- **Acceptance:** one canonical lowering result type consumed by next stages.
- **Surface:** internal-only core.

### M106 — Pure hit testing

- **Goal:** pure function hit-test over resolved geometry + action metadata.
- **Expected files/packages:** `internal/machina/hittest` (or equivalent).
- **Non-goals:** no session/event-loop mutation in hittest core.
- **Tests required:** deterministic overlap/z-order/action resolution tests.
- **Acceptance:** pointer input maps to symbolic action deterministically.
- **Surface:** internal-only core.

### M107 — Render command stream + snapshot backend

- **Goal:** renderer-independent command stream and deterministic snapshot renderer.
- **Expected files/packages:** render command model + snapshot backend/testing utilities.
- **Non-goals:** no native window backend dependency.
- **Tests required:** snapshot stability tests.
- **Acceptance:** renderers consume commands, snapshot backend validates output deterministically.
- **Surface:** internal-only with testing/user artifact visibility.

### M108 — Native session loop contract

- **Goal:** define native-first runtime/session contract over projection, dispatch, patch lifecycle.
- **Expected files/packages:** runtime/session package boundaries around current interpreter adapter logic.
- **Non-goals:** no Gio.
- **Tests required:** lifecycle, event queue, mount/unmount contract tests.
- **Acceptance:** session semantics independent from interpreter internals.
- **Surface:** internal-only.

### M109 — Gio backend spike

- **Goal:** backend spike consuming existing render/session contracts only.
- **Expected files/packages:** new Gio backend package.
- **Non-goals:** no semantic contract ownership in Gio.
- **Tests required:** smoke/integration tests gated from deterministic core CI lanes.
- **Acceptance:** Gio proves backend viability without semantic drift.
- **Surface:** internal-only integration.

### M110 — Storefront migration

- **Goal:** migrate Storefront to canonical `Libraries/UI` surface and production contracts.
- **Expected files/packages:** `Experiments/Storefront/*`, possibly docs/examples.
- **Non-goals:** no new semantic primitives beyond prior milestones.
- **Tests required:** Storefront flow parity tests.
- **Acceptance:** local wrappers removed/reduced; app uses standard `UI.*`.
- **Surface:** user-facing examples.

### M111 — Dispatch helpers M0

- **Goal:** add constrained dispatch helpers compatible with Oct records and current runtime.
- **Expected files/packages:** `Libraries/UI` helper modules.
- **Non-goals:** no reflection/generic field mutation.
- **Tests required:** event-to-action mapping tests.
- **Acceptance:** helpers reduce repetitive event chains while preserving explicitness.
- **Surface:** user-facing.

### M112 — Style/theme records M0

- **Goal:** immutable typed style/theme record model using `with` updates.
- **Expected files/packages:** `Libraries/UI` style modules.
- **Non-goals:** no CSS/class/cascade systems.
- **Tests required:** style record construction/update tests.
- **Acceptance:** style surface is record-native and immutable.
- **Surface:** user-facing.

### M113 — Grid/cell layout

- **Goal:** production grid/cell primitives over row/layout core.
- **Expected files/packages:** layout core + `Libraries/UI` layout APIs.
- **Non-goals:** no backend semantic ownership changes.
- **Tests required:** grid placement/constraint tests.
- **Acceptance:** deterministic grid behavior and ergonomics.
- **Surface:** user-facing + core.

### M114 — WebView/WASM demotion + CI cleanup

- **Goal:** move webview/wasm to secondary lanes after native-core contract stabilizes.
- **Expected files/packages:** CI configs/docs/runtime wiring.
- **Non-goals:** no removal of wasm usefulness.
- **Tests required:** lane split validation (core deterministic vs integration).
- **Acceptance:** core CI does not depend on browser/windowing nondeterminism.
- **Surface:** internal infra.

### M115 — MIR→WASM backend plan

- **Goal:** produce concrete plan for MIR-first wasm backend and browser target alignment.
- **Expected files/packages:** planning docs/architecture notes.
- **Non-goals:** no backend implementation in this milestone.
- **Tests required:** N/A (planning).
- **Acceptance:** agreed roadmap and interfaces for future wasm path.
- **Surface:** internal planning.

## 5) Exactly one recommended next implementation milestone

**Recommend next: `M103 — Libraries/UI M0 wrappers hardening` (first).**

### Why M103 before M104

1. `Libraries/UI` already exists and is already the intended user namespace, so locking it now reduces API churn before deeper IR work.
2. It enforces the authoring boundary (`UI.*` vs raw builtins) early, preventing new experiment drift.
3. It creates a stable user-level contract that M104/M105 can target without repeatedly revisiting public surface semantics.
4. It allows M104 to focus purely on internal layout-row/resolved-doc architecture without simultaneous user-facing naming churn.

### M103 scope outline (no implementation here)

- Declare/verify `UI.*` as canonical user surface in tests/docs.
- Harden wrappers and usage guidance; keep raw builtins internal.
- Add compatibility tests ensuring wrapper behavior parity with current runtime.
- Keep ABI and runtime behavior unchanged.

## 6) Inconsistencies and documentation gaps surfaced

1. **Surface maturity mismatch:** production contract asks for future rich `Libraries/UI` authoring model, and a minimal wrapper package already exists today. This is good progress but should be explicitly treated as *M0*, not complete contract.
2. **Layout core mismatch:** contract targets flat rows and dedicated resolved layout documents; current core still resolves only absolute/anchored boxes attached directly to UIIR nodes.
3. **WASM role mismatch:** current wasm path is active and practical today, while long-term contract positions wasm as a future MIR-backend-first lane.
4. **Dispatch helper gap:** experiments show need (`EventValueDispatch`/`ResolveDispatch` evolution), but standard `Libraries/UI` helpers are not yet standardized.
5. **Style gap:** contract requires immutable style records; no production style library/model exists yet.

## Conclusion

M101 successfully established core extraction boundaries. The highest-leverage next step is **M103** to lock the `UI.*` authoring contract, then proceed to **M104/M105** for internal layout/lowering architecture.


## M103b note: Mount handle naming

`Libraries/UI` uses `UI.Mount(root: UI) -> UI.MountRef` as the canonical lifecycle shape.
`MountRef` is the explicit handle record returned by the `UI.Mount` function; this avoids record/function name collision in Oct namespaces.

## M104 status note

- M104 adds an internal deterministic layout-row substrate (`LayoutRow`/`LayoutDocument`/`ResolvedLayoutDocument`) under `internal/machina/layout`.
- This is internal-only and does not change `Libraries/UI`, UIIR ABI, runtime mount/patch behavior, or WebView/WASM compatibility paths.

## M105 status

M105 adds an internal lowering layer (`internal/machina/lowering`) that converts semantic UIIR into M104 layout rows plus deterministic actions/semantics metadata (and a style placeholder map). This remains internal-only in M105 and is not yet wired into runtime mount/patch/render paths.

## M106 status

M106 adds an internal pure hit-testing package (`internal/machina/hittest`) that maps root-local coordinates to symbolic actions from M105 action metadata over M104 resolved geometry. It is deterministic (reverse pre-order winner policy), uses half-open bounds, and remains internal-only without runtime/backend wiring changes.

## M107 status note

M107 adds internal backend-neutral render command stream and deterministic snapshot recording under `internal/machina/render` for CI-friendly command snapshots. This does not change runtime/interpreter/WebView/WASM behavior.
