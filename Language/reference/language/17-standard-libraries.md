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

## Notes on current documentation boundaries

- The builtin reference intentionally no longer carries the full wrapper catalog; that content is conceptually owned by this page.
- If a library module exists in `Libraries/` but lacks matching detailed reference coverage under `Language/reference`, treat that as a documentation gap to close incrementally.

## String

`Libraries/String/String.Core.oct` provides deterministic report-focused text helpers:

- Preferred M0 namespaced surface (requires `import String`):
  - `String.Join`, `String.Concat`, `String.From<T>`, `String.ReplaceAll`, `String.Contains`, `String.StartsWith`, `String.EndsWith`, `String.Trim`
  - `String.SplitLines`, `String.EscapeJson`, `String.QuoteJson`, `String.ByteLength`, `String.RuneCount`
- Global compatibility/backing builtins remain available during transition (`StringJoin`, `StringQuoteJSON`, etc.).

Artifact guidance:
- Prefer `Artifact.Write*` in `[Artifact]` functions for canonical fail-fast emission without `let _ = ...`.
- `IO.*`, `Csv.*`, `Json.*`, and `WriteOctagon` remain available as lower-level fallible APIs.
- Prefer `String.Join(parts, separator)` for deterministic line construction when preparing report text.

Canonical namespaced authoring pattern:

```oct
import String
let text = String.Join(lines, "\n")

import IO
IO.WriteLines("out/report.md", lines)!

import Csv
Csv.Write("out/metrics.csv", rows)!

import Json
Json.Save("out/metrics.json", summary)!
```

## Markdown

`Libraries/Markdown` provides Markdown M1 report-output helpers.

- Markdown M1 is an output helper, **not** a Markdown parser.
- Block-producing functions return `String[]` lines (not one large `String`).
- Emit generated lines via `Artifact.WriteMarkdown(path, lines)` or `IO.WriteLines(path, lines)!`.
- `Markdown.Table(table)` expects a columnar record-of-string-columns shape.
- `Markdown.TableWithColumns(table, columns)` provides explicit column order control.
- `Markdown.KeyValueTable(keys, values)` provides deterministic scalar settings/metadata output in a two-column table.
- Prefer `Markdown.Title(text)` and `Markdown.Subtitle(text)` as semantic report-heading helpers.
- `Markdown.Title(text)` is an alias for `Markdown.H1(text)`, and `Markdown.Subtitle(text)` is an alias for `Markdown.H2(text)`.
- `Markdown.H1` / `Markdown.H2` / `Markdown.H3` remain available as lower-level Markdown heading helpers.
- `Markdown.Callout(kind, lines)` emits deterministic blockquote callouts for `note`, `info`, `warning`, `danger`, and `success`.
- `Markdown.Image(path, altText)` emits a single image line and performs minimal alt-text safety normalization.
- `Markdown.Figure(path, caption)` emits image + caption report lines.
- `Markdown.Section(title, blocks)` and `Markdown.Subsection(title, blocks)` emit H2/H3 headings plus `Markdown.Report`-style block flattening.
- `Markdown.CodeBlock(language, lines)` uses dynamic backtick fences (`max(3, longestRun + 1)`) to avoid premature closure.
- Markdown M1 does not claim CommonMark-complete compliance and does not implement parser/renderer/PDF/UI behavior.
- Future direction (not implemented): an Oct-owned `.md` dialect may parse structured block tags like `<Code>`, `<Figure>`, and `<Callout>`.
