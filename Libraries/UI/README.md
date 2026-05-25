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
UI.Spacer() -> UI

UI.Absolute(x: Float<px>, y: Float<px>, width: Float<px>, height: Float<px>) -> UI.UIBox
UI.Anchor(left: Float<ui>, top: Float<ui>, right: Float<ui>, bottom: Float<ui>) -> UI.UIBox
UI.Place(box: UI.UIBox, child: UI) -> UI

UI.Mount(root: UI) -> UI.MountRef
UI.Patch(mount: UI.MountRef, next: UI) -> Int
UI.Unmount(mount: UI.MountRef) -> Int
UI.Emit(mount: UI.MountRef, event: String) -> Int ! Error
UI.DrainEvents(mount: UI.MountRef) -> String[]
UI.Signature(node: UI) -> String
```

Compatibility aliases remain available in M0:
- `AbsoluteBox` / `AnchoredBox` (+ `...Z` variants)
- `MountUI` / `PatchUI` / `UnmountUI`

Prefer canonical names in new code.

`MountRef` is the canonical handle/reference record returned by `UI.Mount`.
The distinct naming (`Mount` function, `MountRef` record) intentionally avoids record/function namespace collision in Oct.

## M0 scope (and non-scope)

M0 is semantic UI construction with the current explicit box placement model.
It is intentionally not layout rows, hit-testing, render command streams, style/theme records, dispatch helpers, or CSS-like styling.

Layout is unit-aware:
- absolute coordinates/sizes use `Float<px>`
- anchor fractions use `Float<ui>`

Events are symbolic strings.
State should live in Oct records/app logic; UI is usually a pure projection `View(state) -> UI`.

Octomata is not Dominatus and does not provide stack push/pop UI semantics.

## Example

```oct
package Example

import UI

record AppState {
    Count: Float
}

fn View(state: AppState) -> UI {
    return UI.Canvas([
        UI.Place(
            UI.Anchor(0.1 ui, 0.1 ui, 0.9 ui, 0.3 ui),
            UI.Column([
                UI.Text("count=" + FormatFloat(state.Count, 0)),
                UI.Button("Increment", "counter.increment", true)
            ])
        )
    ])
}
```
