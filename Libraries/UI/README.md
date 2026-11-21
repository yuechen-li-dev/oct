# UI standard library (Machina UI)

`UI` is Oct's standard-library authoring surface for Machina UI (Machine Native UI).
For Oct user code, prefer `UI.*` functions. Raw `UI*` builtins are runtime backing details.

The design direction for the authoring surface (layout tables, typed messages,
stack and grid sugar, and the milestone order) is specified in
[`DESIGN.md`](DESIGN.md). This README documents what is implemented now.

## Apps and messages (UI-M3)

An app declares its model, its message enum, and three functions:

```oct
enum Msg {
    Raise
    Select(String)
}

fn Init() -> Model
fn Update(model: Model, msg: Msg) -> Model
fn View(model: Model) -> UI
```

Buttons carry message values, and `Update` matches them exhaustively:

```oct
UI.Button("Raise", Msg.Raise, model.Setpoint < 30.0C)

fn Update(model: Model, msg: Msg) -> Model {
    return match msg {
        case Msg.Raise => model with { Setpoint: model.Setpoint + 0.5K }
        case Msg.Select(id) => model with { Selected: Option.Some(id) }
    }
}
```

- `UI.Button(label, message, enabled)` takes a value of any enum. It is a
  compiler-owned builtin (generic over the message enum). Message variants
  may carry a payload.
- The button keeps the message value; `Update` receives the value itself.
  Its printed form (`Msg.Select("harbor-board")`) is a tool-facing alias for
  dumps and `oct ui check --click`; nothing parses it back.
- `oct ui` checks when the app loads that `Update`'s second parameter is the
  app's message enum, and when a button is pressed that its message is of
  that enum. A mismatch is an error naming both.
- `UI.Button` still accepts a `UI.EventToken`, and `Update` may still take
  `event: String`, for apps written before UI-M3. New apps use messages.
  `UI.Token`, `UI.ButtonRaw` and the `UI.Dispatch` helpers below exist for
  those apps.

`Examples/MachinaThermostat` is the smallest app; `Experiments/Storefront/M8`
is a full storefront on this surface.

## Canonical API

```oct
UI.Text(content: String) -> UI
UI.Button(label: String, message: Msg, enabled: Bool) -> UI      // Msg: any enum
UI.Spacer() -> UI
UI.Styled(style: UI.Style, node: UI) -> UI

// Placement (UI-M2)
UI.Canvas(children: UI[]) -> UI
UI.At(x: UI.XSpan, y: UI.YSpan, child: UI) -> UI
UI.Region(region: R, x: UI.XSpan, y: UI.YSpan) -> UI.Placement
UI.Layout(rows: UI.Placement[]) -> UI.LayoutTable
UI.Arrange(layout: UI.LayoutTable, context: C, content: fn(C, R) -> UI) -> UI

// Stacks and grids (UI-M3)
UI.Stack(spec: UI.StackSpec, children: UI[]) -> UI
UI.Vertical() -> UI.StackSpec
UI.Horizontal() -> UI.StackSpec
UI.Column(children: UI[]) -> UI                  // Stack(Vertical(), children)
UI.Row(children: UI[]) -> UI                     // Stack(Horizontal(), children)
UI.Fixed(main: Float<px>, child: UI) -> UI
UI.Fill(weight: Float, child: UI) -> UI
UI.Auto(child: UI) -> UI
UI.Size(width: Float<px>, height: Float<px>, child: UI) -> UI
UI.Nudge(dx: Float<px>, dy: Float<px>, child: UI) -> UI
UI.Inset(insets: UI.Insets, child: UI) -> UI
UI.Grid(spec: UI.GridSpec, rows: UI[][]) -> UI
UI.Tracks(columns: UI.Track[]) -> UI.GridSpec
UI.Span(columns: Int, rows: Int, child: UI) -> UI
UI.Skip() -> UI
UI.Chunk(items: UI[], columns: Int) -> UI[][]
UI.Keyed(key: String, child: UI) -> UI

UI.Signature(node: UI) -> String
```

Older spellings remain callable for existing code and are not part of the
documented surface: `UI.AbsoluteBox`/`UI.AnchorBox` (and their `Z` forms,
`Absolute`, `Anchor`, `Place`, `AnchoredBox`), `UI.GridRows` (a grid of equal
fill tracks), `UI.ButtonRaw`, `UI.Token`, and the mount helpers `UI.Mount`,
`UI.Patch`, `UI.Unmount`, `UI.Emit`, `UI.DrainEvents` (and `MountUI`,
`PatchUI`, `UnmountUI`).

`MountRef` is the handle record returned by `UI.Mount`.
The distinct naming (`Mount` function, `MountRef` record) intentionally avoids record/function namespace collision in Oct.

## Stacks, sizing and grids (UI-M3)

Sugar places children with declared sizes and lowers to the same explicit
rectangles a layout table writes. It never negotiates, shrinks or wraps; when
children do not fit, layout fails with an error naming the stack and the
sizes involved.

```oct
UI.Stack(UI.Vertical() with { Gap: 12.0 px  Padding: UI.InsetsAll(16px) }, [
    UI.Fixed(56px, HeaderBar(model)),
    UI.Fill(1.0, ProductGrid(model, 3)),
    Footer(model)
])
```

- `StackSpec` has `Axis`, `Gap`, `Padding`, `Align` (`Start`, `Center`, `End`
  on the cross axis) and `Justify` (`Start`, `Center`, `End`, `SpaceBetween`
  for leftover main space when no child fills). Start from `UI.Vertical()` or
  `UI.Horizontal()` and change fields with `with`.
- Main size is height in a vertical stack and width in a horizontal one.
  `UI.Fixed(main, child)` sets it; `UI.Fill(weight, child)` takes a share of
  what fixed children leave; `UI.Auto(child)` measures the child;
  `UI.Size(w, h, child)` sets both axes.
- Unwrapped children: `Text` and `Button` are measured on both axes and placed
  across by `Align`; a nested stack is measured along the parent's axis and
  stretches across it; `Spacer`, grids and canvases fill (`UI.Fill(1.0)`).
  A fixed or measured container stretches across the stack, as `UI.Fixed`
  does.
- Sizing applies only to a child of a stack; elsewhere it is an error.
- Text is measured with fixed metrics (8 dp per character, 20 dp per line;
  buttons add 24 dp and are at least 80 dp wide) until the backend's
  measurement reaches layout (UI-M4).
- `UI.Nudge(dx, dy, child)` moves a node after placement without moving its
  siblings.
- `UI.Inset(insets, child)` puts space around one child. Padding is layout,
  never style.

Grids have declared tracks and no implicit columns:

```oct
let spec = UI.Tracks([UI.Track.Fixed(280px), UI.Track.Fill(1.0)]) with {
    Rows: [UI.Track.Fixed(56px), UI.Track.Fill(1.0)]
    ColumnGap: 16.0 px
}
UI.Grid(spec, [
    [UI.Span(2, 1, TopBar(model)), UI.Skip()],
    [Sidebar(model), Content(model)]
])
```

- `UI.Track` is `Fixed(Float<px>)`, `Fill(Float)` or `Auto` (as wide or tall
  as its largest single-span cell). Rows past `Rows` use `AutoRow`.
- Every row has one cell per column. A cell a span covers must be
  `UI.Skip()`; an uncovered `UI.Skip()` is an empty cell.
- `UI.Chunk(items, columns)` splits a list into rows, padding the last with
  `UI.Skip()`, so a dynamic list becomes a grid in plain sight.

`UI.Keyed(key, child)` gives a repeated child its id from a domain key instead
of its position, so a card keeps its id when earlier cards are filtered out.
Keys must be unique among siblings.

## Model

Layout is unit-aware: pixel lengths use `Float<px>` and parent fractions use
`Float<ui>`. State lives in the app's model; the UI is a pure projection
`View(model) -> UI`. Octomata flows are not Dominatus and provide no stack
push/pop UI semantics.

## Generic event vocabulary (M0 apps)

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


## Dispatch helpers (M111, M0 apps)

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
- `UI.Insets` (used by stack and grid padding)
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
}
```

### Applying a style (M117)

`UI.Styled(style, node)` attaches a `Style` to a node. Lowering carries it to
render, and native backends paint it:

- `Background` fills the node's rect, `BorderColor` and `BorderWidth` outline
  it, and `Radius` rounds its corners.
- `Foreground` colors the node's own text (a `Text`'s content or a `Button`'s
  label). It is not inherited by children.
- A styled `Button` replaces the default button chrome; a disabled one still
  renders its label muted.
- The style is part of the node's identity: `UI.Signature` changes with it.

`Style` has no padding: inner space is layout (a stack's `Padding` or
`UI.Inset`). `UI.TextStyle` is not applied yet (text size needs real text
measurement in layout).

```oct
let badge = UI.Styled(UI.DefaultStyle() with {
    Background: UI.Rgb(255, 152, 0)
    Foreground: UI.Rgb(255, 255, 255)
    Radius: 4px
}, UI.Text("Status: heating"))
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

record Counter {
    Count: Int
}

enum Msg {
    Increment
}

fn Init() -> Counter {
    return Counter { Count: 0 }
}

fn Update(model: Counter, msg: Msg) -> Counter {
    return match msg {
        case Msg.Increment => model with { Count: model.Count + 1 }
    }
}

fn View(model: Counter) -> UI {
    return UI.Stack(UI.Vertical() with { Gap: 8.0 px  Padding: UI.InsetsAll(16px) }, [
        UI.Text("count=" + FormatFloat(Float(model.Count), 0)),
        UI.Button("Increment", Msg.Increment, true)
    ])
}
```


## Deterministic grid/cell authoring (M114)

`UI.GridRows` is the M114 nested-array grid (outer array = rows, inner arrays = columns) with equal fill tracks. Rows must be rectangular and non-empty; there is no auto-placement or spans. New code uses `UI.Grid(spec, rows)` above. The M0 flat `UI.Grid(children: UI[])` was replaced by it in UI-M3.
