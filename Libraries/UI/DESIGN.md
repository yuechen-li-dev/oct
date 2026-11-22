# Machina UI authoring design (spec)

Status: **accepted design, 2026-11-21.** This is the spec for the current
Machina UI direction: the target surface and the order it lands in. It is not
the reference for what is implemented. Implemented behavior is documented in
[`README.md`](README.md) and `Language/reference/`, which stay the single
source of truth for what works today; anything here marked *new* is planned.
When a milestone lands, its rules move into the README and the reference, and
the corresponding section here is marked implemented.

Decisions in §9 were accepted as recommended.

**Thesis.** Machina UI exists because moving a frame
2px up in HTML/CSS means negotiating with a layout engine you never asked for.
So in Machina, **page geometry is declared, not computed**: a screen's regions
and their rectangles are written down as data, in a layout table declared next
to (not inside) the widget code. Widgets fill named regions and compose
content inside them with stacks and grids, which are sugar that lowers to the
same explicit rectangles and never negotiates. Geometry is something you read
in the source, not something an engine decides and you then ask an inspector
about.

The goal is productivity, not explicitness for its own sake. Explicit geometry
pays because it removes the expensive failure (the box is not where you think,
and nothing in the source says why); it is not a reason to make every box a
hand-written coordinate. This is the same trade Oct makes with explicit
captures instead of closures: a couple of extra lines where they buy
certainty, no extra lines where they do not.

Sources audited:

| Implementation | Revision | What was read |
|---|---|---|
| MachinaLayout.JS | `0949eb9` | `docs/` (authoring, M shorthand, row model, forbidden concepts, M8 coherence audit, style, dispatch, ergonomics and dogfood reports), `src/machina`, samples (dispatch-counter, control-room, codex-product-page) |
| Copeland TS Machina | Copeland `e0f7d61` | `docs/Copeland/machina-layout.md` (layout, layout types, z, CSV overlay, stream, derivations, overflow, text documents), reviews: Machina survey M0, stream composition M0, React friction M0, UI grammar M3, component capsule M0 |
| Machina.UI (C#) | Copeland `e0f7d61` | authoring guide, authoring model history, typed identity and actions, native widgets, M19a product pressure, M19c authoring surface review, `Machina.Core/Authoring/UI.cs`, `Actions/` |
| Oct (current) | `44f5f83d` | `Libraries/UI/*.oct` and README, `internal/machina/{uiir,lowering,layout}`, `Experiments/Storefront` M0–M7 and REPORT, `Examples/MachinaThermostat`, language reference (enums, function values, records and tables, typed markup, templates) |

## 1. What each implementation converged on

### MachinaLayout.JS: rows are the truth, `M.*` is the pen

`LayoutRow[]` (id, parent, order, frame, arrange, view, layer, z, variants) is
the canonical model, and its docs say so in bold: nesting is an output shape,
not an authoring strategy. `M.*` arrived later as "the official authoring
surface" that lowers back to rows. It still requires an explicit id on every
node, a string `view` key, and a React view registry fed by `viewData` and
`nodeData` maps (the dispatch-counter sample needs seven rows, a five-entry view
registry and two data maps for one button). It also has positional overloads
(`M.fill(id, weight, opts, undefined, children)` in the product page).

What aged well, and is now repeated in every later implementation:

- deterministic typed frames (`root`, `anchor`, `fixed`, `fill`, `cell`,
  `guide`) with errors, not clamping (`forbidden-concepts.md`);
- arithmetic stacks and explicit grids: no flexbox negotiation, no implicit
  tracks, no margins, no stretch defaults;
- `px` and normalized `ui` as typed lengths, never strings;
- semantic layers plus small sibling-local `z`;
- style as typed records lowered to CSS (`S.style`, `S.with`, explicit
  `set/inherit/unset` layers), and **state styles keyed by real component
  states** (`S.stateful` with hover/pressed/disabled), not pseudo-classes;
- dispatch as tiny tables: "repeated switch statements are dispatch tables
  with bad ergonomics";
- the ergonomics rule from the Claudefood report: a helper ships only if
  adopting it across the repo deletes code or fixes bugs.

### Copeland TS: UI as language declarations

Copeland went furthest. Its survey (CTS-MACHINA-SURVEY-M0) concluded that rows
are excellent *below* authoring and poor *as* authoring: "hierarchy, node
identity, child ordering, geometry, render slot selection, and state data
routing are split across distant records." It recommended plain functions
returning `View`, typed events, compiler-assigned path identity, and `key` only
for repeated collections. Then it built language-level declarations:

- `layout Name<x, y> { row/column/grid/anchor/overlay/slot }`: immutable
  spatial data with a required origin, named slots as identity;
- `layout type` + `satisfies`: exact topology contracts;
- `bind Layout { slot: View() }` and `stream`: content attached to slots;
- `layers` declarations, `z` bounded to −5..5 (out of range is an error, not
  clamped), paint order `(layer rank, z, authored order)`;
- `csv overlay` rows and `with centerIn(root)`-style derivations;
- typed `overflow` and bounded text fitting ("the layout owns the region; text
  owns how it survives inside it").

The honest result is in UI-GRAMMAR-M3's source accounting: reusing one content
stream under two arrangements works, but the Copeland app is **275 lines
against 175** for ordinary React+TS. Its friction report ranked
"variable-length collections" and "native input" as P1 gaps, and found accepted
gap/padding silently dropped by a backend. Separate geometry declarations cost
lines, and they did not remove the need for ordinary code to choose between
arrangements (`if (state.narrow) return bind Compact using Content`).

### Machina.UI C#: functions for components, typed messages, controlled widgets

C# runs both a flat `UiDocument`/`Row.*` form and a `UI.*` tree builder, plus
`StandardUI`, `StandardView` and `WidgetBuilder`. Its own M19c review calls the
native tree "readable for localized components" and the overlapping entry
points a cost. The lessons that matter most for Oct:

- **Placement-first** (M2a): `At`/`Anchor` place panels, stacks are for true
  sequence; "do not use `HSpace`/`VSpace` to place major panels".
- **Stack items say how they size**: `UI.Fixed(main, child)`,
  `UI.Fill(child, weight)`, `UI.Auto(child)`. Main size is along the stack's
  axis.
- **Style padding is paint metadata** (M4c): it never moves children.
  A component that hosts children creates an explicit content region.
- **Typed messages, not strings** (typed identity doc): `UiAction<TMessage>`
  carries an immutable message; the string alias exists only for tools, and
  "reducers consume the message and never parse the alias". Message equality
  includes the payload, so two buttons with one alias but different recipes are
  different actions.
- **Stable identity from domain keys**, never display text or coordinates
  (`UiIdFamily<TKey>`, `NodeId.Child`), and transient widget state (focus,
  press, scroll, text draft) lives in a session keyed by that identity.
- **Controlled widgets**: the model owns values; a widget proposes changes
  (`Bind.Value(get, set)`, slider Preview/Commit/Cancel). A host that binds
  every widget can assert nothing comes back unconsumed.
- **Measured text with a declared overflow** (`TextLine`, `TextOverflow`:
  Ellipsis, Clip, Throw). An overflowing row is a `LayoutError`, not a glitch.
- **Deterministic layout dump** (`MachinaLayoutDump.Format`) for golden tests
  and reviews.
- **Theme passed explicitly**; no hidden context.

## 2. The consensus, and the evidence against the rest

All three implementations independently agree on:

1. Rows are MIR, inspection and fixture format. They are not what people (or
   models) should write.
2. Components are ordinary functions that return a view value. No component
   classes, registries, hooks or lifecycle.
3. Geometry is deterministic and explicit: frames, fixed and fill sizes,
   grids with declared tracks; errors instead of silent repair. (JS rows and
   C# M2a are placement-first; flow is for true sequences.)
4. Style is immutable typed records with copy-update. No CSS strings, no
   cascade.
5. State lives in the application model; the UI is a projection of it.
6. Events should be typed values. (JS never got there; C# and Copeland did, and
   the string-based paths are where the bugs collected.)
7. Identity must be stable and semantic. Positional identity is fine for static
   structure and wrong for lists.

What the evidence argues **against** porting:

| Idea | Evidence | Verdict for Oct |
|---|---|---|
| Authored `LayoutRow[]` | JS counter needs 7 rows + registry + 2 maps; Copeland survey | MIR only (Oct already does this) |
| String `view`/`slot` keys and data maps | JS samples; Copeland survey | Not needed: a node *is* its content |
| Explicit id on every node | JS `M.*`; C# moved to derived ids | Derived ids; `Keyed` for lists |
| Copeland `layout`/`bind`/`stream` declarations | M3: 275 vs 175 lines; arrangement still chosen by `if` | Not now (see §8) |
| Overlapping entry points (C# rows + tree + StandardView) | C# M5d, M19c | One surface, one way per job |
| Flexbox negotiation, margins, `%` strings, `calc` | JS forbidden-concepts | Never |

## 3. Where Oct stands today

Oct already has the right spine: `UI.*` functions build a tree, UIIR lowers it
to layout rows, the Go layout engine resolves rows, and render/hit-test/session
drive the Gio window. Several pieces the other implementations had to invent
already exist in the Oct language: payload enums with exhaustive `match`,
immutable `with`, `record table`, function values with explicit captures, and
typed markup (Oct-XML).

**Works today** (probe run during this audit): Oct-XML already drives the UI
functions with no changes.

```oct
fn View(model: Model) -> UI {
    return <UI.Column>
        <UI.Text Content="Counter" />
        <UI.Row>
            <UI.ButtonRaw Label="Increment" Event="inc" Enabled={true} />
        </UI.Row>
    </UI.Column>
}
```

`oct ui check` lays it out and `--click Increment` dispatches `"inc"`.

The friction is in Storefront M7 (454 lines), which records what the M0 surface
forces:

- **Strings as events.** 19 event-token functions, 4 `EventValueDispatch`
  parallel-array tables, and `MatchEventPrefix` to recover a product id from
  `"storefront.inspect." + id`. `Update` is a chain of `if x != ""` probes.
  The REPORT's conclusion that "exact-key transitions should use dispatch
  tables" is a workaround for the string transport, not a UI principle.
  An enum with payloads plus `match` replaces all of it and adds exhaustiveness.
- **Anchors are fraction-only.** `View` places seven regions with 13 `ui`
  constants (`AnnouncementTop = 0.095 ui`). The problem is not explicit
  placement; it is that Oct's anchor can only say "from 9.5% to 13% of the
  parent". `layout.resolveFrame` resolves `AnchorFrame` as four fractions and
  `AbsoluteFrame` as a fixed rect; there is no "16px from the left edge,
  stretch to 16px from the right" and no "56px tall, pinned to the top". So a
  56px header has to be written as a percentage that is only right at one
  window height. JS `AnchorFrame` (px or `ui` per edge, plus width/height) and
  C# `UI.Anchor` both have this; Oct needs it.
- **No sizing vocabulary.** Lowering makes every container `Fill(1)` and every
  text `8 dp × chars` wide, 20 dp tall. There is no gap, padding, fixed size or
  alignment at the Oct surface, although the Go layout engine already supports
  gap, padding, fixed and fill frames, grid tracks and spans.
- **Parallel arrays for a catalog.** `ProductCatalog` keeps six arrays in step
  by hand; `record table` now exists for exactly this.
- **Positional identity.** `uiir.WithNodeIDs` assigns `0.1.2`-style ids. That
  is harmless while there is no focus, text entry or scroll state, and becomes
  wrong as soon as there is: filtering a list shifts every later card's id.
  (`uiir.Node.Key` exists but nothing sets it from Oct.)
- **`Style.Padding` is declared but unapplied**, because padding changes
  geometry. That is the C# M4c bug in waiting.
- **Two spellings for one thing**: `Anchor`/`AnchorBox`/`AnchoredBox`,
  `Place`, `MountUI`/`Mount`. Each alias is a choice a model has to make.

## 4. Design laws for Oct Machina UI

These are the rules the sketch below follows. Each one names its source.

1. **A view is a value; a component is a function.** `fn Card(...) -> UI<Msg>`.
   Nothing registers, mounts itself, or holds state. (All three.)
2. **Messages are the app's own enum.** Buttons carry `Msg` values; `Update`
   matches exhaustively. Strings exist only as tool-facing aliases and are
   never parsed back. (C# typed identity; Copeland survey.)
3. **One pattern for configuration: start from a default and `with` it.**
   `UI.Vertical() with { Gap: 12px }`, `UI.DefaultStyle() with { ... }`. Oct has
   no default or named arguments; records plus `with` are already its answer,
   and one uniform pattern is the most predictable thing for a model to write.
4. **Configuration first, content last.** Every container takes its spec
   first and its children last, so the same function works as a call and as an
   Oct-XML tag body. (Oct-XML requires the body parameter to be last.)
5. **Page geometry lives in a layout table.** A screen names its regions with
   an enum and declares one row per region: its horizontal span, vertical
   span and layer, in pixels against the parent. The view says what goes in
   each region; the table says where each region is. Moving a region 2px is
   one number in one row, and widget code is never touched to do it.
6. **Regions are checked both ways.** The table must place every region
   exactly once, and the content function must `switch` over every region, so
   a region with no geometry or no content is an error. No string slot names.
7. **Inside a region, sugar; sugar lowers to placement, never to
   negotiation.** Stacks and grids place children one after another or into
   declared tracks using declared sizes. Overflow is an error naming the node
   path. The dump shows every box in the same span vocabulary the table uses.
   Relative geometry (below the header, plus a 12px gap) is arithmetic in
   `let`s, not a solver. (JS, C#; Copeland derivations and JS `GuideFrame`
   needed single-writer rules and cycle checks that arithmetic does not.)
8. **Padding is layout, never style.** It belongs to the container that
   arranges children. Style is paint only. (C# M4c, Copeland M3.)
9. **Text lives in the box it is given.** The box comes from placement;
   measurement decides what happens when the text does not fit (ellipsis,
   clip, error), not how big the box is. Content-sized boxes exist only as an
   explicit opt-in in sugar (`Auto`). (JS: no intrinsic text sizing; Copeland:
   "the layout owns the region; text owns how it survives inside it"; C#
   TextLine.)
10. **Identity is derived, and explicit only where it must survive change.**
   Path-derived ids by default; `UI.Keyed(key, child)` for repeated items,
   keyed by domain identity. (Copeland survey, C# id families.)
11. **Deterministic, readable inspection.** Every frame can be printed as a
    layout dump a test can compare and a model can read without screenshots.
    (C# layout dump, Oct snapshots.)
12. **One way per job.** Compatibility aliases leave the documented surface.
    (C# M5d, JS M8.)

## 5. Syntax sketch

Types and names marked *new* do not exist yet. Everything else is current Oct.

### 5.1 The app

```oct
package Thermostat

import UI

record Model {
    Setpoint: Float<K>
    Heating: Bool
}

enum Msg {                       // the app's whole event vocabulary
    Raise
    Lower
    ToggleHeating
}

fn Init() -> Model {
    return Model { Setpoint: 20.0C  Heating: false }
}

fn Update(model: Model, msg: Msg) -> Model {
    return match msg {
        case Msg.Raise => model with { Setpoint: model.Setpoint + 0.5K }
        case Msg.Lower => model with { Setpoint: model.Setpoint - 0.5K }
        case Msg.ToggleHeating => model with { Heating: not model.Heating }
    }
}

fn View(model: Model) -> UI<Msg> {
    let celsius = BaseUnit(model.Setpoint) - 273.15
    return UI.Stack(UI.Vertical() with { Gap: 8px  Padding: UI.InsetsAll(16px) }, [
        UI.Text("Setpoint: " + FormatFloat(celsius, 1) + " °C"),
        UI.Styled(StatusStyle(model.Heating), UI.Text(StatusLabel(model.Heating))),
        UI.Stack(UI.Horizontal() with { Gap: 8px }, [
            UI.Button("Lower", Msg.Lower, celsius > 5.0),
            UI.Button("Raise", Msg.Raise, celsius < 30.0),
            UI.Button("Toggle heating", Msg.ToggleHeating, true)
        ])
    ])
}
```

A view this small has one region, so it is all content and needs no layout
table. Screens with several regions get one (§5.4).

Compared with today's thermostat: three event-token functions and three
`if event == UI.EventTokenValue(...)` probes become one enum and one exhaustive
`match`. Adding a fourth message without handling it is a compile error.

The convention becomes `Init() -> Model`, `Update(Model, Msg) -> Model`,
`View(Model) -> UI<Msg>`, optionally `View(Model, UI.Viewport) -> UI<Msg>`
(§5.10). `octapp` reads `Msg` from `Update`'s second parameter and checks that
`View` returns `UI<Msg>` with the same `Msg`.

**`UI<Msg>`** (*new*) is the builtin view type parameterized by message type,
like `Option<T>`. A node that emits no message (`UI.Text`) is typed by the place
it goes, exactly as `Option.None` is today; a `UI<Msg>[]` array or a
`-> UI<Msg>` return supplies it. A `UI<A>` never converts to a `UI<B>`. This is
the Elm `Html msg` design. It is the largest change in this sketch, and §7
discusses a cheaper first step.

### 5.2 Messages with payloads

```oct
enum Section { Shop  Bundles  Journal  Support }
enum Category { All  Desk  Paper  Cable }

enum Msg {
    Nav(Section)
    Filter(Category)
    ToggleNewOnly
    Inspect(String)          // product id
    AddToCart(String)
}

fn Update(model: Store, msg: Msg) -> Store {
    return match msg {
        case Msg.Nav(section) => model with { Nav: section }
        case Msg.Filter(category) => model with { Category: category }
        case Msg.ToggleNewOnly => model with { NewOnly: not model.NewOnly }
        case Msg.Inspect(id) => model with { Selected: Option.Some(id) }
        case Msg.AddToCart(id) => model with { Cart: model.Cart + 1 }
    }
}
```

The button carries the value: `UI.Button("Inspect", Msg.Inspect(p.Id), true)`.
Hit-testing stores the value itself, not a string. Message equality is value
equality, so `Msg.Inspect("a")` and `Msg.Inspect("b")` are different actions.

The tool-facing alias is the canonical printed value, for example
`Msg.Inspect("harbor-board")`. `oct ui check --click` accepts a button label
(today) or an alias (*new*); either way the session dispatches the stored value.
Nothing parses the alias.

Labels for enum values are exhaustive `switch` functions, not lookup tables:

```oct
fn SectionLabel(s: Section) -> String {
    return switch s {
        case Section.Shop => "Shop"
        case Section.Bundles => "Bundles"
        case Section.Journal => "Journal"
        case Section.Support => "Support"
    }
}
```

This replaces Storefront's `KeyLabelMapping` tables and `ResolveLabelMapping`,
and a missing label becomes a compile error instead of a silent fallback.

### 5.3 Content leaves

> **Implemented in UI-M4** ([README](README.md#text-ui-m4),
> `Examples/MachinaText`). `UI.TextStyle` was folded into `TextSpec`.
> `TextSpec.Color` is an `Option<Color>` so text can inherit the styled
> foreground. Wrapped text answers the paragraph question from the review
> without a measuring function in Oct code: a vertical stack asks wrapped text
> (or a stack containing it) for its height at the width the stack gives
> it, so placement stays declared and the measuring stays in one place.
> Headless output uses a fixed metric; the window uses its font's metrics.
> `UI.Divider` is not built yet.

```oct
UI.Text(content: String) -> UI<M>                       // one line, measured width
UI.Paragraph(content: String) -> UI<M>                  // new: wraps to assigned width, measured height
UI.TextWith(spec: UI.TextSpec, content: String) -> UI<M>   // new
UI.Button(label: String, on: M, enabled: Bool) -> UI<M>
UI.Spacer() -> UI<M>                                    // Fill(1) along the parent stack
UI.Divider() -> UI<M>                                   // new: 1px rule across the cross axis
```

`TextSpec` follows the default-and-`with` rule:

```oct
let caption = UI.Body() with {
    Size: 12px
    Color: theme.Muted
    Overflow: UI.Overflow.Ellipsis       // Ellipsis | Clip | Error
}
UI.TextWith(caption, p.Subtitle)
```

`UI.Body()`, `UI.Title()`, `UI.Caption()`, `UI.Mono()` return `TextSpec`
defaults. `UI.TextStyle` (M112) is renamed to `TextSpec` or folded into it; one
name, not two.

### 5.4 Layout tables: where page geometry lives

> **Implemented in UI-M2** ([README](README.md#explicit-placement-and-layout-tables-ui-m2),
> `Examples/MachinaShell`). Differences from the sketch below, all pending
> typed views (L1): the table type is spelled `UI.LayoutTable` (a record and
> a function cannot share the name `Layout`), views return `UI` rather than
> `UI<Msg>`, and region coverage is checked when `UI.Arrange` runs rather
> than at compile time. `UI.Nudge` is part of UI-M3 with the sugar.

A screen declares its regions as an enum and its geometry as a table, apart
from the widgets:

```oct
enum Shell { Header  Rail  Grid  Footer  Cart }

fn WideShell() -> UI.Layout<Shell> {
    return UI.Layout([
        UI.Region(Shell.Header, UI.StretchX(16px, 16px),  UI.Top(16px, 56px)),
        UI.Region(Shell.Rail,   UI.Left(16px, 240px),     UI.StretchY(84px, 68px)),
        UI.Region(Shell.Grid,   UI.StretchX(268px, 16px), UI.StretchY(84px, 68px)),
        UI.Region(Shell.Footer, UI.StretchX(16px, 16px),  UI.Bottom(16px, 40px)),
        UI.Region(Shell.Cart,   UI.Right(24px, 160px),    UI.Bottom(72px, 48px)) with { Layer: UI.Layer.Chrome }
    ])
}
```

The view binds content to regions and contains no geometry at all:

```oct
fn View(model: Store) -> UI<Msg> {
    return UI.Arrange(WideShell(), model, ShellContent)
}

fn ShellContent(model: Store, region: Shell) -> UI<Msg> {
    return switch region {
        case Shell.Header => HeaderBar(model)
        case Shell.Rail => FilterRail(model)
        case Shell.Grid => ProductGrid(model, 3)
        case Shell.Footer => Footer(model)
        case Shell.Cart => CartButton(model)
    }
}
```

Everything below `ShellContent` is the content tree from the rest of this
sketch: stacks, grids, text, buttons.

The vocabulary (*new*):

```oct
UI.Layout(rows: UI.Placement<R>[]) -> UI.Layout<R>               // R is the app's region enum
UI.Region(region: R, x: UI.XSpan, y: UI.YSpan) -> UI.Placement<R>   // Layer Content, Z 0
UI.Arrange(layout: UI.Layout<R>, context: C, content: fn(C, R) -> UI<M>) -> UI<M>

// Horizontal span: one constructor per shape.
UI.Left(left: Float<px>, width: Float<px>) -> UI.XSpan
UI.Right(right: Float<px>, width: Float<px>) -> UI.XSpan
UI.StretchX(left: Float<px>, right: Float<px>) -> UI.XSpan      // both edges; width follows the parent
UI.CenterX(width: Float<px>) -> UI.XSpan
UI.FractionX(left: Float<ui>, right: Float<ui>) -> UI.XSpan     // today's AnchorBox, one axis

// Vertical span: the same five shapes.
UI.Top(top, height)  UI.Bottom(bottom, height)  UI.StretchY(top, bottom)  UI.CenterY(height)  UI.FractionY(top, bottom)
```

Why this shape:

- **One row per region, row-major.** A region's whole geometry is one line,
  and the columns line up, so the table reads as a table. Moving the rail 2px
  up is `84px` → `82px` on the rail's row. Widget code is not opened.
- **Typed keys, checked both ways.** `UI.Layout` rejects a table that misses a
  region or places one twice, naming the region. The content function's
  `switch` is exhaustive, so a region with no content fails to compile. This
  is what JS view keys and Copeland's `bind` channel checks were reaching for,
  and here it falls out of an enum and an existing `switch` rule.
- **No capture ceremony.** `UI.Arrange` passes a context value (usually the
  model) to an ordinary named content function. An anonymous function with an
  explicit `with { model: model }` capture works too, but the named function
  is shorter and testable on its own.
- **Exactly constrained boxes by construction.** Each axis is one value from
  one constructor. JS's `AnchorFrame` has six optional edge/size fields and
  rejects bad combinations at resolve time (`InvalidAnchorHorizontal`); Oct
  has no optional fields, and the constructors make bad combinations
  unwritable.
- **Adaptation without fractions.** `StretchX`, `Right` and `Bottom` follow the
  parent, so the header and grid widen with the window while the rail stays
  240px. That is what Storefront's 13 `ui` constants were failing to say.
- **Defaults and `with`.** `UI.Region` fills `Layer: Content` and `Z: 0`;
  a row that needs more says `with { Layer: UI.Layer.Chrome }` (law 3).

Rows are ordinary values, so shared numbers are ordinary `let`s:

```oct
fn WideShell() -> UI.Layout<Shell> {
    let pad = 16px
    let gap = 12px
    let headerH = 56px
    let footerH = 40px
    let bodyTop = pad + headerH + gap
    let bodyBottom = pad + footerH + gap
    return UI.Layout([
        UI.Region(Shell.Header, UI.StretchX(pad, pad),            UI.Top(pad, headerH)),
        UI.Region(Shell.Rail,   UI.Left(pad, 240px),              UI.StretchY(bodyTop, bodyBottom)),
        UI.Region(Shell.Grid,   UI.StretchX(pad + 240px + gap, pad), UI.StretchY(bodyTop, bodyBottom)),
        UI.Region(Shell.Footer, UI.StretchX(pad, pad),            UI.Bottom(pad, footerH)),
        UI.Region(Shell.Cart,   UI.Right(24px, 160px),            UI.Bottom(pad + footerH + 16px, 48px)) with { Layer: UI.Layer.Chrome }
    ])
}
```

Use literals when the screen is simple and `let`s when edges are shared; a
taller header is then one edit to `headerH`, and the body follows because it is
arithmetic, not because anything reflowed.

**Why not a `record table` literally.** `record table` literals are
column-major: each field is one array (`Stage: [...]  Latency: [...]`). For a
layout that is Storefront's parallel-array problem again: moving the rail
means finding index 1 in three arrays. `UI.Layout` has a table's guarantees
(a fixed set of columns, validated as a whole, one row per key) but is written
row by row. If a row-major literal form for `record table` lands later (L6 in
§7, also useful for catalogs), `UI.Layout` can become a `record table` without
changing how it reads.

**Nesting.** A region's content can be another `UI.Arrange` with its own
enum and table (a card with a pinned badge, an editor with docked panes); its
spans are relative to the region's box. Most region content is stack and grid
sugar instead (§5.5, §5.6).

**One-off boxes.** For a single box where an enum would be ceremony, `UI.At`
is the one-row case and takes the same spans:

```oct
UI.Canvas([
    Card(p),
    UI.At(UI.Right(8px, 64px), UI.Top(8px, 20px), Badge("New"))
])
```

`AbsoluteBox(x, y, w, h, c)` is `At(Left(x, w), Top(y, h), c)` and
`AnchorBox(l, t, r, b, c)` is `At(FractionX(l, r), FractionY(t, b), c)`; both
stay callable outside the documented surface.

**Layers, z and nudges:**

```oct
enum Layer { Base  Content  Chrome  Overlay  Modal  Tooltip }   // declaration order = paint rank
UI.Nudge(dx: Float<px>, dy: Float<px>, child: UI<M>) -> UI<M>   // move after placement, siblings unaffected
```

Paint order is `(layer rank, z, authored order)` (Copeland). `z` is −5..5;
anything outside is an error that suggests a layer, never a clamp. The fixed
`UI.Layer` set replaces unbounded `Z` ints on `AbsoluteBoxZ`/`AnchorBoxZ`; a
library enum is simpler to write than a per-app layer declaration, and these
six cover the layer sets the audited samples declare (JS product page: base,
chrome, media; Copeland `AppLayers`: background, content, overlay, modal).
`Nudge` is JS's `offset`, for a child of sugar that has no row of its own to
edit.

### 5.5 Sugar: stacks

> **Implemented in UI-M3** with §5.6, §5.7 (style changes 1 and 2), §5.8 and
> step 0 of §5.1/§5.2 ([README](README.md#stacks-sizing-and-grids-ui-m3);
> `Experiments/Storefront/M8`). One refinement from building Storefront M8:
> an unwrapped *nested stack* is measured along its parent's axis instead of
> filling (it stretches across), because a row of labels or buttons inside a
> card took a fill share of the card twice in one app. `Text` and `Button`
> are measured as specified; `Spacer`, grids and canvases fill. State styles
> (§5.7 change 3) wait for hover and focus input (UI-M6). `UI.Button` still
> accepts a `UI.EventToken` for apps written before UI-M3.

A stack is the cursor walk you would otherwise write by hand
(`x += width + gap`): it places children one after another along an axis
using their declared sizes. It does not negotiate, shrink or wrap.

```oct
UI.Stack(spec: UI.StackSpec, children: UI<M>[]) -> UI<M>        // new
UI.Vertical() -> UI.StackSpec                                    // new: Gap 0, Padding 0, Align Start
UI.Horizontal() -> UI.StackSpec                                  // new
UI.Column(children: UI<M>[]) -> UI<M>                            // = Stack(Vertical(), children)
UI.Row(children: UI<M>[]) -> UI<M>                               // = Stack(Horizontal(), children)

concept StackSpec {
    Axis: UI.Axis               // Vertical | Horizontal
    Gap: Float<px>
    Padding: UI.Insets
    Align: UI.Align             // Start | Center | End  (cross axis)
    Justify: UI.Justify         // Start | Center | End | SpaceBetween  (main axis, leftover space)
}
```

Sizing wrappers (*new*) say how a child takes space along the stack's main
axis. "Main" is height in a vertical stack and width in a horizontal one, so
`Fixed(56px, header)` means the same thing in either (JS `M.fixed` lesson).

```oct
UI.Fixed(main: Float<px>, child: UI<M>) -> UI<M>
UI.Fill(weight: Float, child: UI<M>) -> UI<M>
UI.Auto(child: UI<M>) -> UI<M>                       // opt-in: size from measured content
UI.Size(width: Float<px>, height: Float<px>, child: UI<M>) -> UI<M>   // both axes, fixed
```

Defaults when a child is unwrapped: `Text` and `Button` are `Auto` (one line,
measured with the deterministic metric, so the same label always gets the same
box); containers are `Fill(1.0)` (today's behavior); `Spacer` is `Fill(1.0)`.
If fixed and auto children plus gaps exceed the main axis, resolution fails
with `machina.layout.stack_overflow`, naming the stack's path and the sizes
involved. It never shrinks or clips silently.

Use a stack for runs of like things inside a region: toolbar buttons, form
rows, list items, a card's lines. Use a layout table for the regions
themselves. When a stack's result is not what you wanted, the dump (§5.12)
shows each child as the spans it resolved to; turning those into rows of a
layout table is always a valid way out of the sugar.

### 5.6 Sugar: grids

```oct
UI.Grid(spec: UI.GridSpec, rows: UI<M>[][]) -> UI<M>            // matrix, as UI.GridRows today
UI.Tracks(columns: UI.Track[]) -> UI.GridSpec                    // new: default spec
UI.Span(columns: Int, rows: Int, child: UI<M>) -> UI<M>          // new
UI.Skip() -> UI<M>                                                // new: a cell covered by a span
UI.Chunk(items: UI<M>[], columns: Int) -> UI<M>[][]              // new: row-major, pads with Skip

enum Track {
    Fixed(Float<px>)
    Fill(Float)
    Auto
}

concept GridSpec {
    Columns: UI.Track[]
    Rows: UI.Track[]             // explicit rows; rows past the end use AutoRow
    AutoRow: UI.Track
    ColumnGap: Float<px>
    RowGap: Float<px>
    Padding: UI.Insets
}
```

```oct
let shell = UI.Tracks([UI.Track.Fixed(280px), UI.Track.Fill(1.0)]) with {
    Rows: [UI.Track.Fixed(56px), UI.Track.Fill(1.0)]
    ColumnGap: 16px
    RowGap: 16px
}
UI.Grid(shell, [
    [UI.Span(2, 1, TopBar(model)), UI.Skip()],
    [Sidebar(model), Content(model)]
])
```

A grid is sugar for a block of placements that share column and row edges:
each cell resolves to spans against the grid's box, and the dump prints it
that way.

Rules (from JS grid and C# matrix grid): rows must be rectangular; every cell a
span covers must be `UI.Skip()`; a span past the tracks is an error; there are
no implicit columns. `UI.Chunk` turns a dynamic list into rows in plain sight,
so the layout engine itself never auto-places.

### 5.7 Style

Style stays records plus `with` (M112/M117). Three changes:

1. **Content last**: `UI.Styled(style, node)` (*changed* from `(node, style)`)
   so the function matches law 4 and works as a markup tag.
2. **Padding leaves `UI.Style`.** It moves to `StackSpec`/`GridSpec`/
   `UI.Inset(insets, child)` (*new*). A styled container that wants inner space
   says so in its layout spec.
3. **State styles** (*new*), keyed by real interaction states that the session
   tracks (JS `S.stateful`):

```oct
fn PrimaryButton(t: Tokens) -> UI.StyleStates {
    let base = UI.DefaultStyle() with {
        Background: t.Primary
        Foreground: t.OnPrimary
        Radius: 6px
    }
    return UI.States(base) with {
        Hover: base with { Background: t.PrimaryHover }
        Pressed: base with { Background: t.PrimaryPressed }
        Focused: base with { BorderColor: t.Focus  BorderWidth: 2px }
        Disabled: base with { Background: t.Surface  Foreground: t.Muted }
    }
}

UI.StyledStates(PrimaryButton(Tokens()), UI.Button("Add to cart", Msg.AddToCart(p.Id), true))
```

Tokens are an ordinary record returned by an ordinary function, which is
explicit without threading a theme argument through every component:

```oct
concept Tokens {
    Surface: UI.Color
    Text: UI.Color
    Muted: UI.Color
    Primary: UI.Color
    // ...
}

fn Tokens() -> Tokens { return Light() }
```

When theming must vary at runtime (dark mode), the choice lives in the model and
components take `t: Tokens` as a parameter. Either way there is no ambient
context.

### 5.8 Identity and lists

```oct
UI.Keyed(key: String, child: UI<M>) -> UI<M>       // new
```

Ids stay path-derived. Under a `Keyed` node the path segment is the key instead
of the child index, so a card keeps its id when earlier cards are filtered out.
Duplicate keys among siblings are an error. Keys come from domain identity
(product id), never from display text or position (C# `UiIdFamily`).

Lists are ordinary loops, with the catalog as a `record table`:

```oct
record table Catalog {
    Id: String
    Title: String
    Subtitle: String
    Price: Float
    Category: Category
}

fn ProductGrid(model: Store, columns: Int) -> UI<Msg> {
    let catalog = Products()
    var cards: UI<Msg>[] = []
    for i in 0..Len(catalog) {
        if Visible(model, catalog[i].Category) {
            cards = Append(cards, UI.Keyed(catalog[i].Id, ProductCard(catalog, i, model)))
        }
    }
    let spec = UI.Tracks([UI.Track.Fill(1.0) ... columns]) with {
        AutoRow: UI.Track.Fixed(220px)
        ColumnGap: 16px
        RowGap: 16px
    }
    return UI.Grid(spec, UI.Chunk(cards, columns))
}
```

(`ProductCard` takes `(catalog, i)` because a `record table` row type has no
source-level name today. §7 lists naming it as a small language item.)

### 5.9 Controlled inputs

Inputs are controlled: the model owns the value, the widget proposes a change
as a message, and the session owns only transient state (focus, caret,
selection, scroll), keyed by node identity (C# widget session).

```oct
UI.Checkbox(label: String, checked: Bool, on: fn(Bool) -> M) -> UI<M>              // new
UI.TextField(value: String, on: fn(String) -> M) -> UI<M>                          // new
UI.Slider(value: Float, min: Float, max: Float, on: fn(Float) -> M) -> UI<M>       // new
UI.Select(options: String[], selected: Int, on: fn(Int) -> M) -> UI<M>             // new
```

```oct
enum Msg {
    SetAudio(Bool)
    QueryChanged(String)
    // ...
}

UI.Checkbox("Include audio", model.Audio, Msg.SetAudio)
UI.TextField(model.Query, Msg.QueryChanged)
```

Passing `Msg.SetAudio` as an `fn(Bool) -> Msg` needs payload-variant
constructors to be function values (*new*, §7; probed: rejected today with
"variant 'Select' requires one payload argument"). Without it, the same works
today with an anonymous function, which needs no capture because it reads no
locals:

```oct
UI.Checkbox("Include audio", model.Audio, fn(v: Bool) -> Msg { return Msg.SetAudio(v) })
```

### 5.10 Responsive arrangement

`UI.Viewport { Width: Float<px>  Height: Float<px> }` (*new*) is the window
size. Most adaptation needs no code at all: `StretchX`/`StretchY` and
`Right`/`Bottom` already follow the parent. When the arrangement itself must
change, choose a different table for the same regions and keep the content:

```oct
fn View(model: Store, viewport: UI.Viewport) -> UI<Msg> {
    let layout = if viewport.Width < 820px { NarrowShell() } else { WideShell() }
    return UI.Arrange(layout, model, ShellContent)
}
```

Both tables place the same `Shell` regions, so `ShellContent` is written once.
This is the content reuse across arrangements that Copeland built `stream` and
`bind using` for (UI-GRAMMAR-M3), here as two functions returning values. Copeland reached the same place
(`if (state.narrow) return bind Compact using Content`); a variants DSL
(`M.when`) adds a second way to write an `if`. Content shared by both shells is
built once with `let` and passed in.

### 5.11 The markup form

Oct-XML is the same calls with named attributes, which is why law 4 matters:

```oct
fn ProductCard(catalog: Catalog, i: Int, model: Store) -> UI<Msg> {
    let p = catalog[i]
    return <UI.Styled Style={CardStyle(model.Selected == Option.Some(p.Id))}>
        <UI.Stack Spec={UI.Vertical() with { Gap: 6px  Padding: UI.InsetsAll(12px) }}>
            <UI.Row>
                <UI.Fill Weight={1.0}><UI.Text Content={p.Title} /></UI.Fill>
                <UI.Text Content={Money(p.Price)} />
            </UI.Row>
            <UI.Paragraph Content={p.Subtitle} />
            <UI.Spacer />
            <UI.Row>
                <UI.Button Label="Inspect" On={Msg.Inspect(p.Id)} Enabled={true} />
                <UI.Button Label="Add to cart" On={Msg.AddToCart(p.Id)} Enabled={true} />
            </UI.Row>
        </UI.Stack>
    </UI.Styled>
}
```

Two markup items are needed (§7): single-child bodies (`Styled`, `Fill`,
`Keyed` take one `UI<M>`, not an array), and `MarkupTextUI` so bare text inside
a container becomes `UI.Text`. Markup stays optional; function calls remain the
default because they are shorter for small widgets and computed lists
(Copeland survey reached the same split).

### 5.12 Inspection and tests

`oct ui check` gains `--dump` (*new*): one line per node, in paint order,
indented by depth, with path id, kind, label and message alias, and its
geometry **in the authoring vocabulary**: the spans that place the box in its
parent, plus the absolute rect. A region shows its table row; a sugar child
shows what the sugar decided, in the same spans. Nothing about layout needs a
separate inspector to explain it. Illustrative format:

```text
Shell.Header           StretchX(16px, 16px)   Top(16px, 56px)          (16,16) 992x56
  .0                   Stack(h, gap 8px)
    .0.0               Left(0px, 72px)        Top(12px, 32px)          Button "Shop" -> Msg.Nav(Section.Shop)
Shell.Grid             StretchX(268px, 16px)  StretchY(84px, 68px)     (268,84) 740x616
  .0.harbor-board      Left(0px, 232px)       Top(0px, 220px)          Styled
Shell.Cart  [Chrome]   Right(24px, 160px)     Bottom(72px, 48px)       (840,648) 160x48
```

`.octest` gets the same view headlessly (*new*):

```oct
[Fact]
fn RaiseMovesSetpoint() {
    let app = UI.Test.Start(Init(), Update, View)
    let next = UI.Test.Click(app, "Raise")!
    Assert.Equal(next.Model.Setpoint, 20.5C)
    Assert.Contains(UI.Test.Dump(next), "Setpoint: 20.5 °C")
}
```

A dump is something a model can read and compare without a screenshot, which
makes it the primary loop for LLM-authored UI. Pixel checks stay for backends.

## 6. Storefront, before and after

What disappears from Storefront M7 under this sketch, by category (counts from
the M7 source):

| M7 construct | Lines (approx.) | Replacement |
|---|---|---|
| 19 `...Event() -> UI.EventToken` functions + 2 prefix functions | 26 | one `enum Msg` (≈10) |
| 4 `EventValueDispatch` tables | 27 | none: `match` |
| `Update` probe chain with `UI.ResolveEventValue` / `MatchEventPrefix` | 25 | exhaustive `match` (≈12) |
| `KeyLabelMapping` ×2 + `ResolveLabelMapping` + label fns | 35 | exhaustive `switch` label fns |
| `ProductCatalog` parallel arrays + `ProductById` search | 45 | `record table Catalog` literal (≈18) + index loop |
| `SourceParts` / `SourceStructure()` | 20 | none: the file's order is the parts list |
| 13 `ui` fraction constants + `Canvas` of `AnchorBox` | 22 | a region enum, a layout table and a content `switch` (≈22), in pixels, adapting by stretch (§5.4) |

That is roughly 200 of 454 lines replaced by about 90 (placement stays about
the same length; it moves into one table and starts meaning pixels), before any visual
improvement (gaps, padding, a real grid, styled cards). The estimate is from
the source, not from a rewrite; rewriting Storefront is the acceptance test
proposed for UI-M3 below, and its line count is the measurement.

**Measured in UI-M3** (`Experiments/Storefront/M8`): 315 lines against M7's
454 (271 against 388 without blank and comment lines), with filtering,
sorting, styled cards and a page that actually lays out: M7's fraction
canvas fails `oct ui check` with a stack overflow (its catalog header row
does not fit the fraction box it was given). No
event strings, dispatch tables or `ui` fractions remain, and a keyed card
keeps its id when earlier cards are filtered out.

## 7. What this needs

### Language

| Item | Needed by | Cost | Without it |
|---|---|---|---|
| **L1. `UI<M>` builtin view type** typed by place, like `Option<T>` | §5.1 typed messages | Typechecker: builtin parametric type, UI builtins become compiler-polymorphic (like `Array.Where`); library wrappers in `Libraries/UI` move to builtins | See step 0 below |
| **L2. Payload-variant constructors as function values** (`Msg.SetAudio : fn(Bool) -> Msg`) | §5.9 inputs | Small: a qualified variant in value position with a single payload | Anonymous fn per input (works today) |
| **L3. Oct-XML single-child body** (final param `T`, exactly one element child) | §5.11 | Small, in elaboration only | `Child={...}` attribute |
| **L4. Oct-XML text adapter for UI** (`MarkupTextUI`) | §5.11 | Library function, no compiler change | `<UI.Text Content=... />` |
| **L5. Name for a `record table` row type** (e.g. `Catalog.Row`) | §5.8 | Small | Pass `(table, i)` |
| **L6. Row-major `record table` literal** (one row per line) | §5.4 note, catalogs | Medium: parser and table construction | `UI.Layout` stays a library table type |

L1 also covers `UI.Layout<R>`, `UI.Placement<R>` and `UI.Arrange`: they are
parametric over the app's region enum `R` (and `UI.Arrange` over the context
type `C`) the same compiler-owned way `UI<M>` is over `M`. `UI.Layout` checks
that every variant of `R` appears exactly once: at compile time for a literal
table, at construction otherwise.

**Step 0, before L1.** Messages can be enum values without `UI<M>`: keep `UI`
untyped, let `UI.Button` take any enum value as a compiler-polymorphic
argument, store the value in the action table, and have `octapp` check at load
that `Update`'s second parameter is an enum and, at dispatch, that the value is
of that enum. That is days, not weeks, removes every string in Storefront, and
moves the type error from compile time to the first click. L1 then makes it a
compile error. Recommendation: do step 0 first, then L1, so the ergonomics land
before the type-system work.

### Runtime and backend

| Item | Needed by | Notes |
|---|---|---|
| R1. Action table holds Oct values; alias printed from value | §5.2 | `octapp.Dispatch` passes the value to `Update` |
| R2a. Layout tables, `Arrange` and `At` with per-axis spans: px edges, stretch, center, fraction | §5.4 | The one real engine change: `layout.resolveFrame` resolves `AnchorFrame` as four fractions only; it needs per-edge px/`ui` plus size, as JS `AnchorFrame` does. Regions lower to ordinary anchor rows |
| R2b. Stack/grid spec, sizing wrappers, spans, `Nudge`, `Keyed` into UIIR and lowering | §5.5, §5.6, §5.8 | The Go layout engine already resolves gap, padding, fixed, fill, tracks and spans; this is mostly lowering. `Nudge` is JS `offset` |
| R3. Text measurement seam: backend measurer → layout | §5.3, §5.5 `Auto`, overflow policy | Gio can measure; headless keeps the deterministic 8 dp metric so dumps stay stable |
| R4. Hover, focus, keyboard and text entry in the Gio backend | §5.7 states, §5.9 | Session-owned transient state keyed by node id |
| R5. Layer/z paint ordering in render | §5.4 | |
| R6. `--dump` (in `At` vocabulary) and `UI.Test.*` | §5.12 | Dump format first; it becomes the test oracle |
| R7. UI builtins in the compiled lane | all | Already a known gap (UI-M0 report) |

## 8. Deliberately not adopted

- **Copeland `layout` / `layout type` / `bind` / `stream` syntax.** The layout
  table takes the idea that worked there (geometry declared apart from
  content, bound by checked names, reusable across arrangements) and
  expresses it with an enum, a library table and a `switch`. No new syntax,
  no origin headers, no topology contracts. Copeland's syntax cost lines
  (275 vs 175); this should cost about one line per region.
- **Authored layout rows, string slot keys, view registries.** Rows stay the
  MIR. The layout table is keyed by an enum the compiler checks, holds only
  geometry, and never carries content or data maps.
- **Guides, constraints, derivation engines** (JS `GuideFrame`, Copeland
  `with placeBelow(dialog, 8px)`). Relative geometry is arithmetic in `let`s
  (law 7): no single-writer rules, no cycle detection, no solver.
- **Dispatch tables as a UI pattern.** They were the string transport's
  workaround; `match` over an enum is the table, checked for completeness.
- **Default and named arguments.** Records with `with` already cover
  configuration (law 3). If spec construction turns out noisy in practice, the
  smaller change is defaulted record fields, not optional parameters.
- **Variants DSL, router, store, hooks, effects, component lifecycle.**
  Ordinary `if`, the model, and `Update` cover them. Async effects (loading,
  saving) are a separate design that should reuse Oct's `async` and flows.
- **CSS-like layout**: flex negotiation, margins, percent strings, implicit
  grid tracks, z-index as architecture, silent clamping, and any layout engine
  whose answer you have to inspect instead of read.

## 9. Decisions (accepted as recommended)

1. **Typed messages**: step 0 (enum values, runtime-checked) then L1 `UI<M>`,
   or go straight to L1? Recommendation: step 0 first.
2. **Breaking renames**: change `UI.Styled` to `(style, node)`, move `Padding`
   out of `UI.Style`, and drop the compatibility aliases from the documented
   surface (keeping them callable for a deprecation window)? Recommendation:
   yes; the surface is young and the aliases cost every author a choice.
3. **Fixed layer set** (`UI.Layer`) instead of per-app layer declarations?
   Recommendation: fixed set.
4. **Acceptance test**: rewrite Storefront under the new surface as the UI-M3
   gate, measured by line count, a clean `--dump`, and the existing click
   scenarios?
5. **Layout table shape**: `UI.Layout` as a row-major library table now, with
   a row-major `record table` literal (L6) as an optional later unification?
   Recommendation: yes; a column-major literal reintroduces parallel arrays.
6. **`Auto` default for text and buttons inside stacks**: keep it (labels size
   to their measured text, deterministically), or require every stack child to
   be wrapped? Recommendation: keep it; it is the one place content sizes a
   box, it only happens inside sugar, and the dump shows the resulting `At`.

## 10. Proposed milestones

| Milestone | Content | Gate |
|---|---|---|
| UI-M2 (**done**) | Layout tables (`UI.Layout`, `UI.Region`, `UI.Arrange`), per-axis spans and `UI.At` (R2a), layers and z (R5), `--dump` in span vocabulary (R6) | Storefront shell as one table; moving a region is a one-row edit; resize keeps the rail at 240px; dump golden tests |
| UI-M3 (**done**) | Step 0 typed messages (R1); stack and grid sugar, `Fixed`/`Fill`/`Auto`, `Nudge`, `Keyed`, padding moved to layout (R2b) | Storefront rewrite: no strings, no `ui` fractions, keyed cards survive filtering, fewer lines |
| UI-M4 (**done**) | Text measurement seam, `Paragraph`, `TextSpec`, overflow policy (R3) | No clipped or wrapped-out-of-view labels at 1×/2× scale |
| UI-M5 | `UI<M>` (L1), variant constructors as values (L2), markup items (L3, L4) | A wrong-message button is a compile error |
| UI-M6 | Inputs and focus (R4), state styles | Form sample: checkbox, text field, slider round-trip through `Update` |
| UI-M7 | Overlays (dialog, popover, toast) on the modal/tooltip layers, `View(model, viewport)` | Dialog over page with focus trapped; narrow/wide shell switch on resize |

Each milestone also updates `Libraries/UI/README.md` and the reference pages it
touches, and marks this spec's corresponding sections implemented.
