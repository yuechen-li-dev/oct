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
- `String.From<T>` is compiler-known constrained generic syntax (closed contracts), not user-defined generic support.
- `ToString(...)` remains available for compatibility, but `String.From<T>` is preferred in report/library code.
- Compatibility globals/backing aliases remain available during transition and should not be the preferred authoring style.

Artifact guidance:
- Prefer `Artifact.Write*` in `[Artifact]` functions.
- `IO.*`, `Csv.*`, `Json.*`, and `WriteOctagon` remain available as lower-level fallible APIs.

## Markdown

`Libraries/Markdown` provides Markdown M1 report-output helpers.

- Markdown M1 is an output helper, **not** a Markdown parser.
- Block helpers return `String[]` lines.
- `Markdown.Title` aliases `Markdown.H1`; `Markdown.Subtitle` aliases `Markdown.H2`.
- `Markdown.H1` / `Markdown.H2` / `Markdown.H3` remain available lower-level heading helpers.
- `Markdown.Report(blocks)` takes a list of blocks (not title+sections positional arguments).
- Canonical report helpers: `Markdown.Report`, `Markdown.Section`, `Markdown.KeyValueTable`, `Markdown.Table`, `Markdown.Callout`.
- Preferred sink for artifact lane output: `Artifact.WriteMarkdown(path, lines)`.

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
