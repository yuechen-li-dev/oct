# UI standard library (Machina UI)

`UI` is Oct's authoring surface for Machina UI (Machine Native UI): views are
values, page geometry is declared in layout tables, and stacks and grids are
sugar over the same explicit rectangles. The design and its milestone history
are in [`DESIGN.md`](DESIGN.md); this README documents what is implemented.
Use `UI.*` functions; raw `UI*` builtins are backing details.

| File | Contents |
|---|---|
| `UI.View.oct` | `UI.Signature`; the compiler-owned button and inputs |
| `UI.Place.oct` | Spans, `UI.At`, `UI.Canvas`, layers, layout tables |
| `UI.Stack.oct` | Stacks, `Column`/`Row`/`Spacer`, sizing, `Nudge`, `Inset`, `Keyed` |
| `UI.Grid.oct` | Tracks, `UI.Grid`, spans, `UI.Chunk` |
| `UI.Text.oct` | `Text`, `Paragraph`, `TextSpec`, overflow, `MarkupTextUI` |
| `UI.Style.oct` | Colors, insets, `Style`, `Styled`, `StyleStates`, `StyledStates` |

Tests are named after the file they cover (`UI.Stack.octest`), and `.octfail`
contracts after the area and the rule (`UI.Messages.WrongMessageButton.octfail`).

## Apps and messages

An app declares its model, its message enum, and three functions:

```oct
enum Msg {
    Raise
    Select(String)
}

fn Init() -> Model
fn Update(model: Model, msg: Msg) -> Model
fn View(model: Model) -> UI<Msg>
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
  may carry a payload. A button that sends `Msg` is a `UI<Msg>` (see
  [Typed views](#typed-views)).
- The button keeps the message value; `Update` receives the value itself.
  Its printed form (`Msg.Select("harbor-board")`) is a tool-facing alias for
  dumps and `oct ui check --click`; nothing parses it back.
- `oct ui` checks when the app loads that `Update`'s second parameter is the
  app's message enum and that `View` returns `UI<Msg>` for that same enum.
  The typechecker has already rejected a button of any other enum.

`Examples/MachinaThermostat` is the smallest app; `Experiments/Storefront/M8`
is a full storefront on this surface.

## Typed views

A view's type says which message its buttons send: `UI<Msg>`. The compiler
checks it, so a button of the wrong enum is a compile error rather than an
error at the first click.

```oct
fn Toolbar(model: Model) -> UI<Msg> {
    var items: UI<Msg>[] = [UI.Text("Sections")]
    for i in 0..Len(model.Sections) {
        items = Append(items, UI.Button(model.Sections[i], Msg.Open(i), true))
    }
    return UI.Row(items)
}

// Bare UI: works with any message type, and returns its argument's.
fn Card(title: String, body: UI) -> UI {
    return UI.Stack(UI.Vertical(), [UI.Text(title), body])
}
```

- A view with no button of its own (`UI.Text`, `UI.Spacer`, a stack of
  them) goes into a `UI<M>` of any `M`, as `Option.None` goes into any
  `Option<T>`. A `UI<A>` never becomes a `UI<B>`.
- A container's message is its children's. Children that send two different
  enums are an error: `UI<Msg> and UI<Other> cannot be in one view`.
- Bare `UI` in a function's signature is message-polymorphic. All bare-`UI`
  parameters and the bare-`UI` result of one call share the arguments'
  message, so `Card("Actions", saveButton)` is a `UI<Msg>`. Every wrapper in
  this library (`UI.Styled`, `UI.Stack`, `UI.Fill`, `UI.Keyed`, `UI.Chunk`...)
  is written this way. Inside such a function the message is unknown, so it
  cannot add a button of one enum: a helper that sends messages is declared
  `-> UI<Msg>`, and the compiler says so.
- Elsewhere bare `UI` is a view with no messages: a record field, an enum
  payload, or a local in a function whose signature has no bare `UI`. A list
  that will hold buttons is declared `var items: UI<Msg>[] = []`; `var items
  = [UI.Text("a")]` is a messageless list.
- A payload variant written without its payload is a function value:
  `Msg.Open` is a `fn(Int) -> Msg`. UI inputs will take it as their
  callback, as in `UI.Checkbox("Audio", model.Audio, Msg.SetAudio)`.

## Inputs and focus

Inputs are controlled: the model owns the value, the input shows it and
proposes a change by calling its `on` function, and `Update` decides what
the model becomes. A payload variant is such a function, so most inputs name
their message directly:

```oct
enum Msg {
    Rename(String)
    SetAudio(Bool)
    SetVolume(Float)
}

UI.TextField(model.Name, Msg.Rename)
UI.Checkbox("Include audio", model.Audio, Msg.SetAudio)
UI.Slider(model.Volume, 0.0, 1.0, Msg.SetVolume)
```

```oct
UI.Checkbox(label: String, checked: Bool, on: fn(Bool) -> M) -> UI<M>
UI.TextField(value: String, on: fn(String) -> M) -> UI<M>
UI.Slider(value: Float, min: Float, max: Float, on: fn(Float) -> M) -> UI<M>
```

- `on` may be any function of that type, including an anonymous one
  (`fn(on: Bool) -> Msg { return Msg.SetAudio(on) }`). Its result type is the
  input's message type, so a `Msg.SetAudio` checkbox is a `UI<Msg>` and joins
  its view like a button.
- A checkbox proposes the opposite of `checked` when pressed or when it has
  focus and Space or Enter is pressed. A text field proposes its value with
  each edit: typed text goes in at the caret, and Backspace and Delete remove
  a character. A slider proposes the value under the pointer when pressed and
  while dragged, and moves by a twentieth of its range with the arrow keys
  (Home and End go to `min` and `max`). `min` must be below `max`, and `value`
  inside them.
- The session owns only transient state, keyed by node identity: which
  control is hovered, which is pressed, which has keyboard focus, and the
  caret in a focused text field. A node keeps it while it keeps its id, so
  give a repeated input `UI.Keyed` if earlier siblings come and go. Tab and
  Shift+Tab move focus through buttons and inputs in paint order; Escape or a
  press on nothing clears it; Enter and Space press a focused button.
- A text field is 200 dp wide and 32 dp tall, a slider 200 dp by 24 dp, and a
  checkbox is its box and label; size them with `UI.Fixed`, `UI.Fill` or
  `UI.Size` like any stack child.
- There is no select or radio input yet.

`oct ui check` can script a session: `--click` presses a control (by label,
or by the message or callback it sends, such as `Msg.Rename`), `--type`
types into the focused text field, and `--key` presses a key, each printing
the next frame. `Examples/MachinaForm` is a settings form on these inputs.

## API at a glance

```oct
UI.Text(content: String) -> UI
UI.Button(label: String, message: Msg, enabled: Bool) -> UI<Msg> // Msg: any enum
UI.Spacer() -> UI
UI.Styled(style: UI.Style, node: UI) -> UI

// Placement
UI.Canvas(children: UI[]) -> UI
UI.At(x: UI.XSpan, y: UI.YSpan, child: UI) -> UI
UI.Region(region: R, x: UI.XSpan, y: UI.YSpan) -> UI.Placement
UI.Layout(rows: UI.Placement[]) -> UI.LayoutTable
UI.Arrange(layout: UI.LayoutTable, context: C, content: fn(C, R) -> UI<M>) -> UI<M>

// Stacks and grids
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

// Inputs and state styles
UI.Checkbox(label: String, checked: Bool, on: fn(Bool) -> M) -> UI<M>
UI.TextField(value: String, on: fn(String) -> M) -> UI<M>
UI.Slider(value: Float, min: Float, max: Float, on: fn(Float) -> M) -> UI<M>
UI.States(base: UI.Style) -> UI.StyleStates
UI.StyledStates(states: UI.StyleStates, node: UI) -> UI

UI.Signature(node: UI) -> String
```

Every `UI` above is bare, so each function returns the message type of its
UI arguments: `UI.Stack(spec, children)` with `UI<Msg>[]` children is a
`UI<Msg>` ([Typed views](#typed-views)).

## Placement and layout tables

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

fn View(model: Store) -> UI<Msg> {
    return UI.Arrange(WideShell(), model, ShellContent)
}

fn ShellContent(model: Store, region: Shell) -> UI<Msg> {
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
UI.Arrange(layout: UI.LayoutTable, context: C, content: fn(C, R) -> UI<M>) -> UI<M>
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
  time. The content function's `switch` is checked at compile time like any
  `switch`.
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

A `UI.Canvas` is a placement surface: each child is placed by `UI.At`, and an
unplaced child is a lowering error. Inside a placed box, use content trees
(stacks, grids, text, buttons). A component function returns content; a
helper that calls `UI.At` is writing page layout.

## Stacks and sizing

A stack places children with declared sizes and lowers to the same explicit
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
- Text and buttons are measured with the session's text metrics (see Text
  below); buttons add 24 dp around their label and are at least 80 dp wide.
- `UI.Nudge(dx, dy, child)` moves a node after placement without moving its
  siblings.
- `UI.Inset(insets, child)` puts space around one child. Padding is layout,
  never style.

`UI.Keyed(key, child)` gives a repeated child its id from a domain key instead
of its position, so a card keeps its id when earlier cards are filtered out.
Keys must be unique among siblings.

## Grids

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

## Text

Text lives in the box it is given. Measurement decides how the box's text
survives, never how big the box is (except where a stack measures a child).

```oct
UI.Text(content: String) -> UI                      // one line of body text
UI.TextWith(spec: UI.TextSpec, content: String) -> UI
UI.Paragraph(content: String) -> UI                 // body text that wraps

UI.Body() / UI.Title() / UI.Caption() / UI.Mono() -> UI.TextSpec

record TextSpec {
    Size: Int<px>            // 16 body, 22 title, 12 caption, 14 mono
    Weight: Int              // 400 regular, 700 bold
    Mono: Bool
    Wrap: Bool
    Overflow: UI.Overflow    // Ellipsis | Clip | Error
    Color: Option<UI.Color>  // None: the styled foreground or the default
}
```

```oct
UI.TextWith(UI.Caption() with { Color: Option.Some(UI.Rgb(110, 116, 128)) }, "Updated today")
UI.TextWith(UI.Body() with { Overflow: UI.Overflow.Error }, order.Id)
```

- **Wrapping.** Text with `Wrap: true` (`UI.Paragraph`) breaks at spaces to
  the width it is given, and inside a word only when the word alone is too
  wide. In a vertical stack it takes the stack's width and is exactly as tall
  as its lines, so what follows moves down as the window narrows; a stack
  that contains it is measured the same way. In a horizontal stack it fills
  (`UI.Fill(1.0)`). In a box it shows the lines that fit.
- **Overflow.** When text does not fit its box: `Ellipsis` (the default)
  shortens the line, and ends the last visible line of wrapped text with
  `…`; `Clip` draws what fits and clips the rest; `Error` fails the frame,
  naming the node and the shortfall. Button labels are shortened with an
  ellipsis.
- **Metrics.** Layout, wrapping and truncation all measure with one set of
  text metrics, so what is drawn is what was measured. The native window
  measures with the font it draws (the Go font collection). `oct ui check`,
  `--dump` and tests use fixed metrics so their output is deterministic:
  each character is half the font size wide (0.6 for `Mono`) and a line is
  1.25 times the size, which is 8 dp and 20 dp at the body size. The fixed
  metric is a little wider than the Go font, so text that fits in
  `oct ui check` also fits in the window. If a frame does not fit with the
  window's metrics, the window keeps the fixed-metric frame and says why.

## Styles

Styles are immutable records:

- `UI.Color`
- `UI.Insets` (used by stack and grid padding)
- `UI.TextSpec` (see Text)
- `UI.Style`

Helper constructors:

- `UI.Rgb(r, g, b)`
- `UI.Rgba(r, g, b, a)`
- `UI.InsetsAll(value)`
- `UI.InsetsXY(x, y)`
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

### Applying a style

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
`UI.Inset`). Text size, weight and color are a `UI.TextSpec`.

```oct
let badge = UI.Styled(UI.DefaultStyle() with {
    Background: UI.Rgb(255, 152, 0)
    Foreground: UI.Rgb(255, 255, 255)
    Radius: 4px
}, UI.Text("Status: heating"))
```

### State styles

A control can declare how it looks in each interaction state the session
tracks. Start every state from a base style with `UI.States` and change the
ones that differ:

```oct
fn PrimaryStates() -> UI.StyleStates {
    let base = UI.DefaultStyle() with { Background: UI.Rgb(59, 110, 214)  Foreground: UI.Rgb(255, 255, 255)  Radius: 6px }
    return UI.States(base) with {
        Hover: base with { Background: UI.Rgb(47, 92, 186) }
        Pressed: base with { Background: UI.Rgb(33, 70, 150) }
        Focused: base with { BorderColor: UI.Rgb(20, 40, 90)  BorderWidth: 2px }
        Disabled: base with { Background: UI.Rgb(236, 239, 244)  Foreground: UI.Rgb(150, 154, 162) }
    }
}

UI.StyledStates(PrimaryStates(), UI.Button("Save", Msg.Save, model.Name != ""))
```

- The first state that holds wins: `Disabled`, then `Pressed`, then
  `Focused`, then `Hover`, else `Normal`. States do not combine.
- A node with state styles draws no default chrome or focus ring; its states
  say how it looks. Without them, buttons darken when hovered and pressed and
  focused controls get an accent ring.
- The states are part of the node's signature.

## Markup

Views can also be written in Oct-XML ([typed markup](../../Language/reference/language/typed-markup.md)).
Buttons and inputs are tags whose attributes are their parameters
(`<UI.Checkbox Label="Audio" Checked={model.Audio} On={Msg.SetAudio} />`).
Containers take their children as an array, wrappers that take one `UI`
(`UI.Styled`, `UI.Fill`, `UI.Keyed`...) take exactly one child, and bare text
among the children of a container is a `UI.Text` (through `MarkupTextUI`):

```oct
<UI.Styled Style={CardStyle()}>
    <UI.Stack Spec={UI.Vertical() with { Gap: 6.0 px }}>
        Desk Starter Grid
        <UI.Button Label="Add to cart" On={Msg.AddToCart(id)} Enabled={true} />
    </UI.Stack>
</UI.Styled>
```

Function calls remain the default; markup suits deep, mostly static trees.
`Examples/MachinaThermostat/main.octest` builds the thermostat's view both
ways and checks they are the same tree.

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

fn View(model: Counter) -> UI<Msg> {
    return UI.Stack(UI.Vertical() with { Gap: 8.0 px  Padding: UI.InsetsAll(16px) }, [
        UI.Text("count=" + FormatFloat(Float(model.Count), 0)),
        UI.Button("Increment", Msg.Increment, true)
    ])
}
```

## Retired surface

The string-event surface the library started with is gone: `UI.EventToken`,
`UI.Token`, `UI.ButtonRaw`, the `UI.EventValueDispatch` helpers, the `UI.Counter`
example, two-step `UI.Place`/`UIBox` placement, `UI.AbsoluteBox` and
`UI.AnchorBox` (use `UI.At` with `Left`/`Top` or `FractionX`/`FractionY`
spans), `UI.GridRows` (use `UI.Grid`), and the `UI.Mount`/`UI.Patch`/`UI.Emit`/
`UI.DrainEvents` bridge wrappers. `oct ui` apps take a message enum in
`Update`; the `event: String` form is no longer accepted.
`Experiments/Storefront/M7` keeps its own copies of the helpers it used, so
that record of the old surface still runs as tests.
