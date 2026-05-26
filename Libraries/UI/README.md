# UI standard library (Machina UI M0)

`UI` is Oct's standard-library authoring surface for Machina UI (Machine Native UI).
For Oct user code, prefer `UI.*` functions. Raw `UI*` builtins are runtime backing details.

## Canonical M0 API

```oct
UI.Text(content: String) -> UI
UI.Button(label: String, event: String, enabled: Bool) -> UI

UI.Row(children: UI[]) -> UI
UI.Column(children: UI[]) -> UI
UI.Canvas(children: UI[]) -> UI
UI.Grid(children: UI[]) -> UI
UI.GridRows(cells: UI[][]) -> UI
UI.Spacer() -> UI

UI.AbsoluteBox(x: Float<px>, y: Float<px>, width: Float<px>, height: Float<px>, child: UI) -> UI
UI.AbsoluteBoxZ(x: Float<px>, y: Float<px>, width: Float<px>, height: Float<px>, z: Int, child: UI) -> UI
UI.AnchorBox(left: Float<ui>, top: Float<ui>, right: Float<ui>, bottom: Float<ui>, child: UI) -> UI
UI.AnchorBoxZ(left: Float<ui>, top: Float<ui>, right: Float<ui>, bottom: Float<ui>, z: Int, child: UI) -> UI

UI.Mount(root: UI) -> UI.MountRef
UI.Patch(mount: UI.MountRef, next: UI) -> Int
UI.Unmount(mount: UI.MountRef) -> Int
UI.Emit(mount: UI.MountRef, event: String) -> Int ! Error
UI.DrainEvents(mount: UI.MountRef) -> String[]
UI.Signature(node: UI) -> String
```

Compatibility aliases remain available for pre-M117 two-step placement authoring:
- `Absolute` / `AbsoluteZ`
- `Anchor` / `AnchorZ`
- `Place(box, child)`
- `AnchoredBox` / `AnchoredBoxZ`
- `MountUI` / `PatchUI` / `UnmountUI`

Prefer canonical names in new code.

`MountRef` is the canonical handle/reference record returned by `UI.Mount`.
The distinct naming (`Mount` function, `MountRef` record) intentionally avoids record/function namespace collision in Oct.

## M0 scope (and non-scope)

M0 is semantic UI construction with the current explicit box placement model.
It is intentionally not layout rows, hit-testing, render command streams, style/theme records, or CSS-like styling.

Layout is unit-aware:
- absolute coordinates/sizes use `Float<px>`
- anchor fractions use `Float<ui>`

Events are symbolic strings.
State should live in Oct records/app logic; UI is usually a pure projection `View(state) -> UI`.

Octomata is not Dominatus and does not provide stack push/pop UI semantics.


## Generic event vocabulary

`UI` exposes a tiny, domain-independent event record surface:

```oct
record UI.UIEvent {
    Token: String
    Payload: String
}

UI.Event(token: String, payload: String) -> UI.UIEvent
UI.EventToken(event: UI.UIEvent) -> String
UI.EventPayload(event: UI.UIEvent) -> String
```

This vocabulary is intentionally small and does not define an app model, app contract, or runtime-owned state container.
Session/runtime app contracts used for backend integration live in internal Go packages, not in the Oct standard-library API surface.


## Dispatch helpers (M111, small/pure)

`UI` includes tiny deterministic dispatch helpers for explicit app update functions:

```oct
record EventValueDispatch {
    Events: String[]
    Values: String[]
}

UI.ResolveEventValue(event: String, table: UI.EventValueDispatch) -> String
UI.MatchEventPrefix(event: String, prefix: String, allowedSuffixes: String[]) -> String
```

These helpers are **not** a router/store/state framework.
They do not mutate records and do not perform reflection-like field updates.
You still write explicit `Update(model, event)` functions and explicit `with` state updates.

```oct
fn Update(model: StoreState, event: String) -> StoreState {
    let nav = UI.ResolveEventValue(event, UI.EventValueDispatch {
        Events: ["nav.shop", "nav.support"]
        Values: ["Shop", "Support"]
    })

    if nav != "" {
        return model with { SelectedNav: nav }
    }

    return model
}
```


## Style records (M112, data-only)

`UI` now includes a small typed style surface made of immutable records:

- `UI.Color`
- `UI.Insets`
- `UI.TextStyle`
- `UI.Style`

Helper constructors:

- `UI.Rgb(r, g, b)`
- `UI.Rgba(r, g, b, a)`
- `UI.InsetsAll(value)`
- `UI.InsetsXY(x, y)`
- `UI.DefaultTextStyle()`
- `UI.DefaultStyle()`

These are plain Oct records, so customization is explicit and immutable via `with` updates.
There are no CSS strings, class names, selectors, or cascade semantics.

```oct
let base = UI.DefaultStyle()
let card = base with {
    Background: UI.Rgb(245, 245, 245)
    Radius: 8px
    Padding: UI.InsetsAll(12px)
}
```

M112 is an Oct-facing style data contract only.
Applying styles in lowering/render backends is future work.

## Canvas/content authoring rule

Canvas is a placement surface. Content is a content surface. These do not mix.

Canvas children should be positioned regions:
- `UI.AnchorBox(...)`
- `UI.AbsoluteBox(...)`

Inside those regions, use content trees:
- `UI.Row`
- `UI.Column`
- `UI.GridRows`
- `UI.Text`
- `UI.Button`
- `UI.Spacer`

Component functions should usually return content trees. If a helper calls `AnchorBox` or `AbsoluteBox`, it is probably writing page layout rather than reusable content.

## Example

```oct
package Example

import UI

record AppState {
    Count: Float
}

fn View(state: AppState) -> UI {
    return UI.Canvas([
        UI.AnchorBox(0.1 ui, 0.1 ui, 0.9 ui, 0.3 ui,
            UI.Column([
                UI.Text("count=" + FormatFloat(state.Count, 0)),
                UI.Button("Increment", "counter.increment", true)
            ])
        )
    ])
}
```


## Deterministic grid/cell authoring (M114)

`UI.GridRows` is the M114 deterministic nested-array grid API (outer array = rows, inner arrays = columns). Rows must be rectangular and non-empty; there is no auto-placement or spans. Use `UI.Spacer()` for explicit empty cells. `UI.Grid(children: UI[])` remains as the legacy flat compatibility wrapper.
