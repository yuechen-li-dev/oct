# Standard Libraries

## Overview

Oct standard libraries are the intended practical API surface for most programs.
Use these modules before reaching for low-level wrapper builtins directly.

This page summarizes the standard-library surface and its ownership boundaries.
For core language/runtime builtins, see [09 builtins](./09-builtins.md).
For Prometheus experimental APIs, see [23 Prometheus](../runtime/23-prometheus.md).

## Core practical modules

Common modules in the standard-library path include:

- `IO.File`
- `IO.Path`
- `IO.Directory`
- `IO.Json`
- `IO.Csv`
- `IO.Xlsx`
- `Archive.Zip`
- `Compression.Gzip`
- `Hash.Core`
- `Image.Core`
- `Plot.Core`
- `Pdf.Core`
- `Text.Regex`
- `Time.Core`

Module source locations (canonical in-repo docs/tests live with library code):

- `Libraries/IO/`
- `Libraries/Archive/`
- `Libraries/Compression/`
- `Libraries/Hash/`
- `Libraries/Image/`
- `Libraries/Plot/`
- `Libraries/Pdf/`
- `Libraries/Text/`
- `Libraries/Time/`

## Plotting tiers (important)

Oct plotting is intentionally split into two tiers:

- **Convenience plotting builtins (no imports):** `PlotLine` and `PlotScatter` are directly available for fast single-series PNG output.
- **Advanced plotting library (imports required):** `Plot.Core` adds explicit plot sizing (`Int<px>`), title/axis labels, legend labels, and histogram support.

Use convenience builtins when you want one-line quick output.
Import and use `Plot.Core` once you need richer control.

Convenience builtin example (no import):

```oct
package Main

fn Main() -> Int {
    return PlotLine([0.0, 1.0, 2.0], [0.0, 1.0, 4.0], "quick.png")
}
```

Advanced library example (requires import):

```oct
package Main

import Plot

fn Main() -> Int ! Error {
    let size = Plot.Size { Width: 800px Height: 600px }
    let labels = Plot.Labels { Title: "Signal" X: "t" Y: "amplitude" Legend: "series-a" }
    return Plot.Line([0.0, 1.0, 2.0], [0.2, 0.9, 1.7], "advanced.png", size, labels)?
}
```

## PDF output posture (important)

`Pdf.Core` is pixel-native composition that outputs PDF:

- page size uses `Int<px>`
- text/image placement uses `Int<px>`
- wrapper internals map pixels to PDF units

Use `Pdf.Core` when you want deterministic page composition in pixel space and PDF as the sink format.

## Usage posture

- Prefer module functions from `Libraries/*` for day-to-day application code.
- Treat direct wrapper builtin calls as low-level boundary tools.
- Keep business logic in Oct library/module code, not in builtin-specific glue.

## Backend support builtins (implementation detail)

Some builtins exist primarily to support standard-library modules and wrapper boundaries.
Examples include file/path/directory/json/csv/zip/gzip/hash/image/regex/time/xlsx/plotting-oriented builtins.

These are valid runtime primitives, but they are not the primary user-level programming story.
The primary user-facing story is the module layer (`IO.*`, `Archive.*`, `Compression.*`, `Hash.*`, `Text.*`, `Time.*`).

A standard library is ordinary Oct source that calls these builtins. Each of its functions has one definition, its source body, and both execution lanes run that body. The lanes differ only in how they carry out a builtin:

- The interpreted lane implements the builtin inside `oct`. It needs no sidecar.
- The compiled lane sends the builtin to one of the first-party Octxiliary sidecars (`octxiliary-archive`, `-compression`, `-csv`, `-hash`, `-image`, `-io`, `-json`, `-pdf`, `-plot`, `-text`, `-time`, `-xlsx`). A compiled program finds a sidecar beside its executable or through `OCT_WRAPPER_PATH`, and reports the sidecar's name when it is missing.

The sidecar is how the compiled lane implements a builtin; it is not a package wrapper. The standard libraries declare no `Wrappers` in their manifests, and a call to one of their functions is not a native operation during artifact evaluation. A direct call to one of these builtins compiles for the same reason the library's call does.

Eight builtins of this group have no compiled implementation yet, and a program that reaches one is refused by the compiled lane with the builtin's name:

- `PdfDrawImage` and `PdfDrawImageSized` take a page and an image, each a handle of a different sidecar. `Pdf.DrawImageBytes` with `Image.EncodePng` is the form both lanes run.
- `JsonLower` and `JsonLoadStructured`, the structured JSON helpers.
- `CsvWriteTable` and `CsvWriteMatrix`.
- `PlotLine` and `PlotScatter`, the short forms without size and labels. `Plot.Line` and `Plot.Scatter` run in both lanes.

Manifest wrapper functions are a different thing: native code outside the toolchain, defined by a package's `manifest.oct` alone. See [33 oct pkg](../tooling/33-oct-pkg.md).

## Notes on current documentation boundaries

- The builtin reference intentionally no longer carries the full wrapper catalog; that content is conceptually owned by this page.
- If a library module exists in `Libraries/` but lacks matching detailed reference coverage under `Language/reference`, treat that as a documentation gap to close incrementally.


## UI (Machina UI authoring surface)

`Libraries/UI` is the canonical standard-library authoring surface for Machina UI (Machine Native UI) in Oct.

Use `UI.*` in Oct programs (`UI.Text`, `UI.Button`, `UI.Row`, `UI.Column`, `UI.Canvas`, `UI.Grid`, `UI.GridRows`, `UI.Spacer`, placement helpers, and mount/patch/emit wrappers). `UI.GridRows(UI[][])` is the M114 nested-array deterministic grid API (rows x columns, rectangular only).
Treat raw UI builtins (`UIText`, `UIButton`, `UIPlaceAbsolute`, etc.) as backing runtime implementation details.

M0 scope is semantic UI construction plus current absolute/anchored box placement.
Layout is unit-aware:
- `UI.AbsoluteBox(..., child)` uses `Float<px>` and is canonical for canvas placement
- `UI.AnchorBox(..., child)` uses `Float<ui>` and is canonical for canvas placement

No CSS/class/cascade system is part of M0.

M112 adds a small immutable typed style-data surface in `Libraries/UI` (`UI.Color`, `UI.Insets`, `UI.TextStyle`, `UI.Style` plus deterministic helper constructors). This remains data-only; style application in lowering/render pipelines is future work.

Small dispatch helpers are available for explicit update functions: `UI.EventValueDispatch`, `UI.ResolveEventValue`, and `UI.MatchEventPrefix`. These helpers are deterministic and do not provide generic record mutation or state-framework behavior.

UI event authoring is nominal: `UI.EventToken` values are used in `UI.Button` and `UI.UIEvent` APIs. Runtime bridge/event drain surfaces may remain string-backed (`UI.DrainEvents` returns `String[]`) for compatibility.

## String

`Libraries/String/String.Core.oct` provides deterministic report-focused text helpers.

Canonical namespaced surface:

- `String.ByteLength`
- `String.RuneCount`
- `String.Join`
- `String.Concat`
- `String.From<T>`
- `String.ReplaceAll`
- `String.Contains`
- `String.StartsWith`
- `String.EndsWith`
- `String.Trim`
- `String.SplitLines`
- `String.EscapeJson`
- `String.QuoteJson`

Examples:

```oct
import String
let summary = String.Concat(["samples=", String.From<Int>(sampleCount)])
let scalar = String.From<Float>(value)
let reportText = String.Join(lines, "\n")
```

Notes:
- `String.From<T>` is compiler-known constrained generic syntax (closed contracts), not user-defined generic support; `Int`, `Float`, `Bool`, and `String` are supported in interpreted and compiled execution.
- `ToString(...)` remains available for compatibility, but `String.From<T>` is preferred in report/library code.
- `String.From<T>` intentionally excludes enums, records, arrays, and dimensioned numeric values such as `Float<K>`; use `FormatFloat` for dimensionless `Float` precision control and keep unit-aware formatting separate.
- Compatibility globals/backing aliases remain available during transition and should not be the preferred authoring style.

Artifact guidance:
- Prefer `Artifact.Write*` in `[Artifact]` functions.
- `Artifact.Write*` is a build-phase capability and is rejected during ordinary runtime execution.
- Paths are relative to the `oct artifact --output-root`; absolute, escaping, and duplicate paths are rejected.
- `IO.*`, `Csv.*`, `Json.*`, and `WriteOctagon` remain ordinary runtime APIs. Only legacy global `WriteOctagon` and confined directory creation are adapted to the artifact capability during the build phase.

## Random

`Libraries/Random` provides reproducible pseudorandom draws. Every draw is a
pure function of a stream, an index and the draw's parameters. There is no
generator state, so nothing is threaded between draws.

```oct
import Random

fn Readings(truth: Float, seed: Int, n: Int) -> Float[] {
    let noise = Random.Seeded(seed)
    let jitter = Random.Fork(noise, "jitter")
    let spikes = Random.Fork(noise, "spike")
    var readings: Float[] = []
    for i in 0..n {
        readings = Append(readings, truth + Random.Normal(jitter, i, 0.0, 0.2) + Random.Spike(spikes, i, 0.1, 3.0))
    }
    return readings
}
```

Rules:

- `Random.Stream` is a record value. Make one with `Random.Seeded(seed: Int)`, and derive independent streams with `Random.Fork(stream, label: String)` or `Random.Child(stream, index: Int)`.
- A draw takes `(stream, index, parameters...)`. The same arguments always give the same result, in any order of evaluation and in both execution lanes.
- Use one `Fork` per independent source of randomness. Changing the parameters of one draw never changes another draw.
- Native draws: `Random.Unit(s, i) -> Float` in `[0, 1)`; `Random.Between(s, i, lo: Float, hi: Float) -> Float` in `[lo, hi)`; `Random.IntBetween(s, i, lo: Int, hi: Int) -> Int` on the closed range; `Random.Normal(s, i, mean: Float, stddev: Float) -> Float`.
- Library helpers: `Chance`, `Exponential`, `Units`, `Normals`, `Spike`, `FlipCoin`, `FlipCoins`, `CountHeads`, `CountTails`, `CoinSideToString`, `RollDie`, `RollDice`, `RollWithAdvantage`, `RollWithDisadvantage`.
- `Seeded`, `Fork`, `Child`, `Unit`, `Between`, `IntBetween` and `Normal` are compiler-owned builtins of package Random. Outside that package only the qualified spelling names them, `import Random` is required, and the bare words are free for other packages to declare.
- Arguments are dimensionless. `Random.Between(s, i, 0.0m, 1.0m)` is a type error.
- `Random` is not fallible. A violated precondition (negative index, `lo > hi`, negative `stddev` or `count`, a probability outside `[0, 1]`) stops the program with a runtime error that `match` does not see.
- `Random` is not a cryptographic source.

Results that depend only on integer arithmetic (`Fork`, `Child`, `Unit`, `Between`, `IntBetween`, `Chance`, coins, dice) are identical on every platform. `Normal` and `Exponential` are guaranteed identical only within one CPU architecture.

Full specification: `internal/random/Random.md`.

## Entropy

`Entropy` reads the operating system's random source. It is the only source of
nondeterminism among the standard libraries.

```oct
import Random

fn NoiseForThisRun() -> Random.Stream ! Error {
    let seed = Entropy.Seed()?
    Print(seed)
    return Random.Seeded(seed)
}
```

Rules:

- `Entropy.Seed() -> Int ! Error`, `Entropy.IntBetween(lo: Int, hi: Int) -> Int ! Error` (closed range), `Entropy.Unit() -> Float ! Error` (in `[0, 1)`), `Entropy.Bytes(count: Int) -> Bytes ! Error`.
- `Entropy` is a compiler-owned namespace. It needs no `import`; `import Entropy` is allowed.
- Every call is fallible. The only `Error` is a failed read of the operating system's source. `lo > hi` and `count < 0` are runtime errors, not `Error` values.
- Artifact evaluation and capability discovery reject every `Entropy` call.
- Record the seed. A logged seed replays every draw made from `Random.Seeded(seed)`.

### Compiler-owned namespaces

`Array`, `Artifact` and `Entropy` are namespaces whose functions are builtins.
Calling them needs no `import`. A call to a name the namespace does not have is
reported as `package '<Namespace>' has no function '<Name>'`.
`Json.Load<T>`, `Json.Parse<T>`, `Json.Save` and `Json.Text` are builtins of
the same kind and need no `import`; the rest of `Json` is still a library.

## Json

`Json.Load<T>(path)` reads a file as a `T`, and `Json.Parse<T>(text)` reads
text as a `T`. `Json.Save(path, value)` writes a value to a file, and
`Json.Text(value)` gives its text. The declared type says how a document is
read and how a value is written: nothing is guessed from the document.

```oct
record table Ticket {
    Id:       String
    Assignee: Option<String>
    Points:   Int
}

fn OpenPoints(path: String) -> Int ! Error {
    let tickets = Json.Load<Ticket>(path)?
    var points = 0
    for row in 0..Len(tickets) {
        points = points + tickets[row].Points
    }
    return points
}

record Summary {
    OpenPoints: Int
    Tickets:    Ticket
}

fn Summarize(from: String, to: String) -> Void ! Error {
    let tickets = Json.Load<Ticket>(from)?
    Json.Save(to, Summary { OpenPoints: OpenPoints(from)? Tickets: tickets })?
}
```

Reading:

- `Json.Load<T>(path: String) -> T ! Error` and
  `Json.Parse<T>(text: String) -> T ! Error`. Both are builtins and need no
  `import`. The type argument is required.
- `T` must have a JSON form, in every part. A `T` that does not is a compile
  error that names the part: `Reading.Phase: Complex has no JSON form`.

| Oct type | JSON |
|---|---|
| `Bool` | `true`, `false` |
| `Int`, `Int<D>` | A number with no fraction and no exponent, in the 64-bit range |
| `Float`, `Float<D>` | Any number that is finite as a 64-bit float. The number is in the unit the type declares |
| `String` | A string |
| An enum whose variants carry no payload | A string that names a variant, matched as keys are |
| `Option<T>` | `null` is `None`; anything else is `Some` of a `T`. As a record field or a table cell, an absent member is `None` too |
| `record` | An object |
| `T[]` | An array |
| `Vector<T>` | An array of numbers |
| `Matrix<T>` | An array of arrays of numbers, all one length |
| `record table` | An array of objects; or, when its first column is a `String`, an object whose keys are that column |
| A refined concept | Its base type, admitted by the concept's requirements |

- No JSON form: `Complex`, `Bytes`, `Range`, `UI`, `Error`, function values,
  tuples, flow instances, an enum with a payload other than `Option`, and
  `Option<Option<T>>`.
- There is no conversion between kinds. `"42"` is not an `Int`, `1` is not a
  `Bool`, and `1.0` is not an `Int`.
- A key and a field are one name when they are equal with `_`, `-`, `.`,
  spaces and case ignored: `read_timeout_ms` is `ReadTimeoutMs`. Two fields of
  one record that are one name by that rule are a compile error at the call.
- Every field needs a member, except an `Option`. A member that names no
  field is an error, and so are a key written twice and two members for one
  field.
- A keyed object reads as a table whose first cell is the key. With one other
  column the member's value is that cell (`{"invoice.failed": 5}`); with
  several, or when the value is an object and that column is not itself read
  from one, the value is an object holding the other cells.
- The text is strict JSON (RFC 8259): no comments, no trailing commas, nothing
  after the value. A leading byte order mark is skipped. Nesting deeper than
  512 is an error.
- An `Error` says where: the file, the path into the document, the line and
  the column. The text is the same in the interpreted and the compiled lane.

  ```
  Json.Load: tickets.json: $[1].assignee (line 9, column 17): expected String or null, found a number
  Json.Load: config.json: $.http (line 4, column 11): unknown members "prot", "tls"; Http has Host, Port, ReadTimeoutMs, WriteTimeoutMs
  Json.Parse: (line 1, column 7): expected a value, found '}'
  Json.Load: missing.json: the file does not exist
  ```

- The type is followed into every package it reaches, whether or not the file
  that makes the call imports it.

Writing:

- `Json.Save(path: String, value: T) -> Void ! Error`,
  `Json.Text(value: T) -> String`, and, during `oct artifact` evaluation,
  `Artifact.WriteJson(path: String, value: T)`. They are builtins and need no
  `import`. They take no type argument: `T` is the type of the value, and it
  must have a JSON form as for reading.
- A record is an object whose keys are the field names as declared, in
  declaration order. A `record table` is an array of row objects, and one row
  (`tickets[0]`) is one object. `Option.None` is `null`; its member is
  written. An enum is the name of its variant as declared. A value of a
  refined concept is a value of its base type, and a number with a dimension
  is the number in the unit its type declares.
- The text is UTF-8, indented by two spaces, and ends with one newline. An
  array whose element type is a scalar is on one line (`[1, 2, 3]`); any
  other array has one element to a line. The layout follows the type, not the
  values, so a file's shape does not change with its data.
- A `Float` is written in the shortest form that reads back as the same
  value, always with a fraction or an exponent: `1.0`, `0.1`, `1500000.0`,
  and `1e+21` or `1e-07` beyond 1e21 and below 1e-6.
- A String escapes `"`, `\` and control characters, and nothing else.
- `Json.Parse<T>(Json.Text(value))` is `value`.
- `Json.Save` replaces the file and makes no directories. A file that cannot
  be written is an `Error`:
  `Json.Save: out/levels.json: the directory does not exist`.
- JSON has no NaN and no infinity. A value that holds one stops the program
  with the place of the value; it is not an `Error`, and `Json.Text` is not
  fallible. Nothing is written.

  ```
  runtime error: Json.Text: $.Levels[1]: NaN has no JSON form
  ```

Both:

- Capability discovery rejects `Json.Load` and `Json.Save`, which touch a
  file. Artifact evaluation rejects them too: it reads no file the program
  names, and writes through `Artifact.WriteJson`.
- The first Json library is still present. `Json.Load(path)` with no type
  argument, `Json.Save(path, text)` and `Artifact.WriteJson(path, text)` given
  a String of JSON text, `Json.Object` and the `IO` JSON functions are its,
  and need `import Json` or `import IO`. Until it is removed, a String is
  therefore saved with `Json.Text` and a file writer, not with `Json.Save`.
  See `internal/json/JSON_V2_LADDER.md`, which is also the full
  specification.

Contracts: `Language/Builtins/Json`.

## Document (OctCument M1)

`Libraries/Document` owns backend-neutral immutable document semantics. Its
records/enums model metadata, styles, page layout, structured inline content,
headings, paragraphs, lists, tables, code blocks, callouts, rules, page breaks,
and groups. High-level helpers remain ordinary Oct functions.

`Document.ToMarkdown(doc)` is the deterministic in-language Markdown renderer.
During explicit artifact evaluation, `Artifact.Markdown(path, doc)` and
`Artifact.Docx(path, doc)` materialize the same `Document.Doc`. DOCX packaging,
OOXML names, relationships, IDs, and ZIP details are renderer-private Go
implementation concerns.

Document-authoring source may use the `*.doc.oct` naming convention, but the
suffix has ordinary `.oct` parsing and typechecking semantics. Under the current
artifact contract, `[Artifact]` entry points remain in a sibling `.octest` file
or `Make.oct`; no document-specific parser or hidden execution environment exists.

Refined Concepts enforce bounded heading levels and non-negative/positive
typographic values. Small immutable `With*` style helpers and the ordinary
`ProfessionalStyle` / `ScientificStyle` functions return the existing semantic
style records. `Document.Template<Parameters>` and
`Document.Instantiate<Parameters>` assemble application-owned metadata, style,
layout, and content functions into the same `Document.Doc` across package
boundaries. They do not create another document IR.

## Markdown

`Libraries/Markdown` provides Markdown M1 report-output helpers.

- Markdown M1 is an output helper, **not** a Markdown parser.
- Block helpers return `String[]` lines.
- `Markdown.Title` aliases `Markdown.H1`; `Markdown.Subtitle` aliases `Markdown.H2`.
- `Markdown.H1` / `Markdown.H2` / `Markdown.H3` remain available lower-level heading helpers.
- `Markdown.Report(blocks)` takes a list of blocks (not title+sections positional arguments).
- Canonical report helpers: `Markdown.Report`, `Markdown.Section`, `Markdown.KeyValueTable`, `Markdown.Table`, `Markdown.Callout`.
- Preferred sink for artifact lane output: `Artifact.WriteMarkdown(path, lines)`.
- These string-first helpers remain compatibility APIs. New structured document
  authoring should use `Document`; Markdown is then one renderer rather than the
  canonical semantic model.

Canonical example:

```oct
import Markdown
import Artifact

let lines = Markdown.Report([
    Markdown.Title("Experiment Report"),
    Markdown.Subtitle("Overview"),
    Markdown.Section("Config", [
        Markdown.KeyValueTable(["seed"], ["42"])
    ]),
    Markdown.Section("Results", [
        Markdown.Table(table),
        Markdown.Callout("info", ["All checks passed."])
    ])
])

Artifact.WriteMarkdown("out/report.md", lines)
```
