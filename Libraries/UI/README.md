# UI standard library (Machina UI M0)

`UI` is Oct's standard-library authoring surface for Machina UI (Machine Native UI).
For Oct user code, prefer `UI.*` functions. Raw `UI*` builtins are runtime backing details.

The design direction for the authoring surface (layout tables, typed messages,
stack and grid sugar, and the milestone order) is specified in
[`DESIGN.md`](DESIGN.md). This README documents what is implemented now.

## Canonical M0 API

```oct
UI.Text(content: String) -> UI
UI.Button(label: String, event: UI.EventToken, enabled: Bool) -> UI
UI.ButtonRaw(label: String, event: String, enabled: Bool) -> UI

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

Events are nominal `UI.EventToken` values at the authoring boundary (string-backed at runtime interop boundaries).
State should live in Oct records/app logic; UI is usually a pure projection `View(state) -> UI`.

Octomata is not Dominatus and does not provide stack push/pop UI semantics.


## Generic event vocabulary

`UI` exposes a tiny, domain-independent event record surface:

```oct
record UI.EventToken {
    Value: String
}

record UI.UIEvent {
    Token: UI.EventToken
    Payload: String
}

UI.Token(value: String) -> UI.EventToken
UI.EventTokenValue(token: UI.EventToken) -> String
UI.Event(token: UI.EventToken, payload: String) -> UI.UIEvent
UI.EventTokenOf(event: UI.UIEvent) -> UI.EventToken
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

### Applying a style (M117)

`UI.Styled(node, style)` attaches a `Style` to a node. Lowering carries it to
render, and native backends paint it:

- `Background` fills the node's rect, `BorderColor` and `BorderWidth` outline
  it, and `Radius` rounds its corners.
- `Foreground` colors the node's own text (a `Text`'s content or a `Button`'s
  label). It is not inherited by children.
- A styled `Button` replaces the default button chrome; a disabled one still
  renders its label muted.
- The style is part of the node's identity: `UI.Signature` changes with it.

Not applied yet: `Padding` (it changes layout geometry) and `UI.TextStyle`
(text size needs real text measurement in layout).

```oct
let badge = UI.Styled(UI.Text("Status: heating"), UI.DefaultStyle() with {
    Background: UI.Rgb(255, 152, 0)
    Foreground: UI.Rgb(255, 255, 255)
    Radius: 4px
})
```

## Explicit placement and layout tables (UI-M2)

Page geometry is declared, not computed. A screen names its regions with an
enum, declares one placement row per region in a layout table, and binds
content to regions with an exhaustive `switch`. See `Examples/MachinaShell`
and [DESIGN.md §5.4](DESIGN.md).

```oct
enum Shell { Header  Rail  Grid  Footer  Cart }

fn WideShell() -> UI.LayoutTable {
    return UI.Layout([
        UI.Region(Shell.Header, UI.StretchX(16px, 16px),  UI.Top(16px, 56px)),
        UI.Region(Shell.Rail,   UI.Left(16px, 240px),     UI.StretchY(84px, 68px)),
        UI.Region(Shell.Grid,   UI.StretchX(268px, 16px), UI.StretchY(84px, 68px)),
        UI.Region(Shell.Footer, UI.StretchX(16px, 16px),  UI.Bottom(16px, 40px)),
        UI.Region(Shell.Cart,   UI.Right(24px, 160px),    UI.Bottom(72px, 40px)) with { Layer: UI.Layer.Chrome }
    ])
}

fn View(model: Store) -> UI {
    return UI.Arrange(WideShell(), model, ShellContent)
}

fn ShellContent(model: Store, region: Shell) -> UI {
    return switch region {
        case Shell.Header => HeaderBar(model)
        // ... one case per region
    }
}
```

Spans constrain one axis of a box, in dp against the parent box. Each
constructor is one shape, so a box is always exactly constrained:

```oct
UI.Left(left: Float<px>, width: Float<px>) -> UI.XSpan
UI.Right(right: Float<px>, width: Float<px>) -> UI.XSpan
UI.StretchX(left: Float<px>, right: Float<px>) -> UI.XSpan     // width follows the parent
UI.CenterX(width: Float<px>) -> UI.XSpan
UI.FractionX(left: Float<ui>, right: Float<ui>) -> UI.XSpan
UI.Top / UI.Bottom / UI.StretchY / UI.CenterY / UI.FractionY   // the same shapes, vertically -> UI.YSpan
```

Layout tables:

```oct
UI.Region(region: R, x: UI.XSpan, y: UI.YSpan) -> UI.Placement    // R: any enum without payloads
UI.Layout(rows: UI.Placement[]) -> UI.LayoutTable
UI.Arrange(layout: UI.LayoutTable, context: C, content: fn(C, R) -> UI) -> UI
UI.At(x: UI.XSpan, y: UI.YSpan, child: UI) -> UI                   // one box in a UI.Canvas
```

- `UI.Region` fills `Layer: UI.Layer.Content` and `Z: 0`; change them with
  `with`. `UI.Placement.Region` holds the region's canonical name
  (`Main.Shell.Header`) for dumps; build rows with `UI.Region`, not by hand.
- `UI.Arrange` calls `content(context, region)` once per row, in row order, and
  places each result in its row's box; the content fills the box.
- `UI.Arrange` fails, naming the region, when the table misses a region,
  places one twice, mixes region enums, places a different enum than the
  content function takes, or gives a `Z` outside −5..5. These are checked when
  the view is built (by `oct ui check` before a window opens), not at compile
  time; compile-time checking arrives with typed views (DESIGN.md L1). The
  content function's `switch` is checked at compile time like any `switch`.
- A stretch whose insets exceed the parent is a layout error, never a clamp.
- Paint order among siblings is `(layer, z, authored order)`, and hit testing
  follows it: a `Chrome` region is clickable above the `Content` region it
  overlaps. `UI.Layer` is `Base`, `Content`, `Chrome`, `Overlay`, `Modal`,
  `Tooltip`.
- `UI.Region` and `UI.Arrange` are compiler-owned builtins (they are generic
  over the region enum); the other names are ordinary `UI` functions.

`oct ui check <app.oct> --dump` prints each frame as a layout dump: one line
per node in paint order, with the spans that place it in its parent and its
absolute rect. Regions show their table row; children of stacks show the
`Left`/`Top` spans the stack resolved to.

## Canvas/content authoring rule

Canvas is a placement surface. Content is a content surface. These do not mix.

Canvas children must be placed boxes; an unplaced child is a lowering error:
- `UI.At(...)`, or regions placed by `UI.Arrange`
- `UI.AnchorBox(...)` and `UI.AbsoluteBox(...)` (older spellings of `UI.At`
  with `FractionX`/`FractionY` and `Left`/`Top`)

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
                UI.Button("Increment", Token("counter.increment"), true)
            ])
        )
    ])
}
```


## Deterministic grid/cell authoring (M114)

`UI.GridRows` is the M114 deterministic nested-array grid API (outer array = rows, inner arrays = columns). Rows must be rectangular and non-empty; there is no auto-placement or spans. Use `UI.Spacer()` for explicit empty cells. `UI.Grid(children: UI[])` remains as the legacy flat compatibility wrapper.
